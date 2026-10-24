package notify

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
)

// Kinds of admin notices. Each one can be turned off in the notification settings
// (config.NotifyDisabledKinds) and is rate limited separately (config.NotifyKindLimitPerHour).
const (
	KindChannelDisabled  = "channel_disabled"
	KindChannelEnabled   = "channel_enabled"
	KindChannelTest      = "channel_test"
	KindLowBalance       = "low_balance"
	KindPriceChange      = "price_change"
	KindModelDrift       = "model_drift"
	KindModelConsistency = "model_consistency"
	KindCredential       = "credential"
)

// Kinds lists every notice kind, in the order the settings page shows them.
var Kinds = []string{
	KindChannelDisabled, KindChannelEnabled, KindChannelTest, KindLowBalance,
	KindPriceChange, KindModelDrift, KindModelConsistency, KindCredential,
}

// KindEnabled reports whether notices of this kind are sent.
func KindEnabled(kind string) bool {
	for _, k := range strings.Split(config.NotifyDisabledKinds, ",") {
		if strings.TrimSpace(k) == kind {
			return false
		}
	}
	return true
}

type kindWindow struct {
	start      time.Time
	sent       int
	suppressed int
}

var (
	limiterMu sync.Mutex
	windows   = map[string]*kindWindow{}
	nowFunc   = time.Now
)

// allow applies the per-kind hourly limit. It returns whether the notice may be sent and how
// many notices of this kind were dropped in the previous window (reported once).
func allow(kind string) (ok bool, droppedBefore int) {
	limit := config.NotifyKindLimitPerHour
	limiterMu.Lock()
	defer limiterMu.Unlock()
	now := nowFunc()
	w := windows[kind]
	if w == nil || now.Sub(w.start) >= time.Hour {
		if w != nil {
			droppedBefore = w.suppressed
		}
		w = &kindWindow{start: now}
		windows[kind] = w
	}
	if limit > 0 && w.sent >= limit {
		w.suppressed++
		return false, 0
	}
	w.sent++
	return true, droppedBefore
}

// SendKind sends an admin notice of the given kind, unless that kind is turned off or has hit
// its hourly limit. The first notice after a limited hour says how many were dropped.
func SendKind(kind, title, message string) {
	if !KindEnabled(kind) {
		return
	}
	ok, dropped := allow(kind)
	if !ok {
		logger.SysLog(fmt.Sprintf("notice %s dropped by the hourly limit: %s", kind, title))
		return
	}
	if dropped > 0 {
		message = fmt.Sprintf("%s\n\n(%d more notices of this kind were dropped in the previous hour by the notification limit.)", message, dropped)
	}
	//lint:ignore SA1029 reason: 需要使用该类型作为错误处理
	ctx := context.WithValue(context.Background(), logger.RequestIdKey, "NotifyTask")
	notifyChannels.Send(ctx, title, message)
}
