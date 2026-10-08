package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/gorm"
)

// 团队级周期预算(T1):跨全体成员合计的日/周/月上限,配置存于 organizations.setting.budget,
// 计数为 organizations 专用列 budget_used/budget_start(沿用 SEC-4 成员预算的「专用列 +
// 单条原子 UPDATE」模式)。语义与成员预算完全对称:入场硬闸走 ReserveOrganizationBudget
// (自增前判上限、超限拒绝),结算/回退差额走 AccrueOrganizationBudgetUsed(计数器,可负、可批)。

// ValidateOrgBudget 校验周期预算配置(nil=不限);枚举与成员预算一致
func ValidateOrgBudget(budget *QuotaResetSetting) error {
	if budget == nil {
		return nil
	}
	if budget.Limit <= 0 {
		return errors.New("budget must be greater than 0")
	}
	switch budget.Period {
	case TokenQuotaResetPeriodDaily, TokenQuotaResetPeriodWeekly, TokenQuotaResetPeriodMonthly:
	default:
		return errors.New("budget period must be daily, weekly or monthly")
	}
	return nil
}

// UpdateOrganizationBudget 更新组织级周期预算(Admin+)。budget 为 nil 表示清除限制;
// 周期变化时计数清零重新计数(与成员预算/令牌周期计数同语义)。
func UpdateOrganizationBudget(org *Organization, budget *QuotaResetSetting) error {
	if err := ValidateOrgBudget(budget); err != nil {
		return err
	}
	setting := org.Setting.Data()
	oldPeriod, newPeriod := "", ""
	if setting.Budget != nil {
		oldPeriod = setting.Budget.Period
	}
	if budget != nil {
		newPeriod = budget.Period
	}
	setting.Budget = budget
	org.Setting.Set(setting)
	org.UpdatedTime = utils.GetTimestamp()
	selectCols := []string{"setting", "updated_time"}
	if oldPeriod != newPeriod {
		org.BudgetUsed = 0
		org.BudgetStart = 0
		selectCols = append(selectCols, "budget_used", "budget_start")
	}
	if err := DB.Model(org).Select(selectCols).Updates(org).Error; err != nil {
		return err
	}
	// 预算配置进入成员守护视图(OrgMemberGuardrail.OrgBudget),变更后须失效全体成员缓存
	invalidateOrgGuardrailCacheAllMembers(org)
	return nil
}

// OrganizationBudgetUsedInPeriod 返回组织本周期已用额度(展示用):未配置预算或已跨期视为 0,
// 与 BudgetExceeded / 成员列表的懒重置折算语义一致
func OrganizationBudgetUsedInPeriod(org *Organization, now time.Time) int {
	if org == nil {
		return 0
	}
	budget := org.Setting.Data().Budget
	if budget == nil || budget.Limit <= 0 {
		return 0
	}
	ps := currentPeriodStart(budget.Period, now)
	if ps.IsZero() || org.BudgetStart < ps.Unix() {
		return 0
	}
	return org.BudgetUsed
}

// orgBudgetPeriodStart 返回组织当前周期起点的 unix 时间(未配置预算返回 nil);
// 未知周期用 -1 使懒重置分支恒不触发(理论不可达,ValidateOrgBudget 已校验)
func orgBudgetPeriodStart(orgId int) (*QuotaResetSetting, int64, error) {
	var org Organization
	if err := DB.Select("id", "setting").First(&org, "id = ?", orgId).Error; err != nil {
		return nil, 0, err
	}
	budget := org.Setting.Data().Budget
	if budget == nil || budget.Limit <= 0 {
		return nil, 0, nil
	}
	psUnix := int64(-1)
	if ps := currentPeriodStart(budget.Period, time.Now()); !ps.IsZero() {
		psUnix = ps.Unix()
	}
	return budget, psUnix, nil
}

// ReserveOrganizationBudget 原子预留组织周期预算(团队硬闸,复刻 ReserveOrgMemberBudget):
// 单条条件 UPDATE —— 跨期(budget_start<ps)按 amount≤limit 判定并清零重计,否则按
// budget_used+amount≤limit 判定并自增。WHERE 基于当前已提交行值求值,行级串行保证跨全体成员的
// 并发预留合计不超上限;RowsAffected==0 即超限拒绝。必须同步执行,绝不走 BatchUpdate 延迟(闸门不能延后)。
func ReserveOrganizationBudget(orgId int, amount int) (bool, error) {
	if amount <= 0 {
		return true, nil
	}
	budget, psUnix, err := orgBudgetPeriodStart(orgId)
	if err != nil {
		return false, err
	}
	if budget == nil {
		return true, nil
	}
	limit := budget.Limit
	res := DB.Model(&Organization{}).
		Where("id = ? AND ((budget_start < ? AND ? <= ?) OR (budget_start >= ? AND budget_used + ? <= ?))",
			orgId, psUnix, amount, limit, psUnix, amount, limit).
		UpdateColumns(map[string]interface{}{
			"budget_used":  gorm.Expr("CASE WHEN budget_start < ? THEN ? ELSE budget_used + ? END", psUnix, amount, amount),
			"budget_start": gorm.Expr("CASE WHEN budget_start < ? THEN ? ELSE budget_start END", psUnix, psUnix),
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected != 0, nil
}

// AccrueOrganizationBudgetUsed 累计组织周期预算用量(delta 可为负,用于回退预扣与结算差额)
func AccrueOrganizationBudgetUsed(orgId int, delta int) error {
	if delta == 0 {
		return nil
	}
	if config.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeOrganizationBudgetUsed, orgId, delta)
		return nil
	}
	return accrueOrganizationBudgetUsed(orgId, delta)
}

func accrueOrganizationBudgetUsed(orgId int, delta int) error {
	budget, psUnix, err := orgBudgetPeriodStart(orgId)
	if err != nil {
		return err
	}
	if budget == nil {
		return nil
	}
	// 单条原子 UPDATE:懒重置(跨期清零并推进起点)+ 自增 + 钳制≥0(同 accrueOrgMemberBudgetUsed)
	return DB.Model(&Organization{}).Where("id = ?", orgId).UpdateColumns(map[string]interface{}{
		"budget_used": gorm.Expr(
			"CASE WHEN budget_start < ? THEN (CASE WHEN ? > 0 THEN ? ELSE 0 END) "+
				"ELSE (CASE WHEN budget_used + ? < 0 THEN 0 ELSE budget_used + ? END) END",
			psUnix, delta, delta, delta, delta),
		"budget_start": gorm.Expr("CASE WHEN budget_start < ? THEN ? ELSE budget_start END", psUnix, psUnix),
	}).Error
}

func batchAccrueOrganizationBudgetUsed(store map[int]int) {
	for id, delta := range store {
		if err := accrueOrganizationBudgetUsed(id, delta); err != nil {
			logger.SysError(fmt.Sprintf("batch accrue organization budget used failed (id=%d): %s", id, err.Error()))
		}
	}
}
