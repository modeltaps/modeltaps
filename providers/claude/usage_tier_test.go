package claude

import (
	"testing"

	"github.com/modeltaps/modeltaps/types"
)

// TestClaudeStreamUsageKeepsSpeed: message_start carries usage.speed and message_delta
// does not; the merged usage keeps fast so billing applies the fast-mode price.
func TestClaudeStreamUsageKeepsSpeed(t *testing.T) {
	start := &Usage{InputTokens: 100, OutputTokens: 1, Speed: "fast", InferenceGeo: "us"}
	usage := &types.Usage{}
	ClaudeUsageToOpenaiUsage(start, usage)

	delta := &Usage{OutputTokens: 50}
	ClaudeUsageMerge(delta, start)
	ClaudeUsageToOpenaiUsage(delta, usage)

	if usage.ServiceTier != "fast" || usage.InferenceGeo != "us" || usage.CompletionTokens != 50 {
		t.Fatalf("tier=%q geo=%q completion=%d", usage.ServiceTier, usage.InferenceGeo, usage.CompletionTokens)
	}

	standard := &types.Usage{}
	ClaudeUsageToOpenaiUsage(&Usage{InputTokens: 1, OutputTokens: 1, Speed: "standard", ServiceTier: "priority"}, standard)
	if standard.ServiceTier != "" {
		t.Fatalf("standard speed / Priority Tier set ServiceTier = %q", standard.ServiceTier)
	}
}
