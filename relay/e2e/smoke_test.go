package e2e

// 骨架冒烟：证明装配 helper + fake provider 能跑通一次真实的非流式 relay。
// 精确计费断言留给后续场景任务，这里只验证「200 + 响应体完整 + 扣费发生」。

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestSmokeChatCompletionsNonStream(t *testing.T) {
	fake := NewFakeProvider(t)
	fake.SetResponse(FakeResponse{
		JSON: ChatCompletionJSON("gpt-4o-mini", "hello from fake", 11, 7),
	})

	h := NewHarness(t, HarnessOptions{FakeBaseURL: fake.Server.URL})

	quotaBefore := h.UserQuota(t)

	w := h.Post("/v1/chat/completions", `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v, body=%s", err, w.Body.String())
	}

	if resp.ID == "" || resp.Object != "chat.completion" || resp.Model != "gpt-4o-mini" {
		t.Fatalf("响应体元信息不完整: %+v", resp)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "hello from fake" {
		t.Fatalf("响应 choices 不完整: %+v", resp.Choices)
	}
	if resp.Usage.CompletionTokens != 7 || resp.Usage.TotalTokens != 18 {
		t.Fatalf("响应 usage 未透传: %+v", resp.Usage)
	}

	// 请求捕获：上游收到的是 chat completions 路径与映射后的模型名。
	captured := fake.LastRequest(t)
	if captured.Path != "/v1/chat/completions" {
		t.Fatalf("上游收到的路径不符: %s", captured.Path)
	}
	if got := captured.Header.Get("Authorization"); got != "Bearer sk-e2e-fake-key" {
		t.Fatalf("上游未收到渠道 key: %q", got)
	}
	if got := captured.JSONBody(t)["model"]; got != "gpt-4o-mini" {
		t.Fatalf("上游收到的 model 不符: %v", got)
	}

	if quotaAfter := h.UserQuota(t); quotaAfter >= quotaBefore {
		t.Fatalf("扣费未发生: before=%d after=%d", quotaBefore, quotaAfter)
	}
}

// 证明 fake provider 的 SSE 能力可被 relay 正常解析并转发到客户端。
func TestSmokeChatCompletionsStream(t *testing.T) {
	fake := NewFakeProvider(t)
	fake.SetResponse(FakeResponse{
		Stream: ChatStreamChunks("gpt-4o-mini", []string{"he", "llo"}, 11, 2),
	})

	h := NewHarness(t, HarnessOptions{FakeBaseURL: fake.Server.URL})
	quotaBefore := h.UserQuota(t)

	w := h.Post("/v1/chat/completions", `{"model":"gpt-4o-mini","stream":true,"messages":[{"role":"user","content":"hi"}]}`)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `"he"`) || !strings.Contains(body, `"llo"`) {
		t.Fatalf("流式分片未转发: %s", body)
	}
	if !strings.HasSuffix(body, "data: [DONE]\n\n") {
		t.Fatalf("流未以 [DONE] 结束: %s", body)
	}
	if quotaAfter := h.UserQuota(t); quotaAfter >= quotaBefore {
		t.Fatalf("流式扣费未发生: before=%d after=%d", quotaBefore, quotaAfter)
	}
}

// 证明 fake provider 的错误注入能力：上游 4xx 会被 relay 透出为非 200。
func TestSmokeUpstreamErrorInjection(t *testing.T) {
	fake := NewFakeProvider(t)
	fake.SetResponse(FakeResponse{
		Status: http.StatusTooManyRequests,
		JSON:   OpenAIErrorJSON("slow down", "rate_limit_error", "rate_limit_exceeded"),
	})

	h := NewHarness(t, HarnessOptions{FakeBaseURL: fake.Server.URL})
	quotaBefore := h.UserQuota(t)

	w := h.Post("/v1/chat/completions", `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`)

	if w.Code == http.StatusOK {
		t.Fatalf("上游 429 时不应返回 200: %s", w.Body.String())
	}
	if quotaAfter := h.UserQuota(t); quotaAfter != quotaBefore {
		t.Fatalf("上游失败应回退预扣: before=%d after=%d", quotaBefore, quotaAfter)
	}
}
