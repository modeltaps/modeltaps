package relay

import (
	"encoding/json"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/providers/claude"
)

// Malformed content blocks get a fixed estimate instead of panicking on a type assertion.
func TestCountClaudeMessageTokensMalformedContent(t *testing.T) {
	prev := config.DisableTokenEncoders
	config.DisableTokenEncoders = true
	t.Cleanup(func() { config.DisableTokenEncoders = prev })

	var content any
	if err := json.Unmarshal([]byte(`["oops", 1, {"type":"text","text":null}, {"type":"text","text":"hi"}]`), &content); err != nil {
		t.Fatal(err)
	}
	req := &claude.ClaudeRequest{
		Model:    "claude-sonnet-4-5",
		Messages: []claude.Message{{Role: "user", Content: content}},
	}
	if got := countClaudeMessageTokens(req); got < 150 {
		t.Fatalf("countClaudeMessageTokens = %d, want >= 150 (three fixed estimates)", got)
	}
}
