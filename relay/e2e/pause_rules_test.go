package e2e

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/relay"
)

// pauseNotices records admin notices sent during the e2e tests.
type noticeRecorder struct {
	mu     sync.Mutex
	titles []string
}

func (r *noticeRecorder) Send(_ context.Context, title, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.titles = append(r.titles, title)
	return nil
}

func (r *noticeRecorder) Name() string { return "e2e-pause-recorder" }

func (r *noticeRecorder) count(part string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, title := range r.titles {
		if strings.Contains(title, part) {
			n++
		}
	}
	return n
}

var pauseNotices = &noticeRecorder{}

func init() {
	notify.AddNotifiers(pauseNotices)
}

func storedPauses(t *testing.T) []*model.SupplyPause {
	t.Helper()
	relay.WaitRelayErrorHandling()
	pauses, err := model.ListPauses()
	if err != nil {
		t.Fatal(err)
	}
	return pauses
}

// A primary out of quota: the request that hits it is served by the backup, the primary is
// paused as a whole channel (with a recovery test), later requests skip it, and admins are told.
func TestQuotaPausesChannelAndFailsOver(t *testing.T) {
	const modelName = "pause-quota-model"
	primary := NewFakeProvider(t)
	primary.SetResponse(FakeResponse{Status: http.StatusTooManyRequests,
		JSON: OpenAIErrorJSON("You exceeded your current quota, please check your plan and billing details.", "insufficient_quota", "insufficient_quota")})
	backup := NewFakeProvider(t)
	backup.SetResponse(FakeResponse{JSON: ChatCompletionJSON(modelName, "ok", 11, 7)})
	h := NewHarness(t, HarnessOptions{Model: modelName, Channels: []ChannelSpec{
		{Name: "primary", BaseURL: primary.Server.URL, Priority: 10},
		{Name: "backup", BaseURL: backup.Server.URL, Priority: 0},
	}})
	setAndRestore(t, &config.RetryTimes, 2)
	pausedBefore := pauseNotices.count("paused")

	for i := 0; i < 3; i++ {
		if w := h.Post("/v1/chat/completions", chatFor(modelName)); w.Code != http.StatusOK {
			t.Fatalf("request %d: status %d: %s", i, w.Code, w.Body.String())
		}
	}
	if len(primary.Requests()) != 1 || len(backup.Requests()) != 3 {
		t.Fatalf("requests: primary %d, backup %d; want the primary tried once", len(primary.Requests()), len(backup.Requests()))
	}
	pauses := storedPauses(t)
	if len(pauses) != 1 || pauses[0].ChannelId != h.Channels[0].Id || pauses[0].Model != "" ||
		pauses[0].Rule != "quota" || !pauses[0].Probe || pauses[0].Seconds != 1800 {
		t.Fatalf("pauses = %+v, want one 30-minute whole-channel quota pause with a probe", pauses)
	}
	if got := pauseNotices.count("paused") - pausedBefore; got != 1 {
		t.Fatalf("paused notices = %d, want 1", got)
	}
}

// An upstream failure pauses the model on that channel only on the third failure in a minute.
func TestRepeatedUpstreamFailurePausesModel(t *testing.T) {
	const modelName = "pause-5xx-model"
	upstream := NewFakeProvider(t)
	upstream.SetResponse(FakeResponse{Status: http.StatusServiceUnavailable, JSON: OpenAIErrorJSON("overloaded", "server_error", "")})
	h := NewHarness(t, HarnessOptions{Model: modelName, FakeBaseURL: upstream.Server.URL})

	for i := 1; i <= 3; i++ {
		h.Post("/v1/chat/completions", chatFor(modelName))
		paused := model.IsPaused(h.Channel.Id, modelName)
		if paused != (i == 3) {
			t.Fatalf("after failure %d: paused=%v, want paused only after the third", i, paused)
		}
	}
	pauses := storedPauses(t)
	if len(pauses) != 1 || pauses[0].Model != modelName || pauses[0].Rule != "upstream_failure" || pauses[0].Seconds != 120 {
		t.Fatalf("pauses = %+v, want a 2-minute pause of the model", pauses)
	}
	if model.IsPaused(h.Channel.Id, "another-model") {
		t.Fatal("a model pause blocked another model on the channel")
	}
}

// A plain 429 is still a short in-memory cooldown: nothing is stored or announced.
func TestRateLimitStaysShortCooldown(t *testing.T) {
	const modelName = "pause-429-model"
	upstream := NewFakeProvider(t)
	upstream.SetResponse(FakeResponse{Status: http.StatusTooManyRequests, JSON: OpenAIErrorJSON("slow down", "rate_limit_error", "")})
	h := NewHarness(t, HarnessOptions{Model: modelName, FakeBaseURL: upstream.Server.URL})

	h.Post("/v1/chat/completions", chatFor(modelName))
	if !model.ChannelGroup.IsInCooldown(h.Channel.Id, modelName) {
		t.Fatal("429 did not cool the model down")
	}
	if pauses := storedPauses(t); len(pauses) != 0 {
		t.Fatalf("a 5-second cooldown was stored: %+v", pauses)
	}
}

// With the rules off, an exhausted quota does what it did before: no pause (automatic
// disabling is off by default), so the next request tries the primary again.
func TestPauseRulesSwitchOff(t *testing.T) {
	const modelName = "pause-off-model"
	primary := NewFakeProvider(t)
	primary.SetResponse(FakeResponse{Status: http.StatusTooManyRequests,
		JSON: OpenAIErrorJSON("You exceeded your current quota", "insufficient_quota", "insufficient_quota")})
	backup := NewFakeProvider(t)
	backup.SetResponse(FakeResponse{JSON: ChatCompletionJSON(modelName, "ok", 11, 7)})
	h := NewHarness(t, HarnessOptions{Model: modelName, Channels: []ChannelSpec{
		{Name: "primary", BaseURL: primary.Server.URL, Priority: 10},
		{Name: "backup", BaseURL: backup.Server.URL, Priority: 0},
	}})
	setAndRestore(t, &config.PauseRulesEnabled, false)
	setAndRestore(t, &config.RetryTimes, 2)
	setAndRestore(t, &config.RetryCooldownSeconds, 0)

	for i := 0; i < 2; i++ {
		if w := h.Post("/v1/chat/completions", chatFor(modelName)); w.Code != http.StatusOK {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
	}
	if pauses := storedPauses(t); len(pauses) != 0 {
		t.Fatalf("pauses stored with the rules off: %+v", pauses)
	}
	if len(primary.Requests()) != 2 {
		t.Fatalf("primary got %d requests, want 2 (nothing keeps it out with the rules off)", len(primary.Requests()))
	}
}
