package notify

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
)

type kindRecorder struct {
	mu   sync.Mutex
	msgs []string
}

func (r *kindRecorder) Send(_ context.Context, _ string, message string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, message)
	return nil
}

func (r *kindRecorder) Name() string { return "kind-test" }

func (r *kindRecorder) take() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.msgs
	r.msgs = nil
	return out
}

func TestSendKind(t *testing.T) {
	rec := &kindRecorder{}
	AddNotifiers(rec)
	oldDisabled, oldLimit, oldNow := config.NotifyDisabledKinds, config.NotifyKindLimitPerHour, nowFunc
	t.Cleanup(func() { config.NotifyDisabledKinds, config.NotifyKindLimitPerHour, nowFunc = oldDisabled, oldLimit, oldNow })
	now := time.Unix(1_000_000, 0)
	nowFunc = func() time.Time { return now }
	windows = map[string]*kindWindow{}

	config.NotifyDisabledKinds = "price_change, model_drift"
	SendKind(KindPriceChange, "t", "m")
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("disabled kind sent: %v", got)
	}

	config.NotifyKindLimitPerHour = 2
	for i := 0; i < 5; i++ {
		SendKind(KindLowBalance, "t", "m")
	}
	if got := rec.take(); len(got) != 2 {
		t.Fatalf("sent %d, want 2 within the hour", len(got))
	}
	// another kind has its own budget
	SendKind(KindChannelDisabled, "t", "m")
	if got := rec.take(); len(got) != 1 {
		t.Fatalf("other kind sent %d", len(got))
	}

	now = now.Add(time.Hour)
	SendKind(KindLowBalance, "t", "next")
	got := rec.take()
	if len(got) != 1 || !containsStr(got[0], "3 more notices") {
		t.Fatalf("after the window: %v", got)
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
