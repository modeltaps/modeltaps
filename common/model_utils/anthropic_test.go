package model_utils

import "testing"

func TestIsAnthropicModelName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"claude-3-5-sonnet-20241022", true},
		{"claude-opus-4", true},
		{"claude-sonnet-4-5", true},
		{"anthropic/claude-3.5-haiku", true},
		{"us.anthropic.claude-3-7-sonnet-20250219-v1:0", true},
		{"claude-3-5-sonnet@20240620", true},
		{"my-opus-prod", true},
		{"house-sonnet", true},
		{"some-haiku-model", true},
		{"Claude-Sonnet-4-5", true},
		{"gpt-4o", false},
		{"deepseek-chat", false},
		{"gemini-1.5-pro", false},
		{"llama-3.1", false},
		{"qwen-2.5", false},
		{"corpus-embed-v2", false},
		{"opusculum-mini", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsAnthropicModelName(c.name); got != c.want {
				t.Fatalf("IsAnthropicModelName(%q) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}
