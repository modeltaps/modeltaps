package relay_util

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/model"
)

// TestCheckOrgMemberGuardrailsFailClosed 守护 SEC-10:守护数据加载失败必须拒绝请求(503 可重试),
// 不得放行(修复前 fail-open,DB/Redis 抖动窗口可绕过模型白名单与预算)。个人令牌行为零变化。
func TestCheckOrgMemberGuardrailsFailClosed(t *testing.T) {
	orig := cacheGetOrgMemberGuardrailFn
	defer func() { cacheGetOrgMemberGuardrailFn = orig }()

	called := false
	cacheGetOrgMemberGuardrailFn = func(shadowUserId int, memberUserId int) (*model.OrgMemberGuardrail, error) {
		called = true
		return nil, errors.New("db down")
	}

	// 个人令牌(createdBy=0):不触达守护加载,直接放行
	q := &Quota{userId: 1, createdBy: 0}
	if errResp := q.checkOrgMemberGuardrails(); errResp != nil {
		t.Fatalf("个人令牌应放行: %+v", errResp)
	}
	if called {
		t.Fatal("个人令牌不应加载守护数据")
	}

	// 组织令牌 + 加载失败 → 503 可重试拒绝(fail-closed)
	q = &Quota{userId: 100, createdBy: 5}
	errResp := q.checkOrgMemberGuardrails()
	if errResp == nil {
		t.Fatal("守护数据加载失败必须拒绝请求(fail-closed),不得放行")
	}
	if errResp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("StatusCode = %d, want 503", errResp.StatusCode)
	}
	if errResp.OpenAIError.Code != "org_guardrail_unavailable" {
		t.Fatalf("Code = %v, want org_guardrail_unavailable", errResp.OpenAIError.Code)
	}
	if !errResp.LocalError {
		t.Fatal("应为 LocalError(本站故障,不计渠道错误)")
	}
}

// TestReserveOrgMemberBudgetFailClosed 守护 SEC-10:预算预留 DB 失败必须拒绝请求(503 可重试),
// 不得退化为普通累计放行;超限拒绝(402)与成功放行语义不变,未配置预算仍为 noop。
func TestReserveOrgMemberBudgetFailClosed(t *testing.T) {
	orig := reserveOrgMemberBudgetFn
	defer func() { reserveOrgMemberBudgetFn = orig }()

	guardrail := &model.OrgMemberGuardrail{
		MemberId: 7,
		Budget:   &model.QuotaResetSetting{Period: "daily", Limit: 100},
	}

	// 预留 DB 失败 → 503 拒绝,不再退化累计放行
	reserveOrgMemberBudgetFn = func(memberId int, amount int) (bool, error) {
		return false, errors.New("db down")
	}
	q := &Quota{orgGuardrail: guardrail}
	errResp := q.reserveOrgMemberBudget(10)
	if errResp == nil {
		t.Fatal("预留 DB 失败必须拒绝请求(fail-closed),不得放行")
	}
	if errResp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("StatusCode = %d, want 503", errResp.StatusCode)
	}
	if errResp.OpenAIError.Code != "org_guardrail_unavailable" {
		t.Fatalf("Code = %v, want org_guardrail_unavailable", errResp.OpenAIError.Code)
	}
	if !errResp.LocalError {
		t.Fatal("应为 LocalError(本站故障,不计渠道错误)")
	}

	// 超限拒绝 → 402 语义不变
	reserveOrgMemberBudgetFn = func(memberId int, amount int) (bool, error) {
		return false, nil
	}
	errResp = q.reserveOrgMemberBudget(10)
	if errResp == nil || errResp.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("超限应返回 402: %+v", errResp)
	}

	// 预留成功 → 放行
	reserveOrgMemberBudgetFn = func(memberId int, amount int) (bool, error) {
		return true, nil
	}
	if errResp := q.reserveOrgMemberBudget(10); errResp != nil {
		t.Fatalf("预留成功应放行: %+v", errResp)
	}

	// 未配置预算 → noop,不触达 DB
	called := false
	reserveOrgMemberBudgetFn = func(memberId int, amount int) (bool, error) {
		called = true
		return false, errors.New("must not be called")
	}
	q = &Quota{}
	if errResp := q.reserveOrgMemberBudget(10); errResp != nil {
		t.Fatalf("未配置预算应放行: %+v", errResp)
	}
	if called {
		t.Fatal("未配置预算不应触达预留 DB")
	}
}

// TestAccrueOrgMemberRealtimeBudget 守护 SEC-12:成员预算流式实时兜底 —— 未越限放行并累计回收基数、
// 「入场 BudgetUsed + tally ≥ 周期上限」越限中断(文案区分 org_member_budget_exceeded)、
// Redis 出错沿用既有语义返回错误、无预算 noop。
func TestAccrueOrgMemberRealtimeBudget(t *testing.T) {
	orig := cacheIncreaseOrgMemberRealtimeBudgetFn
	defer func() { cacheIncreaseOrgMemberRealtimeBudgetFn = orig }()

	guardrail := &model.OrgMemberGuardrail{
		MemberId:   7,
		BudgetUsed: 90,
		Budget:     &model.QuotaResetSetting{Period: "daily", Limit: 100},
	}

	// 未越限:90 + 5 < 100 → 放行,增量计入回收基数
	var gotMemberId, gotAmount int
	tally := int64(0)
	cacheIncreaseOrgMemberRealtimeBudgetFn = func(memberId int, quota int) (int64, error) {
		gotMemberId, gotAmount = memberId, quota
		tally += int64(quota)
		return tally, nil
	}
	q := &Quota{orgGuardrail: guardrail}
	if err := q.accrueOrgMemberRealtimeBudget(5); err != nil {
		t.Fatalf("未越限应放行: %v", err)
	}
	if gotMemberId != 7 || gotAmount != 5 {
		t.Fatalf("tally 参数 = (%d,%d), want (7,5)", gotMemberId, gotAmount)
	}
	if q.memberBudgetCacheQuota != 5 {
		t.Fatalf("memberBudgetCacheQuota = %d, want 5", q.memberBudgetCacheQuota)
	}

	// 越限:90 + (5+5) ≥ 100 → 中断流,错误文案区分 org_member_budget_exceeded
	err := q.accrueOrgMemberRealtimeBudget(5)
	if err == nil {
		t.Fatal("入场 BudgetUsed + tally ≥ 周期上限必须中断流")
	}
	if !strings.Contains(err.Error(), "org_member_budget_exceeded") {
		t.Fatalf("错误文案应含 org_member_budget_exceeded: %v", err)
	}
	// 越限增量已写入 Redis,同样计入回收基数(结算清理需完整回收)
	if q.memberBudgetCacheQuota != 10 {
		t.Fatalf("memberBudgetCacheQuota = %d, want 10", q.memberBudgetCacheQuota)
	}

	// Redis 出错 → 沿用 UpdateUserRealtimeQuota 既有语义返回错误(不 fail-open)
	cacheIncreaseOrgMemberRealtimeBudgetFn = func(memberId int, quota int) (int64, error) {
		return 0, errors.New("redis down")
	}
	if err := q.accrueOrgMemberRealtimeBudget(5); err == nil {
		t.Fatal("Redis 出错必须返回错误中断流")
	}

	// 无预算(个人令牌/未配置预算)→ noop,不触达 Redis,零行为变化
	called := false
	cacheIncreaseOrgMemberRealtimeBudgetFn = func(memberId int, quota int) (int64, error) {
		called = true
		return 0, nil
	}
	q = &Quota{}
	if err := q.accrueOrgMemberRealtimeBudget(5); err != nil {
		t.Fatalf("无预算应为 noop: %v", err)
	}
	if called || q.memberBudgetCacheQuota != 0 {
		t.Fatal("无预算不应触达 Redis tally")
	}
}

// TestPreQuotaConsumptionOrgGuardrail 守护 SEC-9/SEC-10(B4,realtime 入场同此路径):
// PreQuotaConsumption 入口 —— 组织令牌守护加载失败/白名单外模型拒绝;
// 通过后 orgGuardrail 被填充(B3 实时越限中断依赖其非 nil);个人令牌不填充。
func TestPreQuotaConsumptionOrgGuardrail(t *testing.T) {
	orig := cacheGetOrgMemberGuardrailFn
	defer func() { cacheGetOrgMemberGuardrailFn = orig }()

	// 组织令牌 + 守护加载失败 → 503 拒绝
	cacheGetOrgMemberGuardrailFn = func(shadowUserId int, memberUserId int) (*model.OrgMemberGuardrail, error) {
		return nil, errors.New("db down")
	}
	q := &Quota{userId: 100, createdBy: 5, modelName: "gpt-4o-realtime"}
	errResp := q.PreQuotaConsumption()
	if errResp == nil || errResp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("守护加载失败应 503 拒绝: %+v", errResp)
	}

	// 白名单不含请求模型 → 403 拒绝
	cacheGetOrgMemberGuardrailFn = func(shadowUserId int, memberUserId int) (*model.OrgMemberGuardrail, error) {
		return &model.OrgMemberGuardrail{MemberId: 7, ModelWhitelist: []string{"gpt-4o-mini"}}, nil
	}
	q = &Quota{userId: 100, createdBy: 5, modelName: "gpt-4o-realtime"}
	errResp = q.PreQuotaConsumption()
	if errResp == nil || errResp.StatusCode != http.StatusForbidden {
		t.Fatalf("白名单外模型应 403 拒绝: %+v", errResp)
	}
	if errResp.OpenAIError.Code != "org_model_not_allowed" {
		t.Fatalf("Code = %v, want org_model_not_allowed", errResp.OpenAIError.Code)
	}

	// 守护通过(价格为零 → 无预扣,不触达额度 DB)→ 放行且 orgGuardrail 非 nil
	guardrail := &model.OrgMemberGuardrail{MemberId: 7}
	cacheGetOrgMemberGuardrailFn = func(shadowUserId int, memberUserId int) (*model.OrgMemberGuardrail, error) {
		return guardrail, nil
	}
	q = &Quota{userId: 100, createdBy: 5, modelName: "gpt-4o-realtime"}
	if errResp := q.PreQuotaConsumption(); errResp != nil {
		t.Fatalf("守护通过应放行: %+v", errResp)
	}
	if q.orgGuardrail != guardrail {
		t.Fatal("PreQuotaConsumption 通过后 orgGuardrail 必须被填充")
	}

	// 个人令牌(createdBy=0)→ 放行且不填充守护
	called := false
	cacheGetOrgMemberGuardrailFn = func(shadowUserId int, memberUserId int) (*model.OrgMemberGuardrail, error) {
		called = true
		return nil, nil
	}
	q = &Quota{userId: 1, createdBy: 0, modelName: "gpt-4o-realtime"}
	if errResp := q.PreQuotaConsumption(); errResp != nil {
		t.Fatalf("个人令牌应放行: %+v", errResp)
	}
	if called || q.orgGuardrail != nil {
		t.Fatal("个人令牌不应加载/填充守护数据")
	}
}

// TestUndoReleasesOrgMemberRealtimeBudget verifier P1:Undo 路径回收成员预算实时 tally,
// 与 completedQuotaConsumption 的 defer 对称;HandelStatus=false(未预扣)同样回收。
func TestUndoReleasesOrgMemberRealtimeBudget(t *testing.T) {
	orig := cacheDecreaseOrgMemberRealtimeBudgetFn
	defer func() { cacheDecreaseOrgMemberRealtimeBudgetFn = orig }()

	var calls, gotAmount int
	cacheDecreaseOrgMemberRealtimeBudgetFn = func(memberId int, quota int) (int64, error) {
		calls++
		gotAmount = quota
		return 0, nil
	}

	guardrail := &model.OrgMemberGuardrail{
		MemberId: 7,
		Budget:   &model.QuotaResetSetting{Period: "daily", Limit: 100},
	}
	q := &Quota{orgGuardrail: guardrail, memberBudgetCacheQuota: 9, HandelStatus: false}
	q.Undo(nil)
	if calls != 1 || gotAmount != 9 {
		t.Fatalf("回收 = (calls=%d, amount=%d), want (1,9)", calls, gotAmount)
	}
	if q.memberBudgetCacheQuota != 0 {
		t.Fatalf("回收后 memberBudgetCacheQuota = %d, want 0", q.memberBudgetCacheQuota)
	}
}

// TestReleaseOrgMemberRealtimeBudget 守护 SEC-12:结算回收 —— 按累计量回收一次并清零,
// 无累计/无守护时 noop(个人令牌零行为变化)。
func TestReleaseOrgMemberRealtimeBudget(t *testing.T) {
	orig := cacheDecreaseOrgMemberRealtimeBudgetFn
	defer func() { cacheDecreaseOrgMemberRealtimeBudgetFn = orig }()

	var gotMemberId, gotAmount, calls int
	cacheDecreaseOrgMemberRealtimeBudgetFn = func(memberId int, quota int) (int64, error) {
		gotMemberId, gotAmount = memberId, quota
		calls++
		return 0, nil
	}

	guardrail := &model.OrgMemberGuardrail{
		MemberId: 7,
		Budget:   &model.QuotaResetSetting{Period: "daily", Limit: 100},
	}
	q := &Quota{orgGuardrail: guardrail, memberBudgetCacheQuota: 12}
	q.releaseOrgMemberRealtimeBudget()
	if calls != 1 || gotMemberId != 7 || gotAmount != 12 {
		t.Fatalf("回收 = (calls=%d, member=%d, amount=%d), want (1,7,12)", calls, gotMemberId, gotAmount)
	}
	if q.memberBudgetCacheQuota != 0 {
		t.Fatalf("回收后 memberBudgetCacheQuota = %d, want 0", q.memberBudgetCacheQuota)
	}

	// 已回收后再调用 → noop,不重复回收
	q.releaseOrgMemberRealtimeBudget()
	if calls != 1 {
		t.Fatal("无累计不应再次触达 Redis")
	}

	// 无守护(个人令牌)→ noop
	q = &Quota{memberBudgetCacheQuota: 0}
	q.releaseOrgMemberRealtimeBudget()
	if calls != 1 {
		t.Fatal("个人令牌不应触达 Redis")
	}
}
