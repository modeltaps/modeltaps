package e2e

// Coding-agent client endpoints (/claude-code, /codex/v1, /coding-agent): each path serves the
// bare path's protocol through the bare handler chain, so streaming, billing, auth and errors
// must match the bare path, while the model lists come in each client's own shape, the
// Claude Code display IDs are turned back into real model names before the upstream sees them,
// and the consume log records the client.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/agentclient"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
)

const (
	agentClaudeModel = "claude-sonnet-4-6"
	agentCodexModel  = "gpt-5.5"
)

// request sends one request with the seeded key in both credential headers, as Claude Code
// does, plus the given headers.
func (h *Harness) request(method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	return h.requestWithKey(h.Token.Key, method, path, body, headers)
}

func (h *Harness) requestWithKey(key, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	var reader *bytes.Buffer
	if body != "" {
		reader = bytes.NewBufferString(body)
	} else {
		reader = &bytes.Buffer{}
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.Engine.ServeHTTP(w, req)
	return w
}

// seedModelInfo adds catalog rows and reloads the price table, which carries them.
func seedModelInfo(t *testing.T, infos ...*model.ModelInfo) {
	t.Helper()
	for _, info := range infos {
		if info.Status == "" {
			info.Status = model.ModelStatusPublished
		}
		if err := model.DB.Create(info).Error; err != nil {
			t.Fatalf("seed model_info %s: %v", info.Model, err)
		}
	}
	if err := model.PricingInstance.Init(); err != nil {
		t.Fatalf("reload pricing: %v", err)
	}
}

func anthropicMessageJSON(modelName, text string, inputTokens, outputTokens int) map[string]any {
	return map[string]any{
		"id":            "msg_e2e",
		"type":          "message",
		"role":          "assistant",
		"model":         modelName,
		"content":       []any{map[string]any{"type": "text", "text": text}},
		"stop_reason":   "end_turn",
		"stop_sequence": nil,
		"usage":         map[string]any{"input_tokens": inputTokens, "output_tokens": outputTokens},
	}
}

func newAnthropicHarness(t *testing.T, baseURL string) *Harness {
	t.Helper()
	return NewHarness(t, HarnessOptions{
		Model:    agentClaudeModel,
		Channels: []ChannelSpec{{Name: "fake-anthropic", Type: config.ChannelTypeAnthropic, BaseURL: baseURL}},
		Prices:   []PriceSpec{{Model: agentClaudeModel, Input: billingInputRatio, Output: billingOutputRatio}},
	})
}

// claudeCodeBody is a Messages request as Claude Code sends it: a cache_control breakpoint on
// the system block and the session in metadata.user_id.
func claudeCodeBody(modelID string, stream bool) string {
	return `{"model":"` + modelID + `","max_tokens":64,"stream":` + boolJSON(stream) +
		`,"system":[{"type":"text","text":"You are Claude Code.","cache_control":{"type":"ephemeral"}}]` +
		`,"messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral"}}]}]}`
}

func boolJSON(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// /claude-code/v1/messages, non-streaming, on a native Anthropic channel: the [1m] display ID is
// stripped before the upstream sees it, cache_control reaches the upstream unchanged, the
// response is a Messages response, and billing and the log use the real model.
func TestClaudeCodeEndpointNonStream(t *testing.T) {
	const inputTokens, outputTokens = 12, 8
	fake := NewFakeProvider(t)
	fake.SetResponse(FakeResponse{JSON: anthropicMessageJSON(agentClaudeModel, "hello from claude", inputTokens, outputTokens)})
	h := newAnthropicHarness(t, fake.Server.URL)
	before := h.UserQuota(t)

	displayID := agentclient.ClaudeCodeModelID(agentClaudeModel, agentclient.LongContextTokens)
	w := h.request(http.MethodPost, "/claude-code/v1/messages", claudeCodeBody(displayID, false), map[string]string{
		"User-Agent":        "claude-cli/2.1.300 (external, cli)",
		"anthropic-version": "2023-06-01",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Type    string `json:"type"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Type != "message" || len(resp.Content) != 1 || resp.Content[0].Text != "hello from claude" {
		t.Fatalf("not a Messages response: %s", w.Body.String())
	}

	captured := fake.LastRequest(t)
	if captured.Path != "/v1/messages" {
		t.Fatalf("upstream path = %s, want /v1/messages", captured.Path)
	}
	upstream := captured.JSONBody(t)
	if upstream["model"] != agentClaudeModel {
		t.Fatalf("upstream model = %v, want %s (display ID must be stripped)", upstream["model"], agentClaudeModel)
	}
	if strings.Contains(string(captured.Body), "[1m]") || strings.Contains(string(captured.Body), agentclient.ClaudeCodeModelPrefix) {
		t.Fatalf("upstream body still carries the display ID: %s", captured.Body)
	}
	if strings.Count(string(captured.Body), `"cache_control":{"type":"ephemeral"}`) != 2 {
		t.Fatalf("cache_control must reach the upstream unchanged: %s", captured.Body)
	}

	want := expectQuota(inputTokens, outputTokens, 1)
	if got := before - h.UserQuota(t); got != want {
		t.Fatalf("billed %d, want %d", got, want)
	}
	log := soleConsumeLog(t)
	if log.ModelName != agentClaudeModel || log.Quota != want || log.Client != agentclient.ClaudeCode {
		t.Fatalf("log = model %q quota %d client %q, want %s / %d / claude-code", log.ModelName, log.Quota, log.Client, agentClaudeModel, want)
	}
	if _, ok := log.Metadata.Data()[model.LogMetaClient]; ok {
		t.Fatalf("the client belongs in its column, not the metadata: %v", log.Metadata.Data())
	}
}

// /claude-code/v1/messages, streaming, through the Claude-to-OpenAI conversion: Messages events,
// the stripped model upstream, billing from the final usage, the client in the log.
func TestClaudeCodeEndpointStream(t *testing.T) {
	const promptTokens, completionTokens = 13, 9
	fake := NewFakeProvider(t)
	fake.SetResponse(FakeResponse{Stream: ChatStreamChunks(claudeEntryModel, []string{"he", "llo"}, promptTokens, completionTokens)})
	h := newGroqHarness(t, fake.Server.URL, 1)
	before := h.UserQuota(t)

	w := h.request(http.MethodPost, "/claude-code/v1/messages",
		claudeCodeBody(agentclient.ClaudeCodeModelPrefix+claudeEntryModel+agentclient.ClaudeCodeLongContext, true), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d, body=%s", w.Code, w.Body.String())
	}
	for _, event := range []string{"event: message_start", "event: content_block_delta", "event: message_stop"} {
		if !strings.Contains(w.Body.String(), event) {
			t.Fatalf("stream lacks %q: %s", event, w.Body.String())
		}
	}
	if got := fake.LastRequest(t).JSONBody(t)["model"]; got != claudeEntryModel {
		t.Fatalf("upstream model = %v, want %s", got, claudeEntryModel)
	}
	want := expectQuota(promptTokens, completionTokens, 1)
	if got := before - h.UserQuota(t); got != want {
		t.Fatalf("billed %d, want %d", got, want)
	}
	log := soleConsumeLog(t)
	if !log.IsStream || log.ModelName != claudeEntryModel || log.Client != agentclient.ClaudeCode {
		t.Fatalf("log = %+v", log)
	}
}

// The same request on /claude-code and on the bare /claude path bills the same; only the
// client column tells them apart.
func TestClaudeCodeEndpointBillsLikeBarePath(t *testing.T) {
	fake := NewFakeProvider(t)
	fake.SetResponse(FakeResponse{JSON: ChatCompletionJSON(claudeEntryModel, "hi", 11, 7)})
	h := newGroqHarness(t, fake.Server.URL, 1.5)

	before := h.UserQuota(t)
	if w := h.Post("/claude/v1/messages", claudeMessagesBody(false)); w.Code != http.StatusOK {
		t.Fatalf("bare path: %d %s", w.Code, w.Body.String())
	}
	mid := h.UserQuota(t)
	if w := h.Post("/claude-code/v1/messages", claudeMessagesBody(false)); w.Code != http.StatusOK {
		t.Fatalf("client path: %d %s", w.Code, w.Body.String())
	}
	if bare, client := before-mid, mid-h.UserQuota(t); bare != client || bare == 0 {
		t.Fatalf("bare path billed %d, client path %d", bare, client)
	}
	logs := consumeLogs(t)
	if len(logs) != 2 || logs[0].Client != "" || logs[1].Client != agentclient.ClaudeCode || logs[0].Quota != logs[1].Quota {
		t.Fatalf("logs = %+v", logs)
	}
}

// Authentication and its errors are those of the bare path.
func TestAgentEndpointsAuthMatchesBarePath(t *testing.T) {
	fake := NewFakeProvider(t)
	h := newGroqHarness(t, fake.Server.URL, 1)
	badKey := strings.Repeat("x", 48)

	pairs := []struct{ bare, client, body string }{
		{"/claude/v1/messages", "/claude-code/v1/messages", claudeMessagesBody(false)},
		{"/claude/v1/messages", "/coding-agent/v1/messages", claudeMessagesBody(false)},
		{"/v1/chat/completions", "/codex/v1/chat/completions", `{"model":"x","messages":[]}`},
		{"/v1/responses", "/codex/v1/responses", `{"model":"x","input":"hi"}`},
		{"/v1/chat/completions", "/coding-agent/v1/chat/completions", `{"model":"x","messages":[]}`},
	}
	for _, p := range pairs {
		bare := h.requestWithKey(badKey, http.MethodPost, p.bare, p.body, nil)
		client := h.requestWithKey(badKey, http.MethodPost, p.client, p.body, nil)
		if bare.Code != http.StatusUnauthorized || client.Code != bare.Code || withoutRequestID(client.Body.String()) != withoutRequestID(bare.Body.String()) {
			t.Fatalf("%s answered %d %s, %s answered %d %s", p.bare, bare.Code, bare.Body.String(), p.client, client.Code, client.Body.String())
		}
	}
	if len(fake.Requests()) != 0 {
		t.Fatalf("refused requests must not reach the upstream")
	}
}

var requestIDInError = regexp.MustCompile(`\(request id: [^)]*\)`)

// withoutRequestID drops the per-request ID from an error body.
func withoutRequestID(body string) string {
	return requestIDInError.ReplaceAllString(body, "")
}

// count_tokens answers with a local estimate and neither routes nor bills.
func TestClaudeCodeCountTokens(t *testing.T) {
	fake := NewFakeProvider(t)
	h := newAnthropicHarness(t, fake.Server.URL)
	before := h.UserQuota(t)

	for _, path := range []string{"/claude-code/v1/messages/count_tokens", "/coding-agent/v1/messages/count_tokens", "/claude/v1/messages/count_tokens"} {
		w := h.request(http.MethodPost, path, claudeCodeBody(agentclient.ClaudeCodeModelID(agentClaudeModel, 0), false), nil)
		var resp struct {
			InputTokens int `json:"input_tokens"`
		}
		if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &resp) != nil || resp.InputTokens <= 0 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	if len(fake.Requests()) != 0 || h.UserQuota(t) != before || len(consumeLogs(t)) != 0 {
		t.Fatalf("count_tokens must not route, bill or log")
	}
}

// /claude-code/v1/models lists every text model under claude-code/, adds [1m] to the models with
// a context of one million tokens or more, and leaves out the models a coding agent cannot use.
func TestClaudeCodeModelsList(t *testing.T) {
	const longModel, embedModel = "gpt-5.5", "text-embedding-3-small"
	fake := NewFakeProvider(t)
	h := NewHarness(t, HarnessOptions{
		Model: claudeEntryModel,
		Channels: []ChannelSpec{{
			Name: "fake-openai", BaseURL: fake.Server.URL,
			Models: strings.Join([]string{claudeEntryModel, longModel, embedModel}, ","),
		}},
		Prices: []PriceSpec{{Model: claudeEntryModel, Input: 1, Output: 1}, {Model: longModel, Input: 1, Output: 1}, {Model: embedModel, Input: 1, Output: 1}},
	})
	seedModelInfo(t, &model.ModelInfo{Model: longModel, Name: "GPT-5.5", ContextLength: 1_050_000})

	w := h.request(http.MethodGet, "/claude-code/v1/models?limit=1000", "", map[string]string{"x-api-key": h.Token.Key, "anthropic-version": "2023-06-01"})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", w.Code, w.Body.String())
	}
	var list struct {
		Data []struct {
			Type        string `json:"type"`
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
		HasMore *bool   `json:"has_more"`
		FirstID *string `json:"first_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v %s", err, w.Body.String())
	}
	got := map[string]string{}
	for _, m := range list.Data {
		if m.Type != "model" {
			t.Fatalf("entry type = %q: %s", m.Type, w.Body.String())
		}
		got[m.ID] = m.DisplayName
	}
	want := map[string]string{
		"claude-code/" + claudeEntryModel: claudeEntryModel,
		"claude-code/gpt-5.5[1m]":         "GPT-5.5",
	}
	if len(got) != len(want) {
		t.Fatalf("models = %v, want %v", got, want)
	}
	for id, name := range want {
		if got[id] != name {
			t.Fatalf("models = %v, want %v", got, want)
		}
	}
	if list.HasMore == nil || *list.HasMore || list.FirstID == nil {
		t.Fatalf("Anthropic list fields missing: %s", w.Body.String())
	}
}

// codexModelsShape decodes the fields Codex requires (no serde default in
// codex-rs/protocol/src/openai_models.rs); a missing one fails the whole list there.
type codexModelsShape struct {
	Models []struct {
		Slug                     *string `json:"slug"`
		DisplayName              *string `json:"display_name"`
		SupportedReasoningLevels []struct {
			Effort      string `json:"effort"`
			Description string `json:"description"`
		} `json:"supported_reasoning_levels"`
		ShellType        *string `json:"shell_type"`
		Visibility       *string `json:"visibility"`
		SupportedInAPI   *bool   `json:"supported_in_api"`
		Priority         *int    `json:"priority"`
		BaseInstructions *string `json:"base_instructions"`
		SupportVerbosity *bool   `json:"support_verbosity"`
		TruncationPolicy *struct {
			Mode  string `json:"mode"`
			Limit int64  `json:"limit"`
		} `json:"truncation_policy"`
		ExperimentalSupportedTools *[]string `json:"experimental_supported_tools"`
		ApplyPatchToolType         *string   `json:"apply_patch_tool_type"`
		ContextWindow              *int64    `json:"context_window"`
		InputModalities            []string  `json:"input_modalities"`
	} `json:"models"`
}

// /codex/v1/models answers in Codex's ModelsResponse shape with every required field.
func TestCodexModelsList(t *testing.T) {
	const embedModel = "text-embedding-3-small"
	fake := NewFakeProvider(t)
	h := NewHarness(t, HarnessOptions{
		Model: agentCodexModel,
		Channels: []ChannelSpec{{
			Name: "fake-openai", BaseURL: fake.Server.URL,
			Models: strings.Join([]string{agentCodexModel, claudeEntryModel, embedModel}, ","),
		}},
		Prices: []PriceSpec{{Model: agentCodexModel, Input: 1, Output: 1}, {Model: claudeEntryModel, Input: 1, Output: 1}, {Model: embedModel, Input: 1, Output: 1}},
	})
	seedModelInfo(t, &model.ModelInfo{
		Model: agentCodexModel, Name: "GPT-5.5", ContextLength: 400_000,
		InputModalities: `["text","image"]`, Capabilities: `["tool_call","reasoning"]`,
	})

	w := h.request(http.MethodGet, "/codex/v1/models?client_version=0.140.0", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", w.Code, w.Body.String())
	}
	var list codexModelsShape
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v %s", err, w.Body.String())
	}
	if len(list.Models) != 2 {
		t.Fatalf("want the two text models, got %s", w.Body.String())
	}
	slugs := map[string]bool{}
	for _, m := range list.Models {
		if m.Slug == nil || m.DisplayName == nil || m.ShellType == nil || m.Visibility == nil || m.SupportedInAPI == nil ||
			m.Priority == nil || m.BaseInstructions == nil || *m.BaseInstructions == "" || m.SupportVerbosity == nil ||
			m.TruncationPolicy == nil || m.ExperimentalSupportedTools == nil || m.SupportedReasoningLevels == nil {
			t.Fatalf("entry lacks a required field: %s", w.Body.String())
		}
		// The instructions are Codex's own prompt (relay/third_party/codex/prompt.md), whole.
		if !strings.HasPrefix(*m.BaseInstructions, "You are a coding agent running in the Codex CLI") || len(*m.BaseInstructions) < 10000 {
			t.Fatalf("base_instructions is not the Codex prompt (%d bytes)", len(*m.BaseInstructions))
		}
		slugs[*m.Slug] = true
		if *m.Slug == agentCodexModel {
			if *m.DisplayName != "GPT-5.5" || m.ContextWindow == nil || *m.ContextWindow != 400_000 ||
				m.ApplyPatchToolType == nil || *m.ApplyPatchToolType != "freeform" || len(m.SupportedReasoningLevels) == 0 ||
				len(m.InputModalities) != 2 {
				t.Fatalf("gpt-5.5 entry = %s", w.Body.String())
			}
		}
		if *m.Slug == claudeEntryModel && m.ApplyPatchToolType != nil {
			t.Fatalf("a model without custom tools must not get the freeform apply_patch: %s", w.Body.String())
		}
	}
	if !slugs[agentCodexModel] || !slugs[claudeEntryModel] {
		t.Fatalf("slugs = %v", slugs)
	}
}

func responsesStreamChunks(modelName, text string, inputTokens, outputTokens int) []string {
	return []string{
		mustJSON(map[string]any{"type": "response.created", "response": map[string]any{"id": "resp_e2e", "object": "response", "model": modelName, "status": "in_progress"}}),
		mustJSON(map[string]any{"type": "response.output_text.delta", "item_id": "msg_e2e", "output_index": 0, "content_index": 0, "delta": text}),
		mustJSON(map[string]any{"type": "response.completed", "response": map[string]any{
			"id": "resp_e2e", "object": "response", "model": modelName, "status": "completed",
			"output": []any{map[string]any{"type": "message", "id": "msg_e2e", "status": "completed", "role": "assistant",
				"content": []any{map[string]any{"type": "output_text", "text": text}}}},
			"usage": map[string]any{"input_tokens": inputTokens, "output_tokens": outputTokens, "total_tokens": inputTokens + outputTokens},
		}}),
	}
}

func newCodexHarness(t *testing.T, baseURL string) *Harness {
	t.Helper()
	return NewHarness(t, HarnessOptions{
		Model:    agentCodexModel,
		Channels: []ChannelSpec{{Name: "fake-openai", BaseURL: baseURL}},
		Prices:   []PriceSpec{{Model: agentCodexModel, Input: billingInputRatio, Output: billingOutputRatio}},
	})
}

// codexBody is a Responses request as Codex sends it, prompt_cache_key included.
func codexBody(stream bool, cacheKey string) string {
	return `{"model":"` + agentCodexModel + `","stream":` + boolJSON(stream) + `,"instructions":"You are Codex.",` +
		`"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}],"prompt_cache_key":"` + cacheKey + `"}`
}

// /codex/v1/responses, non-streaming and streaming: served as /v1/responses, prompt_cache_key
// forwarded unchanged, billed as on /v1/responses, logged as Codex.
func TestCodexEndpointResponses(t *testing.T) {
	const inputTokens, outputTokens = 5, 3
	for _, stream := range []bool{false, true} {
		t.Run("stream="+boolJSON(stream), func(t *testing.T) {
			fake := NewFakeProvider(t)
			if stream {
				fake.SetResponse(FakeResponse{Stream: responsesStreamChunks(agentCodexModel, "hi from codex", inputTokens, outputTokens)})
			} else {
				fake.SetResponse(FakeResponse{JSON: responsesJSON(agentCodexModel, "hi from codex")})
			}
			h := newCodexHarness(t, fake.Server.URL)
			cacheKey := "codex-" + utils.GetRandomString(8)

			before := h.UserQuota(t)
			w := h.request(http.MethodPost, "/codex/v1/responses", codexBody(stream, cacheKey), map[string]string{
				"User-Agent": "codex_cli_rs/0.140.0 (Mac OS 26.0.0; arm64) iTerm.app/3.6.0",
				"session_id": cacheKey,
			})
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "hi from codex") {
				t.Fatalf("want 200 with the reply, got %d %s", w.Code, w.Body.String())
			}
			if stream && !strings.Contains(w.Body.String(), "response.completed") {
				t.Fatalf("not a Responses stream: %s", w.Body.String())
			}
			captured := fake.LastRequest(t)
			if captured.Path != "/v1/responses" {
				t.Fatalf("upstream path = %s, want /v1/responses", captured.Path)
			}
			if got := captured.JSONBody(t)["prompt_cache_key"]; got != cacheKey {
				t.Fatalf("prompt_cache_key upstream = %v, want %s", got, cacheKey)
			}
			want := expectQuota(inputTokens, outputTokens, 1)
			if got := before - h.UserQuota(t); got != want {
				t.Fatalf("billed %d, want %d", got, want)
			}
			log := soleConsumeLog(t)
			if log.Client != agentclient.Codex || log.ModelName != agentCodexModel || log.IsStream != stream || log.Quota != want {
				t.Fatalf("log = %+v", log)
			}
		})
	}
}

// A Codex session sticks to one channel (session affinity, CHAN-8) on the client endpoint too.
func TestCodexEndpointSessionAffinity(t *testing.T) {
	fakeA, fakeB := NewFakeProvider(t), NewFakeProvider(t)
	for _, f := range []*FakeProvider{fakeA, fakeB} {
		f.SetResponse(FakeResponse{JSON: responsesJSON(agentCodexModel, "ok")})
	}
	h := NewHarness(t, HarnessOptions{
		Model:    agentCodexModel,
		Channels: []ChannelSpec{{Name: "a", BaseURL: fakeA.Server.URL}, {Name: "b", BaseURL: fakeB.Server.URL}},
	})
	session := "codex-session-" + utils.GetRandomString(12)
	for i := 0; i < 6; i++ {
		if w := h.request(http.MethodPost, "/codex/v1/responses", codexBody(false, session), map[string]string{"session_id": session}); w.Code != http.StatusOK {
			t.Fatalf("request %d: %d %s", i, w.Code, w.Body.String())
		}
	}
	a, b := len(fakeA.Requests()), len(fakeB.Requests())
	if a+b != 6 || (a != 0 && b != 0) {
		t.Fatalf("one session went to both channels: a=%d b=%d", a, b)
	}
	logs := consumeLogs(t)
	if affinity := logs[len(logs)-1].Metadata.Data()["session_affinity"]; affinity != "hit" {
		t.Fatalf("session_affinity = %v, want hit", affinity)
	}
}

// /coding-agent/v1 behaves as /v1 and marks the traffic; a User-Agent naming the agent refines
// the mark, and the bare path records a known agent by its User-Agent.
func TestCodingAgentEndpointClient(t *testing.T) {
	fake := NewFakeProvider(t)
	fake.SetResponse(FakeResponse{JSON: ChatCompletionJSON("gpt-4o-mini", "hello", 10, 5)})
	h := NewHarness(t, HarnessOptions{FakeBaseURL: fake.Server.URL})
	body := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`

	cases := []struct {
		path, userAgent, want string
	}{
		{"/coding-agent/v1/chat/completions", "", agentclient.CodingAgent},
		{"/coding-agent/v1/chat/completions", "opencode/1.2.3", agentclient.OpenCode},
		{"/v1/chat/completions", "GeminiCLI/0.30.0 (darwin; arm64)", agentclient.GeminiCLI},
		{"/v1/chat/completions", "codex_cli_rs/0.140.0", agentclient.Codex},
		{"/v1/chat/completions", "curl/8.0", ""},
	}
	for i, tc := range cases {
		w := h.request(http.MethodPost, tc.path, body, map[string]string{"User-Agent": tc.userAgent})
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
		if got := fake.LastRequest(t).Path; got != "/v1/chat/completions" {
			t.Fatalf("%s reached the upstream at %s", tc.path, got)
		}
		logs := consumeLogs(t)
		if len(logs) != i+1 || logs[i].Client != tc.want {
			t.Fatalf("%s with %q: client = %q, want %q", tc.path, tc.userAgent, logs[len(logs)-1].Client, tc.want)
		}
	}
}

// /coding-agent/v1/messages is the Anthropic shape; /coding-agent/v1/models answers each protocol
// in its own list shape.
func TestCodingAgentEndpointAnthropic(t *testing.T) {
	fake := NewFakeProvider(t)
	fake.SetResponse(FakeResponse{JSON: anthropicMessageJSON(agentClaudeModel, "hello", 4, 2)})
	h := newAnthropicHarness(t, fake.Server.URL)

	w := h.request(http.MethodPost, "/coding-agent/v1/messages", claudeCodeBody(agentClaudeModel, false), map[string]string{"anthropic-version": "2023-06-01"})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"type":"message"`) {
		t.Fatalf("want a Messages response, got %d %s", w.Code, w.Body.String())
	}
	if log := soleConsumeLog(t); log.Client != agentclient.CodingAgent {
		t.Fatalf("client = %q", log.Client)
	}

	anthropic := h.requestWithKey("", http.MethodGet, "/coding-agent/v1/models", "", map[string]string{
		"Authorization": "", "x-api-key": h.Token.Key, "anthropic-version": "2023-06-01",
	})
	if anthropic.Code != http.StatusOK || !strings.Contains(anthropic.Body.String(), `"type":"model"`) || strings.Contains(anthropic.Body.String(), `"object"`) {
		t.Fatalf("Anthropic list = %d %s", anthropic.Code, anthropic.Body.String())
	}
	openai := h.request(http.MethodGet, "/coding-agent/v1/models", "", nil)
	if openai.Code != http.StatusOK || !strings.Contains(openai.Body.String(), `"object":"list"`) {
		t.Fatalf("OpenAI list = %d %s", openai.Code, openai.Body.String())
	}
}

// A Claude Code session sticks to one channel through its session header.
func TestClaudeCodeEndpointSessionAffinity(t *testing.T) {
	fakeA, fakeB := NewFakeProvider(t), NewFakeProvider(t)
	for _, f := range []*FakeProvider{fakeA, fakeB} {
		f.SetResponse(FakeResponse{JSON: anthropicMessageJSON(agentClaudeModel, "ok", 3, 1)})
	}
	h := NewHarness(t, HarnessOptions{
		Model: agentClaudeModel,
		Channels: []ChannelSpec{
			{Name: "a", Type: config.ChannelTypeAnthropic, BaseURL: fakeA.Server.URL},
			{Name: "b", Type: config.ChannelTypeAnthropic, BaseURL: fakeB.Server.URL},
		},
	})
	session := "cc-session-" + utils.GetRandomString(12)
	for i := 0; i < 6; i++ {
		w := h.request(http.MethodPost, "/claude-code/v1/messages", claudeCodeBody(agentclient.ClaudeCodeModelID(agentClaudeModel, 0), false),
			map[string]string{"x-claude-code-session-id": session})
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: %d %s", i, w.Code, w.Body.String())
		}
	}
	a, b := len(fakeA.Requests()), len(fakeB.Requests())
	if a+b != 6 || (a != 0 && b != 0) {
		t.Fatalf("one session went to both channels: a=%d b=%d", a, b)
	}
}
