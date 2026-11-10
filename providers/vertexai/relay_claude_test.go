package vertexai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/cache"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const testProjectID = "my-proj-123"

var testChannelSeq atomic.Int32

type vertexUpstream struct {
	path    string
	headers http.Header
	body    []byte
}

// newVertexClaudeProvider points a Vertex AI provider at a fake upstream through a custom base_url
// template: the six placeholders land in the URL path so a test can assert on all of them.
func newVertexClaudeProvider(t *testing.T, rawBody string, clientHeaders http.Header, status int, contentType, response string) (*VertexAIProvider, *vertexUpstream, *gin.Context) {
	t.Helper()
	seen := &vertexUpstream{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.path = r.URL.Path
		seen.headers = r.Header.Clone()
		seen.body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(srv.Close)

	oldLogger := logger.Logger
	logger.Logger = zap.NewNop()
	t.Cleanup(func() { logger.Logger = oldLogger })
	requester.InitHttpClient()
	cache.InitCacheManager()

	channelID := 91000 + int(testChannelSeq.Add(1))
	// Seed the OAuth token so no call to Google is made.
	if err := cache.SetCache(fmt.Sprintf("%s:%d", TokenCacheKey, channelID), "test-token", time.Hour); err != nil {
		t.Fatal(err)
	}

	baseURL := srv.URL + "/%s/%s/%s/%s/%s/%s"
	other := "us-east5|" + testProjectID
	channel := &model.Channel{Id: channelID, Type: config.ChannelTypeVertexAI, Key: "{}", Other: other, BaseURL: &baseURL}
	provider := VertexAIProviderFactory{}.Create(channel).(*VertexAIProvider)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/claude/v1/messages", bytes.NewReader([]byte(rawBody)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	for name, values := range clientHeaders {
		for _, v := range values {
			ctx.Request.Header.Add(name, v)
		}
	}
	provider.SetContext(ctx)
	provider.SetUsage(&types.Usage{})
	return provider, seen, ctx
}

func claudeRequestFromBody(t *testing.T, raw string) *claude.ClaudeRequest {
	t.Helper()
	request := &claude.ClaudeRequest{}
	if err := json.Unmarshal([]byte(raw), request); err != nil {
		t.Fatal(err)
	}
	return request
}

const nativeRequestBody = `{"model":"claude-sonnet-4-5-20250929","max_tokens":256,"speed":"fast","stream":false,` +
	`"system":[{"type":"text","text":"be brief","cache_control":{"type":"ephemeral"}}],` +
	`"messages":[{"role":"user","content":"hi"}],"output_config":{"effort":"low"}}`

const nativeResponseBody = `{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-5@20250929",` +
	`"content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn",` +
	`"usage":{"input_tokens":11,"output_tokens":5,"cache_read_input_tokens":3}}`

func TestCreateClaudeChatPassesBodyThroughToAnthropicPublisher(t *testing.T) {
	provider, seen, ctx := newVertexClaudeProvider(t, nativeRequestBody,
		http.Header{"Anthropic-Beta": {"context-1m-2025-08-07,claude-code-20250219"}},
		http.StatusOK, "application/json", nativeResponseBody)

	response, errWithCode := provider.CreateClaudeChat(claudeRequestFromBody(t, nativeRequestBody))
	if errWithCode != nil {
		t.Fatal(errWithCode.Message)
	}

	// Host, project, region, publisher, model (date suffix converted) and action.
	wantPath := "/us-east5-aiplatform.googleapis.com/" + testProjectID + "/us-east5/anthropic/claude-sonnet-4-5@20250929/rawPredict"
	if seen.path != wantPath {
		t.Fatalf("path = %q, want %q", seen.path, wantPath)
	}
	if got := seen.headers.Get("Authorization"); got != "Bearer test-token" {
		t.Fatalf("Authorization = %q", got)
	}
	// The unsupported token is dropped, the supported one is forwarded.
	if got := seen.headers.Get("Anthropic-Beta"); got != "context-1m-2025-08-07" {
		t.Fatalf("anthropic-beta = %q", got)
	}

	body := string(seen.body)
	if gjson.Get(body, "model").Exists() {
		t.Fatalf("model must not be sent in the body: %s", body)
	}
	if got := gjson.Get(body, "anthropic_version").String(); got != "vertex-2023-10-16" {
		t.Fatalf("anthropic_version = %q", got)
	}
	// Fields the struct does not model survive byte pass-through.
	if !gjson.Get(body, "system.0.cache_control.type").Exists() || gjson.Get(body, "output_config.effort").String() != "low" {
		t.Fatalf("unknown fields were lost: %s", body)
	}
	// speed=fast bills 2x, so it is stripped by default on every native Claude channel.
	if gjson.Get(body, "speed").Exists() {
		t.Fatalf("speed must be stripped: %s", body)
	}

	usage := provider.GetUsage()
	if usage.PromptTokens == 0 || usage.CompletionTokens != 5 {
		t.Fatalf("usage not recorded: %+v", usage)
	}
	if response.Model != "claude-sonnet-4-5@20250929" {
		t.Fatalf("model = %q", response.Model)
	}
	raw, ok := ctx.Get(config.GinRawResponseBodyKey)
	if !ok || string(raw.([]byte)) != nativeResponseBody {
		t.Fatalf("raw response was not stored for pass-through: %v", raw)
	}
}

func TestCreateClaudeChatKeepsExplicitBetaHeader(t *testing.T) {
	provider, seen, _ := newVertexClaudeProvider(t, nativeRequestBody,
		http.Header{"Anthropic-Beta": {"context-1m-2025-08-07"}},
		http.StatusOK, "application/json", nativeResponseBody)
	pinned := `{"anthropic-beta":"claude-code-20250219"}`
	provider.Channel.ModelHeaders = &pinned

	if _, errWithCode := provider.CreateClaudeChat(claudeRequestFromBody(t, nativeRequestBody)); errWithCode != nil {
		t.Fatal(errWithCode.Message)
	}
	if got := seen.headers.Get("Anthropic-Beta"); got != "claude-code-20250219" {
		t.Fatalf("an administrator-pinned anthropic-beta must be sent as is, got %q", got)
	}
}

func TestCreateClaudeChatStream(t *testing.T) {
	sse := "event: message_start\n" +
		`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-5@20250929","content":[],"usage":{"input_tokens":20,"output_tokens":1}}}` + "\n\n" +
		"event: content_block_delta\n" +
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}` + "\n\n" +
		"event: message_delta\n" +
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":9}}` + "\n\n" +
		"event: message_stop\n" +
		`data: {"type":"message_stop"}` + "\n\n"
	streamBody := strings.Replace(nativeRequestBody, `"stream":false`, `"stream":true`, 1)
	provider, seen, _ := newVertexClaudeProvider(t, streamBody, nil, http.StatusOK, "text/event-stream", sse)

	stream, errWithCode := provider.CreateClaudeChatStream(claudeRequestFromBody(t, streamBody))
	if errWithCode != nil {
		t.Fatal(errWithCode.Message)
	}
	dataChan, errChan := stream.Recv()
	var received strings.Builder
loop:
	for {
		select {
		case line, ok := <-dataChan:
			if !ok {
				break loop
			}
			received.WriteString(line)
		case err := <-errChan:
			if err != nil && err != io.EOF {
				t.Fatalf("stream error: %v", err)
			}
			break loop
		case <-time.After(5 * time.Second):
			t.Fatal("timed out reading the stream")
		}
	}

	if !strings.HasSuffix(seen.path, "/anthropic/claude-sonnet-4-5@20250929/streamRawPredict") {
		t.Fatalf("path = %q", seen.path)
	}
	if got := seen.headers.Get("Accept"); got != "text/event-stream" {
		t.Fatalf("Accept = %q", got)
	}
	if !strings.Contains(received.String(), `"text":"hello"`) {
		t.Fatalf("stream content missing: %q", received.String())
	}
	usage := provider.GetUsage()
	if usage.CompletionTokens != 9 {
		t.Fatalf("stream usage = %+v", usage)
	}
}

func TestCreateClaudeChatRedactsProjectIDInErrors(t *testing.T) {
	cases := map[string]string{
		"google shaped": `{"error":{"code":404,"message":"Publisher Model ` + "`projects/" + testProjectID + "/locations/us-east5/publishers/anthropic/models/x`" + ` was not found","status":"NOT_FOUND"}}`,
		"claude shaped": `{"type":"error","error":{"type":"not_found_error","message":"model not found in project ` + testProjectID + `"}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			provider, _, _ := newVertexClaudeProvider(t, nativeRequestBody, nil, http.StatusNotFound, "application/json", body)

			_, errWithCode := provider.CreateClaudeChat(claudeRequestFromBody(t, nativeRequestBody))
			if errWithCode == nil {
				t.Fatal("expected an error")
			}
			if strings.Contains(errWithCode.Message, testProjectID) {
				t.Fatalf("project id leaked: %s", errWithCode.Message)
			}
			if !strings.Contains(errWithCode.Message, "xxxxx") || !strings.Contains(errWithCode.Message, "not") {
				t.Fatalf("the upstream message should be surfaced, got %q", errWithCode.Message)
			}
		})
	}
}

func TestCreateClaudeChatRejectsUnusableBaseURL(t *testing.T) {
	provider, seen, _ := newVertexClaudeProvider(t, nativeRequestBody, nil, http.StatusOK, "application/json", nativeResponseBody)
	// A legacy five-placeholder template cannot address the anthropic publisher.
	legacy := "http://127.0.0.1:1/%s/%s/%s/%s/%s"
	provider.Channel.BaseURL = &legacy

	_, errWithCode := provider.CreateClaudeChat(claudeRequestFromBody(t, nativeRequestBody))
	if errWithCode == nil {
		t.Fatal("expected invalid_vertexai_config")
	}
	if seen.path != "" {
		t.Fatalf("no request may be sent, got %q", seen.path)
	}
}

func TestPatchClaudeRequestBodyFollowsThinkingConstraints(t *testing.T) {
	raw := `{"model":"claude-sonnet-4-5-20250929","max_tokens":100,"thinking":{"type":"enabled","budget_tokens":2000},` +
		`"messages":[{"role":"user","content":"hi"}]}`
	provider, _, _ := newVertexClaudeProvider(t, raw, nil, http.StatusOK, "application/json", nativeResponseBody)

	request := claudeRequestFromBody(t, raw)
	request.MaxTokens = 6000 // raised by the thinking constraints
	request.Thinking = nil   // cleared by the tool_choice constraint

	patched, ok := provider.patchClaudeRequestBody(request)
	if !ok {
		t.Fatal("pass-through must be available")
	}
	if got := gjson.GetBytes(patched, "max_tokens").Int(); got != 6000 {
		t.Fatalf("max_tokens = %d, want 6000", got)
	}
	if gjson.GetBytes(patched, "thinking").Exists() {
		t.Fatalf("thinking must be removed: %s", patched)
	}
}

func TestFilterVertexBetaTokens(t *testing.T) {
	got := filterVertexBetaTokens([]string{
		"context-1m-2025-08-07, claude-code-20250219 ,oauth-2025-04-20",
		"context-1m-2025-08-07,interleaved-thinking-2025-05-14",
		"",
	})
	if want := "context-1m-2025-08-07,interleaved-thinking-2025-05-14"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := filterVertexBetaTokens([]string{"claude-code-20250219"}); got != "" {
		t.Fatalf("a fully denied value must disappear, got %q", got)
	}
}

func TestApplyClaudeBetaHeaderFiltersPassThroughRules(t *testing.T) {
	provider, _, _ := newVertexClaudeProvider(t, nativeRequestBody, nil, http.StatusOK, "application/json", nativeResponseBody)
	// A "*" pass-through rule may already have copied the client's value into the headers.
	headers := map[string]string{"Anthropic-Beta": "claude-code-20250219,context-1m-2025-08-07"}
	provider.applyClaudeBetaHeader(headers)
	if got := headers["anthropic-beta"]; got != "context-1m-2025-08-07" {
		t.Fatalf("anthropic-beta = %q", got)
	}
	if _, stale := headers["Anthropic-Beta"]; stale {
		t.Fatal("the unfiltered header must be removed")
	}
}
