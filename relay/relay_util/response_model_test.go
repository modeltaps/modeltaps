package relay_util

import "testing"

func TestResponseModelMismatch(t *testing.T) {
	cases := []struct {
		returned, requested, upstream string
		mismatch                      bool
	}{
		{"gpt-4o", "gpt-4o", "gpt-4o", false},
		{"gpt-4o-2024-08-06", "gpt-4o", "gpt-4o", false},                    // dated variant
		{"GPT-4o", "gpt-4o", "", false},                                     // case
		{"anthropic/claude-opus-4.5", "claude-opus-4-5", "", false},         // provider path, dots
		{"us.anthropic.claude-opus-5-5-v1:0", "claude-opus-5-5", "", false}, // regional profile id
		{"claude-sonnet-5", "my-alias", "claude-sonnet-5", false},           // mapped name
		{"gpt-4o-mini", "gpt-6.1-sol", "gpt-6.1-sol", true},                 // swapped for another model
		{"deepseek-chat", "claude-opus-5-5", "anthropic/claude-opus-5-5", true},
		{"", "gpt-4o", "gpt-4o", false}, // upstream declared nothing
	}
	for _, tc := range cases {
		if got := responseModelMismatch(tc.returned, tc.requested, tc.upstream); got != tc.mismatch {
			t.Errorf("returned=%q requested=%q upstream=%q: mismatch=%v, want %v", tc.returned, tc.requested, tc.upstream, got, tc.mismatch)
		}
	}
}
