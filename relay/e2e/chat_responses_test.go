package e2e

// Chat-to-Responses compatibility path (relayChat.compatibleSend): models in the
// config.DefaultNeed2ResponseModels table arrive on /v1/chat/completions and are sent upstream as
// /v1/responses, then converted back to chat.

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func responsesJSON(modelName, text string) map[string]any {
	return map[string]any{
		"id":     "resp_e2e",
		"object": "response",
		"status": "completed",
		"model":  modelName,
		"output": []any{map[string]any{
			"type":   "message",
			"id":     "msg_e2e",
			"status": "completed",
			"role":   "assistant",
			"content": []any{map[string]any{
				"type": "output_text",
				"text": text,
			}},
		}},
		"usage": map[string]any{"input_tokens": 5, "output_tokens": 3, "total_tokens": 8},
	}
}

func assertChatCompletion(t *testing.T, body []byte, wantContent string) {
	t.Helper()
	var resp struct {
		Object  string `json:"object"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode response: %v, body=%s", err, body)
	}
	if resp.Object != "chat.completion" || len(resp.Choices) != 1 || resp.Choices[0].Message.Content != wantContent {
		t.Fatalf("expected a chat completion with %q, got %s", wantContent, body)
	}
}

// The upstream Responses request must carry the channel-mapped model name, not
// the name the client sent.
func TestChatToResponsesUsesMappedModel(t *testing.T) {
	fake := NewFakeProvider(t)
	fake.SetResponse(FakeResponse{JSON: responsesJSON("o3-pro", "hi from responses")})

	h := NewHarness(t, HarnessOptions{
		Model:  "my-pro",
		Prices: []PriceSpec{{Model: "my-pro", Input: 1, Output: 1}, {Model: "o3-pro", Input: 1, Output: 1}},
		Channels: []ChannelSpec{{
			Name:         "mapped",
			BaseURL:      fake.Server.URL,
			ModelMapping: `{"my-pro":"o3-pro"}`,
		}},
	})

	w := h.Post("/v1/chat/completions", `{"model":"my-pro","messages":[{"role":"user","content":"hi"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d, body=%s", w.Code, w.Body.String())
	}
	assertChatCompletion(t, w.Body.Bytes(), "hi from responses")

	captured := fake.LastRequest(t)
	if captured.Path != "/v1/responses" {
		t.Fatalf("upstream path = %s, want /v1/responses", captured.Path)
	}
	if got := captured.JSONBody(t)["model"]; got != "o3-pro" {
		t.Fatalf("upstream model = %v, want o3-pro", got)
	}
}

// A first attempt on the plain chat branch allows raw passthrough on the shared
// gin.Context. When the retry lands on the Responses path of a passthrough channel,
// the client must still get a chat completion, not raw Responses bytes.
func TestChatToResponsesRetryDoesNotPassThroughRawResponses(t *testing.T) {
	failing := NewFakeProvider(t)
	failing.SetResponse(FakeResponse{
		Status: http.StatusInternalServerError,
		JSON:   OpenAIErrorJSON("upstream exploded", "server_error", "internal_error"),
	})
	responses := NewFakeProvider(t)
	responses.SetResponse(FakeResponse{JSON: responsesJSON("o3-pro", "hi from responses")})

	h := NewHarness(t, HarnessOptions{
		Model:  "my-pro",
		Prices: []PriceSpec{{Model: "my-pro", Input: 1, Output: 1}, {Model: "o3-pro", Input: 1, Output: 1}},
		Channels: []ChannelSpec{
			{Name: "chat", BaseURL: failing.Server.URL, Priority: 10, PassThroughBody: true},
			{
				Name:            "responses",
				BaseURL:         responses.Server.URL,
				Priority:        1,
				ModelMapping:    `{"my-pro":"o3-pro"}`,
				PassThroughBody: true,
			},
		},
	})
	setAndRestore(t, &config.RetryTimes, 3)

	w := h.Post("/v1/chat/completions", `{"model":"my-pro","messages":[{"role":"user","content":"hi"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 after retry, got %d, body=%s", w.Code, w.Body.String())
	}
	if got := failing.LastRequest(t).Path; got != "/v1/chat/completions" {
		t.Fatalf("first attempt path = %s, want /v1/chat/completions", got)
	}
	if got := responses.LastRequest(t).Path; got != "/v1/responses" {
		t.Fatalf("retry path = %s, want /v1/responses", got)
	}
	assertChatCompletion(t, w.Body.Bytes(), "hi from responses")
}
