package e2e

// Content moderation over the full relay chain: text that only appears in places the old check
// skipped (system prompts, later content parts, Responses input and instructions, tool
// descriptions) is refused with a local 400, and the fake upstream receives nothing.

import (
	"net/http"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/safty"
	"github.com/modeltaps/modeltaps/safty/providers/keyword"
	"github.com/modeltaps/modeltaps/safty/types"
)

const moderationBlockedWord = "forbidden-topic"

func enableModeration(t *testing.T) {
	t.Helper()
	setAndRestore(t, &config.EnableSafe, true)
	setAndRestore(t, &config.SafeToolName, "Keyword")
	setAndRestore(t, &config.SafeKeyWords, []string{moderationBlockedWord})
	prev, had := safty.Tools["Keyword"]
	safty.Tools["Keyword"] = keyword.NewKeywordChecker()
	t.Cleanup(func() {
		if had {
			safty.Tools["Keyword"] = prev
		} else {
			delete(safty.Tools, "Keyword")
		}
	})
}

func TestContentModerationRefusesBeforeUpstream(t *testing.T) {
	const word = moderationBlockedWord
	cases := []struct {
		name string
		path string
		body string
	}{
		{"chat system message", "/v1/chat/completions",
			`{"model":"gpt-4o-mini","messages":[{"role":"system","content":"` + word + `"},{"role":"user","content":"hi"}]}`},
		{"chat later content part", "/v1/chat/completions",
			`{"model":"gpt-4o-mini","messages":[{"role":"user","content":[{"type":"text","text":"hi"},{"type":"text","text":"` + word + `"}]}]}`},
		{"chat tool description", "/v1/chat/completions",
			`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"f","description":"` + word + `","parameters":{"type":"object"}}}]}`},
		{"responses input items", "/v1/responses",
			`{"model":"gpt-4o-mini","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"` + word + `"}]}]}`},
		{"responses instructions", "/v1/responses",
			`{"model":"gpt-4o-mini","input":"hi","instructions":"` + word + `"}`},
		{"messages system", "/claude/v1/messages",
			`{"model":"gpt-4o-mini","max_tokens":16,"system":[{"type":"text","text":"` + word + `"}],"messages":[{"role":"user","content":"hi"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := NewFakeProvider(t)
			h := NewHarness(t, HarnessOptions{FakeBaseURL: fake.Server.URL})
			enableModeration(t)

			w := h.Post(tc.path, tc.body)
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), types.SafeDefaultErrorCode) {
				t.Fatalf("want 400 %s, got %d body=%s", types.SafeDefaultErrorCode, w.Code, w.Body.String())
			}
			if n := len(fake.Requests()); n != 0 {
				t.Fatalf("the upstream received %d requests for refused content", n)
			}
		})
	}
}

// Clean requests still reach the upstream with moderation on.
func TestContentModerationPassesCleanRequest(t *testing.T) {
	fake := NewFakeProvider(t)
	h := NewHarness(t, HarnessOptions{FakeBaseURL: fake.Server.URL})
	enableModeration(t)

	w := h.Post("/v1/chat/completions",
		`{"model":"gpt-4o-mini","messages":[{"role":"system","content":"be brief"},{"role":"user","content":[{"type":"text","text":"hi"}]}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	if n := len(fake.Requests()); n != 1 {
		t.Fatalf("want one upstream request, got %d", n)
	}
}
