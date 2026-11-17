package controller

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// rejectInvalidUsageAlertTiers answers with an error and returns true when a channel's usage
// alert tiers are not valid; valid tiers are stored in their normal form.
func rejectInvalidUsageAlertTiers(c *gin.Context, raw *string) bool {
	if raw == nil {
		return false
	}
	normalized, err := model.NormalizeUsageAlertTiers(*raw)
	if err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeUsageAlertTiersInvalid, err.Error(), nil)
		return true
	}
	*raw = normalized
	return false
}

// usageAlertNow is the clock of the usage alerts, replaced in tests.
var usageAlertNow = time.Now

// usageWindowKey names a usage window for its alert state: its name, else its length.
func usageWindowKey(w model.UsageWindow, index int) string {
	if w.Name != "" {
		return w.Name
	}
	if name := model.WindowName(w.WindowSeconds); name != "" {
		return name
	}
	return "window" + strconv.Itoa(index+1)
}

// usageWindowLabel is how a notice names a window: "5-hour window", "7-day window".
func usageWindowLabel(key string) string {
	if len(key) > 1 {
		if n, err := strconv.Atoi(key[:len(key)-1]); err == nil && n > 0 {
			switch key[len(key)-1] {
			case 'h':
				return fmt.Sprintf("%d-hour window", n)
			case 'd':
				return fmt.Sprintf("%d-day window", n)
			}
		}
	}
	return key + " window"
}

func formatPercent(v float64) string {
	return strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64) + "%"
}

// reachedTier is the highest tier the usage has reached, 0 when none.
func reachedTier(tiers []float64, used float64) float64 {
	reached := 0.0
	for _, tier := range tiers {
		if used >= tier {
			reached = tier
		}
	}
	return reached
}

type usageAlertMessage struct {
	title   string
	content string
}

// checkUsageAlerts sends a subscription_usage notice when a usage window of a subscription
// channel reaches one of its alert tiers. Each tier goes out once per window period: the state
// starts over when the window resets (its reset time has passed, or the usage fell back below
// the tier alerted). When the usage jumps past several tiers at once, one notice names the
// highest. Tiers turned off (site or channel "0") send nothing.
func checkUsageAlerts(channel *model.Channel, windows *model.ChannelUsageWindows) {
	if channel == nil || windows == nil {
		return
	}
	tiers := channel.UsageAlertTierList()
	if len(tiers) == 0 {
		return
	}
	now := usageAlertNow().Unix()
	states := channel.UsageAlertStates()
	changed := false
	var notices []usageAlertMessage
	for i, w := range windows.Windows {
		key := usageWindowKey(w, i)
		state, alerted := states[key]
		// The stored usage belongs to a period that has ended (Claude Code only updates it on
		// traffic): the window is back to zero, so there is nothing to alert on.
		if w.ResetAt > 0 && w.ResetAt <= now {
			if alerted {
				delete(states, key)
				changed = true
			}
			continue
		}
		if alerted && ((state.ResetAt > 0 && state.ResetAt <= now) || w.UsedPercent < state.Tier) {
			delete(states, key)
			alerted = false
			changed = true
		}
		tier := reachedTier(tiers, w.UsedPercent)
		if tier == 0 || (alerted && tier <= state.Tier) {
			continue
		}
		states[key] = model.UsageAlertState{Tier: tier, ResetAt: w.ResetAt, AlertedAt: now}
		changed = true
		notices = append(notices, usageAlertNoticeFor(channel, windows, w, key, tier, now))
	}
	if !changed {
		return
	}
	// Record before sending, as the low-balance notice does: a failed write must not turn into
	// a notice on every refresh.
	if err := channel.UpdateUsageAlerts(states); err != nil {
		logger.SysError(fmt.Sprintf("save usage alerts for channel %s (ID %d) failed: %s", channel.Name, channel.Id, err.Error()))
		return
	}
	for _, n := range notices {
		notify.SendKind(notify.KindSubscriptionUsage, n.title, n.content)
	}
}

func usageAlertNoticeFor(channel *model.Channel, windows *model.ChannelUsageWindows, w model.UsageWindow, key string, tier float64, now int64) usageAlertMessage {
	label := usageWindowLabel(key)
	title := fmt.Sprintf("Channel %s: %s usage reached %s", channel.Name, label, formatPercent(tier))
	if tier >= 100 {
		title = fmt.Sprintf("Channel %s: %s usage limit reached", channel.Name, label)
	}
	lines := []string{
		fmt.Sprintf("Channel: %s (ID %d)", channel.Name, channel.Id),
		fmt.Sprintf("Window: %s (%s)", label, key),
		fmt.Sprintf("Used: %s (alert tier %s)", formatPercent(w.UsedPercent), formatPercent(tier)),
	}
	if w.ResetAt > 0 {
		lines = append(lines, "Resets at: "+time.Unix(w.ResetAt, 0).Format(time.RFC3339))
	}
	if windows.PlanType != "" {
		lines = append(lines, "Plan: "+windows.PlanType)
	}
	lines = append(lines, "Checked at: "+time.Unix(now, 0).Format(time.RFC3339))
	return usageAlertMessage{title: title, content: strings.Join(lines, "\n")}
}
