package claude

import (
	"encoding/json"
	"strings"
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

// A numeric usage.cost, which a router may add to a Claude format response, is kept for the log.
func TestClaudeUsageKeepsUpstreamCost(t *testing.T) {
	var cUsage Usage
	if err := json.Unmarshal([]byte(`{"input_tokens":10,"output_tokens":2,"cost":0.0042}`), &cUsage); err != nil {
		t.Fatal(err)
	}
	usage := &types.Usage{}
	if !ClaudeUsageToOpenaiUsage(&cUsage, usage) || usage.UpstreamCost != 0.0042 {
		t.Fatalf("usage = %+v", usage)
	}
	if out, _ := json.Marshal(cUsage); strings.Contains(string(out), "cost") {
		t.Errorf("the cost is serialized: %s", out)
	}

	// A stream reports it in message_start or message_delta; either is kept.
	start := Usage{InputTokens: 10, UpstreamCost: 0.001}
	delta := Usage{OutputTokens: 5}
	ClaudeUsageMerge(&delta, &start)
	if delta.UpstreamCost != 0.001 {
		t.Errorf("merged cost = %v", delta.UpstreamCost)
	}

	var odd Usage
	if err := json.Unmarshal([]byte(`{"input_tokens":10,"output_tokens":2,"cost":"1"}`), &odd); err != nil || odd.UpstreamCost != 0 || odd.InputTokens != 10 {
		t.Errorf("a cost that is not a number: %+v, %v", odd, err)
	}
}

// The native stream hands the upstream message id to the provider's handler, and keeps a cost
// reported in message_delta.
func TestClaudeRelayStreamReportsMessageIDAndCost(t *testing.T) {
	var gotID string
	h := &ClaudeRelayStreamHandler{Usage: &types.Usage{}, Prefix: `data: {`, SkipModelUnify: true,
		MessageIDHandler: func(id string) { gotID = id }}
	dataChan, errChan := make(chan string, 8), make(chan error, 8)
	for _, line := range []string{
		`data: {"type":"message_start","message":{"id":"gen-abc","type":"message","role":"assistant","model":"m","usage":{"input_tokens":10,"output_tokens":1}}}`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5,"cost":0.0031}}`,
	} {
		raw := []byte(line)
		h.HandlerStream(&raw, dataChan, errChan)
	}
	if len(errChan) != 0 {
		t.Fatalf("stream error: %v", <-errChan)
	}
	if gotID != "gen-abc" {
		t.Errorf("message id = %q", gotID)
	}
	if h.Usage.UpstreamCost != 0.0031 || h.Usage.CompletionTokens != 5 || h.Usage.PromptTokens != 10 {
		t.Errorf("usage = %+v", h.Usage)
	}
}
