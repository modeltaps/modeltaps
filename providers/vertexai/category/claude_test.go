package category

import "testing"

func TestNormalizeClaudeModelName(t *testing.T) {
	cases := map[string]string{
		// dated IDs become name@date
		"claude-sonnet-4-5-20250929": "claude-sonnet-4-5@20250929",
		"claude-3-haiku-20240307":    "claude-3-haiku@20240307",
		"claude-opus-4-1-20250805":   "claude-opus-4-1@20250805",
		// irregular IDs come from the override table
		"claude-3-5-sonnet-20241022": "claude-3-5-sonnet-v2@20241022",
		// IDs that already use the Vertex form, or carry no date, are untouched
		"claude-sonnet-4-5@20250929": "claude-sonnet-4-5@20250929",
		"claude-opus-5-5":            "claude-opus-5-5",
		"claude-opus-4-8":            "claude-opus-4-8",
	}
	for in, want := range cases {
		if got := NormalizeClaudeModelName(in); got != want {
			t.Errorf("NormalizeClaudeModelName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCategoryPublishers(t *testing.T) {
	if got := CategoryMap["claude"].Publisher; got != PublisherAnthropic {
		t.Errorf("claude publisher = %q", got)
	}
	if got := CategoryMap["gemini"].Publisher; got != PublisherGoogle {
		t.Errorf("gemini publisher = %q", got)
	}
}
