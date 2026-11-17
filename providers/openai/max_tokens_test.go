package openai

import (
	"testing"

	"github.com/modeltaps/modeltaps/types"
)

// SEC-56: o-series and gpt-5 take max_completion_tokens. An explicit max_completion_tokens is
// kept and max_tokens is only the fallback, the same order as the Claude and Gemini conversions.
func TestOtherProcessingMaxCompletionTokensFallback(t *testing.T) {
	cases := []struct {
		name                string
		model               string
		maxTokens, maxCompl int
		wantMax, wantCompl  int
	}{
		{"o-series only max_tokens", "o3", 2000, 0, 0, 2000},
		{"o-series both set", "o3", 64000, 1, 0, 1},
		{"gpt-5 only max_completion_tokens", "gpt-5", 0, 3000, 0, 3000},
		{"other model untouched", "gpt-4o", 2000, 3000, 2000, 3000},
	}
	for _, tc := range cases {
		request := &types.ChatCompletionRequest{Model: tc.model, MaxTokens: tc.maxTokens, MaxCompletionTokens: tc.maxCompl}
		otherProcessing(request, "")
		if request.MaxTokens != tc.wantMax || request.MaxCompletionTokens != tc.wantCompl {
			t.Fatalf("%s: max_tokens=%d max_completion_tokens=%d, want %d/%d",
				tc.name, request.MaxTokens, request.MaxCompletionTokens, tc.wantMax, tc.wantCompl)
		}
	}
}
