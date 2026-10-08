package relay_util

// 组织令牌成员级守护/预算域:入场白名单校验、预算原子预留(硬闸)与结算/回退时的预算累计。
// 由 quota.go 的 PreQuotaConsumption / completedQuotaConsumption / Undo 调用。

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"
)

// SEC-10 测试注入点:授权类守护依赖的 model 层入口,单测可替换以模拟 DB/Redis 故障(fail-closed 路径)
var (
	cacheGetOrgMemberGuardrailFn = model.CacheGetOrgMemberGuardrail
	reserveOrgMemberBudgetFn     = model.ReserveOrgMemberBudget
	reserveOrganizationBudgetFn  = model.ReserveOrganizationBudget
)

// SEC-12 测试注入点:成员预算实时 tally 依赖的 model 层入口,单测可替换以模拟 Redis 行为
var (
	cacheIncreaseOrgMemberRealtimeBudgetFn = model.CacheIncreaseOrgMemberRealtimeBudget
	cacheDecreaseOrgMemberRealtimeBudgetFn = model.CacheDecreaseOrgMemberRealtimeBudget
)

// reserveOrgMemberBudget 对配置了预算的组织成员做入场原子预留(SEC-9 硬闸):并发预留经 DB 行级串行,
// 合计不超过周期上限,超限返回零费用拒绝错误。预留 DB 失败时 fail-closed(SEC-10):返回 503 可重试错误
// 拒绝请求,不再退化为普通累计放行,堵住 DB 抖动窗口绕过预算硬闸的通路(与 checkOrgMemberGuardrails 一致)。
// 非组织令牌/未配置预算为 noop。
func (q *Quota) reserveOrgMemberBudget(amount int) *types.OpenAIErrorWithStatusCode {
	if !q.orgGuardrail.HasBudget() || amount <= 0 {
		return nil
	}
	ok, err := reserveOrgMemberBudgetFn(q.orgGuardrail.MemberId, amount)
	if err != nil {
		logger.SysError("failed to reserve org member budget: " + err.Error())
		return common.ErrorWrapperLocal(
			errors.New("organization member guardrail is temporarily unavailable, please retry"),
			"org_guardrail_unavailable", http.StatusServiceUnavailable)
	}
	if !ok {
		return common.ErrorWrapperLocal(
			errors.New("organization member budget exceeded for current period"),
			"org_member_budget_exceeded", http.StatusPaymentRequired)
	}
	return nil
}

// reserveOrganizationBudget 对配置了团队级预算的组织做入场原子预留(T1 硬闸):跨全体成员的并发预留
// 经 DB 行级串行,合计不超过周期上限,超限返回零费用拒绝错误(org_budget_exceeded,与成员预算超限区分)。
// 预留 DB 失败时 fail-closed(同 SEC-10):返回 503 可重试错误拒绝请求。
// 非组织令牌/未配置团队预算为 noop。
func (q *Quota) reserveOrganizationBudget(amount int) *types.OpenAIErrorWithStatusCode {
	if !q.orgGuardrail.HasOrgBudget() || amount <= 0 {
		return nil
	}
	ok, err := reserveOrganizationBudgetFn(q.orgGuardrail.OrganizationId, amount)
	if err != nil {
		logger.SysError("failed to reserve organization budget: " + err.Error())
		return common.ErrorWrapperLocal(
			errors.New("organization guardrail is temporarily unavailable, please retry"),
			"org_guardrail_unavailable", http.StatusServiceUnavailable)
	}
	if !ok {
		return common.ErrorWrapperLocal(
			errors.New("organization budget exceeded for current period"),
			"org_budget_exceeded", http.StatusPaymentRequired)
	}
	return nil
}

// accrueOrganizationBudget 累计团队预算用量(delta 可为负);未配置团队预算或非组织令牌时为 noop
func (q *Quota) accrueOrganizationBudget(delta int) {
	if !q.orgGuardrail.HasOrgBudget() || delta == 0 {
		return
	}
	if err := model.AccrueOrganizationBudgetUsed(q.orgGuardrail.OrganizationId, delta); err != nil {
		logger.SysError("failed to accrue organization budget used: " + err.Error())
	}
}

// isOrgToken 组织令牌判定:userId 为影子账户,createdBy 为实际成员(同 GetLogMeta 散点③)
func (q *Quota) isOrgToken() bool {
	return q.createdBy > 0 && q.createdBy != q.userId
}

// checkOrgMemberGuardrails 组织令牌执行成员级限制(规格 §3.5):模型白名单 + 周期预算。
// 须在任何预扣发生前调用,拒绝时零费用;守护数据加载失败 fail-closed(SEC-10):返回 503 可重试错误
// 拒绝请求,不再放行,堵住缓存/DB 抖动窗口绕过白名单与预算的通路(授权类校验失败时放行等于无守护)。
func (q *Quota) checkOrgMemberGuardrails() *types.OpenAIErrorWithStatusCode {
	if !q.isOrgToken() {
		return nil
	}
	guardrail, err := cacheGetOrgMemberGuardrailFn(q.userId, q.createdBy)
	if err != nil {
		logger.SysError(fmt.Sprintf("failed to load org member guardrail (shadow=%d, member=%d): %s", q.userId, q.createdBy, err.Error()))
		return common.ErrorWrapperLocal(
			errors.New("organization member guardrail is temporarily unavailable, please retry"),
			"org_guardrail_unavailable", http.StatusServiceUnavailable)
	}
	if !guardrail.ModelAllowed(q.modelName) {
		return common.ErrorWrapperLocal(
			fmt.Errorf("model %s is not allowed by the organization member model whitelist", q.modelName),
			"org_model_not_allowed", http.StatusForbidden)
	}
	if guardrail.BudgetExceeded(time.Now()) {
		return common.ErrorWrapperLocal(
			errors.New("organization member budget exceeded for current period"),
			"org_member_budget_exceeded", http.StatusPaymentRequired)
	}
	q.orgGuardrail = guardrail
	return nil
}

// accrueOrgMemberBudget 累计成员预算用量(delta 可为负);未配置预算或非组织令牌时为 noop
func (q *Quota) accrueOrgMemberBudget(delta int) {
	if !q.orgGuardrail.HasBudget() || delta == 0 {
		return
	}
	if err := model.AccrueOrgMemberBudgetUsed(q.orgGuardrail.MemberId, delta); err != nil {
		logger.SysError("failed to accrue org member budget used: " + err.Error())
	}
}

// accrueOrgMemberRealtimeBudget 成员预算流式实时兜底(SEC-12):镜像 token 额度实时路径,把本次
// 增量累计入按 memberId 维度的 Redis tally(跨并发请求共享,带 TTL),当「入场 BudgetUsed + tally ≥
// 周期上限」时返回错误中断流(文案含 org_member_budget_exceeded,与 token 额度不足区分)。
// Redis 操作出错沿用 UpdateUserRealtimeQuota 既有语义返回错误,不引入新的 fail-open。
// 未配置预算/个人令牌为 noop;Redis 关闭时调用方已提前返回,退化为 SEC-9 入场预留硬闸。
func (q *Quota) accrueOrgMemberRealtimeBudget(increaseQuota int) error {
	if !q.orgGuardrail.HasBudget() || increaseQuota <= 0 {
		return nil
	}
	tally, err := cacheIncreaseOrgMemberRealtimeBudgetFn(q.orgGuardrail.MemberId, increaseQuota)
	if err != nil {
		return errors.New("error update org member realtime budget cache: " + err.Error())
	}
	// 越限增量已写入 Redis,同样计入回收基数,保证结算清理完整
	q.memberBudgetCacheQuota += increaseQuota
	if int64(q.orgGuardrail.BudgetUsed)+tally >= int64(q.orgGuardrail.Budget.Limit) {
		return errors.New("org_member_budget_exceeded: organization member budget exceeded for current period")
	}
	return nil
}

// releaseOrgMemberRealtimeBudget 结算时回收本请求累计的成员预算实时 tally(镜像 cacheQuota 回收
// 模式,由 completedQuotaConsumption 的 defer 调用);回收失败仅记日志,键有 TTL 兜底不泄漏。
func (q *Quota) releaseOrgMemberRealtimeBudget() {
	if q.memberBudgetCacheQuota <= 0 || q.orgGuardrail == nil {
		return
	}
	if _, err := cacheDecreaseOrgMemberRealtimeBudgetFn(q.orgGuardrail.MemberId, q.memberBudgetCacheQuota); err != nil {
		logger.SysError("failed to release org member realtime budget tally: " + err.Error())
	}
	q.memberBudgetCacheQuota = 0
}
