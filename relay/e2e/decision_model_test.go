package e2e

import (
	"net/http"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/model"
)

// A decision model generates no text: chat, Responses and Claude requests for it are refused
// with a 400 before any upstream is called, so no line is tried or cooled down.
func TestDecisionModelRefusedOnTextEntries(t *testing.T) {
	upstream := NewFakeProvider(t)
	h := NewHarness(t, HarnessOptions{
		Channels: []ChannelSpec{{Name: "c", BaseURL: upstream.Server.URL, Models: "unrelated"}},
	})
	useSupplies(t,
		[]model.ModelInfo{{Model: "heygen-decision-1", Status: model.ModelStatusPublished, Endpoints: `["decisions"]`}},
		[]model.ModelSupply{{Model: "heygen-decision-1", ChannelId: h.Channel.Id, UpstreamModel: "heygen/heygen-decision-1", Enabled: true}})

	cases := map[string]string{
		"/v1/chat/completions": `{"model":"heygen-decision-1","messages":[{"role":"user","content":"hi"}]}`,
		"/v1/responses":        `{"model":"heygen-decision-1","input":"hi"}`,
		"/claude/v1/messages":  `{"model":"heygen-decision-1","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`,
	}
	for path, body := range cases {
		w := h.Post(path, body)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "decision model") {
			t.Errorf("%s: status %d body %s", path, w.Code, w.Body.String())
		}
	}
	if n := len(upstream.Requests()); n != 0 {
		t.Errorf("upstream called %d times, want 0", n)
	}
}
