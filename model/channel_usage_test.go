package model

import (
	"net/http"
	"testing"
)

func TestParseClaudeUnifiedHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("anthropic-ratelimit-unified-status", "allowed")
	h.Set("anthropic-ratelimit-unified-5h-utilization", "0.42")
	h.Set("anthropic-ratelimit-unified-5h-reset", "1800000000")
	h.Set("anthropic-ratelimit-unified-7d-utilization", "0.1")
	h.Set("anthropic-ratelimit-unified-reset", "1800000000") // overall reset, not a window
	w := ParseClaudeUnifiedHeaders(h)
	if w == nil || len(w.Windows) != 2 || w.Status != "allowed" || w.Source != "headers" {
		t.Fatalf("windows = %+v", w)
	}
	five, seven := w.Windows[0], w.Windows[1]
	if five.Name != "5h" || five.UsedPercent != 42 || five.ResetAt != 1800000000 || five.WindowSeconds != 18000 {
		t.Errorf("5h = %+v", five)
	}
	if seven.Name != "7d" || seven.UsedPercent != 10 || seven.WindowSeconds != 604800 {
		t.Errorf("7d = %+v", seven)
	}
	if ParseClaudeUnifiedHeaders(http.Header{"X-Other": {"1"}}) != nil {
		t.Error("headers without windows should give nil")
	}
}

func TestWindowName(t *testing.T) {
	for s, want := range map[int64]string{18000: "5h", 604800: "7d", 90: "90s", 0: ""} {
		if got := WindowName(s); got != want {
			t.Errorf("%d -> %q, want %q", s, got, want)
		}
	}
}
