package openrouter

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const claudeCodeStyleBody = `{"model":"claude-sonnet-4-5","max_tokens":1024,` +
	`"system":[{"type":"text","text":"You are Claude Code.","cache_control":{"type":"ephemeral"}}],` +
	`"tools":[{"name":"Bash","description":"run","input_schema":{"type":"object"},"cache_control":{"type":"ephemeral"}}],` +
	`"messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral"}}]}]}`

// TestOpenRouterClaudeNativePassThrough 核验 Claude 格式请求直达 OpenRouter 的 Anthropic 兼容入口：
// 路径 /api/v1/messages、Bearer 鉴权（不能带 x-api-key）、客户端的 cache_control 断点逐字保留、
// 模型名按 model_mapping 改写、响应里的缓存用量进入结算。
func TestOpenRouterClaudeNativePassThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}

	var gotPath, gotAuth, gotAPIKey string
	var gotBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth, gotAPIKey = r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("x-api-key")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"anthropic/claude-sonnet-4-5",` +
			`"content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn",` +
			`"usage":{"input_tokens":10,"output_tokens":2,"cache_creation_input_tokens":900,"cache_read_input_tokens":4000}}`))
	}))
	defer upstream.Close()

	baseURL := upstream.URL + "/api"
	channel := &model.Channel{Type: config.ChannelTypeOpenRouter, Key: "sk-or-test", BaseURL: &baseURL}
	p := OpenRouterProviderFactory{}.Create(channel).(*OpenRouterProvider)

	body := []byte(claudeCodeStyleBody)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")
	c.Set(config.GinRequestBodyKey, body)
	p.SetContext(c)
	p.SetUsage(&types.Usage{})

	req := &claude.ClaudeRequest{}
	if err := json.Unmarshal(body, req); err != nil {
		t.Fatal(err)
	}
	// 模拟 model_mapping：对外名 → OpenRouter 上游名
	req.Model = "anthropic/claude-sonnet-4-5"

	resp, errWithCode := p.CreateClaudeChat(req)
	if errWithCode != nil {
		t.Fatalf("CreateClaudeChat error: %+v", errWithCode)
	}
	if gotPath != "/api/v1/messages" {
		t.Errorf("path = %q, want /api/v1/messages", gotPath)
	}
	if gotAuth != "Bearer sk-or-test" {
		t.Errorf("Authorization = %q, want Bearer sk-or-test", gotAuth)
	}
	if gotAPIKey != "" {
		t.Errorf("x-api-key must not be sent to OpenRouter, got %q", gotAPIKey)
	}
	if gjson.GetBytes(gotBody, "model").String() != "anthropic/claude-sonnet-4-5" {
		t.Errorf("upstream model = %s", gjson.GetBytes(gotBody, "model").String())
	}
	for _, path := range []string{"system.0.cache_control.type", "tools.0.cache_control.type", "messages.0.content.0.cache_control.type"} {
		if got := gjson.GetBytes(gotBody, path).String(); got != "ephemeral" {
			t.Errorf("%s = %q, want ephemeral (cache_control dropped)", path, got)
		}
	}
	if resp == nil || len(resp.Content) != 1 {
		t.Fatalf("resp = %+v", resp)
	}
	usage := p.GetUsage()
	if usage.PromptTokensDetails.CachedReadTokens != 4000 || usage.PromptTokensDetails.CachedWriteTokens != 900 {
		t.Errorf("cache usage not carried: %+v", usage.PromptTokensDetails)
	}
}
