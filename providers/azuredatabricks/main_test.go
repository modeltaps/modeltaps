package azuredatabricks

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/modeltaps/modeltaps/types"
)

func effort(s string) *string { return &s }

func convert(t *testing.T, r *types.ChatCompletionRequest) map[string]any {
	t.Helper()
	r.Messages = []types.ChatCompletionMessage{{Role: types.ChatMessageRoleUser, Content: "hi"}}
	req, errWithCode := (&AzureDatabricksProvider{}).convertRequest(r)
	if errWithCode != nil {
		t.Fatalf("convertRequest: %v", errWithCode)
	}
	raw, _ := json.Marshal(req)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	return body
}

func TestConvertRequestThinking(t *testing.T) {
	cases := []struct {
		name     string
		req      *types.ChatCompletionRequest
		thinking string // JSON of the thinking field, "" when absent
		effort   string
	}{
		{"new Opus: adaptive + effort, no budget", &types.ChatCompletionRequest{Model: "databricks-claude-opus-4-7", MaxTokens: 4096, ReasoningEffort: effort("high")}, `{"type":"adaptive"}`, "high"},
		{"Opus 5.5 via reasoning object", &types.ChatCompletionRequest{Model: "databricks-claude-opus-5-5", MaxTokens: 4096, Reasoning: &types.ChatReasoning{Effort: "xhigh"}}, `{"type":"adaptive"}`, "xhigh"},
		{"Haiku 5.5 caps the effort at high", &types.ChatCompletionRequest{Model: "databricks-claude-haiku-5-5", MaxTokens: 4096, ReasoningEffort: effort("max")}, `{"type":"adaptive"}`, "high"},
		{"Fable takes no thinking parameter", &types.ChatCompletionRequest{Model: "databricks-claude-fable-5-2", MaxTokens: 4096, ReasoningEffort: effort("low")}, "", "low"},
		{"Sonnet 4.6 adaptive, xhigh lowered", &types.ChatCompletionRequest{Model: "databricks-claude-sonnet-4-6", MaxTokens: 4096, ReasoningEffort: effort("xhigh")}, `{"type":"adaptive"}`, "high"},
		{"budget only: adaptive without effort", &types.ChatCompletionRequest{Model: "databricks-claude-opus-4-8", MaxTokens: 4096, Reasoning: &types.ChatReasoning{MaxTokens: 2000}}, `{"type":"adaptive"}`, ""},
		{"older model keeps the budget", &types.ChatCompletionRequest{Model: "databricks-claude-sonnet-4-5", MaxTokens: 4096, ReasoningEffort: effort("medium")}, `{"budget_tokens":2048,"type":"enabled"}`, ""},
		{"no reasoning asked", &types.ChatCompletionRequest{Model: "databricks-claude-opus-5-5", MaxTokens: 4096}, "", ""},
	}
	for _, tc := range cases {
		body := convert(t, tc.req)
		got := ""
		if th, ok := body["thinking"]; ok {
			raw, _ := json.Marshal(th)
			got = string(raw)
		}
		if got != tc.thinking {
			t.Errorf("%s: thinking = %s, want %s", tc.name, got, tc.thinking)
		}
		gotEffort := ""
		if oc, ok := body["output_config"].(map[string]any); ok {
			gotEffort, _ = oc["effort"].(string)
		}
		if gotEffort != tc.effort {
			t.Errorf("%s: effort = %q, want %q", tc.name, gotEffort, tc.effort)
		}
		if tc.thinking == `{"type":"adaptive"}` && body["max_tokens"] != float64(4096) {
			t.Errorf("%s: max_tokens changed to %v", tc.name, body["max_tokens"])
		}
	}
}

func TestSplitContent(t *testing.T) {
	cases := []struct{ raw, text, reasoning string }{
		{`"plain answer"`, "plain answer", ""},
		{`[{"type":"reasoning","summary":[{"type":"summary_text","text":"think ","signature":"s"},{"type":"summary_text","text":"more"}]},{"type":"text","text":"answer"}]`, "answer", "think more"},
		{`null`, "", ""},
		{``, "", ""},
	}
	for _, tc := range cases {
		text, reasoning := splitContent(json.RawMessage(tc.raw))
		if text != tc.text || reasoning != tc.reasoning {
			t.Errorf("splitContent(%s) = %q, %q; want %q, %q", tc.raw, text, reasoning, tc.text, tc.reasoning)
		}
	}
}

func TestConvertResponseWithReasoning(t *testing.T) {
	body := `{"id":"1","object":"chat.completion","created":1,"model":"databricks-claude-sonnet-4-5","choices":[{"index":0,"message":{"role":"assistant","content":[{"type":"reasoning","summary":[{"type":"summary_text","text":"Rayleigh scattering."}]},{"type":"text","text":"Because of scattering."}]},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}`
	resp := &http.Response{Body: io.NopCloser(bytes.NewReader([]byte(body)))}
	got, err := (&AzureDatabricksProvider{}).convertResponse(resp)
	if err != nil {
		t.Fatalf("convertResponse: %v", err)
	}
	m := got.Choices[0].Message
	if m.Content != "Because of scattering." || m.ReasoningContent != "Rayleigh scattering." {
		t.Fatalf("message = %#v", m)
	}
	if got.Usage == nil || got.Usage.TotalTokens != 30 {
		t.Fatalf("usage = %#v", got.Usage)
	}
}

func TestFlattenStreamChunk(t *testing.T) {
	plain := `{"choices":[{"index":0,"delta":{"content":"hi"}}]}`
	if out := string(flattenStreamChunk([]byte(plain))); out != plain {
		t.Errorf("plain chunk changed: %s", out)
	}
	blocks := `{"choices":[{"index":0,"delta":{"content":[{"type":"reasoning","summary":[{"type":"summary_text","text":"why"}]}]}}]}`
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(flattenStreamChunk([]byte(blocks)), &chunk); err != nil {
		t.Fatal(err)
	}
	if d := chunk.Choices[0].Delta; d.Content != "" || d.ReasoningContent != "why" {
		t.Errorf("delta = %#v", d)
	}
	if out := string(flattenStreamChunk([]byte("not json"))); out != "not json" {
		t.Errorf("bad chunk changed: %s", out)
	}
}
