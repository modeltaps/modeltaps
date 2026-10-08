package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/cache"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/gorm"
)

// 成员级守护(T5,规格 §3.5):周期预算 + 模型白名单,存于 org_members.setting JSON。
// 预算复用令牌周期语义(QuotaResetSetting + UTC 边界懒重置);白名单空=不限制,
// 成员未配置白名单时回退组织级缺省(organizations.setting.model_whitelist)。

// OrgMemberGuardrailCacheKey 缓存键:影子账户ID + 成员用户ID(relay 链路可直接取到这两个值)
const OrgMemberGuardrailCacheKey = "org_member_guardrail:%d:%d"

var orgMemberGuardrailCacheTTL = time.Minute

// OrgMemberSetting org_members.setting JSON 结构。
// 周期计数 BudgetUsed/BudgetStart 已移出为 OrganizationMember 专用列(SEC-4),不再存于此 JSON。
type OrgMemberSetting struct {
	Budget         *QuotaResetSetting `json:"budget,omitempty"`          // 周期预算;nil 或 limit<=0 表示不限
	ModelWhitelist []string           `json:"model_whitelist,omitempty"` // 空=回退组织级缺省
}

// OrgMemberGuardrail relay 执行点使用的成员限制视图(已合并组织级白名单缺省)
type OrgMemberGuardrail struct {
	OrganizationId int
	MemberId       int // org_members.id,预算累计的批量更新 key
	MemberUserId   int
	Budget         *QuotaResetSetting
	BudgetUsed     int
	BudgetStart    int64
	ModelWhitelist []string
	// OrgBudget 组织级周期预算配置(T1)。此处只带配置不带计数:组织预算的判定完全由
	// ReserveOrganizationBudget 的原子预留承担,避免每次累计都要失效全体成员的守护缓存
	OrgBudget *QuotaResetSetting
}

func (g *OrgMemberGuardrail) HasBudget() bool {
	return g != nil && g.Budget != nil && g.Budget.Limit > 0
}

// HasOrgBudget 组织是否配置了团队级周期预算(relay 据此决定是否逐请求走组织预算硬闸)
func (g *OrgMemberGuardrail) HasOrgBudget() bool {
	return g != nil && g.OrgBudget != nil && g.OrgBudget.Limit > 0
}

// BudgetExceeded 判断成员周期预算是否已用尽。
// BudgetStart 早于当前周期起点视为已跨期(等效 budget_used=0),边界语义同 tokenPeriodQuotaExceeded。
func (g *OrgMemberGuardrail) BudgetExceeded(now time.Time) bool {
	if !g.HasBudget() {
		return false
	}
	ps := currentPeriodStart(g.Budget.Period, now)
	if ps.IsZero() {
		return false
	}
	return g.BudgetStart >= ps.Unix() && g.BudgetUsed >= g.Budget.Limit
}

// ModelAllowed 白名单匹配:空=不限制;不区分大小写;支持 "*" 后缀做前缀匹配(如 gpt-4*)
func (g *OrgMemberGuardrail) ModelAllowed(modelName string) bool {
	return orgModelWhitelistAllowed(g.ModelWhitelist, modelName)
}

func orgModelWhitelistAllowed(whitelist []string, modelName string) bool {
	if len(whitelist) == 0 {
		return true
	}
	name := strings.ToLower(strings.TrimSpace(modelName))
	for _, item := range whitelist {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		if strings.HasSuffix(item, "*") {
			if strings.HasPrefix(name, strings.TrimSuffix(item, "*")) {
				return true
			}
		} else if item == name {
			return true
		}
	}
	return false
}

func buildOrgMemberGuardrail(org *Organization, member *OrganizationMember) *OrgMemberGuardrail {
	setting := member.Setting.Data()
	orgSetting := org.Setting.Data()
	whitelist := setting.ModelWhitelist
	if len(whitelist) == 0 {
		whitelist = orgSetting.ModelWhitelist
	}
	return &OrgMemberGuardrail{
		OrganizationId: org.Id,
		MemberId:       member.Id,
		MemberUserId:   member.UserId,
		Budget:         setting.Budget,
		BudgetUsed:     member.BudgetUsed,
		BudgetStart:    member.BudgetStart,
		ModelWhitelist: whitelist,
		OrgBudget:      orgSetting.Budget,
	}
}

// invalidateOrgGuardrailCacheAllMembers 组织级配置(白名单缺省/团队预算)变更后失效全体成员的守护缓存
func invalidateOrgGuardrailCacheAllMembers(org *Organization) {
	if !config.RedisEnabled || org == nil || org.Id == 0 {
		return
	}
	var userIds []int
	if err := DB.Model(&OrganizationMember{}).Where("organization_id = ?", org.Id).Pluck("user_id", &userIds).Error; err != nil {
		logger.SysError(fmt.Sprintf("failed to list org members for guardrail cache invalidation (org=%d): %s", org.Id, err.Error()))
		return
	}
	for _, userId := range userIds {
		invalidateOrgMemberGuardrailCache(org.ShadowUserId, userId)
	}
}

func getOrgMemberGuardrailFromDB(shadowUserId int, memberUserId int) (*OrgMemberGuardrail, error) {
	var org Organization
	if err := DB.Select("id", "setting").Where("shadow_user_id = ?", shadowUserId).First(&org).Error; err != nil {
		return nil, err
	}
	member, err := GetOrganizationMember(org.Id, memberUserId)
	if err != nil {
		return nil, err
	}
	return buildOrgMemberGuardrail(&org, member), nil
}

// CacheGetOrgMemberGuardrail relay 热路径入口;预算累计/配置更新会主动失效缓存
func CacheGetOrgMemberGuardrail(shadowUserId int, memberUserId int) (*OrgMemberGuardrail, error) {
	if !config.RedisEnabled {
		return getOrgMemberGuardrailFromDB(shadowUserId, memberUserId)
	}
	return cache.GetOrSetCache(
		fmt.Sprintf(OrgMemberGuardrailCacheKey, shadowUserId, memberUserId),
		orgMemberGuardrailCacheTTL,
		func() (*OrgMemberGuardrail, error) {
			return getOrgMemberGuardrailFromDB(shadowUserId, memberUserId)
		},
		cache.CacheTimeout)
}

func invalidateOrgMemberGuardrailCache(shadowUserId int, memberUserId int) {
	if !config.RedisEnabled || shadowUserId == 0 {
		return
	}
	if err := cache.DeleteCache(fmt.Sprintf(OrgMemberGuardrailCacheKey, shadowUserId, memberUserId)); err != nil {
		logger.SysError("failed to invalidate org member guardrail cache: " + err.Error())
	}
}

// invalidateOrgMemberGuardrailCacheByOrg 累计/cron 路径只有组织ID,需反查影子账户ID再失效缓存
func invalidateOrgMemberGuardrailCacheByOrg(organizationId int, memberUserId int) {
	if !config.RedisEnabled {
		return
	}
	var org Organization
	if err := DB.Select("shadow_user_id").Where("id = ?", organizationId).First(&org).Error; err != nil {
		logger.SysError(fmt.Sprintf("failed to resolve shadow user for guardrail cache invalidation (org=%d): %s", organizationId, err.Error()))
		return
	}
	invalidateOrgMemberGuardrailCache(org.ShadowUserId, memberUserId)
}

// UpdateOrgMemberLimits 更新成员预算与模型白名单(Owner/Admin 调用)。
// BudgetUsed/BudgetStart 由服务端维护:预算周期变化时清零重新计数(与令牌周期计数同语义)。
func UpdateOrgMemberLimits(org *Organization, targetUserId int, budget *QuotaResetSetting, whitelist []string) (*OrganizationMember, error) {
	if budget != nil {
		if budget.Limit <= 0 {
			return nil, errors.New("budget must be greater than 0")
		}
		switch budget.Period {
		case TokenQuotaResetPeriodDaily, TokenQuotaResetPeriodWeekly, TokenQuotaResetPeriodMonthly:
		default:
			return nil, errors.New("budget period must be daily, weekly or monthly")
		}
	}
	member, err := GetOrganizationMember(org.Id, targetUserId)
	if err != nil {
		return nil, errors.New("this user is not a member of the organization")
	}
	setting := member.Setting.Data()
	oldPeriod, newPeriod := "", ""
	if setting.Budget != nil {
		oldPeriod = setting.Budget.Period
	}
	if budget != nil {
		newPeriod = budget.Period
	}
	setting.Budget = budget
	setting.ModelWhitelist = whitelist
	member.Setting.Set(setting)
	member.UpdatedTime = utils.GetTimestamp()
	selectCols := []string{"setting", "updated_time"}
	// 周期变化时清零重新计数(与令牌周期计数同语义);预算计数现为专用列
	if oldPeriod != newPeriod {
		member.BudgetUsed = 0
		member.BudgetStart = 0
		selectCols = append(selectCols, "budget_used", "budget_start")
	}
	if err := DB.Model(member).Select(selectCols).Updates(member).Error; err != nil {
		return nil, err
	}
	invalidateOrgMemberGuardrailCache(org.ShadowUserId, targetUserId)
	return member, nil
}

// AccrueOrgMemberBudgetUsed 累计成员周期预算用量(delta 可为负,用于回退预扣)。
// 仅在成员配置了 budget 时由 relay 计费路径触发;id 为 org_members.id。
func AccrueOrgMemberBudgetUsed(memberId int, delta int) error {
	if delta == 0 {
		return nil
	}
	if config.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeOrgMemberBudgetUsed, memberId, delta)
		return nil
	}
	return accrueOrgMemberBudgetUsed(memberId, delta)
}

func accrueOrgMemberBudgetUsed(memberId int, delta int) error {
	// 仅读取周期配置以算当前周期起点(配置极少变更,非竞态值);budget_used 由下方单条原子语句变更
	var member OrganizationMember
	if err := DB.Select("id", "organization_id", "user_id", "setting").First(&member, "id = ?", memberId).Error; err != nil {
		return err
	}
	budget := member.Setting.Data().Budget
	if budget == nil {
		return nil
	}
	// 未知周期(理论不可达,UpdateOrgMemberLimits 已校验)用 -1 使懒重置分支恒不触发,仅做自增
	psUnix := int64(-1)
	if ps := currentPeriodStart(budget.Period, time.Now()); !ps.IsZero() {
		psUnix = ps.Unix()
	}
	// 单条原子 UPDATE:懒重置(跨期清零并推进起点)+ 自增 + 钳制≥0。同一行的并发 UPDATE 由 DB
	// 行级串行执行,每条语句基于当前已提交值求值,故无读-改-写丢更新(对照 token 额度 gorm.Expr)。
	res := DB.Model(&OrganizationMember{}).Where("id = ?", memberId).UpdateColumns(map[string]interface{}{
		"budget_used": gorm.Expr(
			"CASE WHEN budget_start < ? THEN (CASE WHEN ? > 0 THEN ? ELSE 0 END) "+
				"ELSE (CASE WHEN budget_used + ? < 0 THEN 0 ELSE budget_used + ? END) END",
			psUnix, delta, delta, delta, delta),
		"budget_start": gorm.Expr("CASE WHEN budget_start < ? THEN ? ELSE budget_start END", psUnix, psUnix),
	})
	// budget_used 参与 relay 校验,写入后必须失效缓存(同 token period_used 模式)
	if res.Error == nil {
		invalidateOrgMemberGuardrailCacheByOrg(member.OrganizationId, member.UserId)
	}
	return res.Error
}

// ReserveOrgMemberBudget 原子预留成员周期预算(SEC-9 硬闸):在既有「懒重置 + 自增 + 钳制」原子
// UPDATE 之上再加一条周期上限判据 —— 仅当自增后不超过 limit(在期内)或跨期后 amount≤limit 时才写入,
// 命中返回 (true,nil),未命中(会超限)返回 (false,nil)。并发预留由 DB 行级串行执行,合计不超过周期上限
// (同 PreDecreaseUserQuota 的 WHERE 守卫 + RowsAffected 范式,SEC-1)。仅供 relay 入场闸使用;结算/回退
// 差额仍走 AccrueOrgMemberBudgetUsed(可负、可批)。amount≤0 或未配置预算直接放行(冗余保护,门控在上游)。
//
// 与 AccrueOrgMemberBudgetUsed 的区别:后者是「计数器」(永不拒绝,可超过 limit),本函数是「闸门」
// (自增前判上限,超限拒绝)。故本函数必须同步执行,绝不走 BatchUpdate 延迟(闸门不能延后)。
func ReserveOrgMemberBudget(memberId int, amount int) (bool, error) {
	if amount <= 0 {
		return true, nil
	}
	var member OrganizationMember
	if err := DB.Select("id", "organization_id", "user_id", "setting").First(&member, "id = ?", memberId).Error; err != nil {
		return false, err
	}
	budget := member.Setting.Data().Budget
	if budget == nil || budget.Limit <= 0 {
		return true, nil
	}
	// 未知周期(理论不可达)用 -1 使懒重置分支恒不触发,退化为「在期内自增判上限」
	psUnix := int64(-1)
	if ps := currentPeriodStart(budget.Period, time.Now()); !ps.IsZero() {
		psUnix = ps.Unix()
	}
	limit := budget.Limit
	// 单条条件 UPDATE:跨期(budget_start<ps)按 amount≤limit 判定并清零重计,否则按 budget_used+amount≤limit
	// 判定并自增;WHERE 基于当前已提交行值求值,行级串行保证并发预留合计不超上限。RowsAffected==0 即超限拒绝。
	res := DB.Model(&OrganizationMember{}).
		Where("id = ? AND ((budget_start < ? AND ? <= ?) OR (budget_start >= ? AND budget_used + ? <= ?))",
			memberId, psUnix, amount, limit, psUnix, amount, limit).
		UpdateColumns(map[string]interface{}{
			"budget_used":  gorm.Expr("CASE WHEN budget_start < ? THEN ? ELSE budget_used + ? END", psUnix, amount, amount),
			"budget_start": gorm.Expr("CASE WHEN budget_start < ? THEN ? ELSE budget_start END", psUnix, psUnix),
		})
	if res.Error != nil {
		return false, res.Error
	}
	if res.RowsAffected == 0 {
		return false, nil // 会超过周期上限,拒绝(调用方应零费用拒绝请求)
	}
	// budget_used 参与 relay 校验,写入后失效缓存(同 accrue 路径)
	invalidateOrgMemberGuardrailCacheByOrg(member.OrganizationId, member.UserId)
	return true, nil
}

func batchAccrueOrgMemberBudgetUsed(store map[int]int) {
	for id, delta := range store {
		if err := accrueOrgMemberBudgetUsed(id, delta); err != nil {
			logger.SysError(fmt.Sprintf("batch accrue org member budget used failed (id=%d): %s", id, err.Error()))
		}
	}
}

// ResetOrgMemberBudget 批量把已跨期成员的 budget_used 落库清零(幂等)。
// 精确的 UTC 0 点边界由校验/累计路径的懒重置保证,cron 只负责持久化清扫(同 ResetTokenPeriodQuota)。
func ResetOrgMemberBudget() {
	now := time.Now()
	var members []*OrganizationMember
	result := DB.Where("setting IS NOT NULL").FindInBatches(&members, 500, func(tx *gorm.DB, batch int) error {
		for _, member := range members {
			budget := member.Setting.Data().Budget
			if budget == nil {
				continue
			}
			ps := currentPeriodStart(budget.Period, now)
			if ps.IsZero() || member.BudgetStart >= ps.Unix() {
				continue
			}
			// 跨期清零并推进起点(专用列);budget_start < ? 守卫保证幂等
			if err := tx.Model(&OrganizationMember{}).Where("id = ? AND budget_start < ?", member.Id, ps.Unix()).
				UpdateColumns(map[string]interface{}{"budget_used": 0, "budget_start": ps.Unix()}).Error; err != nil {
				logger.SysError(fmt.Sprintf("reset org member budget failed (id=%d): %s", member.Id, err.Error()))
				continue
			}
			invalidateOrgMemberGuardrailCacheByOrg(member.OrganizationId, member.UserId)
		}
		return nil
	})
	if result.Error != nil {
		logger.SysError("reset org member budget scan failed: " + result.Error.Error())
	}
}
