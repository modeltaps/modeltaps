package e2e

// 核心计费场景 E2E：非流式精确计费、余额不足 402（个人/组织两分支）、
// 流式按流末 usage 结算、流中断 TextBuilder 兜底补计费。
//
// 计费口径（relay/relay_util/quota.go）：
//   quota = ceil(promptTokens*Input*groupRatio + completionTokens*Output*groupRatio)
// 价格倍率刻意取 Input=2 / Output=3（而非 1/1），使输入侧与输出侧的贡献可被区分。
//
// 全局单例每个测试重置，故本包测试不得 t.Parallel。

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"
)

const (
	billingInputRatio  = 2.0
	billingOutputRatio = 3.0
)

// expectQuota 复算计费口径，供断言使用。
func expectQuota(promptTokens, completionTokens int, groupRatio float64) int {
	return int(math.Ceil(float64(promptTokens)*billingInputRatio*groupRatio +
		float64(completionTokens)*billingOutputRatio*groupRatio))
}

// errorBody 解析 relay 的 JSON 错误响应体，返回 code 与 message。
func errorBody(t *testing.T, raw []byte) (code, message string) {
	t.Helper()
	var resp struct {
		Error struct {
			Code    any    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("解析错误响应失败: %v, body=%s", err, string(raw))
	}
	codeStr, _ := resp.Error.Code.(string)
	return codeStr, resp.Error.Message
}

// soleConsumeLog 断言恰好只有一条消费日志并返回它（复用 scenario_test.go 的 consumeLogs）。
func soleConsumeLog(t *testing.T) model.Log {
	t.Helper()
	logs := consumeLogs(t)
	if len(logs) != 1 {
		t.Fatalf("期望恰好 1 条消费日志，实际 %d 条: %+v", len(logs), logs)
	}
	return logs[0]
}

// 场景①非流式成功：200 + 余额差精确等于 usage×倍率 + 消费日志落库。
func TestBillingNonStreamExact(t *testing.T) {
	const promptTokens, completionTokens = 11, 7
	const groupRatio = 1.5

	fake := NewFakeProvider(t)
	fake.SetResponse(FakeResponse{
		JSON: ChatCompletionJSON("gpt-4o-mini", "hello from fake", promptTokens, completionTokens),
	})

	h := NewHarness(t, HarnessOptions{
		FakeBaseURL: fake.Server.URL,
		GroupRatio:  groupRatio,
		Prices:      []PriceSpec{{Model: "gpt-4o-mini", Input: billingInputRatio, Output: billingOutputRatio}},
	})

	quotaBefore := h.UserQuota(t)
	w := h.Post("/v1/chat/completions", `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}

	wantQuota := expectQuota(promptTokens, completionTokens, groupRatio)
	if got := quotaBefore - h.UserQuota(t); got != wantQuota {
		t.Fatalf("余额差不符：期望扣 %d，实际扣 %d", wantQuota, got)
	}

	log := soleConsumeLog(t)
	if log.UserId != h.User.Id || log.ChannelId != h.Channel.Id {
		t.Fatalf("消费日志归属不符: userId=%d channelId=%d", log.UserId, log.ChannelId)
	}
	if log.PromptTokens != promptTokens || log.CompletionTokens != completionTokens {
		t.Fatalf("消费日志 tokens 不符: prompt=%d completion=%d", log.PromptTokens, log.CompletionTokens)
	}
	if log.Quota != wantQuota {
		t.Fatalf("消费日志 quota 不符：期望 %d，实际 %d", wantQuota, log.Quota)
	}
	if log.ModelName != "gpt-4o-mini" || log.TokenName != h.Token.Name || log.IsStream {
		t.Fatalf("消费日志元信息不符: %+v", log)
	}
	// 未配置渠道成本倍率时不计成本。
	if log.CostQuota != 0 {
		t.Fatalf("未配置成本倍率时 cost_quota 应为 0，实际 %d", log.CostQuota)
	}

	meta := log.Metadata.Data()
	if meta["group_ratio"] != groupRatio || meta["input_ratio"] != billingInputRatio || meta["output_ratio"] != billingOutputRatio {
		t.Fatalf("消费日志 metadata 倍率不符: %+v", meta)
	}
}

// 场景②-A 个人令牌余额不足：402 insufficient_user_quota，零费用且不触达上游。
func TestBillingInsufficientUserQuota(t *testing.T) {
	fake := NewFakeProvider(t)
	h := NewHarness(t, HarnessOptions{
		FakeBaseURL: fake.Server.URL,
		// 低于 PreConsumedQuota（500）保底预扣额，使预扣校验必然失败。
		UserQuota: 100,
		Prices:    []PriceSpec{{Model: "gpt-4o-mini", Input: billingInputRatio, Output: billingOutputRatio}},
	})

	quotaBefore := h.UserQuota(t)
	w := h.Post("/v1/chat/completions", `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`)

	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("期望 402，实际 %d，body=%s", w.Code, w.Body.String())
	}
	code, message := errorBody(t, w.Body.Bytes())
	if code != "insufficient_user_quota" {
		t.Fatalf("错误 code 不符: %q, body=%s", code, w.Body.String())
	}
	if !strings.Contains(message, "user quota is not enough") {
		t.Fatalf("错误文案不符: %q", message)
	}

	if got := h.UserQuota(t); got != quotaBefore {
		t.Fatalf("零费用拒绝时额度不应变化: before=%d after=%d", quotaBefore, got)
	}
	if logs := consumeLogs(t); len(logs) != 0 {
		t.Fatalf("拒绝路径不应落消费日志，实际 %d 条", len(logs))
	}
	if reqs := fake.Requests(); len(reqs) != 0 {
		t.Fatalf("拒绝应发生在触达上游之前，实际上游收到 %d 次请求", len(reqs))
	}
}

// 场景②-B 组织令牌积分池耗尽：402 insufficient_org_quota，与个人分支的 code 明确区分。
func TestBillingInsufficientOrgQuota(t *testing.T) {
	fake := NewFakeProvider(t)
	h := NewHarness(t, HarnessOptions{
		FakeBaseURL: fake.Server.URL,
		Prices:      []PriceSpec{{Model: "gpt-4o-mini", Input: billingInputRatio, Output: billingOutputRatio}},
	})
	// 组织令牌：user_id 为影子账户、created_by 为成员，命中 quota.go 的 isOrgToken 分支。
	// 成员不配预算（budget=nil），把变量收敛到「影子账户即组织积分池」这一条。
	tokenKey, shadowUserId, _ := orgTokenFixture(t, h, "default", nil, 0)
	// 把积分池压到保底预扣额（PreConsumedQuota=500）以下，使预扣校验必然失败。
	if err := model.DB.Model(&model.User{}).Where("id = ?", shadowUserId).
		Update("quota", 100).Error; err != nil {
		t.Fatalf("压低影子账户额度失败: %v", err)
	}

	quotaBefore := readUserQuota(shadowUserId)
	w := h.PostWithToken(tokenKey, "/v1/chat/completions",
		`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`)

	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("期望 402，实际 %d，body=%s", w.Code, w.Body.String())
	}
	code, message := errorBody(t, w.Body.Bytes())
	if code != "insufficient_org_quota" {
		t.Fatalf("组织令牌应返回 insufficient_org_quota，实际 %q, body=%s", code, w.Body.String())
	}
	if !strings.Contains(message, "organization quota is not enough") {
		t.Fatalf("错误文案不符: %q", message)
	}

	if got := readUserQuota(shadowUserId); got != quotaBefore {
		t.Fatalf("零费用拒绝时影子账户额度不应变化: before=%d after=%d", quotaBefore, got)
	}
	if logs := consumeLogs(t); len(logs) != 0 {
		t.Fatalf("拒绝路径不应落消费日志，实际 %d 条", len(logs))
	}
	if reqs := fake.Requests(); len(reqs) != 0 {
		t.Fatalf("拒绝应发生在触达上游之前，实际上游收到 %d 次请求", len(reqs))
	}
}

// 场景③流式成功：SSE 分片逐块透传，结算取流末 usage 分片而非本地估算。
func TestBillingStreamSettlesOnFinalUsage(t *testing.T) {
	const promptTokens, completionTokens = 13, 9
	const groupRatio = 2.0

	fake := NewFakeProvider(t)
	fake.SetResponse(FakeResponse{
		Stream: ChatStreamChunks("gpt-4o-mini", []string{"he", "ll", "o"}, promptTokens, completionTokens),
	})

	h := NewHarness(t, HarnessOptions{
		FakeBaseURL: fake.Server.URL,
		GroupRatio:  groupRatio,
		Prices:      []PriceSpec{{Model: "gpt-4o-mini", Input: billingInputRatio, Output: billingOutputRatio}},
	})

	quotaBefore := h.UserQuota(t)
	w := h.Post("/v1/chat/completions",
		`{"model":"gpt-4o-mini","stream":true,"messages":[{"role":"user","content":"hi"}]}`)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, frag := range []string{`"he"`, `"ll"`, `"o"`} {
		if !strings.Contains(body, frag) {
			t.Fatalf("流式分片 %s 未逐块透传: %s", frag, body)
		}
	}
	if !strings.HasSuffix(body, "data: [DONE]\n\n") {
		t.Fatalf("流未以 [DONE] 结束: %s", body)
	}

	// 结算取流末 usage：completion=9 与三个分片的本地估算无关。
	wantQuota := expectQuota(promptTokens, completionTokens, groupRatio)
	if got := quotaBefore - h.UserQuota(t); got != wantQuota {
		t.Fatalf("流式余额差不符：期望扣 %d，实际扣 %d", wantQuota, got)
	}

	log := soleConsumeLog(t)
	if !log.IsStream {
		t.Fatalf("流式消费日志 is_stream 应为 true: %+v", log)
	}
	if log.PromptTokens != promptTokens || log.CompletionTokens != completionTokens {
		t.Fatalf("流式日志未采用流末 usage: prompt=%d completion=%d", log.PromptTokens, log.CompletionTokens)
	}
	if log.Quota != wantQuota {
		t.Fatalf("流式日志 quota 不符：期望 %d，实际 %d", wantQuota, log.Quota)
	}
}

// 场景④流式中断：上游中途断开，既没发 usage 分片也没发 finish_reason，
// relay/main.go 的 TextBuilder 兜底用 CountTokenText 估算 completion，仍产生消费记录。
func TestBillingStreamInterruptedFallsBackToTextBuilder(t *testing.T) {
	const groupRatio = 1.0
	const userContent = "hi"
	contents := []string{"hello ", "interrupted ", "stream"}

	fake := NewFakeProvider(t)
	// 只发内容分片、不带 usage，且写完即断开（不补 [DONE]）。
	fake.SetResponse(FakeResponse{
		Stream:       ChatStreamChunksNoUsage("gpt-4o-mini", contents),
		StreamNoDone: true,
	})

	h := NewHarness(t, HarnessOptions{
		FakeBaseURL: fake.Server.URL,
		GroupRatio:  groupRatio,
		Prices:      []PriceSpec{{Model: "gpt-4o-mini", Input: billingInputRatio, Output: billingOutputRatio}},
	})

	quotaBefore := h.UserQuota(t)
	w := h.Post("/v1/chat/completions",
		`{"model":"gpt-4o-mini","stream":true,"messages":[{"role":"user","content":"`+userContent+`"}]}`)

	// 中断前已透传的分片仍应到达客户端；上游既未给 usage 也未给 finish_reason。
	body := w.Body.String()
	if !strings.Contains(body, `"hello "`) {
		t.Fatalf("中断前的分片应已透传: %s", body)
	}
	if strings.Contains(body, `"usage"`) || strings.Contains(body, `"finish_reason":"stop"`) {
		t.Fatalf("上游中断场景不应出现 usage / finish_reason 分片: %s", body)
	}

	// 上游未给 usage，故 prompt 侧沿用本地 tokenize 的结果（口径同 relay/chat.go getPromptTokens），
	// completion 侧走 relay/main.go 的兜底：CountTokenText(累积文本)。
	wantPrompt := common.CountTokenMessages(
		[]types.ChatCompletionMessage{{Role: "user", Content: userContent}},
		"gpt-4o-mini", config.PreCostDefault)
	wantCompletion := common.CountTokenText(strings.Join(contents, ""), "gpt-4o-mini")
	if wantPrompt <= 0 || wantCompletion <= 0 {
		t.Fatalf("兜底 token 估算应为正数: prompt=%d completion=%d", wantPrompt, wantCompletion)
	}
	wantQuota := expectQuota(wantPrompt, wantCompletion, groupRatio)

	if got := quotaBefore - h.UserQuota(t); got != wantQuota {
		t.Fatalf("流中断补计费不符：期望扣 %d，实际扣 %d", wantQuota, got)
	}

	log := soleConsumeLog(t)
	if !log.IsStream {
		t.Fatalf("流中断的消费日志 is_stream 应为 true: %+v", log)
	}
	if log.PromptTokens != wantPrompt || log.CompletionTokens != wantCompletion {
		t.Fatalf("流中断日志 tokens 不符: prompt=%d completion=%d（期望 %d/%d）",
			log.PromptTokens, log.CompletionTokens, wantPrompt, wantCompletion)
	}
	if log.Quota != wantQuota {
		t.Fatalf("流中断日志 quota 不符：期望 %d，实际 %d", wantQuota, log.Quota)
	}
}
