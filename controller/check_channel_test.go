package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/controller/check_channel"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var fakeChallenge = regexp.MustCompile(`Q: (\d+) ([+-]) (\d+) = \?\\nA:`)

// fakeCheckUpstream answers the five check requests the way a well-behaved OpenAI upstream would,
// on both the chat completions and the Responses API, and records the models it was asked for.
type fakeCheckUpstream struct {
	mu     sync.Mutex
	models []string
	paths  []string
}

func (f *fakeCheckUpstream) answer(body string) (text string, tool bool, fail bool) {
	switch {
	case strings.Contains(body, `"user11"`):
		return "", false, true
	case strings.Contains(body, `"multiply"`):
		return "", true, false
	case strings.Contains(body, "math_reasoning"):
		return `{"steps":[{"explanation":"subtract 7, divide by 8","output":"x = -3.75"}],"final_answer":"-3.75"}`, false, false
	case strings.Contains(body, "data:image/png"):
		// The fake cannot read images, so this check fails and the model ends as a warning.
		return "000000", false, false
	}
	// The last question is the real one; the ones before it are few-shot examples.
	all := fakeChallenge.FindAllStringSubmatch(body, -1)
	if len(all) == 0 {
		return "hi", false, false
	}
	m := all[len(all)-1]
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[3])
	if m[2] == "+" {
		return strconv.Itoa(a + b), false, false
	}
	return strconv.Itoa(a - b), false, false
}

func (f *fakeCheckUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var req struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(raw, &req)
	f.mu.Lock()
	f.models = append(f.models, req.Model)
	f.paths = append(f.paths, r.URL.Path)
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	text, tool, fail := f.answer(string(raw))
	if fail {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"message":"Invalid value: 'user11' (request id: 20261014abc)","type":"invalid_request_error"}}`)
		return
	}

	if strings.HasSuffix(r.URL.Path, "/responses") {
		output := []any{map[string]any{
			"type": "message", "role": "assistant", "status": "completed",
			"content": []any{map[string]any{"type": "output_text", "text": text}},
		}}
		if tool {
			output = []any{
				map[string]any{"type": "function_call", "call_id": "c1", "name": "multiply", "arguments": `{"a":3,"b":12}`, "status": "completed"},
				map[string]any{"type": "function_call", "call_id": "c2", "name": "add", "arguments": `{"a":11,"b":49}`, "status": "completed"},
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "resp_1", "object": "response", "model": req.Model, "status": "completed", "output": output,
			"usage": map[string]any{"input_tokens": 20, "output_tokens": 2, "total_tokens": 22},
		})
		return
	}

	message := map[string]any{"role": "assistant", "content": text}
	if tool {
		message["content"] = nil
		message["tool_calls"] = []any{
			map[string]any{"id": "c1", "type": "function", "function": map[string]any{"name": "multiply", "arguments": `{"a":3,"b":12}`}},
			map[string]any{"id": "c2", "type": "function", "function": map[string]any{"name": "add", "arguments": `{"a":11,"b":49}`}},
		}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": "chatcmpl-1", "object": "chat.completion", "model": req.Model,
		"choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": "stop"}},
		"usage":   map[string]any{"prompt_tokens": 20, "completion_tokens": 2, "total_tokens": 22},
	})
}

func callCheckChannel(id int, query string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/channel/check/"+strconv.Itoa(id)+query, nil)
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(id)}}
	CheckChannel(c)
	return w
}

type checkResponse struct {
	Success bool                       `json:"success"`
	Message string                     `json:"message"`
	Data    *check_channel.ModelResult `json:"data"`
}

func decodeCheck(t *testing.T, w *httptest.ResponseRecorder) checkResponse {
	t.Helper()
	var got checkResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode %s: %v", w.Body.String(), err)
	}
	return got
}

func checkItems(result *check_channel.ModelResult) map[string]*check_channel.CheckProcessResult {
	items := map[string]*check_channel.CheckProcessResult{}
	for _, item := range result.Items {
		items[item.Key] = item
	}
	return items
}

func setupCheckChannelTest(t *testing.T) {
	t.Helper()
	setModelInfoModeSwitch(t, false)
	oldLogger := logger.Logger
	logger.Logger = zap.NewNop()
	t.Cleanup(func() { logger.Logger = oldLogger })
	requester.InitHttpClient()
}

func TestCheckChannelRunsAllChecksWithModelMapping(t *testing.T) {
	setupCheckChannelTest(t)
	testDB := setupChannelTestDB(t)

	upstream := &fakeCheckUpstream{}
	server := httptest.NewServer(upstream)
	t.Cleanup(server.Close)

	baseURL := server.URL
	mapping := `{"my-alias":"gpt-4o-mini"}`
	channel := &model.Channel{
		Type:         config.ChannelTypeOpenAI,
		Key:          "sk-test",
		Name:         "fake",
		Status:       config.ChannelStatusEnabled,
		BaseURL:      &baseURL,
		Models:       "my-alias",
		ModelMapping: &mapping,
	}
	if err := testDB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}

	got := decodeCheck(t, callCheckChannel(channel.Id, "?model=my-alias"))
	if !got.Success || got.Data == nil {
		t.Fatalf("check failed: %+v", got)
	}
	if got.Data.Model != "my-alias" {
		t.Fatalf("model = %q", got.Data.Model)
	}
	for _, m := range upstream.models {
		if m != "gpt-4o-mini" {
			t.Fatalf("upstream got model %q, want the mapped gpt-4o-mini (all: %v)", m, upstream.models)
		}
	}
	if len(upstream.models) != 5 {
		t.Fatalf("upstream got %d requests, want 5", len(upstream.models))
	}

	items := checkItems(got.Data)
	want := map[string]int{
		"base":  check_channel.CheckStatusSuccess,
		"image": check_channel.CheckStatusFailed,
		"json":  check_channel.CheckStatusSuccess,
		"tool":  check_channel.CheckStatusSuccess,
		"relay": check_channel.CheckStatusWarning,
	}
	for key, status := range want {
		if items[key] == nil || items[key].Status != status {
			t.Fatalf("%s = %+v, want status %d", key, items[key], status)
		}
	}
	// Only the base check failing fails the model; the failed image check makes it a warning.
	if got.Data.Status != check_channel.CheckStatusWarning {
		t.Fatalf("overall status = %d, want warning", got.Data.Status)
	}
	if items["relay"].Category != "" {
		t.Fatalf("the relay check must not classify its expected error, got %q", items["relay"].Category)
	}
	if items["relay"].Results[0].Code != "request_id_found" {
		t.Fatalf("relay result = %+v", items["relay"].Results[0])
	}
}

func TestCheckChannelUsesResponsesForResponsesModels(t *testing.T) {
	setupCheckChannelTest(t)
	testDB := setupChannelTestDB(t)

	upstream := &fakeCheckUpstream{}
	server := httptest.NewServer(upstream)
	t.Cleanup(server.Close)

	baseURL := server.URL
	need := "gpt-4o-mini"
	channel := &model.Channel{
		Type:                config.ChannelTypeOpenAI,
		Key:                 "sk-test",
		Name:                "responses",
		Status:              config.ChannelStatusEnabled,
		BaseURL:             &baseURL,
		Models:              "gpt-4o-mini",
		Need2ResponseModels: &need,
	}
	if err := testDB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}

	got := decodeCheck(t, callCheckChannel(channel.Id, "?model=gpt-4o-mini"))
	if !got.Success {
		t.Fatalf("check failed: %+v", got)
	}
	for _, p := range upstream.paths {
		if !strings.HasSuffix(p, "/responses") {
			t.Fatalf("request went to %s, want the Responses API (all: %v)", p, upstream.paths)
		}
	}
	items := checkItems(got.Data)
	if items["base"].Status != check_channel.CheckStatusSuccess || items["tool"].Status != check_channel.CheckStatusSuccess {
		t.Fatalf("base = %+v, tool = %+v", items["base"], items["tool"])
	}
}

func TestCheckChannelSkipsRelayCheckOnClaudeConversion(t *testing.T) {
	setupCheckChannelTest(t)
	testDB := setupChannelTestDB(t)

	var mu sync.Mutex
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(raw))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"1"}],"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":1}}`)
	}))
	t.Cleanup(server.Close)

	baseURL := server.URL
	channel := &model.Channel{
		Type:    config.ChannelTypeAnthropic,
		Key:     "sk-ant-test",
		Name:    "claude",
		Status:  config.ChannelStatusEnabled,
		BaseURL: &baseURL,
		Models:  "claude-sonnet-4-5",
	}
	if err := testDB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}

	got := decodeCheck(t, callCheckChannel(channel.Id, "?model=claude-sonnet-4-5"))
	if !got.Success {
		t.Fatalf("check failed: %+v", got)
	}
	if len(bodies) != 4 {
		t.Fatalf("upstream got %d requests, want 4 (the relay check is not sent)", len(bodies))
	}
	for _, b := range bodies {
		if strings.Contains(b, "user11") {
			t.Fatal("the invalid role must not reach the upstream")
		}
	}
	items := checkItems(got.Data)
	if items["relay"].Status != check_channel.CheckStatusSkipped || items["relay"].Results[0].Code != "request_id_role_rewritten" {
		t.Fatalf("relay = %+v", items["relay"])
	}
	if items["json"].Status != check_channel.CheckStatusSkipped {
		t.Fatalf("json = %+v, want skipped on the Claude conversion", items["json"].Results[0])
	}
}

func TestCheckChannelRejectsBadInput(t *testing.T) {
	setupCheckChannelTest(t)
	setupChannelTestDB(t)

	if got := decodeCheck(t, callCheckChannel(1, "")); got.Success || got.Message != check_channel.ErrModelRequired.Error() {
		t.Fatalf("missing model: %+v", got)
	}
	if got := decodeCheck(t, callCheckChannel(404, "?model=gpt-4o")); got.Success {
		t.Fatalf("unknown channel: %+v", got)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/channel/check/abc?model=x", nil)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	CheckChannel(c)
	if got := decodeCheck(t, w); got.Success {
		t.Fatalf("bad id: %+v", got)
	}
}
