package controller

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// Channel keys are never shown to admins, but the gateway sends a channel's key to its base URL.
// An admin who could point a channel at an address they control and then run a channel test would
// receive the key anyway. So only the root user may move a stored key to another address; other
// admins must type the key in again with the new address (then they knew it already).

func isRootRequest(c *gin.Context) bool {
	return c.GetInt("role") >= config.RoleRootUser
}

func normalizeBaseURL(u *string) string {
	if u == nil {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(*u), "/")
}

// baseURLChanged reports whether an update sets a different base URL. A nil BaseURL in the
// request leaves the stored one unchanged.
func baseURLChanged(old *model.Channel, update *model.Channel) bool {
	if update.BaseURL == nil {
		return false
	}
	return normalizeBaseURL(old.BaseURL) != normalizeBaseURL(update.BaseURL)
}

// rejectBaseURLMoveWithoutKey refuses a non-root edit that changes the base URL while keeping the
// stored key. It writes the response and returns true when refused.
func rejectBaseURLMoveWithoutKey(c *gin.Context, old *model.Channel, update *model.Channel, keyEntered bool) bool {
	if isRootRequest(c) || keyEntered || !baseURLChanged(old, update) {
		return false
	}
	common.APIRespondWithCode(c, http.StatusOK, common.APICodeChannelBaseURLNeedsKey,
		"only the root user can change the base URL without entering the key again", nil)
	return true
}

// rejectCopyKeyMismatch refuses a non-root copy of a stored key into a channel with another type
// or base URL. It writes the response and returns true when refused.
func rejectCopyKeyMismatch(c *gin.Context, source *model.Channel, target *model.Channel) bool {
	if isRootRequest(c) {
		return false
	}
	if source.Type == target.Type && normalizeBaseURL(source.BaseURL) == normalizeBaseURL(target.BaseURL) {
		return false
	}
	common.APIRespondWithCode(c, http.StatusOK, common.APICodeChannelCopyKeyMismatch,
		"only the root user can copy a key to a channel with a different type or base URL", nil)
	return true
}
