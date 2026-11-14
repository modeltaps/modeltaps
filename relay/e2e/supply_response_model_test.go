package e2e

import (
	"net/http"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/tidwall/gjson"
)

const (
	supplyPublicModel   = "claude-opus-5"
	supplyUpstreamModel = "anthropic/claude-opus-5"
)

func claudeMessageJSON(modelName string) string {
	return `{"id":"msg_1","type":"message","role":"assistant","model":"` + modelName + `",` +
		`"content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":2}}`
}

func claudeStreamEvents(modelName string) []string {
	return []string{
		`{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"` + modelName +
			`","content":[],"usage":{"input_tokens":5,"output_tokens":1}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`,
		`{"type":"message_stop"}`,
	}
}

// A supply that renames the model sends the platform spelling upstream, but every response
// carries the public name the client asked for, with the unified-response-model switches off.
// Vercel serves /v1/responses through its chat endpoint and Claude requests natively.
func TestSupplyResponsesCarryPublicModelName(t *testing.T) {
	setAndRestore(t, &config.UnifiedRequestResponseModelEnabled, false)

	cases := []struct {
		name     string
		path     string
		body     string
		upstream FakeResponse
		// models extracts every model name the client sees.
		models func(body string) []string
	}{
		{
			name:     "chat",
			path:     "/v1/chat/completions",
			body:     chatFor(supplyPublicModel),
			upstream: FakeResponse{JSON: ChatCompletionJSON(supplyUpstreamModel, "ok", 5, 2)},
			models:   func(b string) []string { return []string{gjson.Get(b, "model").String()} },
		},
		{
			name: "chat stream",
			path: "/v1/chat/completions",
			body: `{"model":"` + supplyPublicModel + `","stream":true,"messages":[{"role":"user","content":"hi"}]}`,
			upstream: FakeResponse{
				Stream: ChatStreamChunks(supplyUpstreamModel, []string{"o", "k"}, 5, 2),
			},
			models: func(b string) []string { return sseModels(b, "model") },
		},
		{
			name:     "responses",
			path:     "/v1/responses",
			body:     `{"model":"` + supplyPublicModel + `","input":"hi"}`,
			upstream: FakeResponse{JSON: ChatCompletionJSON(supplyUpstreamModel, "ok", 5, 2)},
			models:   func(b string) []string { return []string{gjson.Get(b, "model").String()} },
		},
		{
			name:     "responses stream",
			path:     "/v1/responses",
			body:     `{"model":"` + supplyPublicModel + `","stream":true,"input":"hi"}`,
			upstream: FakeResponse{Stream: ChatStreamChunks(supplyUpstreamModel, []string{"o", "k"}, 5, 2)},
			models:   func(b string) []string { return sseModels(b, "response.model") },
		},
		{
			name:     "claude",
			path:     "/claude/v1/messages",
			body:     `{"model":"` + supplyPublicModel + `","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`,
			upstream: FakeResponse{Raw: claudeMessageJSON(supplyUpstreamModel), Headers: map[string]string{"Content-Type": "application/json"}},
			models:   func(b string) []string { return []string{gjson.Get(b, "model").String()} },
		},
		{
			name:     "claude stream",
			path:     "/claude/v1/messages",
			body:     `{"model":"` + supplyPublicModel + `","max_tokens":64,"stream":true,"messages":[{"role":"user","content":"hi"}]}`,
			upstream: FakeResponse{Stream: claudeStreamEvents(supplyUpstreamModel), StreamNoDone: true},
			models:   func(b string) []string { return sseModels(b, "message.model") },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := NewFakeProvider(t)
			upstream.SetResponse(tc.upstream)
			h := NewHarness(t, HarnessOptions{
				Model: supplyPublicModel,
				Channels: []ChannelSpec{{
					Name: "vercel", Type: config.ChannelTypeVercelAIGateway, BaseURL: upstream.Server.URL, Models: "unrelated",
				}},
			})
			useSupplies(t,
				[]model.ModelInfo{{Model: supplyPublicModel, Status: model.ModelStatusPublished}},
				[]model.ModelSupply{{Model: supplyPublicModel, ChannelId: h.Channel.Id, UpstreamModel: supplyUpstreamModel, Enabled: true}})

			w := h.Post(tc.path, tc.body)
			if w.Code != http.StatusOK {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if got := upstream.LastRequest(t).JSONBody(t)["model"]; got != supplyUpstreamModel {
				t.Fatalf("upstream model = %v, want %s", got, supplyUpstreamModel)
			}
			body := w.Body.String()
			models := tc.models(body)
			if len(models) == 0 {
				t.Fatalf("no model field in response: %s", body)
			}
			for _, m := range models {
				if m != supplyPublicModel {
					t.Fatalf("response model = %q, want %q; body=%s", m, supplyPublicModel, body)
				}
			}
		})
	}
}

// A supply whose upstream name equals the public name leaves the upstream's answer alone, as
// before: a dated variant in the stream still shows unless a unified-response-model switch is on.
func TestSupplyWithSameNameKeepsUpstreamResponseModel(t *testing.T) {
	setAndRestore(t, &config.UnifiedRequestResponseModelEnabled, false)
	upstream := NewFakeProvider(t)
	upstream.SetResponse(FakeResponse{Stream: ChatStreamChunks("gpt-4o-mini-2024-07-18", []string{"ok"}, 5, 2)})
	h := NewHarness(t, HarnessOptions{
		Channels: []ChannelSpec{{Name: "c", BaseURL: upstream.Server.URL, Models: "unrelated"}},
	})
	useSupplies(t,
		[]model.ModelInfo{{Model: "gpt-4o-mini", Status: model.ModelStatusPublished}},
		[]model.ModelSupply{{Model: "gpt-4o-mini", ChannelId: h.Channel.Id, UpstreamModel: "gpt-4o-mini", Enabled: true}})

	w := h.Post("/v1/chat/completions", `{"model":"gpt-4o-mini","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	models := sseModels(w.Body.String(), "model")
	if len(models) == 0 {
		t.Fatalf("no model field in response: %s", w.Body.String())
	}
	for _, m := range models {
		if m != "gpt-4o-mini-2024-07-18" {
			t.Fatalf("response model = %q, want the upstream's dated name", m)
		}
	}
}

// sseModels returns the value at path from every SSE data line that has it.
func sseModels(body, path string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		data, ok := strings.CutPrefix(strings.TrimSpace(line), "data:")
		if !ok {
			continue
		}
		if v := gjson.Get(strings.TrimSpace(data), path); v.Exists() {
			out = append(out, v.String())
		}
	}
	return out
}
