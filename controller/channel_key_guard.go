package controller

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// Channel keys are never shown to admins, but the gateway sends a channel's key to the host its
// settings point at. An admin who could point a channel at an address they control and then run a
// channel test would receive the key anyway. So only the root user may move a stored key to
// another target; other admins must type the key in again with the new target (then they knew it
// already).
//
// The target is every setting that decides where a request, and the key with it, goes: the type
// (which provider builds the URL and how the key is sent), the base URL, Other (a region that
// becomes part of the host for Vertex AI and Azure Speech, an API version elsewhere), the proxy,
// and the plugin sections that set an endpoint or request paths.

// channelTargetPluginSections are the plugin sections that change the request host or path:
// endpoint overrides of the Google OAuth channels and the request paths of a Custom channel
// (a path can start with "@host" and so move the request to another host).
var channelTargetPluginSections = []string{"antigravity", "customize", "geminicli"}

// channelTarget is the part of a channel that decides where its key is sent.
type channelTarget struct {
	Type    int
	BaseURL string
	Other   string
	Proxy   string
	Plugin  string
}

func isRootRequest(c *gin.Context) bool {
	return c.GetInt("role") >= config.RoleRootUser
}

func normalizeBaseURL(u *string) string {
	if u == nil {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(*u), "/")
}

func normalizeProxy(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

// targetPlugin is the target-related part of a plugin setting as canonical JSON ("" when there is
// none). Only non-empty strings count: the providers ignore other values in these sections, and
// the console saves unset fields as "" or false. encoding/json sorts map keys, so equal settings
// give equal strings.
func targetPlugin(ch *model.Channel) string {
	if ch.Plugin == nil {
		return ""
	}
	data := ch.Plugin.Data()
	part := make(map[string]map[string]string)
	for _, section := range channelTargetPluginSections {
		for param, value := range data[section] {
			text, ok := value.(string)
			if !ok || text == "" {
				continue
			}
			if part[section] == nil {
				part[section] = make(map[string]string)
			}
			part[section][param] = text
		}
	}
	if len(part) == 0 {
		return ""
	}
	raw, err := json.Marshal(part)
	if err != nil {
		// Unreachable for decoded JSON; an unreadable setting never compares equal to another.
		return "\x00invalid"
	}
	return string(raw)
}

// storedTarget is the target of a channel as it is (or will be) stored.
func storedTarget(ch *model.Channel) channelTarget {
	return channelTarget{
		Type:    ch.Type,
		BaseURL: normalizeBaseURL(ch.BaseURL),
		Other:   ch.Other,
		Proxy:   normalizeProxy(ch.Proxy),
		Plugin:  targetPlugin(ch),
	}
}

// targetAfterUpdate is the target the channel has after the update is saved. A whole save
// (overwrite, see model.Channel.UpdateRaw) writes every field, empty ones included; a partial save
// skips zero values and nil pointers, which keep the stored setting.
func targetAfterUpdate(old *model.Channel, update *model.Channel, overwrite bool) channelTarget {
	if overwrite {
		return storedTarget(update)
	}
	after := storedTarget(old)
	if update.Type != 0 {
		after.Type = update.Type
	}
	if update.BaseURL != nil {
		after.BaseURL = normalizeBaseURL(update.BaseURL)
	}
	if update.Other != "" {
		after.Other = update.Other
	}
	if update.Proxy != nil {
		after.Proxy = normalizeProxy(update.Proxy)
	}
	if update.Plugin != nil {
		after.Plugin = targetPlugin(update)
	}
	return after
}

// targetChanged reports whether saving the update moves the channel to another target.
func targetChanged(old *model.Channel, update *model.Channel, overwrite bool) bool {
	return storedTarget(old) != targetAfterUpdate(old, update, overwrite)
}

// storedKeysKept reports whether an update keeps a stored secret of the channel: the key when no
// key is entered, the management key (used to read the balance) when none is entered.
func storedKeysKept(old *model.Channel, update *model.Channel) bool {
	if update.Key == "" {
		return true
	}
	return update.ManageKey == nil && old.GetManageKey() != ""
}

func respondTargetNeedsKey(c *gin.Context) {
	common.APIRespondWithCode(c, http.StatusOK, common.APICodeChannelBaseURLNeedsKey,
		"only the root user can change the type, base URL, other setting, proxy or endpoint plugins of a channel without entering its key (and management key, if any) again", nil)
}

// rejectTargetMoveWithoutKey refuses a non-root edit that changes the channel's target while
// keeping a stored key. overwrite tells how the update is saved (see targetAfterUpdate). It writes
// the response and returns true when refused.
func rejectTargetMoveWithoutKey(c *gin.Context, old *model.Channel, update *model.Channel, overwrite bool) bool {
	if isRootRequest(c) || !storedKeysKept(old, update) || !targetChanged(old, update, overwrite) {
		return false
	}
	respondTargetNeedsKey(c)
	return true
}

// rejectTagTargetMoveWithoutKey refuses a non-root tag group edit that changes the target of any
// member. A tag edit saves the shared settings whole but never the type or the members' keys (an
// entered key only adds channels), so each member keeps its stored key whatever is entered.
func rejectTagTargetMoveWithoutKey(c *gin.Context, members []*model.Channel, update *model.Channel) bool {
	if isRootRequest(c) {
		return false
	}
	for _, member := range members {
		shared := *update
		shared.Type = member.Type
		if targetChanged(member, &shared, true) {
			respondTargetNeedsKey(c)
			return true
		}
	}
	return false
}

// rejectOtherMoveWithoutKey refuses a non-root batch edit of Other on channels whose Other it
// changes, since Other can be part of the host. It writes the response and returns true when
// refused.
func rejectOtherMoveWithoutKey(c *gin.Context, channels []*model.Channel, other string) bool {
	if isRootRequest(c) {
		return false
	}
	for _, ch := range channels {
		if ch.Other != other {
			respondTargetNeedsKey(c)
			return true
		}
	}
	return false
}

// rejectCopyKeyMismatch refuses a non-root copy of a stored key into a channel with another
// target. It writes the response and returns true when refused.
func rejectCopyKeyMismatch(c *gin.Context, source *model.Channel, target *model.Channel) bool {
	if isRootRequest(c) {
		return false
	}
	if storedTarget(source) == storedTarget(target) {
		return false
	}
	common.APIRespondWithCode(c, http.StatusOK, common.APICodeChannelCopyKeyMismatch,
		"only the root user can copy a key to a channel with a different type, base URL, other setting, proxy or endpoint plugins", nil)
	return true
}
