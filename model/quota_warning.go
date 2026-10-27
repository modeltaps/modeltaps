package model

import (
	"fmt"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/redis"
)

// A quota warning is a state change: one email when the balance drops below the threshold (or runs
// out), and another only after the balance has clearly come back above it and drops again. Each
// warning kind has a "sent" lock that must be taken (SETNX) to send; once the balance is back at
// the rearm level (threshold plus a margin, or above the refund margin for "used up") the lock is
// released. The hysteresis follows the balance, not the size of a single charge, so it also works
// for many small charges.
//   - quotaWarningSentTTL caps the "sent" lock, so a balance that stays below the threshold (say the
//     threshold is set above it) still gets one reminder a day.
//   - quotaWarningBurstTTL only dedupes concurrent requests and covers a release/acquire race across
//     nodes. A drop again within 10 minutes of a rearm is not reported; the user just got the same
//     email.
const (
	quotaWarningSentTTL  = 24 * time.Hour
	quotaWarningBurstTTL = 10 * time.Minute
)

var (
	// quotaWarningLocalDedup is the in-process dedupe, used when Redis is off or SETNX fails, so a
	// low balance never sends one email per request.
	quotaWarningLocalDedup = struct {
		sync.Mutex
		expireAt map[string]time.Time
	}{expireAt: map[string]time.Time{}}

	// quotaWarningNow is the clock of the dedupe windows; tests move it.
	quotaWarningNow = time.Now

	// sendQuotaWarningEmailFunc sends the email asynchronously; tests swap in a collector.
	sendQuotaWarningEmailFunc = func(userId int, quota int, noMoreQuota bool) {
		go sendQuotaWarningEmail(userId, quota, noMoreQuota)
	}
)

func quotaWarningSentKey(userId int, noMoreQuota bool) string {
	return fmt.Sprintf("notify_lock:quota_warning:%d:%t", userId, noMoreQuota)
}

func quotaWarningBurstKey(userId int, noMoreQuota bool) string {
	return fmt.Sprintf("notify_lock:quota_warning:burst:%d:%t", userId, noMoreQuota)
}

// quotaWarningLocalAcquire takes key once in this process; it returns false while the key is held.
// A hit returns at once without a sweep: every request of a low-balance user lands here, so the
// O(n) cleanup under the lock only runs on an insert, which happens on a state change.
func quotaWarningLocalAcquire(key string, ttl time.Duration, now time.Time) bool {
	d := &quotaWarningLocalDedup
	d.Lock()
	defer d.Unlock()
	if exp, ok := d.expireAt[key]; ok && now.Before(exp) {
		return false
	}
	for k, exp := range d.expireAt {
		if !now.Before(exp) {
			delete(d.expireAt, k)
		}
	}
	d.expireAt[key] = now.Add(ttl)
	return true
}

func quotaWarningLocalRelease(key string) {
	d := &quotaWarningLocalDedup
	d.Lock()
	delete(d.expireAt, key)
	d.Unlock()
}

// quotaWarningLocalTTL is how long the in-process hold lives. Without Redis it is the only state
// and keeps the full ttl. With Redis the shared state wins and the local hold only covers a failed
// SETNX, so it must not outlive the burst window: a rearm on another node deletes the Redis key
// only, and a 24h local hold would keep this node silent.
func quotaWarningLocalTTL(ttl time.Duration) time.Duration {
	if config.RedisEnabled {
		return min(ttl, quotaWarningBurstTTL)
	}
	return ttl
}

// acquireQuotaWarningLock passes the in-process window first, then the Redis window shared by all
// nodes. A failed SETNX sends rather than silently drops the warning.
func acquireQuotaWarningLock(key string, ttl time.Duration) bool {
	if !quotaWarningLocalAcquire(key, quotaWarningLocalTTL(ttl), quotaWarningNow()) {
		return false
	}
	if !config.RedisEnabled {
		return true
	}
	ok, err := redis.RedisSetNX(key, "1", ttl)
	if err != nil {
		logger.SysError(fmt.Sprintf("quota warning notify dedup SETNX failed (key=%s): %v", key, err))
		return true
	}
	return ok
}

// shouldSendQuotaWarningNotify takes the "sent" lock, then the burst lock.
func shouldSendQuotaWarningNotify(userId int, noMoreQuota bool) bool {
	if !acquireQuotaWarningLock(quotaWarningSentKey(userId, noMoreQuota), quotaWarningSentTTL) {
		return false
	}
	return acquireQuotaWarningLock(quotaWarningBurstKey(userId, noMoreQuota), quotaWarningBurstTTL)
}

// rearmQuotaWarning releases the "sent" locks once the balance is back at a safe level, so the
// next drop sends again. Top-ups happen outside the charge path, so the rise itself is never seen
// here; every charge that leaves a healthy balance releases unconditionally. That is a map delete
// and one Redis DEL, small next to the database and cache writes of the charge itself.
func rearmQuotaWarning(userId int, low bool, exhausted bool) {
	keys := make([]string, 0, 2)
	if low {
		keys = append(keys, quotaWarningSentKey(userId, false))
	}
	if exhausted {
		keys = append(keys, quotaWarningSentKey(userId, true))
	}
	if len(keys) == 0 {
		return
	}
	for _, k := range keys {
		quotaWarningLocalRelease(k)
	}
	if config.RedisEnabled {
		if err := redis.RedisDel(keys...); err != nil {
			logger.SysError(fmt.Sprintf("quota warning notify rearm failed (user=%d): %v", userId, err))
		}
	}
}

// quotaWarningRefundMargin is the rearm margin of the "used up" warning. Each settlement may
// refund the part of the pre-charge it did not use, so a balance bouncing around zero is normal;
// the refund is at most the fixed pre-charge PreConsumedQuota.
func quotaWarningRefundMargin() int {
	if config.PreConsumedQuota < 0 {
		return 0
	}
	return config.PreConsumedQuota
}

// quotaWarningThresholdMargin is the rearm margin of the "running low" warning: the refund margin,
// or 10% of the threshold when that is larger, so small swings around the threshold do not send
// again.
func quotaWarningThresholdMargin(threshold int) int {
	return max(quotaWarningRefundMargin(), threshold/10)
}

// quotaWarningState is where the balance is after a charge.
type quotaWarningState int

const (
	quotaWarningStateSafe      quotaWarningState = iota // above threshold + margin: rearm both warnings
	quotaWarningStateBand                               // hysteresis band around the threshold: do nothing
	quotaWarningStateLow                                // below threshold, not used up: send "running low"
	quotaWarningStateExhausted                          // used up: send "used up"
)

// evaluateQuotaWarning classifies the balance after a charge. lowRearm reports whether it is high
// enough to rearm "running low", exhaustedRearm the same for "used up"; they are independent, so a
// large threshold does not raise the rearm level of "used up".
func evaluateQuotaWarning(quotaAfter, threshold int) (state quotaWarningState, lowRearm bool, exhaustedRearm bool) {
	exhaustedRearm = quotaAfter > quotaWarningRefundMargin()
	lowRearm = quotaAfter >= threshold+quotaWarningThresholdMargin(threshold)
	switch {
	case quotaAfter <= 0:
		state = quotaWarningStateExhausted
	case quotaAfter < threshold:
		state = quotaWarningStateLow
	case lowRearm:
		state = quotaWarningStateSafe
	default:
		state = quotaWarningStateBand
	}
	return state, lowRearm, exhaustedRearm
}

// loadQuotaBeforeSettlement reads the balance a positive settlement starts from, when a warning may
// be sent. Callers pass remind=false for organization tokens, which skips the read; a shadow
// account found by the read is skipped too.
//
// Limitation with BatchUpdateEnabled: settlements only queue their charge, and the row changes when
// the queue is flushed (every BatchUpdateInterval seconds). Until then every read here misses all
// queued charges, so the balance looks higher than it is and a crossing is reported only by the
// first charge checked after the flush, which can be any number of settlements later, or by a
// pre-charge, which is never queued. If the flushed balance is already far below the threshold,
// the warning sent then shows that balance.
func loadQuotaBeforeSettlement(userId int, remind bool) (int, bool) {
	if !remind || !config.QuotaRemindEnabled {
		return 0, false
	}
	var user User
	if err := DB.Select("quota", "type").Where("id = ?", userId).Take(&user).Error; err != nil {
		logger.SysError(fmt.Sprintf("failed to load the balance for the quota warning (user=%d): %v", userId, err))
		return 0, false
	}
	if IsShadowUser(&user) {
		return 0, false
	}
	return user.Quota, true
}

// notifyQuotaWarningIfNeeded checks after a successful charge whether to send a quota warning.
// quotaBefore is the balance before the charge and delta the amount charged.
func notifyQuotaWarningIfNeeded(userId int, quotaBefore int, delta int) {
	if delta <= 0 || !config.QuotaRemindEnabled {
		return
	}
	quotaAfter := quotaBefore - delta
	state, lowRearm, exhaustedRearm := evaluateQuotaWarning(quotaAfter, config.QuotaRemindThreshold)
	rearmQuotaWarning(userId, lowRearm, exhaustedRearm)
	switch state {
	case quotaWarningStateExhausted:
		if shouldSendQuotaWarningNotify(userId, true) {
			// The final settlement may leave the balance negative; the email never shows that.
			sendQuotaWarningEmailFunc(userId, max(quotaAfter, 0), true)
		}
	case quotaWarningStateLow:
		if shouldSendQuotaWarningNotify(userId, false) {
			sendQuotaWarningEmailFunc(userId, quotaAfter, false)
		}
	}
}
