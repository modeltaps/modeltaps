package claude

import (
	"encoding/json"
	"testing"

	"github.com/modeltaps/modeltaps/types"
)

func TestThinkingModelGenerations(t *testing.T) {
	cases := []struct {
		name             string
		adaptive, hidden bool
	}{
		{"claude-opus-5-5", true, true},
		{"anthropic/claude-opus-5.5", true, true},
		{"claude-opus-4-8", true, true},
		{"claude-opus-4-7", true, true},
		{"claude-opus-4-6", true, false},
		{"anthropic/claude-sonnet-4.6", true, false},
		{"claude-sonnet-5-5", true, true},
		{"claude-sonnet-5", true, true},
		{"claude-haiku-5-5", true, true},
		{"claude-fable-5-2", true, true},
		{"claude-mythos-5-1", true, true},
		{"claude-sonnet-4-5-20250929", false, false},
		{"claude-haiku-4-5", false, false},
		{"claude-opus-4-20250514", false, false},
		{"claude-3-7-sonnet-20250219", false, false},
		{"gpt-5.5", false, false},
		{"claude-sonnet-5-5#thinking", true, true},
	}
	for _, tc := range cases {
		if got := AdaptiveThinkingSupported(tc.name); got != tc.adaptive {
			t.Errorf("AdaptiveThinkingSupported(%q) = %v, want %v", tc.name, got, tc.adaptive)
		}
		if got := ThinkingHiddenByDefault(tc.name); got != tc.hidden {
			t.Errorf("ThinkingHiddenByDefault(%q) = %v, want %v", tc.name, got, tc.hidden)
		}
	}
}

func TestClaudeEffort(t *testing.T) {
	cases := map[[2]string]string{
		{"minimal", "claude-opus-5-5"}: "low",
		{"LOW", "claude-opus-5-5"}:     "low",
		{"medium", "claude-opus-5-5"}:  "medium",
		{"xhigh", "claude-opus-5-5"}:   "xhigh",
		{"xhigh", "claude-opus-4-6"}:   "high",
		{"max", "claude-sonnet-4-6"}:   "max",
		{"none", "claude-opus-5-5"}:    "",
		{"", "claude-opus-5-5"}:        "",
	}
	for in, want := range cases {
		if got := claudeEffort(in[0], in[1]); got != want {
			t.Errorf("claudeEffort(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

func effortPtr(s string) *string { return &s }

func TestConvertFromChatOpenaiAdaptiveThinking(t *testing.T) {
	msgs := []types.ChatCompletionMessage{{Role: types.ChatMessageRoleUser, Content: "hi"}}

	// reasoning_effort alone turns thinking on: adaptive, summarized, effort in output_config,
	// no token budget (a 400 on these models) and max_tokens left as the client sent it.
	got := convertForTest(t, &types.ChatCompletionRequest{Model: "claude-opus-5-5", MaxTokens: 4096, ReasoningEffort: effortPtr("high"), Messages: msgs})
	if got.Thinking == nil || *got.Thinking != (Thinking{Type: "adaptive", Display: "summarized"}) {
		t.Fatalf("Thinking = %#v, want adaptive + summarized", got.Thinking)
	}
	if got.OutputConfig == nil || got.OutputConfig.Effort != "high" {
		t.Fatalf("OutputConfig = %#v, want effort high", got.OutputConfig)
	}
	if got.MaxTokens != 4096 {
		t.Fatalf("MaxTokens = %d, want 4096", got.MaxTokens)
	}
	raw, _ := json.Marshal(got)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	if th := body["thinking"].(map[string]any); th["budget_tokens"] != nil {
		t.Fatalf("budget_tokens must not be sent: %v", th)
	}

	// A reasoning object with only a token budget: adaptive without an effort.
	got = convertForTest(t, &types.ChatCompletionRequest{Model: "claude-sonnet-5-5", MaxTokens: 4096, Reasoning: &types.ChatReasoning{MaxTokens: 2048}, Messages: msgs})
	if got.Thinking == nil || got.Thinking.Type != "adaptive" || got.Thinking.BudgetTokens != 0 || got.OutputConfig != nil {
		t.Fatalf("Thinking = %#v, OutputConfig = %#v", got.Thinking, got.OutputConfig)
	}

	// Sonnet 4.6 shows thinking by default: adaptive, no display override.
	got = convertForTest(t, &types.ChatCompletionRequest{Model: "claude-sonnet-4-6", MaxTokens: 4096, ReasoningEffort: effortPtr("low"), Messages: msgs})
	if got.Thinking == nil || *got.Thinking != (Thinking{Type: "adaptive"}) || got.OutputConfig.Effort != "low" {
		t.Fatalf("Thinking = %#v, OutputConfig = %#v", got.Thinking, got.OutputConfig)
	}

	// Older models keep the token budget.
	got = convertForTest(t, &types.ChatCompletionRequest{Model: "claude-haiku-4-5", MaxTokens: 4096, ReasoningEffort: effortPtr("medium"), Messages: msgs})
	if got.Thinking == nil || got.Thinking.Type != "enabled" || got.Thinking.BudgetTokens != 2048 || got.OutputConfig != nil {
		t.Fatalf("Thinking = %#v, OutputConfig = %#v", got.Thinking, got.OutputConfig)
	}

	// No reasoning asked for: no thinking.
	got = convertForTest(t, &types.ChatCompletionRequest{Model: "claude-opus-5-5", MaxTokens: 4096, Messages: msgs})
	if got.Thinking != nil || got.OutputConfig != nil {
		t.Fatalf("Thinking = %#v, OutputConfig = %#v, want none", got.Thinking, got.OutputConfig)
	}

	// "none" asks for the fastest answer: no thinking configuration, the lowest effort.
	got = convertForTest(t, &types.ChatCompletionRequest{Model: "claude-opus-5-5", MaxTokens: 4096, ReasoningEffort: effortPtr("none"), Messages: msgs})
	if got.Thinking != nil || got.OutputConfig == nil || got.OutputConfig.Effort != "low" {
		t.Fatalf("none: Thinking = %#v, OutputConfig = %#v, want no thinking and effort low", got.Thinking, got.OutputConfig)
	}
	got = convertForTest(t, &types.ChatCompletionRequest{Model: "claude-haiku-4-5", MaxTokens: 4096, Reasoning: &types.ChatReasoning{Effort: "None"}, Messages: msgs})
	if got.Thinking != nil || got.OutputConfig != nil || got.MaxTokens != 4096 {
		t.Fatalf("none on an older model: Thinking = %#v, OutputConfig = %#v", got.Thinking, got.OutputConfig)
	}
}
