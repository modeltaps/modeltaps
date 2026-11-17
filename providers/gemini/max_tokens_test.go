package gemini

import (
	"testing"

	"github.com/modeltaps/modeltaps/types"
)

// SEC-56: the output limit sent to Gemini is one the client declared, so the pre-consumption
// estimate (the largest declared limit) covers it.
func TestConvertFromChatOpenaiMaxOutputTokensFallback(t *testing.T) {
	cases := []struct {
		name                string
		maxTokens, maxCompl int
		want                int
	}{
		{"only max_completion_tokens", 0, 3000, 3000},
		{"max_tokens wins", 2000, 3000, 2000},
		{"only max_tokens", 2000, 0, 2000},
		{"none", 0, 0, 0},
	}
	for _, tc := range cases {
		geminiRequest, errWithCode := ConvertFromChatOpenai(&types.ChatCompletionRequest{
			Model:               "gemini-3-flash",
			MaxTokens:           tc.maxTokens,
			MaxCompletionTokens: tc.maxCompl,
			Messages:            []types.ChatCompletionMessage{{Role: types.ChatMessageRoleUser, Content: "hi"}},
		})
		if errWithCode != nil {
			t.Fatalf("%s: %v", tc.name, errWithCode)
		}
		if got := geminiRequest.GenerationConfig.MaxOutputTokens; got != tc.want {
			t.Fatalf("%s: MaxOutputTokens = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// The thinking budget is kept below the resolved limit, also when it comes from max_completion_tokens.
func TestConvertFromChatOpenaiThinkingBudgetUsesFallbackLimit(t *testing.T) {
	geminiRequest, errWithCode := ConvertFromChatOpenai(&types.ChatCompletionRequest{
		Model:               "gemini-3-flash",
		MaxCompletionTokens: 2000,
		Reasoning:           &types.ChatReasoning{MaxTokens: 5000},
		Messages:            []types.ChatCompletionMessage{{Role: types.ChatMessageRoleUser, Content: "hi"}},
	})
	if errWithCode != nil {
		t.Fatal(errWithCode)
	}
	cfg := geminiRequest.GenerationConfig.ThinkingConfig
	if cfg == nil || cfg.ThinkingBudget == nil || *cfg.ThinkingBudget != 1999 {
		t.Fatalf("ThinkingConfig = %+v, want budget 1999", cfg)
	}
}
