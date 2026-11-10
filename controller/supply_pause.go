package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

func pauseTarget(p *model.SupplyPause, channelName string) string {
	if p.Model == "" {
		return fmt.Sprintf("Channel %q (#%d)", channelName, p.ChannelId)
	}
	return fmt.Sprintf("Model %s on channel %q (#%d)", p.Model, channelName, p.ChannelId)
}

func pauseUntil(p *model.SupplyPause) string {
	return time.Unix(p.Until, 0).Format("2006-01-02 15:04:05 MST")
}

// NotifySupplyPaused tells admins that a rule paused a channel or a model on it.
func NotifySupplyPaused(p *model.SupplyPause, channelName string) {
	target := pauseTarget(p, channelName)
	next := "It resumes then."
	if p.Probe {
		next = "A test request runs then; it resumes if the test passes."
	}
	notify.SendKind(notify.KindSupplyPaused, target+" paused",
		fmt.Sprintf("%s is paused until %s by the %q rule. %s Reason: %s", target, pauseUntil(p), p.Rule, next, p.Reason))
}

func notifySupplyResumed(p *model.SupplyPause, channelName, how string) {
	target := pauseTarget(p, channelName)
	notify.SendKind(notify.KindSupplyResumed, target+" resumed",
		fmt.Sprintf("%s is back in rotation (%s). It was paused by the %q rule.", target, how, p.Rule))
}

func notifySupplyStillPaused(p *model.SupplyPause, channelName, testError string) {
	target := pauseTarget(p, channelName)
	notify.SendKind(notify.KindSupplyPaused, target+" still paused",
		fmt.Sprintf("%s failed its recovery test and stays paused until %s. Test error: %s", target, pauseUntil(p), testError))
}

// failoverNoticeSent remembers, per model and first failing channel, when a failover notice
// went out, so a model that keeps failing over is reported once an hour.
var failoverNoticeSent = struct {
	sync.Mutex
	at map[string]int64
}{at: map[string]int64{}}

// NotifySupplyFailover reports a request that a backup channel served after fromChannel failed.
func NotifySupplyFailover(modelName string, fromChannel, toChannel *model.Channel) {
	if fromChannel == nil || toChannel == nil || fromChannel.Id == toChannel.Id {
		return
	}
	key := fmt.Sprintf("%s|%d", modelName, fromChannel.Id)
	now := time.Now().Unix()
	failoverNoticeSent.Lock()
	if last, ok := failoverNoticeSent.at[key]; ok && now-last < 3600 {
		failoverNoticeSent.Unlock()
		return
	}
	failoverNoticeSent.at[key] = now
	failoverNoticeSent.Unlock()
	notify.SendKind(notify.KindSupplyFailover, fmt.Sprintf("Model %s switched to a backup channel", modelName),
		fmt.Sprintf("A request for %s failed on channel %q (#%d) and was served by channel %q (#%d). Further switches from that channel are not reported for an hour.",
			modelName, fromChannel.Name, fromChannel.Id, toChannel.Name, toChannel.Id))
}

// probeModelFor picks the model a recovery test sends: the supply's upstream name when supplies
// route, the paused model itself, or the channel's test model for a whole-channel pause.
func probeModelFor(channel *model.Channel, pausedModel string) string {
	if pausedModel != "" {
		if supply, ok := model.ChannelGroup.SupplyFor(pausedModel, channel.Id); ok {
			return supply.UpstreamModel
		}
		if !strings.HasSuffix(pausedModel, "*") {
			return pausedModel
		}
	}
	return channel.TestModel
}

// recoverDuePauses ends the pauses whose time is up: without a probe they resume; with one a
// test request decides, and a failed test doubles the pause.
func recoverDuePauses(now int64) {
	pauses, err := model.DuePauses(now)
	if err != nil {
		logger.SysError("load due supply pauses failed: " + err.Error())
		return
	}
	for _, p := range pauses {
		channel, err := model.GetChannelById(p.ChannelId)
		if err != nil || channel.Status != config.ChannelStatusEnabled {
			// The channel is gone or disabled; nothing to resume into.
			model.ResumePause(p)
			continue
		}
		if !p.Probe {
			if model.ResumePause(p) && p.Notify {
				notifySupplyResumed(p, channel.Name, "pause ended")
			}
			continue
		}
		openaiErr, testErr := testChannelWithTimeout(channel, probeModelFor(channel, p.Model), testChannelTimeout)
		if testErr == nil && openaiErr == nil {
			if model.ResumePause(p) && p.Notify {
				notifySupplyResumed(p, channel.Name, "recovery test passed")
			}
			continue
		}
		message := ""
		if testErr != nil {
			message = testErr.Error()
		}
		if openaiErr != nil {
			message = openaiErr.OpenAIError.Message
		}
		if model.ExtendPause(p, time.Now().Unix()) && p.Notify {
			notifySupplyStillPaused(p, channel.Name, utils.MaskSensitiveInfo(message))
		}
	}
}

// StartPauseRecovery checks for due pauses every interval while pause rules are on.
func StartPauseRecovery(interval time.Duration) {
	for {
		time.Sleep(interval)
		if config.PauseRulesEnabled {
			recoverDuePauses(time.Now().Unix())
		}
	}
}

// quotaPauseRule is the first rule of the quota category for channel, used when a balance
// refresh finds an empty balance; the built-in quota rule when no rule has that category.
func quotaPauseRule(channel *model.Channel) model.PauseRule {
	for _, rule := range model.PauseRulesForChannel(channel) {
		if rule.Category == model.PauseCategoryQuota && rule.Action == model.PauseActionPause && rule.Seconds > 0 && !rule.Disabled {
			return rule
		}
	}
	return model.DefaultPauseRules()[0]
}

// handleBalanceForPauses applies a refreshed balance: an empty balance pauses the channel like
// an exhausted quota, money in the account ends the channel's quota pauses early. It reports
// whether it handled the balance (pause rules are on).
func handleBalanceForPauses(channel *model.Channel, balance float64) bool {
	if !config.PauseRulesEnabled {
		return false
	}
	if balance > 0 {
		resumed, err := model.ResumeChannelPausesInCategory(channel.Id, model.PauseCategoryQuota)
		if err != nil {
			logger.SysError(fmt.Sprintf("resume quota pauses failed (channel=%d): %v", channel.Id, err))
		}
		for _, p := range resumed {
			if p.Notify {
				notifySupplyResumed(p, channel.Name, "the balance refresh found money in the account")
			}
		}
		return true
	}
	rule := quotaPauseRule(channel)
	now := time.Now().Unix()
	pause := &model.SupplyPause{
		ChannelId: channel.Id, Until: now + int64(rule.Seconds), Seconds: int64(rule.Seconds),
		Rule: rule.Name, Category: model.PauseCategoryQuota, Probe: rule.Probe, Notify: rule.Notify,
		Reason: fmt.Sprintf("balance refresh reported %.4f", balance),
	}
	started, err := model.SavePause(pause)
	if err != nil {
		logger.SysError(fmt.Sprintf("pause channel for empty balance failed (channel=%d): %v", channel.Id, err))
	} else if started && pause.Notify {
		NotifySupplyPaused(pause, channel.Name)
	}
	return true
}

type supplyPauseView struct {
	*model.SupplyPause
	ChannelName string `json:"channel_name"`
}

// GetSupplyPauses lists the current pauses.
func GetSupplyPauses(c *gin.Context) {
	pauses, err := model.ListPauses()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	views := make([]supplyPauseView, 0, len(pauses))
	for _, p := range pauses {
		view := supplyPauseView{SupplyPause: p}
		if channel, err := model.GetChannelById(p.ChannelId); err == nil {
			view.ChannelName = channel.Name
		}
		views = append(views, view)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": views})
}

// CreateSupplyPause pauses a channel (or one model on it) by hand.
func CreateSupplyPause(c *gin.Context) {
	var req struct {
		ChannelId int    `json:"channel_id"`
		Model     string `json:"model"`
		Seconds   int64  `json:"seconds"`
		Probe     bool   `json:"probe"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if _, err := model.GetChannelById(req.ChannelId); err != nil {
		common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeModelSupplyChannelNotFound, "the channel does not exist", nil)
		return
	}
	if req.Seconds <= 0 {
		common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeSupplyPauseSecondsInvalid, "seconds must be greater than 0", nil)
		return
	}
	now := time.Now().Unix()
	pause := &model.SupplyPause{
		ChannelId: req.ChannelId, Model: strings.TrimSpace(req.Model), Until: now + req.Seconds, Seconds: req.Seconds,
		Rule: "manual", Probe: req.Probe, Reason: fmt.Sprintf("paused by admin #%d", c.GetInt("id")),
	}
	if _, err := model.SavePause(pause); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

// DeleteSupplyPause resumes a paused channel or model now.
func DeleteSupplyPause(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if _, err := model.ResumePauseById(id); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

// GetPauseRules returns what the rules settings page shows: whether pause rules are on, whether
// custom rules replace the defaults, the rules in effect, the built-in defaults, and with
// ?channel_id= that channel's own rules (tried before the site-wide ones).
func GetPauseRules(c *gin.Context) {
	data := gin.H{
		"enabled":  config.PauseRulesEnabled,
		"custom":   strings.TrimSpace(config.PauseRules) != "",
		"rules":    model.EffectivePauseRules(),
		"defaults": model.DefaultPauseRules(),
	}
	if raw := c.Query("channel_id"); raw != "" {
		channelId, err := strconv.Atoi(raw)
		if err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
		channel, err := model.GetChannelById(channelId)
		if err != nil {
			common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeModelSupplyChannelNotFound, "the channel does not exist", nil)
			return
		}
		rules := model.ChannelPauseRules(channel)
		if rules == nil {
			rules = []model.PauseRule{}
		}
		data["channel_rules"] = rules
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
}

// rejectInvalidPauseRules answers a channel save whose own pause rules do not parse.
func rejectInvalidPauseRules(c *gin.Context, raw *string) bool {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return false
	}
	if _, err := model.ParsePauseRules(*raw); err != nil {
		common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeSupplyPauseRulesInvalid, err.Error(), map[string]any{"message": err.Error()})
		return true
	}
	return false
}
