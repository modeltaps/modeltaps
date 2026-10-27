package common

import (
	"encoding/json"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/types"
)

// Malformed content parts must be skipped instead of panicking on a type assertion.
func TestCountTokenMessagesMalformedContent(t *testing.T) {
	prev := config.DisableTokenEncoders
	config.DisableTokenEncoders = true
	t.Cleanup(func() { config.DisableTokenEncoders = prev })

	raw := `[
		"not an object",
		42,
		{"type":"text","text":null},
		{"type":"text","text":7},
		{"type":"image_url","image_url":{"url":null}},
		{"type":"image_url","image_url":{"url":""}},
		{"type":"image_url","image_url":{"url":5,"detail":3}},
		{"type":"text","text":"hello"}
	]`
	var content any
	if err := json.Unmarshal([]byte(raw), &content); err != nil {
		t.Fatal(err)
	}
	messages := []types.ChatCompletionMessage{{Role: "user", Content: content}}

	if got := CountTokenMessages(messages, "gpt-4o", config.PreCostDefault); got <= 0 {
		t.Fatalf("CountTokenMessages = %d, want > 0", got)
	}
	input := []any{map[string]any{"role": "user", "content": content}}
	if got := CountTokenInputMessages(input, "gpt-4o", config.PreCostDefault); got <= 3 {
		t.Fatalf("CountTokenInputMessages = %d, want > 0", got)
	}
}
