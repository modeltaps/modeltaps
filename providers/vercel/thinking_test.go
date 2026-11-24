package vercel

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/tidwall/gjson"
)

func TestWithThinkingDisplay(t *testing.T) {
	cases := []struct {
		name, body string
		added      bool
	}{
		{"effort on a new model", `{"model":"anthropic/claude-opus-5.5","reasoning_effort":"low"}`, true},
		{"reasoning object", `{"model":"anthropic/claude-sonnet-5.5","reasoning":{"max_tokens":2000}}`, true},
		{"no reasoning asked", `{"model":"anthropic/claude-opus-5.5"}`, false},
		{"model shows thinking already", `{"model":"anthropic/claude-sonnet-4.6","reasoning_effort":"low"}`, false},
		{"not Claude", `{"model":"openai/gpt-5.5","reasoning_effort":"low"}`, false},
		{"client set its own", `{"model":"anthropic/claude-opus-5.5","reasoning_effort":"low","providerOptions":{"anthropic":{"thinking":{"type":"adaptive"}}}}`, false},
		{"not JSON", `nope`, false},
	}
	for _, tc := range cases {
		out := withThinkingDisplay([]byte(tc.body))
		got := gjson.GetBytes(out, "providerOptions.anthropic.thinking")
		if tc.added {
			if got.Get("type").String() != "adaptive" || got.Get("display").String() != "summarized" {
				t.Errorf("%s: thinking = %s, want adaptive + summarized", tc.name, got.Raw)
			}
		} else if string(out) != tc.body {
			t.Errorf("%s: body changed to %s", tc.name, out)
		}
	}
}

func TestWithThinkingDisplayKeepsGatewayOptions(t *testing.T) {
	out := withThinkingDisplay([]byte(`{"model":"anthropic/claude-opus-5.5","reasoning_effort":"low","providerOptions":{"gateway":{"order":["anthropic"]}}}`))
	if gjson.GetBytes(out, "providerOptions.gateway.order.0").String() != "anthropic" {
		t.Fatalf("gateway order lost: %s", out)
	}
	if gjson.GetBytes(out, "providerOptions.anthropic.thinking.display").String() != "summarized" {
		t.Fatalf("display missing: %s", out)
	}
}

func TestThinkingDisplayHookOnlyOnChat(t *testing.T) {
	body := `{"model":"anthropic/claude-opus-5.5","reasoning_effort":"low"}`
	for path, changed := range map[string]bool{"/v1/chat/completions": true, "/v1/messages": false, "/v1/responses": false} {
		req, _ := http.NewRequest(http.MethodPost, "https://ai-gateway.vercel.sh"+path, bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		if err := chainHooks(nil, thinkingDisplayHook)(req); err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(req.Body)
		if (string(got) != body) != changed {
			t.Errorf("%s: changed = %v, want %v (%s)", path, string(got) != body, changed, got)
		}
		if int(req.ContentLength) != len(got) {
			t.Errorf("%s: ContentLength %d, body %d", path, req.ContentLength, len(got))
		}
	}
}
