package vertexai

import (
	"io"
	"net/http"
	"testing"

	"github.com/tidwall/gjson"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/types"
)

const plainNativeBody = `{"model":"claude-sonnet-4-5-20250929","max_tokens":256,"system":"be brief","messages":[{"role":"user","content":"hi"}]}`

// A key or request set to "on" gets breakpoints on Claude over Vertex AI, natively and from chat.
func TestVertexClaudeGetsBreakpointsWhenModeIsOn(t *testing.T) {
	old := config.ClaudePromptCachingEnabled
	config.ClaudePromptCachingEnabled = false
	t.Cleanup(func() { config.ClaudePromptCachingEnabled = old })
	provider, seen, ctx := newVertexClaudeProvider(t, plainNativeBody, nil, http.StatusOK, "application/json", nativeResponseBody)
	ctx.Set(config.GinPromptCacheModeKey, config.PromptCacheModeOn)
	if _, errWithCode := provider.CreateClaudeChat(claudeRequestFromBody(t, plainNativeBody)); errWithCode != nil {
		t.Fatal(errWithCode.Message)
	}
	if got := gjson.GetBytes(seen.body, "system.0.cache_control.type").String(); got != "ephemeral" {
		t.Fatalf("native body has no breakpoint: %s", seen.body)
	}
	if seen.headers.Get("Anthropic-Beta") != "" {
		t.Fatalf("no beta header expected, got %q", seen.headers.Get("Anthropic-Beta"))
	}
	if !ctx.GetBool("prompt_caching_injected") {
		t.Fatal("injection not recorded")
	}

	chat, _, chatCtx := newVertexClaudeProvider(t, `{}`, nil, http.StatusOK, "application/json", nativeResponseBody)
	chatCtx.Set(config.GinPromptCacheModeKey, config.PromptCacheModeOn)
	chat.Category = nil
	request := &types.ChatCompletionRequest{Model: "claude-sonnet-4-5-20250929", MaxTokens: 64, Messages: []types.ChatCompletionMessage{
		{Role: "system", Content: "be brief"}, {Role: "user", Content: "hi"}}}
	req, errWithCode := chat.getChatRequest(request)
	if errWithCode != nil {
		t.Fatal(errWithCode.Message)
	}
	body := readBody(t, req)
	if got := gjson.Get(body, "system.0.cache_control.type").String(); got != "ephemeral" {
		t.Fatalf("chat body has no breakpoint: %s", body)
	}
}

func TestVertexClaudeLeftAloneWhenAuto(t *testing.T) {
	old := config.ClaudePromptCachingEnabled
	config.ClaudePromptCachingEnabled = false
	t.Cleanup(func() { config.ClaudePromptCachingEnabled = old })
	provider, seen, _ := newVertexClaudeProvider(t, plainNativeBody, nil, http.StatusOK, "application/json", nativeResponseBody)
	if _, errWithCode := provider.CreateClaudeChat(claudeRequestFromBody(t, plainNativeBody)); errWithCode != nil {
		t.Fatal(errWithCode.Message)
	}
	if gjson.GetBytes(seen.body, "system.0.cache_control").Exists() || gjson.GetBytes(seen.body, "system").Type != gjson.String {
		t.Fatalf("auto with the site switch off must not add breakpoints: %s", seen.body)
	}
}

func readBody(t *testing.T, req *http.Request) string {
	t.Helper()
	data, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
