package relay

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/requester"
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
//
// A request the upstream did not answer in time is not taken at its word: it may have been a
// long request on a healthy channel. It is not counted by rules with Times, and the first rule
// that matches it pauses only after a test request to the channel failed too (confirmStalled).
func applyPauseRules(ctx context.Context, c *gin.Context, channel *model.Channel, apiErr *types.OpenAIErrorWithStatusCode) bool {
	if apiErr == nil || apiErr.LocalError || channel == nil {
		return false
	}
	routingModel := RoutingModelName(c)
	now := time.Now().Unix()
	failure := pauseFailureOf(channel, apiErr, now)
	unanswered := requester.IsUpstreamTimeout(apiErr)

	for _, rule := range model.PauseRulesForChannel(channel) {
		if !rule.Matches(failure) {
			continue
		}
		if rule.Times > 1 && !unanswered {
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
		order := pauseOrder{
			channel: channel, routingModel: routingModel, rule: rule, scope: scope, seconds: seconds,
			statusCode: apiErr.StatusCode, reason: utils.MaskSensitiveInfo(apiErr.OpenAIError.Message),
		}
		if unanswered {
			confirmStalled(ctx, order)
			return false
		}
		startPause(ctx, order)
		return true
	}
	return false
}

// pauseOrder is a pause a rule decided on: what to pause, for how long, and why.
type pauseOrder struct {
	channel      *model.Channel
	routingModel string
	rule         model.PauseRule
	scope        string
	seconds      int64
	statusCode   int
	reason       string
}

// startPause carries a pause out. A pause under a minute is a cooldown kept in memory; a longer
// one takes effect on this node at once, and is stored (shared with other nodes) and announced
// off the request path.
func startPause(ctx context.Context, order pauseOrder) {
	channel := order.channel
	if order.seconds < model.MinPersistedPauseSeconds {
		model.ChannelGroup.SetCooldownsWithDuration(channel.Id, order.routingModel, order.seconds)
		logger.LogWarn(ctx, fmt.Sprintf("channel_cooldown channel_id=%d model=%q status_code=%d duration=%ds rule=%q",
			channel.Id, order.routingModel, order.statusCode, order.seconds, order.rule.Name))
		return
	}

	pause := &model.SupplyPause{
		ChannelId: channel.Id, Until: time.Now().Unix() + order.seconds, Seconds: order.seconds,
		Rule: order.rule.Name, Category: order.rule.Category, Probe: order.rule.Probe, Notify: order.rule.Notify,
		Reason: order.reason,
	}
	if order.scope == model.PauseScopeModel {
		pause.Model = order.routingModel
	}
	model.PreIndexPause(pause)
	logger.LogWarn(ctx, fmt.Sprintf("supply_pause channel_id=%d model=%q scope=%s status_code=%d duration=%ds rule=%q probe=%t",
		channel.Id, pause.Model, order.scope, order.statusCode, order.seconds, order.rule.Name, order.rule.Probe))
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
}

// stallProbeTimeout is how long the test request that confirms a stalled upstream may take. It
// is a small request with a known answer, so unlike a caller's request it has no reason to be
// slow. stallProbeInterval is the least time between two such tests of one channel and model:
// under load a stalled upstream fails many requests at once, and one test settles it for all.
var (
	stallProbeTimeout        = 30 * time.Second
	stallProbeInterval int64 = 30
)

// probeSupply sends the test request; tests replace it.
var probeSupply = controller.ProbeSupply

var stallProbes = struct {
	sync.Mutex
	last map[string]int64
}{last: map[string]int64{}}

// confirmStalled sends a test request to a channel that left a request unanswered and carries
// the pause out only when the test fails as well. It returns at once; the test runs off the
// request path.
func confirmStalled(ctx context.Context, order pauseOrder) {
	probeModel := ""
	if order.scope == model.PauseScopeModel {
		probeModel = order.routingModel
	}
	key := fmt.Sprintf("%d|%s", order.channel.Id, probeModel)
	now := time.Now().Unix()
	stallProbes.Lock()
	if last, ok := stallProbes.last[key]; ok && now-last < stallProbeInterval {
		stallProbes.Unlock()
		return
	}
	stallProbes.last[key] = now
	stallProbes.Unlock()

	channelId := order.channel.Id
	relayErrorNotifyWG.Add(1)
	go func() {
		defer relayErrorNotifyWG.Done()
		err := probeSupply(channelId, probeModel, stallProbeTimeout)
		if err == nil {
			logger.LogInfo(ctx, fmt.Sprintf("stall_probe_passed channel_id=%d model=%q", channelId, probeModel))
			return
		}
		order.reason = utils.MaskSensitiveInfo("no answer in time, and a test request failed: " + err.Error())
		startPause(ctx, order)
	}()
}
