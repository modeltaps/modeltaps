package controller

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers"
	providersBase "github.com/modeltaps/modeltaps/providers/base"
)

// credentialRefreshAhead is how long before expiry a credential is refreshed: one run
// interval plus a margin, so no credential expires between two runs.
func credentialRefreshAhead(interval time.Duration) time.Duration {
	return interval + 15*time.Minute
}

// RefreshChannelCredentialsOnce refreshes the OAuth credentials of every account channel
// (Codex, Claude Code, Gemini CLI, Antigravity) that expires within ahead, including
// auto-disabled channels so they can recover; manually disabled channels are skipped.
// Failures are logged and kept as one credential alert per channel: a notice tells the channels
// that started failing, a channel that keeps failing is not told again, and one that refreshes
// (or is no longer checked) closes its alert.
func RefreshChannelCredentialsOnce(ahead time.Duration) (refreshed int, failed []string) {
	channels, err := model.GetAllChannels()
	if err != nil {
		logger.SysError("credential refresh: list channels failed: " + err.Error())
		return 0, nil
	}
	var failing, fresh []string
	for _, ch := range channels {
		if ch.Status == config.ChannelStatusManuallyDisabled {
			continue
		}
		provider := providers.GetProvider(ch, nil)
		refresher, ok := provider.(providersBase.CredentialRefresher)
		if !ok {
			continue
		}
		did, err := refresher.RefreshCredentialsAhead(context.Background(), ahead)
		if err != nil {
			reason := utils.MaskSensitiveInfo(err.Error())
			logger.SysError(fmt.Sprintf("credential refresh failed for channel %d (%s): %s", ch.Id, ch.Name, reason))
			line := fmt.Sprintf("- %s (#%d): %s", ch.Name, ch.Id, reason)
			failed = append(failed, line)
			subject := fmt.Sprintf("channel:%d", ch.Id)
			failing = append(failing, subject)
			// The item is the failure itself, not its text: a changing error message is the
			// same condition.
			if isNew, err := model.ReportAlert(notify.KindCredential, subject, []string{"refresh failed"}); err != nil || len(isNew) > 0 {
				fresh = append(fresh, line)
			}
			continue
		}
		if did {
			refreshed++
		}
	}
	if err := model.ResolveAlertsExcept(notify.KindCredential, failing); err != nil {
		logger.SysError("credential alerts: " + err.Error())
	}
	if len(fresh) > 0 {
		notify.SendKind(notify.KindCredential,
			fmt.Sprintf("%d channel credential(s) could not be refreshed", len(fresh)),
			"Sign in to these channels again from the channel form, or their requests will fail once the current token expires.\n\n"+strings.Join(fresh, "\n"))
	}
	return refreshed, failed
}

// AutomaticallyRefreshChannelCredentials runs RefreshChannelCredentialsOnce every
// ChannelCredentialRefreshMinutes (read each round; 0 pauses the task).
func AutomaticallyRefreshChannelCredentials() {
	for {
		minutes := config.ChannelCredentialRefreshMinutes
		if minutes <= 0 {
			time.Sleep(time.Minute)
			continue
		}
		interval := time.Duration(minutes) * time.Minute
		time.Sleep(interval)
		if config.ChannelCredentialRefreshMinutes <= 0 {
			continue
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.SysError(fmt.Sprintf("credential refresh panic: %v\n%s", r, debug.Stack()))
				}
			}()
			n, failed := RefreshChannelCredentialsOnce(credentialRefreshAhead(interval))
			if n > 0 || len(failed) > 0 {
				logger.SysLog(fmt.Sprintf("credential refresh: %d refreshed, %d failed", n, len(failed)))
			}
		}()
	}
}
