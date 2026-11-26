package controller

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	providersBase "github.com/modeltaps/modeltaps/providers/base"
)

// The scheduled reads of a channel's upstream account (the balance and the account cost report)
// run unattended, so a failure must not turn into a stream of retries:
//
//   - A refused key (HTTP 401/403) stops the read for good. Some upstreams count every refused
//     request toward locking the whole account, and the same key is refused again anyway. The
//     refusal is kept as a key_rejected alert and told once; the scheduled read skips the channel
//     while the alert is open. Saving the channel with a different key closes the alert, and so
//     does a manual refresh that works.
//   - Any other failure (a timeout, an upstream error) is retried later and later: after n
//     failures in a row the next scheduled read waits backgroundReadBackoff(n).

// keyRejectedAlertKind is the alert kind of a channel whose upstream refused its key on a
// scheduled read. Its items are the reads that were refused.
const keyRejectedAlertKind = "key_rejected"

// The reads a key_rejected alert can name.
const (
	backgroundReadBalance   = "balance"
	backgroundReadDailyCost = "daily_cost"
)

func keyRejectedSubject(channelID int) string {
	return fmt.Sprintf("channel:%d", channelID)
}

// backgroundReadStopped reports whether the upstream refused the channel's key for this read and
// the key has not changed since.
func backgroundReadStopped(channel *model.Channel, read string) bool {
	items, err := model.AlertItems(keyRejectedAlertKind, keyRejectedSubject(channel.Id))
	if err != nil {
		// Unsure: do not ask an upstream that may be counting refusals.
		logger.SysError("key rejected alert: " + err.Error())
		return true
	}
	return slices.Contains(items, read)
}

// noteBackgroundRead records how a read of the channel went: a refused key stops the read and is
// told once; success clears that read's stop and backoff; anything else backs off.
func noteBackgroundRead(channel *model.Channel, read string, err error) {
	key := backoffKey(channel.Id, read)
	switch {
	case err == nil:
		resetBackgroundBackoff(key)
		resolveKeyRejected(channel.Id, read)
	case errors.Is(err, providersBase.ErrKeyRejected):
		resetBackgroundBackoff(key)
		reportKeyRejected(channel, read, err)
	default:
		recordBackgroundFailure(key)
	}
}

func reportKeyRejected(channel *model.Channel, read string, err error) {
	subject := keyRejectedSubject(channel.Id)
	items, _ := model.AlertItems(keyRejectedAlertKind, subject)
	fresh, alertErr := model.ReportAlert(keyRejectedAlertKind, subject, append(slices.Clone(items), read))
	if alertErr != nil {
		logger.SysError("key rejected alert: " + alertErr.Error())
		return
	}
	reason := utils.MaskSensitiveInfo(err.Error())
	logger.SysError(fmt.Sprintf("the upstream refused the key of channel %s (ID %d) for the %s; scheduled reads stop until the key changes: %s",
		channel.Name, channel.Id, read, reason))
	if len(fresh) == 0 {
		return
	}
	notify.SendKind(notify.KindCredential,
		fmt.Sprintf("Channel %s: the upstream refused the key", channel.Name),
		fmt.Sprintf("Channel: %s (ID %d)\nRead: %s\nError: %s\n\nThe scheduled %s read of this channel has stopped, so it does not add failed attempts that the upstream may count toward locking the account. Fix the channel's key or management key; saving the channel with a new key starts the read again.",
			channel.Name, channel.Id, read, reason, read))
}

func resolveKeyRejected(channelID int, read string) {
	subject := keyRejectedSubject(channelID)
	items, err := model.AlertItems(keyRejectedAlertKind, subject)
	if err != nil || !slices.Contains(items, read) {
		return
	}
	rest := slices.DeleteFunc(slices.Clone(items), func(item string) bool { return item == read })
	if _, err := model.ReportAlert(keyRejectedAlertKind, subject, rest); err != nil {
		logger.SysError("key rejected alert: " + err.Error())
	}
}

// clearKeyRejectedOnKeyChange closes the channel's key_rejected alert when an edit changed its
// key or management key, so the scheduled reads try the new key.
func clearKeyRejectedOnKeyChange(old *model.Channel, channelID int) {
	if old == nil {
		return
	}
	fresh, err := model.GetChannelById(channelID)
	if err != nil {
		return
	}
	if fresh.Key == old.Key && fresh.GetManageKey() == old.GetManageKey() {
		return
	}
	if _, err := model.ReportAlert(keyRejectedAlertKind, keyRejectedSubject(channelID), nil); err != nil {
		logger.SysError("key rejected alert: " + err.Error())
	}
	resetBackgroundBackoff(backoffKey(channelID, backgroundReadBalance))
	resetBackgroundBackoff(backoffKey(channelID, backgroundReadDailyCost))
}

// backgroundReadBackoff is how long a read waits after n failures in a row: an hour, doubling
// each time, at most a day.
func backgroundReadBackoff(n int) time.Duration {
	if n <= 0 {
		return 0
	}
	d := time.Hour
	for i := 1; i < n && d < 24*time.Hour; i++ {
		d *= 2
	}
	return min(d, 24*time.Hour)
}

type backgroundBackoff struct {
	failures int
	nextAt   time.Time
}

var (
	backgroundBackoffMu sync.Mutex
	backgroundBackoffs  = map[string]*backgroundBackoff{}
	backgroundNow       = time.Now
)

func backoffKey(channelID int, read string) string {
	return fmt.Sprintf("%d:%s", channelID, read)
}

// backgroundReadDue reports whether a read that failed before may run again now.
func backgroundReadDue(channelID int, read string) bool {
	backgroundBackoffMu.Lock()
	defer backgroundBackoffMu.Unlock()
	b := backgroundBackoffs[backoffKey(channelID, read)]
	return b == nil || !backgroundNow().Before(b.nextAt)
}

func recordBackgroundFailure(key string) {
	backgroundBackoffMu.Lock()
	defer backgroundBackoffMu.Unlock()
	b := backgroundBackoffs[key]
	if b == nil {
		b = &backgroundBackoff{}
		backgroundBackoffs[key] = b
	}
	b.failures++
	b.nextAt = backgroundNow().Add(backgroundReadBackoff(b.failures))
}

func resetBackgroundBackoff(key string) {
	backgroundBackoffMu.Lock()
	defer backgroundBackoffMu.Unlock()
	delete(backgroundBackoffs, key)
}

// backgroundReadAllowed is the check before a scheduled read: not stopped by a refused key and
// not waiting out a backoff.
func backgroundReadAllowed(channel *model.Channel, read string) bool {
	return backgroundReadDue(channel.Id, read) && !backgroundReadStopped(channel, read)
}
