package openai

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"gorm.io/datatypes"
)

func customChannel(baseURL string, upstream map[string]interface{}) *model.Channel {
	channel := &model.Channel{Type: config.ChannelTypeCustom, Key: "sk-custom", BaseURL: &baseURL}
	if upstream != nil {
		data := datatypes.NewJSONType(model.PluginType{"upstream_protocol": upstream})
		channel.Plugin = &data
	}
	return channel
}

func TestGetCustomAnthropicSettings(t *testing.T) {
	tests := []struct {
		name    string
		channel *model.Channel
		want    CustomAnthropicSettings
	}{
		{"no plugin", customChannel("https://x", nil),
			CustomAnthropicSettings{Mode: UpstreamProtocolOpenAI, MessagesPath: "/v1/messages"}},
		{"unknown mode falls back to openai", customChannel("https://x", map[string]interface{}{"mode": "grpc"}),
			CustomAnthropicSettings{Mode: UpstreamProtocolOpenAI, MessagesPath: "/v1/messages"}},
		{"claude models over anthropic", customChannel("https://x", map[string]interface{}{"mode": "anthropic_claude"}),
			CustomAnthropicSettings{Mode: UpstreamProtocolAnthropicClaude, MessagesPath: "/v1/messages"}},
		{"all anthropic, path and bearer", customChannel("https://x", map[string]interface{}{
			"mode": "anthropic", "messages_path": "anthropic/v1/messages", "auth": "Bearer"}),
			CustomAnthropicSettings{Mode: UpstreamProtocolAnthropic, MessagesPath: "/anthropic/v1/messages", BearerAuth: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetCustomAnthropicSettings(tt.channel); got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}

	// Only the custom channel type reads the plugin.
	other := customChannel("https://x", map[string]interface{}{"mode": "anthropic"})
	other.Type = config.ChannelTypeAzure
	if GetCustomAnthropicSettings(other).Mode != UpstreamProtocolOpenAI {
		t.Fatal("a non-custom channel must keep the OpenAI mode")
	}
}

func TestCustomUsesAnthropic(t *testing.T) {
	claudeOnly := customChannel("https://x", map[string]interface{}{"mode": "anthropic_claude"})
	all := customChannel("https://x", map[string]interface{}{"mode": "anthropic"})
	openai := customChannel("https://x", nil)

	cases := []struct {
		channel *model.Channel
		model   string
		want    bool
	}{
		{claudeOnly, "claude-sonnet-4-5", true},
		{claudeOnly, "anthropic/claude-opus-5", true},
		{claudeOnly, "gpt-5.5", false},
		{all, "glm-4.6", true},
		{openai, "claude-sonnet-4-5", false},
	}
	for _, tc := range cases {
		if got := CustomUsesAnthropic(tc.channel, tc.model); got != tc.want {
			t.Errorf("mode=%s model=%s: got %v, want %v",
				GetCustomAnthropicSettings(tc.channel).Mode, tc.model, got, tc.want)
		}
	}
}

type recordedRequest struct {
	path, auth, apiKey string
	body               []byte
}

func newCustomUpstream(t *testing.T, got *recordedRequest) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path, got.auth, got.apiKey = r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("x-api-key")
		got.body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/messages") {
			_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-5",` +
				`"content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn",` +
				`"usage":{"input_tokens":10,"output_tokens":2,"cache_creation_input_tokens":900,"cache_read_input_tokens":4000}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion","model":"gpt-5.5",` +
			`"choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],` +
			`"usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}}`))
	}))
}

func newCustomProvider(t *testing.T, channel *model.Channel, path string, body []byte) *OpenAIProvider {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	p := CreateOpenAIProvider(channel, channel.GetBaseURL())
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(config.GinRequestBodyKey, body)
	p.SetContext(c)
	p.SetUsage(&types.Usage{})
	return p
}

// A Claude model requested over the OpenAI chat protocol on a custom channel that declares the
// Anthropic protocol for Claude models goes to the channel's Messages endpoint, and the cache
// usage it reports reaches billing.
func TestCustomChannelChatGoesToMessagesForClaude(t *testing.T) {
	var got recordedRequest
	upstream := newCustomUpstream(t, &got)
	defer upstream.Close()

	channel := customChannel(upstream.URL, map[string]interface{}{"mode": "anthropic_claude"})
	body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`)
	p := newCustomProvider(t, channel, "/v1/chat/completions", body)

	req := &types.ChatCompletionRequest{}
	if err := json.Unmarshal(body, req); err != nil {
		t.Fatal(err)
	}
	resp, errWithCode := p.CreateChatCompletion(req)
	if errWithCode != nil {
		t.Fatalf("CreateChatCompletion error: %+v", errWithCode)
	}
	if got.path != "/v1/messages" {
		t.Errorf("path = %q, want /v1/messages", got.path)
	}
	if got.apiKey != "sk-custom" || got.auth != "" {
		t.Errorf("auth headers = x-api-key %q / Authorization %q, want x-api-key only", got.apiKey, got.auth)
	}
	if gjson.GetBytes(got.body, "messages.0.role").String() != "user" || !gjson.GetBytes(got.body, "max_tokens").Exists() {
		t.Errorf("upstream body is not an Anthropic Messages request: %s", got.body)
	}
	if resp == nil || len(resp.Choices) != 1 || resp.Choices[0].Message.StringContent() != "hello" {
		t.Fatalf("resp = %+v", resp)
	}
	usage := p.GetUsage()
	if usage.PromptTokensDetails.CachedTokens == 0 && usage.PromptTokensDetails.CachedReadTokens == 0 {
		t.Errorf("cache read tokens did not reach usage: %+v", usage.PromptTokensDetails)
	}
}

// Other models on the same channel keep the OpenAI endpoint.
func TestCustomChannelChatKeepsOpenAIForOtherModels(t *testing.T) {
	var got recordedRequest
	upstream := newCustomUpstream(t, &got)
	defer upstream.Close()

	channel := customChannel(upstream.URL, map[string]interface{}{"mode": "anthropic_claude"})
	body := []byte(`{"model":"gpt-5.5","messages":[{"role":"user","content":"hi"}]}`)
	p := newCustomProvider(t, channel, "/v1/chat/completions", body)

	req := &types.ChatCompletionRequest{}
	if err := json.Unmarshal(body, req); err != nil {
		t.Fatal(err)
	}
	if _, errWithCode := p.CreateChatCompletion(req); errWithCode != nil {
		t.Fatalf("CreateChatCompletion error: %+v", errWithCode)
	}
	if got.path != "/v1/chat/completions" {
		t.Errorf("path = %q, want /v1/chat/completions", got.path)
	}
	if got.auth != "Bearer sk-custom" {
		t.Errorf("Authorization = %q, want Bearer sk-custom", got.auth)
	}
}

// An Anthropic request forwarded through the delegate keeps the client's cache breakpoints and
// honours the configured path and bearer auth.
func TestCustomClaudeProviderKeepsCacheBreakpoints(t *testing.T) {
	var got recordedRequest
	upstream := newCustomUpstream(t, &got)
	defer upstream.Close()

	channel := customChannel(upstream.URL, map[string]interface{}{
		"mode": "anthropic", "messages_path": "/anthropic/v1/messages", "auth": "bearer"})
	body := []byte(`{"model":"glm-4.6","max_tokens":64,` +
		`"system":[{"type":"text","text":"sys","cache_control":{"type":"ephemeral"}}],` +
		`"messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral"}}]}]}`)
	p := newCustomProvider(t, channel, "/claude/v1/messages", body)

	delegate := p.CustomClaudeProvider("glm-4.6")
	if delegate == nil {
		t.Fatal("expected an Anthropic delegate for mode=anthropic")
	}
	req := &claude.ClaudeRequest{}
	if err := json.Unmarshal(body, req); err != nil {
		t.Fatal(err)
	}
	if _, errWithCode := delegate.CreateClaudeChat(req); errWithCode != nil {
		t.Fatalf("CreateClaudeChat error: %+v", errWithCode)
	}
	if got.path != "/anthropic/v1/messages" {
		t.Errorf("path = %q, want /anthropic/v1/messages", got.path)
	}
	if got.auth != "Bearer sk-custom" || got.apiKey != "" {
		t.Errorf("auth headers = Authorization %q / x-api-key %q, want bearer only", got.auth, got.apiKey)
	}
	for _, path := range []string{"system.0.cache_control.type", "messages.0.content.0.cache_control.type"} {
		if gjson.GetBytes(got.body, path).String() != "ephemeral" {
			t.Errorf("%s dropped: %s", path, got.body)
		}
	}
}

// Providers that embed OpenAIProvider never get a delegate, whatever their plugin says.
func TestCustomClaudeProviderOnlyForCustomChannels(t *testing.T) {
	channel := customChannel("https://x", map[string]interface{}{"mode": "anthropic"})
	channel.Type = config.ChannelTypeOpenAI
	p := CreateOpenAIProvider(channel, "https://x")
	if p.CustomClaudeProvider("claude-sonnet-4-5") != nil {
		t.Fatal("an OpenAI channel must not delegate to Anthropic")
	}
}
