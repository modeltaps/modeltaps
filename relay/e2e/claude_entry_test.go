package e2e

// Claude 入口（/claude/v1/messages）在「新纳入白名单的对话渠道」上的端到端回归。
// 取 Groq（config.ChannelTypeGroq）作代表：它没有原生 Claude 实现，只满足 base.ChatInterface，
// 故必然走 Claude→OpenAI→Claude 转换链，正是 Wave 1 扩白名单后新增的归宿。
//
// 断言两侧：① 响应确为 Claude 协议形态（非流式 message / 流式 SSE 事件序列）且 usage 非零；
// ② 计费口径与同一渠道走 OpenAI 入口完全一致（同 usage → 同扣费 → 同消费日志）。
//
// 全局单例每个测试重置，故本文件同样不得 t.Parallel。

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// claudeEntryModel 走 Groq 渠道，模型名与 OpenAI 侧一致，便于两入口对照计费。
const claudeEntryModel = "llama-3.1-70b"

// newGroqHarness 装配一个仅含 Groq 渠道的 harness，价格倍率复用 billing_test.go 的 2/3。
func newGroqHarness(t *testing.T, baseURL string, groupRatio float64) *Harness {
	t.Helper()
	return NewHarness(t, HarnessOptions{
		Model:      claudeEntryModel,
		GroupRatio: groupRatio,
		Channels: []ChannelSpec{{
			Name:    "fake-groq",
			Type:    config.ChannelTypeGroq,
			BaseURL: baseURL,
		}},
		Prices: []PriceSpec{{Model: claudeEntryModel, Input: billingInputRatio, Output: billingOutputRatio}},
	})
}

// claudeMessagesBody 构造一次 Claude Messages 请求体。
func claudeMessagesBody(stream bool) string {
	streamField := "false"
	if stream {
		streamField = "true"
	}
	return `{"model":"` + claudeEntryModel + `","max_tokens":64,"stream":` + streamField +
		`,"messages":[{"role":"user","content":"hi"}]}`
}

// Claude 入口非流式：Groq 渠道经转换链应答 Claude message 形态，usage 非零，计费按 Groq 渠道价。
func TestClaudeEntryNonStreamOnNewlyAllowedChannel(t *testing.T) {
	const promptTokens, completionTokens = 11, 7
	const groupRatio = 1.5

	fake := NewFakeProvider(t)
	fake.SetResponse(FakeResponse{
		JSON: ChatCompletionJSON(claudeEntryModel, "hello from groq", promptTokens, completionTokens),
	})

	h := newGroqHarness(t, fake.Server.URL, groupRatio)
	quotaBefore := h.UserQuota(t)

	w := h.Post("/claude/v1/messages", claudeMessagesBody(false))
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Role    string `json:"role"`
		Model   string `json:"model"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析 Claude 响应失败: %v, body=%s", err, w.Body.String())
	}

	// Claude 协议形态：message/assistant + text 内容块 + end_turn（由 OpenAI 的 stop 转换而来）。
	if resp.Type != "message" || resp.Role != "assistant" || !strings.HasPrefix(resp.ID, "msg_") {
		t.Fatalf("响应不是 Claude message 形态: %+v", resp)
	}
	if len(resp.Content) != 1 || resp.Content[0].Type != "text" || resp.Content[0].Text != "hello from groq" {
		t.Fatalf("Claude content 块不符: %+v", resp.Content)
	}
	if resp.StopReason != "end_turn" {
		t.Fatalf("stop_reason 未由 stop 转换为 end_turn: %q", resp.StopReason)
	}
	if resp.Usage.InputTokens != promptTokens || resp.Usage.OutputTokens != completionTokens {
		t.Fatalf("Claude usage 未透传上游 tokens: %+v", resp.Usage)
	}

	// 上游确实收到 OpenAI 形态的 chat completions 请求（转换链生效）。
	captured := fake.LastRequest(t)
	if captured.Path != "/v1/chat/completions" {
		t.Fatalf("上游路径不符（应经 OpenAI 兼容转换链）: %s", captured.Path)
	}
	body := captured.JSONBody(t)
	if body["model"] != claudeEntryModel {
		t.Fatalf("上游收到的 model 不符: %v", body["model"])
	}
	if _, ok := body["messages"]; !ok {
		t.Fatalf("上游请求体应为 OpenAI messages 形态: %v", body)
	}

	// 计费口径与 OpenAI 入口同渠道一致：quota = ceil(prompt*Input*g + completion*Output*g)。
	wantQuota := expectQuota(promptTokens, completionTokens, groupRatio)
	if got := quotaBefore - h.UserQuota(t); got != wantQuota {
		t.Fatalf("Claude 入口扣费不符：期望 %d，实际 %d", wantQuota, got)
	}

	log := soleConsumeLog(t)
	if log.ChannelId != h.Channel.Id {
		t.Fatalf("消费日志应归属 Groq 渠道 %d，实际 %d", h.Channel.Id, log.ChannelId)
	}
	if log.PromptTokens != promptTokens || log.CompletionTokens != completionTokens {
		t.Fatalf("消费日志 tokens 不符: prompt=%d completion=%d", log.PromptTokens, log.CompletionTokens)
	}
	if log.Quota != wantQuota || log.ModelName != claudeEntryModel || log.IsStream {
		t.Fatalf("消费日志不符: %+v", log)
	}
}

// Claude 入口流式：Groq 渠道经转换链应答 Claude SSE 事件序列，usage 取流末分片，计费同口径。
func TestClaudeEntryStreamOnNewlyAllowedChannel(t *testing.T) {
	const promptTokens, completionTokens = 13, 9
	const groupRatio = 2.0

	fake := NewFakeProvider(t)
	fake.SetResponse(FakeResponse{
		Stream: ChatStreamChunks(claudeEntryModel, []string{"he", "ll", "o"}, promptTokens, completionTokens),
	})

	h := newGroqHarness(t, fake.Server.URL, groupRatio)
	quotaBefore := h.UserQuota(t)

	w := h.Post("/claude/v1/messages", claudeMessagesBody(true))
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}

	// Claude SSE 事件序列（而非 OpenAI 的 chat.completion.chunk + [DONE]）。
	body := w.Body.String()
	for _, want := range []string{
		"event: message_start",
		"event: content_block_start",
		"event: content_block_delta",
		"event: content_block_stop",
		"event: message_delta",
		"event: message_stop",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("流式响应缺少 Claude 事件 %q: %s", want, body)
		}
	}
	if strings.Contains(body, "chat.completion.chunk") || strings.Contains(body, "[DONE]") {
		t.Fatalf("Claude 入口不应透出 OpenAI 流形态: %s", body)
	}
	for _, frag := range []string{`"he"`, `"ll"`, `"o"`} {
		if !strings.Contains(body, frag) {
			t.Fatalf("文本分片 %s 未逐块透传: %s", frag, body)
		}
	}
	// message_delta 携带非零 usage，取自流末 usage 分片。
	wantUsage := `"usage":{"input_tokens":13,"output_tokens":9}`
	if !strings.Contains(body, wantUsage) {
		t.Fatalf("message_delta 未带流末 usage %s: %s", wantUsage, body)
	}

	wantQuota := expectQuota(promptTokens, completionTokens, groupRatio)
	if got := quotaBefore - h.UserQuota(t); got != wantQuota {
		t.Fatalf("Claude 流式扣费不符：期望 %d，实际 %d", wantQuota, got)
	}

	log := soleConsumeLog(t)
	if !log.IsStream {
		t.Fatalf("流式消费日志 is_stream 应为 true: %+v", log)
	}
	if log.PromptTokens != promptTokens || log.CompletionTokens != completionTokens || log.Quota != wantQuota {
		t.Fatalf("流式消费日志不符: %+v（期望 %d/%d/%d）", log, promptTokens, completionTokens, wantQuota)
	}
}

// 同一 Groq 渠道、同一 usage，Claude 入口与 OpenAI 入口的计费必须逐项一致。
// 这是「新纳入渠道不改变计费口径」的直接证明：两次请求只差入口路径。
func TestClaudeEntryBillingMatchesOpenAIEntry(t *testing.T) {
	const promptTokens, completionTokens = 11, 7
	const groupRatio = 1.5

	fake := NewFakeProvider(t)
	fake.SetResponse(FakeResponse{
		JSON: ChatCompletionJSON(claudeEntryModel, "hello from groq", promptTokens, completionTokens),
	})

	h := newGroqHarness(t, fake.Server.URL, groupRatio)

	before := h.UserQuota(t)
	if w := h.Post("/claude/v1/messages", claudeMessagesBody(false)); w.Code != http.StatusOK {
		t.Fatalf("Claude 入口期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	afterClaude := h.UserQuota(t)

	if w := h.Post("/v1/chat/completions",
		`{"model":"`+claudeEntryModel+`","messages":[{"role":"user","content":"hi"}]}`); w.Code != http.StatusOK {
		t.Fatalf("OpenAI 入口期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	afterOpenAI := h.UserQuota(t)

	claudeCost := before - afterClaude
	openaiCost := afterClaude - afterOpenAI
	if claudeCost != openaiCost {
		t.Fatalf("两入口扣费不一致: claude=%d openai=%d", claudeCost, openaiCost)
	}
	if want := expectQuota(promptTokens, completionTokens, groupRatio); claudeCost != want {
		t.Fatalf("扣费与计费口径不符：期望 %d，实际 %d", want, claudeCost)
	}

	logs := consumeLogs(t)
	if len(logs) != 2 {
		t.Fatalf("期望两条消费日志（每入口一条），实际 %d 条: %+v", len(logs), logs)
	}
	a, b := logs[0], logs[1]
	if a.Quota != b.Quota || a.PromptTokens != b.PromptTokens || a.CompletionTokens != b.CompletionTokens {
		t.Fatalf("两入口消费日志计费项不一致: %+v vs %+v", a, b)
	}
	if a.ChannelId != h.Channel.Id || b.ChannelId != h.Channel.Id {
		t.Fatalf("两条日志都应归属 Groq 渠道 %d: %+v vs %+v", h.Channel.Id, a, b)
	}
}
