package relay

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/controller"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
)

// lastFailurePausedKey records whether the last failed attempt paused its channel, for the
// retry log's cooldown flag.
const lastFailurePausedKey = "last_failure_paused"

// pauseCounters counts matches of rules with Times, per rule, channel and model.
var pauseCounters = struct {
	sync.Mutex
	hits map[string][]int64
}{hits: map[string][]int64{}}

// countPauseHit records a match and returns how many fell within the window, including this one.
func countPauseHit(key string, window, now int64) int {
	pauseCounters.Lock()
	defer pauseCounters.Unlock()
	kept := pauseCounters.hits[key][:0]
	for _, at := range pauseCounters.hits[key] {
		if now-at < window {
			kept = append(kept, at)
		}
	}
	kept = append(kept, now)
	pauseCounters.hits[key] = kept
	return len(kept)
}

func resetPauseHits(key string) {
	pauseCounters.Lock()
	delete(pauseCounters.hits, key)
	pauseCounters.Unlock()
}

func pauseFailureOf(channel *model.Channel, apiErr *types.OpenAIErrorWithStatusCode, now int64) model.PauseFailure {
	code := ""
	if apiErr.OpenAIError.Code != nil {
		code = fmt.Sprint(apiErr.OpenAIError.Code)
	}
	return model.PauseFailure{
		StatusCode:     apiErr.StatusCode,
		Message:        apiErr.OpenAIError.Message,
		ErrorType:      apiErr.OpenAIError.Type,
		ErrorCode:      code,
		Unusable:       controller.IsChannelUnusableError(channel.Type, apiErr),
		ModelNotFound:  isModelNotFoundErr(apiErr),
		RateLimitReset: apiErr.RateLimitResetAt > now,
	}
}

// applyPauseRules runs the pause rules for one failed attempt on channel and reports whether it
// paused the channel or the model on it. Gateway-local errors say nothing about the channel and
// are left alone, and a session held on its channel by strict affinity is only moved by a
// channel-wide pause (the channel is out of balance or its key is gone).
func applyPauseRules(ctx context.Context, c *gin.Context, channel *model.Channel, apiErr *types.OpenAIErrorWithStatusCode) bool {
	if apiErr == nil || apiErr.LocalError || channel == nil {
		return false
	}
	routingModel := RoutingModelName(c)
	now := time.Now().Unix()
	failure := pauseFailureOf(channel, apiErr, now)

	for _, rule := range model.PauseRulesForChannel(channel) {
		if !rule.Matches(failure) {
			continue
		}
		if rule.Times > 1 {
			key := fmt.Sprintf("%s|%d|%s", rule.Name, channel.Id, routingModel)
			if countPauseHit(key, int64(rule.WindowSeconds), now) < rule.Times {
				return false
			}
			resetPauseHits(key)
		}
		if rule.Action != model.PauseActionPause {
			return false
		}
		scope := rule.Scope
		if scope == "" {
			scope = model.PauseScopeModel
		}
		if scope != model.PauseScopeChannel && strictAffinityHolds(c) {
			return false
		}
		seconds := rule.PauseSecondsFor(failure, now, apiErr.RateLimitResetAt)
		if seconds <= 0 {
			return false
		}

		if seconds < model.MinPersistedPauseSeconds {
			model.ChannelGroup.SetCooldownsWithDuration(channel.Id, routingModel, seconds)
			logger.LogWarn(ctx, fmt.Sprintf("channel_cooldown channel_id=%d model=%q status_code=%d duration=%ds rule=%q",
				channel.Id, routingModel, apiErr.StatusCode, seconds, rule.Name))
			return true
		}

		pause := &model.SupplyPause{
			ChannelId: channel.Id, Until: now + seconds, Seconds: seconds,
			Rule: rule.Name, Category: rule.Category, Probe: rule.Probe, Notify: rule.Notify,
			Reason: utils.MaskSensitiveInfo(apiErr.OpenAIError.Message),
		}
		if scope == model.PauseScopeModel {
			pause.Model = routingModel
		}
		// Take effect on this node now; the database write (shared with other nodes) and the
		// notice happen off the request path.
		model.PreIndexPause(pause)
		logger.LogWarn(ctx, fmt.Sprintf("supply_pause channel_id=%d model=%q scope=%s status_code=%d duration=%ds rule=%q probe=%t",
			channel.Id, pause.Model, scope, apiErr.StatusCode, seconds, rule.Name, rule.Probe))
		channelName := channel.Name
		relayErrorNotifyWG.Add(1)
		go func() {
			defer relayErrorNotifyWG.Done()
			started, err := model.SavePause(pause)
			if err != nil {
				logger.SysError(fmt.Sprintf("save supply pause failed (channel=%d): %v", pause.ChannelId, err))
				return
			}
			if started && pause.Notify {
				controller.NotifySupplyPaused(pause, channelName)
			}
		}()
		return true
	}
	return false
}
