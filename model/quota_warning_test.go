package model

import (
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
)

func resetQuotaWarningLocalDedup() {
	quotaWarningLocalDedup.Lock()
	quotaWarningLocalDedup.expireAt = map[string]time.Time{}
	quotaWarningLocalDedup.Unlock()
}

type sentWarning struct {
	userId      int
	quota       int
	noMoreQuota bool
}

// captureQuotaWarningEmails swaps the sender for a synchronous collector, so the cases are deterministic.
func captureQuotaWarningEmails(t *testing.T) func() []sentWarning {
	t.Helper()
	var sent []sentWarning
	orig := sendQuotaWarningEmailFunc
	sendQuotaWarningEmailFunc = func(userId int, quota int, noMoreQuota bool) {
		sent = append(sent, sentWarning{userId, quota, noMoreQuota})
	}
	t.Cleanup(func() { sendQuotaWarningEmailFunc = orig })
	return func() []sentWarning { return sent }
}

// fakeQuotaWarningClock replaces the clock of the dedupe windows with one the test moves.
func fakeQuotaWarningClock(t *testing.T) func(d time.Duration) {
	t.Helper()
	now := time.Unix(1_000_000, 0)
	orig := quotaWarningNow
	quotaWarningNow = func() time.Time { return now }
	t.Cleanup(func() { quotaWarningNow = orig })
	return func(d time.Duration) { now = now.Add(d) }
}

// setupQuotaWarningTest fixes the threshold at 500000 ($1), the pre-charge at 500 and Redis off.
// Rearm levels: "running low" at 500000 + max(50000, 500) = 550000, "used up" above 500.
func setupQuotaWarningTest(t *testing.T) {
	t.Helper()
	resetQuotaWarningLocalDedup()
	origEnabled, origThreshold, origRedis, origPre := config.QuotaRemindEnabled, config.QuotaRemindThreshold, config.RedisEnabled, config.PreConsumedQuota
	config.QuotaRemindEnabled = true
	config.QuotaRemindThreshold = 500_000
	config.RedisEnabled = false
	config.PreConsumedQuota = 500
	t.Cleanup(func() {
		config.QuotaRemindEnabled, config.QuotaRemindThreshold, config.RedisEnabled, config.PreConsumedQuota = origEnabled, origThreshold, origRedis, origPre
		resetQuotaWarningLocalDedup()
	})
}

// drain charges from `from` down to `to` in fixed steps, like real small charges.
func drain(userId, from, to, step int) {
	for q := from; q > to; q -= step {
		d := step
		if q-d < to {
			d = q - to
		}
		notifyQuotaWarningIfNeeded(userId, q, d)
	}
}

func TestEvaluateQuotaWarning(t *testing.T) {
	setupQuotaWarningTest(t)
	cases := []struct {
		name                   string
		after, thresh          int
		state                  quotaWarningState
		lowRearm, exhaustRearm bool
	}{
		{"well above", 1_000_000, 500_000, quotaWarningStateSafe, true, true},
		{"exactly rearm level", 550_000, 500_000, quotaWarningStateSafe, true, true},
		{"just under rearm level", 549_999, 500_000, quotaWarningStateBand, false, true},
		{"exactly threshold", 500_000, 500_000, quotaWarningStateBand, false, true},
		{"just below threshold", 499_999, 500_000, quotaWarningStateLow, false, true},
		{"low but above refund margin", 501, 500_000, quotaWarningStateLow, false, true},
		{"low within refund margin", 500, 500_000, quotaWarningStateLow, false, false},
		{"exhausted zero", 0, 500_000, quotaWarningStateExhausted, false, false},
		{"exhausted negative", -37, 500_000, quotaWarningStateExhausted, false, false},
		{"threshold zero positive within margin", 1, 0, quotaWarningStateBand, false, false},
		{"threshold zero above margin", 501, 0, quotaWarningStateSafe, true, true},
		{"threshold zero exhausted", 0, 0, quotaWarningStateExhausted, false, false},
		{"small threshold uses refund margin", 1_600, 1_000, quotaWarningStateSafe, true, true},
		{"small threshold band", 1_499, 1_000, quotaWarningStateBand, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state, low, ex := evaluateQuotaWarning(c.after, c.thresh)
			if state != c.state || low != c.lowRearm || ex != c.exhaustRearm {
				t.Fatalf("evaluateQuotaWarning(%d,%d) = (%v,%v,%v), want (%v,%v,%v)",
					c.after, c.thresh, state, low, ex, c.state, c.lowRearm, c.exhaustRearm)
			}
		})
	}
}

// Small charges cross the threshold, a top-up brings the balance back, small charges cross again:
// two warnings on the same day.
func TestNotifyQuotaWarning_SmallStepsRecrossAfterTopUp(t *testing.T) {
	setupQuotaWarningTest(t)
	sent := captureQuotaWarningEmails(t)
	advance := fakeQuotaWarningClock(t)

	drain(1, 540_000, 450_000, 10_000) // each charge 10000, far below the 50000 margin
	advance(2 * time.Hour)
	drain(1, 1_000_000, 450_000, 10_000) // drops again after a top-up

	got := sent()
	if len(got) != 2 || got[0].noMoreQuota || got[1].noMoreQuota {
		t.Fatalf("expected 2 low warnings, got %+v", got)
	}
	if got[0].quota != 490_000 || got[1].quota != 490_000 {
		t.Fatalf("expected both sent on first step below threshold, got %+v", got)
	}
}

// Small charges down to zero, a top-up, down to zero again: "used up" and "running low" twice each.
func TestNotifyQuotaWarning_SmallStepsExhaustTwice(t *testing.T) {
	setupQuotaWarningTest(t)
	sent := captureQuotaWarningEmails(t)
	advance := fakeQuotaWarningClock(t)

	drain(2, 520_000, 0, 300) // the last charges start below 500
	advance(time.Hour)
	drain(2, 600_000, -37, 300)

	got := sent()
	want := []sentWarning{{2, 499_900, false}, {2, 0, true}, {2, 499_800, false}, {2, 0, true}}
	if len(got) != len(want) {
		t.Fatalf("expected %d warnings, got %+v", len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("warning[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// Refunds bouncing the balance around the threshold do not send again: back into the
// 500000-549999 band does not rearm.
func TestNotifyQuotaWarning_BounceInBandDoesNotRefire(t *testing.T) {
	setupQuotaWarningTest(t)
	sent := captureQuotaWarningEmails(t)
	advance := fakeQuotaWarningClock(t)

	notifyQuotaWarningIfNeeded(3, 500_200, 300) // crosses, one email
	for i := 0; i < 6; i++ {
		advance(11 * time.Minute)                   // the burst lock has expired
		notifyQuotaWarningIfNeeded(3, 500_400, 500) // refunded to 500400, charged down to 499900
	}

	if got := sent(); len(got) != 1 {
		t.Fatalf("expected band bounce to be absorbed, got %+v", got)
	}
}

// Same near zero: back to 1-500 does not rearm "used up".
func TestNotifyQuotaWarning_BounceNearZeroDoesNotRefire(t *testing.T) {
	setupQuotaWarningTest(t)
	sent := captureQuotaWarningEmails(t)
	advance := fakeQuotaWarningClock(t)

	notifyQuotaWarningIfNeeded(4, 200, 200)
	for i := 0; i < 6; i++ {
		advance(11 * time.Minute)
		notifyQuotaWarningIfNeeded(4, 400, 400)
	}

	got := sent()
	if len(got) != 1 || !got[0].noMoreQuota {
		t.Fatalf("expected one exhausted warning, got %+v", got)
	}
}

// Concurrent requests read the same balance and all see the crossing: one email.
func TestNotifyQuotaWarning_ConcurrentDuplicateSuppressed(t *testing.T) {
	setupQuotaWarningTest(t)
	sent := captureQuotaWarningEmails(t)

	for i := 0; i < 3; i++ {
		notifyQuotaWarningIfNeeded(5, 500_200, 300)
	}

	if got := sent(); len(got) != 1 {
		t.Fatalf("expected 1 warning, got %+v", got)
	}
}

// The pre-charge crossed and sent; the final settlement, still below the threshold, is held by the
// "sent" lock.
func TestNotifyQuotaWarning_PostConsumeAfterPreConsumeSuppressed(t *testing.T) {
	setupQuotaWarningTest(t)
	sent := captureQuotaWarningEmails(t)
	advance := fakeQuotaWarningClock(t)

	notifyQuotaWarningIfNeeded(6, 500_200, 300) // pre-charge
	advance(11 * time.Minute)
	notifyQuotaWarningIfNeeded(6, 499_900, 50) // final settlement

	if got := sent(); len(got) != 1 {
		t.Fatalf("expected post-consume to be suppressed, got %+v", got)
	}
}

// A threshold above the balance never reaches the rearm level; the 24h cap sends one a day.
func TestNotifyQuotaWarning_ThresholdAboveBalanceDailyFallback(t *testing.T) {
	setupQuotaWarningTest(t)
	sent := captureQuotaWarningEmails(t)
	advance := fakeQuotaWarningClock(t)

	config.QuotaRemindThreshold = 5_000_000_000 // $10000
	for i := 0; i < 5; i++ {
		notifyQuotaWarningIfNeeded(7, 200_000-i*500, 500)
	}
	advance(25 * time.Hour)
	notifyQuotaWarningIfNeeded(7, 190_000, 500)

	got := sent()
	if len(got) != 2 || got[0].noMoreQuota || got[1].noMoreQuota {
		t.Fatalf("expected one warning per 24h window, got %+v", got)
	}
	if got[0].quota != 199_500 {
		t.Fatalf("email should carry post-charge balance, got %+v", got[0])
	}
}

func TestNotifyQuotaWarning_Skips(t *testing.T) {
	t.Run("global disabled", func(t *testing.T) {
		setupQuotaWarningTest(t)
		config.QuotaRemindEnabled = false
		sent := captureQuotaWarningEmails(t)
		notifyQuotaWarningIfNeeded(12, 500_200, 300)
		if got := sent(); len(got) != 0 {
			t.Fatalf("expected none, got %+v", got)
		}
	})
	t.Run("refund delta", func(t *testing.T) {
		setupQuotaWarningTest(t)
		sent := captureQuotaWarningEmails(t)
		notifyQuotaWarningIfNeeded(13, 400_000, -100)
		if got := sent(); len(got) != 0 {
			t.Fatalf("expected none, got %+v", got)
		}
	})
	t.Run("balance healthy", func(t *testing.T) {
		setupQuotaWarningTest(t)
		sent := captureQuotaWarningEmails(t)
		notifyQuotaWarningIfNeeded(14, 10_000_000, 100)
		if got := sent(); len(got) != 0 {
			t.Fatalf("expected none, got %+v", got)
		}
	})
	t.Run("threshold zero only exhausted", func(t *testing.T) {
		setupQuotaWarningTest(t)
		config.QuotaRemindThreshold = 0
		sent := captureQuotaWarningEmails(t)
		notifyQuotaWarningIfNeeded(15, 50, 10)
		if got := sent(); len(got) != 0 {
			t.Fatalf("expected none, got %+v", got)
		}
	})
}

// With Redis the local hold must not outlive the burst window, or a rearm on another node has no
// effect here.
func TestQuotaWarningLocalTTL_CappedWhenRedisEnabled(t *testing.T) {
	orig := config.RedisEnabled
	t.Cleanup(func() { config.RedisEnabled = orig })

	config.RedisEnabled = false
	if got := quotaWarningLocalTTL(quotaWarningSentTTL); got != quotaWarningSentTTL {
		t.Fatalf("without redis local ttl should be full %v, got %v", quotaWarningSentTTL, got)
	}

	config.RedisEnabled = true
	if got := quotaWarningLocalTTL(quotaWarningSentTTL); got != quotaWarningBurstTTL {
		t.Fatalf("with redis local ttl should be capped to %v, got %v", quotaWarningBurstTTL, got)
	}
	if got := quotaWarningLocalTTL(quotaWarningBurstTTL); got != quotaWarningBurstTTL {
		t.Fatalf("burst ttl should pass through, got %v", got)
	}
}

func TestQuotaWarningLocalAcquire_ExpiresAndPurges(t *testing.T) {
	resetQuotaWarningLocalDedup()
	defer resetQuotaWarningLocalDedup()

	now := time.Unix(1_000_000, 0)
	if !quotaWarningLocalAcquire("a", time.Minute, now) {
		t.Fatal("first acquire should succeed")
	}
	if quotaWarningLocalAcquire("a", time.Minute, now.Add(30*time.Second)) {
		t.Fatal("second acquire within window should fail")
	}
	if !quotaWarningLocalAcquire("a", time.Minute, now.Add(time.Minute)) {
		t.Fatal("acquire after expiry should succeed")
	}
	// expired keys are swept on insert, so the map does not grow without bound
	quotaWarningLocalAcquire("b", time.Minute, now.Add(2*time.Minute))
	quotaWarningLocalAcquire("c", time.Minute, now.Add(10*time.Minute))
	quotaWarningLocalDedup.Lock()
	n := len(quotaWarningLocalDedup.expireAt)
	quotaWarningLocalDedup.Unlock()
	if n != 1 {
		t.Fatalf("expected only live key to remain, got %d entries", n)
	}
}
