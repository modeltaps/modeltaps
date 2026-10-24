package e2e

import (
	"net/http"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// TestChannelMetricsRecorded: a request that failed on one channel and succeeded on another
// counts one failed attempt for the first and one success for the second.
func TestChannelMetricsRecorded(t *testing.T) {
	failing := NewFakeProvider(t)
	failing.SetResponse(FakeResponse{Status: http.StatusInternalServerError, JSON: OpenAIErrorJSON("boom", "server_error", "internal_error")})
	healthy := NewFakeProvider(t)
	healthy.SetResponse(FakeResponse{JSON: ChatCompletionJSON("gpt-4o-mini", "ok", 5, 3)})
	h := NewHarness(t, HarnessOptions{Channels: []ChannelSpec{
		{Name: "failing", BaseURL: failing.Server.URL, Priority: 10},
		{Name: "healthy", BaseURL: healthy.Server.URL, Priority: 1},
	}})
	setAndRestore(t, &config.RetryTimes, 3)
	setAndRestore(t, &config.ChannelMetricsEnabled, true)
	// counters left by earlier tests land in this database on flush; clear them first
	model.FlushChannelMetrics()
	model.DB.Where("1 = 1").Delete(&model.ChannelMetric{})

	if w := h.Post("/v1/chat/completions", `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`); w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	model.FlushChannelMetrics()
	now := time.Now().Unix()
	rows, err := model.GetChannelMetrics(now-3600, now, 0, "gpt-4o-mini")
	if err != nil {
		t.Fatal(err)
	}
	got := map[int]model.ChannelMetric{}
	for _, r := range rows {
		got[r.ChannelId] = *r
	}
	f, s := got[h.Channels[0].Id], got[h.Channels[1].Id]
	if f.RequestCount != 1 || f.SuccessCount != 0 || s.RequestCount != 1 || s.SuccessCount != 1 {
		t.Fatalf("failing=%+v healthy=%+v", f, s)
	}
}
