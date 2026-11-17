package claude

import (
	"testing"

	"github.com/modeltaps/modeltaps/types"
)

// SEC-56: the output limit sent to Claude is one the client declared, so the pre-consumption
// estimate (the largest declared limit) covers it.
func TestConvertFromChatOpenaiMaxTokensFallback(t *testing.T) {
	cases := []struct {
		name                string
		maxTokens, maxCompl int
		want                int
	}{
		{"only max_completion_tokens", 0, 3000, 3000},
		{"max_tokens wins", 2000, 3000, 2000},
		{"only max_tokens", 2000, 0, 2000},
	}
	for _, tc := range cases {
		claudeRequest, errWithCode := ConvertFromChatOpenai(&types.ChatCompletionRequest{
			Model:               "claude-sonnet-5-5",
			MaxTokens:           tc.maxTokens,
			MaxCompletionTokens: tc.maxCompl,
			Messages:            []types.ChatCompletionMessage{{Role: types.ChatMessageRoleUser, Content: "hi"}},
		})
		if errWithCode != nil {
			t.Fatalf("%s: %v", tc.name, errWithCode)
		}
		if claudeRequest.MaxTokens != tc.want {
			t.Fatalf("%s: MaxTokens = %d, want %d", tc.name, claudeRequest.MaxTokens, tc.want)
		}
	}
}
