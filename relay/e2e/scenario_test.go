package e2e

// E3 场景：5xx 重试换渠道、4xx 透传 + 预扣 Undo、SEC-9 成员预算硬闸、上游超时。
// 复用 E1 的 harness / fake provider；全局单例由 harness 重置，故本文件同样不得 t.Parallel。

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"

	"github.com/spf13/viper"
)

// preConsumeForcingQuota 让预扣真实发生：PreQuotaConsumption 在
// 「用户额度 > 100 × 预扣额」时会跳过预扣，而预扣额约为 promptTokens×倍率 + PreConsumedQuota(500)。
// 取 20000 既高于预扣额（不会因额度不足被提前拒绝），又低于 100 倍阈值。
const preConsumeForcingQuota = 20_000

// setRelayRequestTimeout 调小非流式上游请求超时（common/requester 的 relayRequestTimeout 为包私有，
// 只能经 viper + InitHttpClient 改写），并在测试结束还原为默认值。
func setRelayRequestTimeout(t *testing.T, seconds int) {
	t.Helper()
	const defaultSeconds = 300
	old := viper.GetInt("relay_request_timeout")
	if old == 0 {
		old = defaultSeconds
	}
	viper.Set("relay_request_timeout", seconds)
	requester.InitHttpClient()
	t.Cleanup(func() {
		viper.Set("relay_request_timeout", old)
		requester.InitHttpClient()
	})
}

// consumeLogs 读出本次测试库里的全部消费日志（LogTypeConsume）。
func consumeLogs(t *testing.T) []model.Log {
	t.Helper()
	var logs []model.Log
	if err := model.DB.Where("type = ?", model.LogTypeConsume).Find(&logs).Error; err != nil {
		t.Fatalf("读取消费日志失败: %v", err)
	}
	return logs
}

// readUserQuota 供 FakeResponse.Hook 在上游 handler goroutine 上读额度（不能用 t.Fatalf）；
// 读失败返回 0，由主协程的断言判为异常。
func readUserQuota(userId int) int {
	quota, err := model.GetUserQuota(userId)
	if err != nil {
		return 0
	}
	return quota
}

// channelUsedQuota 读取渠道累计用量，用于断言计费归属到了哪个渠道。
func channelUsedQuota(t *testing.T, channelId int) int64 {
	t.Helper()
	var ch model.Channel
	if err := model.DB.First(&ch, channelId).Error; err != nil {
		t.Fatalf("读取渠道 %d 失败: %v", channelId, err)
	}
	return ch.UsedQuota
}

// 场景①：第一渠道 500 → shouldRetry 为真 → shouldCooldowns 把它写入 skip_channel_ids →
// 第二渠道成功。断言：200、两个渠道各收到一次请求、计费与消费日志都归属第二渠道。
func TestRetryOn5xxSwitchesChannelAndBillsSecond(t *testing.T) {
	failing := NewFakeProvider(t)
	failing.SetResponse(FakeResponse{
		Status: http.StatusInternalServerError,
		JSON:   OpenAIErrorJSON("upstream exploded", "server_error", "internal_error"),
	})
	healthy := NewFakeProvider(t)
	healthy.SetResponse(FakeResponse{JSON: ChatCompletionJSON("gpt-4o-mini", "second channel wins", 11, 7)})

	h := NewHarness(t, HarnessOptions{
		Channels: []ChannelSpec{
			// 高优先级渠道先被选中（balancer 按 priority 降序遍历），保证首次必然打到失败渠道，
			// 避免 E1 默认的等权随机选择让本用例偶发跑反。
			{Name: "failing", BaseURL: failing.Server.URL, Priority: 10},
			{Name: "healthy", BaseURL: healthy.Server.URL, Priority: 1},
		},
	})
	// E1 默认 RetryTimes=0（不重试）；本场景必须开启重试才能验证换渠道。
	setAndRestore(t, &config.RetryTimes, 3)

	w := h.Post("/v1/chat/completions", `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`)

	if w.Code != http.StatusOK {
		t.Fatalf("重试后应成功返回 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "second channel wins") {
		t.Fatalf("响应体应来自第二渠道: %s", w.Body.String())
	}

	if got := len(failing.Requests()); got != 1 {
		t.Fatalf("失败渠道应只被打一次（skip_channel_ids 生效），实际 %d 次", got)
	}
	if got := len(healthy.Requests()); got != 1 {
		t.Fatalf("健康渠道应被重试打中一次，实际 %d 次", got)
	}

	failingId, healthyId := h.Channels[0].Id, h.Channels[1].Id
	if used := channelUsedQuota(t, failingId); used != 0 {
		t.Fatalf("失败渠道不应产生计费，used_quota=%d", used)
	}
	if used := channelUsedQuota(t, healthyId); used <= 0 {
		t.Fatalf("计费应归属第二渠道，used_quota=%d", used)
	}

	logs := consumeLogs(t)
	if len(logs) != 1 {
		t.Fatalf("应只落一条消费日志，实际 %d 条", len(logs))
	}
	if logs[0].ChannelId != healthyId {
		t.Fatalf("消费日志渠道归属 = %d，期望第二渠道 %d", logs[0].ChannelId, healthyId)
	}
	if logs[0].CompletionTokens != 7 {
		t.Fatalf("消费日志应记录第二渠道的 usage，completion_tokens=%d", logs[0].CompletionTokens)
	}
}

// 场景②：上游 400 → shouldRetry 为假（不重试）→ 错误体过 FilterOpenAIErr 白名单透传原文 →
// 预扣被 Undo 回滚。
func TestUpstream4xxPassthroughAndUndo(t *testing.T) {
	fake := NewFakeProvider(t)
	// 用户额度压到 100x 跳过阈值之下，逼出真实预扣，使 Undo 断言可证伪。
	h := NewHarness(t, HarnessOptions{
		FakeBaseURL: fake.Server.URL,
		UserQuota:   preConsumeForcingQuota,
		TokenQuota:  preConsumeForcingQuota,
	})
	// 开启重试，用来证明「不重试」来自 shouldRetry 的 4xx 判定，而非 RetryTimes=0。
	setAndRestore(t, &config.RetryTimes, 3)

	var inFlightQuota atomic.Int64
	fake.SetResponse(FakeResponse{
		Status: http.StatusBadRequest,
		JSON:   OpenAIErrorJSON("invalid temperature", "invalid_request_error", "invalid_value"),
		Hook:   func() { inFlightQuota.Store(int64(readUserQuota(h.User.Id))) },
	})

	quotaBefore := h.UserQuota(t)
	w := h.Post("/v1/chat/completions", `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("上游 400 应透传状态码，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if got := len(fake.Requests()); got != 1 {
		t.Fatalf("4xx 不应重试，上游收到 %d 次请求", got)
	}

	var resp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    any    `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析错误响应失败: %v, body=%s", err, w.Body.String())
	}
	// FilterOpenAIErr 的 400 白名单：保留上游原文，仅补 request id。
	if !strings.Contains(resp.Error.Message, "invalid temperature") {
		t.Fatalf("400 应透传上游原文: %+v", resp.Error)
	}
	if !strings.Contains(resp.Error.Message, "request id:") {
		t.Fatalf("错误消息应带 request id: %+v", resp.Error)
	}
	// 品牌名/上游身份类 type 会被改写为 system_error，绝不外泄。
	if resp.Error.Type == "modeltaps_error" || strings.HasSuffix(resp.Error.Type, "_api_error") {
		t.Fatalf("错误 type 泄漏了内部标签: %+v", resp.Error)
	}

	if got := inFlightQuota.Load(); got == 0 || got >= int64(quotaBefore) {
		t.Fatalf("请求在途时应已发生预扣: before=%d in_flight=%d", quotaBefore, got)
	}
	if quotaAfter := h.UserQuota(t); quotaAfter != quotaBefore {
		t.Fatalf("预扣应被 Undo 回滚: before=%d after=%d", quotaBefore, quotaAfter)
	}
	if logs := consumeLogs(t); len(logs) != 0 {
		t.Fatalf("上游 400 且无输出时不应落消费日志，实际 %d 条", len(logs))
	}
}

// orgTokenFixture 为 SEC-9 场景 seed 一套「组织 + 影子账户 + 成员 + 组织令牌」，
// 返回令牌 key、影子账户 id 与 org_members 行 id。
// 组织令牌判定见 relay_util.Quota.isOrgToken：user_id 为影子账户、created_by 为成员。
func orgTokenFixture(t *testing.T, h *Harness, group string, budget *model.QuotaResetSetting, budgetUsed int) (tokenKey string, shadowUserId int, memberRowId int) {
	t.Helper()

	shadow := &model.User{
		Username:    "org-e2e-shadow",
		Password:    "shadow-placeholder",
		DisplayName: "Organization e2e",
		Role:        config.RoleGuestUser,
		Status:      config.UserStatusEnabled,
		Type:        config.UserTypeOrgShadow,
		AccessToken: "org-e2e-shadow-token",
		AffCode:     "orge2esd",
		Quota:       10_000_000,
		Group:       group,
	}
	if err := model.DB.Create(shadow).Error; err != nil {
		t.Fatalf("创建影子账户失败: %v", err)
	}

	org := &model.Organization{
		Name:         "E2E Org",
		Slug:         "e2e-org",
		Status:       model.OrganizationStatusEnabled,
		ShadowUserId: shadow.Id,
		CreatedBy:    h.User.Id,
	}
	if err := model.DB.Create(org).Error; err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}

	member := &model.OrganizationMember{
		OrganizationId: org.Id,
		UserId:         h.User.Id,
		Role:           model.OrgRoleMember,
	}
	member.Setting.Set(model.OrgMemberSetting{Budget: budget})
	if err := model.DB.Create(member).Error; err != nil {
		t.Fatalf("创建组织成员失败: %v", err)
	}
	// 周期计数为专用列（SEC-4），直接写入伪造「本周期已用尽」的状态。
	if err := model.DB.Model(&model.OrganizationMember{}).Where("id = ?", member.Id).
		UpdateColumns(map[string]any{
			"budget_used":  budgetUsed,
			"budget_start": time.Now().UTC().Unix(),
		}).Error; err != nil {
		t.Fatalf("写入成员预算计数失败: %v", err)
	}

	token := &model.Token{
		UserId:      shadow.Id,
		CreatedBy:   h.User.Id,
		Name:        "org-e2e-token",
		Status:      config.TokenStatusEnabled,
		ExpiredTime: -1,
		RemainQuota: 10_000_000,
		Group:       group,
	}
	if err := model.DB.Create(token).Error; err != nil {
		t.Fatalf("创建组织令牌失败: %v", err)
	}
	return token.Key, shadow.Id, member.Id
}

// 场景③：组织成员本周期预算已用尽 → checkOrgMemberGuardrails 在 PreQuotaConsumption 入口
// 直接 402 拒绝。断言：零费用（影子账户额度与成员预算计数都不动）、上游零请求、无消费日志。
func TestOrgMemberBudgetHardLimitRejectsWithZeroCost(t *testing.T) {
	fake := NewFakeProvider(t)
	h := NewHarness(t, HarnessOptions{FakeBaseURL: fake.Server.URL})
	// 拒绝发生在预扣之前，与是否可重试无关；开着重试以确保「零请求」断言更强。
	setAndRestore(t, &config.RetryTimes, 3)

	budget := &model.QuotaResetSetting{Period: model.TokenQuotaResetPeriodDaily, Limit: 100}
	tokenKey, shadowUserId, memberRowId := orgTokenFixture(t, h, "default", budget, budget.Limit)

	quotaBefore, err := model.GetUserQuota(shadowUserId)
	if err != nil {
		t.Fatalf("读取影子账户额度失败: %v", err)
	}

	w := h.PostWithToken(tokenKey, "/v1/chat/completions",
		`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`)

	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("成员预算超限应返回 402，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if got := len(fake.Requests()); got != 0 {
		t.Fatalf("硬闸应在触达上游前拒绝，上游收到 %d 次请求", got)
	}

	quotaAfter, err := model.GetUserQuota(shadowUserId)
	if err != nil {
		t.Fatalf("读取影子账户额度失败: %v", err)
	}
	if quotaAfter != quotaBefore {
		t.Fatalf("拒绝必须零费用: before=%d after=%d", quotaBefore, quotaAfter)
	}

	var member model.OrganizationMember
	if err := model.DB.First(&member, memberRowId).Error; err != nil {
		t.Fatalf("读取成员预算计数失败: %v", err)
	}
	if member.BudgetUsed != budget.Limit {
		t.Fatalf("拒绝不应改动成员预算计数: budget_used=%d，期望 %d", member.BudgetUsed, budget.Limit)
	}

	if logs := consumeLogs(t); len(logs) != 0 {
		t.Fatalf("零费用拒绝不应落消费日志，实际 %d 条", len(logs))
	}
}

// 场景③b：团队级周期预算已用尽（T1）→ reserveOrganizationBudget 在池预扣之前 402 拒绝。
// 成员未配置预算，验证团队预算独立生效，且组织令牌不再走 100x 跳过。
// 断言：零费用（影子账户额度与组织预算计数都不动）、上游零请求、无消费日志。
func TestOrgBudgetHardLimitRejectsWithZeroCost(t *testing.T) {
	fake := NewFakeProvider(t)
	h := NewHarness(t, HarnessOptions{FakeBaseURL: fake.Server.URL})
	setAndRestore(t, &config.RetryTimes, 3)

	tokenKey, shadowUserId, _ := orgTokenFixture(t, h, "default", nil, 0)

	var org model.Organization
	if err := model.DB.First(&org, "shadow_user_id = ?", shadowUserId).Error; err != nil {
		t.Fatalf("读取组织失败: %v", err)
	}
	budget := &model.QuotaResetSetting{Period: model.TokenQuotaResetPeriodDaily, Limit: 100}
	setting := org.Setting.Data()
	setting.Budget = budget
	org.Setting.Set(setting)
	// 团队预算计数为专用列，直接写入伪造「本周期已用尽」的状态
	org.BudgetUsed = budget.Limit
	org.BudgetStart = time.Now().UTC().Unix()
	if err := model.DB.Model(&org).Select("setting", "budget_used", "budget_start").Updates(&org).Error; err != nil {
		t.Fatalf("写入团队预算失败: %v", err)
	}

	quotaBefore, err := model.GetUserQuota(shadowUserId)
	if err != nil {
		t.Fatalf("读取影子账户额度失败: %v", err)
	}

	w := h.PostWithToken(tokenKey, "/v1/chat/completions",
		`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`)

	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("团队预算超限应返回 402，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "org_budget_exceeded") {
		t.Fatalf("错误码应为 org_budget_exceeded，body=%s", w.Body.String())
	}
	if got := len(fake.Requests()); got != 0 {
		t.Fatalf("硬闸应在触达上游前拒绝，上游收到 %d 次请求", got)
	}

	quotaAfter, err := model.GetUserQuota(shadowUserId)
	if err != nil {
		t.Fatalf("读取影子账户额度失败: %v", err)
	}
	if quotaAfter != quotaBefore {
		t.Fatalf("拒绝必须零费用: before=%d after=%d", quotaBefore, quotaAfter)
	}

	var reloaded model.Organization
	if err := model.DB.First(&reloaded, org.Id).Error; err != nil {
		t.Fatalf("读取团队预算计数失败: %v", err)
	}
	if reloaded.BudgetUsed != budget.Limit {
		t.Fatalf("拒绝不应改动团队预算计数: budget_used=%d，期望 %d", reloaded.BudgetUsed, budget.Limit)
	}

	if logs := consumeLogs(t); len(logs) != 0 {
		t.Fatalf("零费用拒绝不应落消费日志，实际 %d 条", len(logs))
	}
}

// 场景④：上游挂起超过 relayRequestTimeout → SendRequest 的 ctx 超时 → 预扣被 Undo，
// 错误经 FilterOpenAIErr 坍缩为 503。
func TestUpstreamTimeoutUndoesPreConsumedQuota(t *testing.T) {
	fake := NewFakeProvider(t)
	// 用户额度压到 100x 跳过阈值之下，逼出真实预扣，使 Undo 断言可证伪。
	h := NewHarness(t, HarnessOptions{
		FakeBaseURL: fake.Server.URL,
		UserQuota:   preConsumeForcingQuota,
		TokenQuota:  preConsumeForcingQuota,
	})
	setRelayRequestTimeout(t, 1)

	// Hook 跑在 httptest 的 handler goroutine 上，用 atomic 交接以免与主协程读竞争。
	var inFlightQuota atomic.Int64
	fake.SetResponse(FakeResponse{
		// 挂起时长必须大于上面配置的 1s 超时。
		Delay: 3 * time.Second,
		JSON:  ChatCompletionJSON("gpt-4o-mini", "too late", 11, 7),
		Hook:  func() { inFlightQuota.Store(int64(readUserQuota(h.User.Id))) },
	})

	quotaBefore := h.UserQuota(t)
	w := h.Post("/v1/chat/completions", `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("上游超时应坍缩为 503，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if got := inFlightQuota.Load(); got == 0 || got >= int64(quotaBefore) {
		t.Fatalf("请求在途时应已发生预扣: before=%d in_flight=%d", quotaBefore, got)
	}
	if quotaAfter := h.UserQuota(t); quotaAfter != quotaBefore {
		t.Fatalf("超时后预扣应被 Undo 回滚: before=%d after=%d", quotaBefore, quotaAfter)
	}
	if logs := consumeLogs(t); len(logs) != 0 {
		t.Fatalf("超时无输出时不应落消费日志，实际 %d 条", len(logs))
	}
}
