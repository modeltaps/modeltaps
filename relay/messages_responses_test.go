package relay

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// upstreamCall is what the mock upstream received.
type upstreamCall struct {
	path, auth, apiKey string
	body               []byte
}

// newMockUpstream answers every request with reply and records it.
func newMockUpstream(t *testing.T, got *upstreamCall, contentType, reply string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path, got.auth, got.apiKey = r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("x-api-key")
		got.body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(server.Close)
	return server
}

// newEntryContext builds a gin context for a client request to path with body, as the entry
// middleware leaves it (body cached for pass-through).
func newEntryContext(path string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(config.GinRequestBodyKey, body)
	return c, recorder
}

func newClaudeEntryRelay(t *testing.T, channel *model.Channel, body []byte) (*relayClaudeOnly, *httptest.ResponseRecorder) {
	t.Helper()
	c, recorder := newEntryContext("/claude/v1/messages", body)
	r := NewRelayClaudeOnly(c)
	if err := r.setRequest(); err != nil {
		t.Fatal(err)
	}
	r.provider = providers.GetProvider(channel, c)
	r.provider.SetUsage(&types.Usage{})
	r.modelName = r.claudeRequest.Model
	return r, recorder
}

func newResponsesEntryRelay(t *testing.T, channel *model.Channel, body []byte) (*relayResponses, *httptest.ResponseRecorder) {
	t.Helper()
	c, recorder := newEntryContext("/v1/responses", body)
	r := NewRelayResponses(c)
	if err := r.setRequest(); err != nil {
		t.Fatal(err)
	}
	r.provider = providers.GetProvider(channel, c)
	r.provider.SetUsage(&types.Usage{})
	r.modelName = r.responsesRequest.Model
	return r, recorder
}

func upstreamChannel(channelType int, baseURL, key string) *model.Channel {
	return &model.Channel{Id: 7, Type: channelType, Key: key, BaseURL: &baseURL}
}

// The Anthropic entry sends a GPT model on an OpenAI channel to /v1/responses as a Responses
// request (not the client's Messages body), and replies in Messages form with the cache usage.
func TestClaudeEntry_GPTGoesToResponses(t *testing.T) {
	var got upstreamCall
	upstream := newMockUpstream(t, &got, "application/json", `{"id":"resp_1","object":"response","model":"gpt-5",
		"status":"completed","output":[{"type":"message","id":"m","role":"assistant","status":"completed",
		"content":[{"type":"output_text","text":"hello"}]}],
		"usage":{"input_tokens":5000,"output_tokens":7,"total_tokens":5007,"input_tokens_details":{"cached_tokens":4096},
		"output_tokens_details":{"reasoning_tokens":0}}}`)

	body := []byte(`{"model":"gpt-5","max_tokens":256,"system":"be brief",
		"messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral"}}]}]}`)
	r, recorder := newClaudeEntryRelay(t, upstreamChannel(config.ChannelTypeOpenAI, upstream.URL, "sk-openai"), body)

	if err, _ := r.send(); err != nil {
		t.Fatalf("send: %+v", err)
	}
	if got.path != "/v1/responses" {
		t.Fatalf("upstream path = %q, want /v1/responses", got.path)
	}
	if gjson.GetBytes(got.body, "messages").Exists() || gjson.GetBytes(got.body, "instructions").String() != "be brief" ||
		gjson.GetBytes(got.body, "input.0.content.0.text").String() != "hi" ||
		gjson.GetBytes(got.body, "max_output_tokens").Int() != 256 {
		t.Fatalf("upstream body is not the converted Responses request: %s", got.body)
	}

	reply := recorder.Body.Bytes()
	if gjson.GetBytes(reply, "type").String() != "message" || gjson.GetBytes(reply, "content.0.text").String() != "hello" ||
		gjson.GetBytes(reply, "usage.cache_read_input_tokens").Int() != 4096 ||
		gjson.GetBytes(reply, "usage.input_tokens").Int() != 904 {
		t.Fatalf("client reply = %s", reply)
	}
	usage := r.provider.GetUsage()
	if usage.PromptTokens != 5000 || usage.PromptTokensDetails.CachedTokens != 4096 || usage.CompletionTokens != 7 {
		t.Fatalf("billed usage = %+v", usage)
	}
}

const codexStream = "event: response.created\n" +
	`data: {"type":"response.created","response":{"id":"resp_c","model":"gpt-5-codex","status":"in_progress"}}` + "\n\n" +
	"event: response.output_item.added\n" +
	`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"read_file","arguments":""}}` + "\n\n" +
	"event: response.function_call_arguments.delta\n" +
	`data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":"{\"path\":\"a.go\"}"}` + "\n\n" +
	"event: response.output_item.done\n" +
	`data: {"type":"response.output_item.done","output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"read_file","arguments":"{\"path\":\"a.go\"}"}}` + "\n\n" +
	"event: response.completed\n" +
	`data: {"type":"response.completed","response":{"id":"resp_c","model":"gpt-5-codex","status":"completed",` +
	`"usage":{"input_tokens":3000,"output_tokens":9,"total_tokens":3009,"input_tokens_details":{"cached_tokens":2048}}}}` + "\n\n"

// A streamed Anthropic-entry request on a Codex channel goes straight to the Codex Responses
// endpoint and streams back as Messages events, tool call and cache usage included.
func TestClaudeEntry_CodexStreamsOverResponses(t *testing.T) {
	var got upstreamCall
	upstream := newMockUpstream(t, &got, "text/event-stream", codexStream)

	body := []byte(`{"model":"gpt-5-codex","max_tokens":256,"stream":true,
		"tools":[{"name":"read_file","input_schema":{"type":"object"}}],
		"messages":[{"role":"user","content":"read a.go"}]}`)
	r, recorder := newClaudeEntryRelay(t, upstreamChannel(config.ChannelTypeCodex, upstream.URL, "codex-token"), body)

	if err, _ := r.send(); err != nil {
		t.Fatalf("send: %+v", err)
	}
	if !strings.HasSuffix(got.path, "/codex/responses") {
		t.Fatalf("upstream path = %q, want the Codex responses endpoint", got.path)
	}
	if gjson.GetBytes(got.body, "messages").Exists() || gjson.GetBytes(got.body, "tools.0.name").String() != "read_file" ||
		gjson.GetBytes(got.body, "input.0.content.0.text").String() != "read a.go" {
		t.Fatalf("upstream body = %s", got.body)
	}

	out := recorder.Body.String()
	for _, want := range []string{
		"event: message_start", `"type":"tool_use"`, `"id":"call_1"`, `"partial_json":"{\"path\":\"a.go\"}"`,
		`"stop_reason":"tool_use"`, `"cache_read_input_tokens":2048`, "event: message_stop",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stream lacks %s:\n%s", want, out)
		}
	}
	usage := r.provider.GetUsage()
	if usage.PromptTokens != 3000 || usage.PromptTokensDetails.CachedTokens != 2048 || usage.CompletionTokens != 9 {
		t.Fatalf("billed usage = %+v", usage)
	}
}

const claudeMessage = `{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-5",
	"content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn",
	"usage":{"input_tokens":10,"output_tokens":2,"cache_creation_input_tokens":900,"cache_read_input_tokens":4000}}`

// The Responses entry sends a Claude model on an Anthropic channel to /v1/messages as a
// Messages request, and replies in Responses form; cache reads and writes reach billing.
func TestResponsesEntry_ClaudeGoesToMessages(t *testing.T) {
	var got upstreamCall
	upstream := newMockUpstream(t, &got, "application/json", claudeMessage)

	body := []byte(`{"model":"claude-sonnet-4-5","instructions":"be brief","max_output_tokens":128,
		"input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)
	r, recorder := newResponsesEntryRelay(t, upstreamChannel(config.ChannelTypeAnthropic, upstream.URL, "sk-ant"), body)

	if err, _ := r.send(); err != nil {
		t.Fatalf("send: %+v", err)
	}
	if got.path != "/v1/messages" || got.apiKey != "sk-ant" {
		t.Fatalf("upstream path = %q, x-api-key = %q", got.path, got.apiKey)
	}
	// The site-wide prompt cache switch is on by default, so the system prompt arrives as a
	// block with a cache breakpoint.
	if gjson.GetBytes(got.body, "input").Exists() || gjson.GetBytes(got.body, "system.0.text").String() != "be brief" ||
		gjson.GetBytes(got.body, "system.0.cache_control.type").String() != "ephemeral" ||
		gjson.GetBytes(got.body, "messages.0.content.0.text").String() != "hi" ||
		gjson.GetBytes(got.body, "max_tokens").Int() != 128 {
		t.Fatalf("upstream body is not the converted Messages request: %s", got.body)
	}

	reply := recorder.Body.Bytes()
	if gjson.GetBytes(reply, "object").String() != "response" || gjson.GetBytes(reply, "output.0.content.0.text").String() != "hello" ||
		gjson.GetBytes(reply, "usage.input_tokens").Int() != 4910 ||
		gjson.GetBytes(reply, "usage.input_tokens_details.cached_tokens").Int() != 4000 {
		t.Fatalf("client reply = %s", reply)
	}
	usage := r.provider.GetUsage()
	if usage.PromptTokensDetails.CachedReadTokens != 4000 || usage.PromptTokensDetails.CachedWriteTokens != 900 ||
		usage.PromptTokens != 4910 || usage.CompletionTokens != 2 {
		t.Fatalf("billed usage = %+v", usage.PromptTokensDetails)
	}
}

const claudeStream = "event: message_start\n" +
	`data: {"type":"message_start","message":{"id":"msg_s","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[],` +
	`"usage":{"input_tokens":10,"output_tokens":1,"cache_creation_input_tokens":900,"cache_read_input_tokens":4000}}}` + "\n\n" +
	"event: content_block_start\n" +
	`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\n" +
	"event: content_block_delta\n" +
	`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}` + "\n\n" +
	"event: content_block_stop\n" +
	`data: {"type":"content_block_stop","index":0}` + "\n\n" +
	"event: message_delta\n" +
	`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":12}}` + "\n\n" +
	"event: message_stop\n" +
	`data: {"type":"message_stop"}` + "\n\n"

// A streamed Responses-entry request on an Anthropic-protocol custom channel goes to its
// Messages endpoint once and streams back as Responses events with the cache usage.
func TestResponsesEntry_CustomAnthropicStreams(t *testing.T) {
	var got upstreamCall
	upstream := newMockUpstream(t, &got, "text/event-stream", claudeStream)

	channel := withUpstreamProtocol(upstreamChannel(config.ChannelTypeCustom, upstream.URL, "sk-custom"), "anthropic_claude")
	body := []byte(`{"model":"claude-sonnet-4-5","stream":true,"input":"hi"}`)
	r, recorder := newResponsesEntryRelay(t, channel, body)

	if err, _ := r.send(); err != nil {
		t.Fatalf("send: %+v", err)
	}
	if got.path != "/v1/messages" || gjson.GetBytes(got.body, "messages.0.role").String() != "user" {
		t.Fatalf("upstream path = %q body = %s", got.path, got.body)
	}

	out := recorder.Body.String()
	for _, want := range []string{
		"event: response.created", `"delta":"hello"`, "event: response.completed",
		`"cached_tokens":4000`, `"input_tokens":4910`, `"output_tokens":12`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stream lacks %s:\n%s", want, out)
		}
	}
	usage := r.provider.GetUsage()
	if usage.PromptTokensDetails.CachedReadTokens != 4000 || usage.PromptTokensDetails.CachedWriteTokens != 900 || usage.CompletionTokens != 12 {
		t.Fatalf("billed usage = %+v / %+v", usage, usage.PromptTokensDetails)
	}
}

// A Claude Code channel on the Responses entry sends Messages with its OAuth token.
func TestResponsesEntry_ClaudeCodeUsesOAuth(t *testing.T) {
	var got upstreamCall
	upstream := newMockUpstream(t, &got, "application/json", claudeMessage)

	body := []byte(`{"model":"claude-sonnet-4-5","input":"hi"}`)
	r, recorder := newResponsesEntryRelay(t, upstreamChannel(config.ChannelTypeClaudeCode, upstream.URL, "oauth-token"), body)

	if err, _ := r.send(); err != nil {
		t.Fatalf("send: %+v", err)
	}
	if got.path != "/v1/messages" || got.auth != "Bearer oauth-token" || got.apiKey != "" {
		t.Fatalf("upstream path = %q Authorization = %q x-api-key = %q", got.path, got.auth, got.apiKey)
	}
	if !strings.Contains(gjson.GetBytes(got.body, "system").Raw, "Claude Code") {
		t.Errorf("Claude Code system instruction missing: %s", got.body)
	}
	if gjson.GetBytes(recorder.Body.Bytes(), "output.0.content.0.text").String() != "hello" {
		t.Fatalf("client reply = %s", recorder.Body.String())
	}
}

// Non-Claude models and OpenAI-protocol channels keep the Responses (or chat) paths.
func TestResponsesEntryClaudeProvider(t *testing.T) {
	cases := []struct {
		name    string
		channel *model.Channel
		model   string
		want    bool
	}{
		{"anthropic", upstreamChannel(config.ChannelTypeAnthropic, "https://x", "k"), "claude-sonnet-4-5", true},
		{"claude code", upstreamChannel(config.ChannelTypeClaudeCode, "https://x", "k"), "claude-sonnet-4-5", true},
		{"bedrock claude", upstreamChannel(config.ChannelTypeBedrock, "https://x", "a|b|us-east-1"), "claude-sonnet-4-5", true},
		{"vertex gemini", upstreamChannel(config.ChannelTypeVertexAI, "https://x", "k"), "gemini-2.5-pro", false},
		{"custom anthropic", withUpstreamProtocol(upstreamChannel(config.ChannelTypeCustom, "https://x", "k"), "anthropic_claude"), "claude-opus-5", true},
		{"custom openai", upstreamChannel(config.ChannelTypeCustom, "https://x", "k"), "claude-opus-5", false},
		{"openai", upstreamChannel(config.ChannelTypeOpenAI, "https://x", "k"), "gpt-5", false},
	}
	for _, tc := range cases {
		c, _ := newEntryContext("/v1/responses", nil)
		provider := providers.GetProvider(tc.channel, c)
		if got := responsesEntryClaudeProvider(provider, tc.model) != nil; got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestIsResponsesModelName(t *testing.T) {
	for name, want := range map[string]bool{
		"gpt-5": true, "gpt-4o-mini": true, "openai/gpt-5.5": true, "o3": true, "o4-mini": true,
		"codex-mini-latest": true, "claude-sonnet-4-5": false, "omni-moderation": false, "deepseek-chat": false,
	} {
		if got := isResponsesModelName(name); got != want {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
}

// A channel that turns Responses off keeps the chat conversion on the Anthropic entry.
func TestClaudeEntry_CompatibleResponseKeepsChat(t *testing.T) {
	channel := upstreamChannel(config.ChannelTypeOpenAI, "https://x", "k")
	channel.CompatibleResponse = true
	c, _ := newEntryContext("/claude/v1/messages", nil)
	provider := providers.GetProvider(channel, c)
	if got := resolveClaudeRoute(channel.Type, "gpt-5", provider); got != claudeRouteOpenAIConvert {
		t.Fatalf("route = %v, want chat conversion", got)
	}
}
