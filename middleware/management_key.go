package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// Context keys set when a request to the management API is authenticated by a management key (or
// a legacy access token) instead of a console session.
const (
	ManagementKeyIdContextKey    = "management_key_id"
	ManagementKeyScopeContextKey = "management_key_scope"
)

// managementKeyRoutes is the allow-list of what a read or write management key may call, keyed by
// method and the route template (gin's FullPath). Anything not listed is refused with 403, so a
// new route is closed to management keys until it is added here on purpose. The key acts as its
// user with the user's current role: listed routes still go through UserAuth and OrgAuth, so an
// organization route never lets a key do more than its creator's role allows, and organization
// API keys it creates carry the creator as created_by (member budgets and model lists apply).
//
// Deliberately left out: the site admin console (channels, options, users, ...), password, login
// methods, 2FA / passkeys, sessions, payments and top-ups, management keys themselves, the legacy
// access token endpoint, organization membership and settings, and full request / response
// bodies of logs (they hold prompts).
var managementKeyRoutes = map[string]string{
	// Balance and usage.
	"GET /api/user/self":           model.ManagementKeyScopeRead,
	"GET /api/user/dashboard":      model.ManagementKeyScopeRead,
	"GET /api/user/self/analytics": model.ManagementKeyScopeRead,
	"GET /api/user/invoice":        model.ManagementKeyScopeRead,
	"GET /api/user/invoice/detail": model.ManagementKeyScopeRead,

	// Logs (without request / response bodies).
	"GET /api/log/self":           model.ManagementKeyScopeRead,
	"GET /api/log/self/stat":      model.ManagementKeyScopeRead,
	"GET /api/log/self/histogram": model.ManagementKeyScopeRead,
	"GET /api/log/self/export":    model.ManagementKeyScopeRead,
	"GET /api/mj/self":            model.ManagementKeyScopeRead,
	"GET /api/task/self":          model.ManagementKeyScopeRead,

	// API keys: the list and details only ever hold the masked key.
	"GET /api/token/":               model.ManagementKeyScopeRead,
	"GET /api/token/:id":            model.ManagementKeyScopeRead,
	"GET /api/token/:id/models":     model.ManagementKeyScopeRead,
	"POST /api/token/":              model.ManagementKeyScopeWrite,
	"PUT /api/token/":               model.ManagementKeyScopeWrite,
	"DELETE /api/token/:id":         model.ManagementKeyScopeWrite,
	"POST /api/token/:id/reset_key": model.ManagementKeyScopeWrite,

	// Organizations the user belongs to, within the user's organization role.
	"GET /api/org/":                               model.ManagementKeyScopeRead,
	"GET /api/org/:id/":                           model.ManagementKeyScopeRead,
	"GET /api/org/:id/token":                      model.ManagementKeyScopeRead,
	"GET /api/org/:id/token/:token_id":            model.ManagementKeyScopeRead,
	"GET /api/org/:id/token/:token_id/models":     model.ManagementKeyScopeRead,
	"GET /api/org/:id/logs":                       model.ManagementKeyScopeRead,
	"GET /api/org/:id/logs/stat":                  model.ManagementKeyScopeRead,
	"GET /api/org/:id/logs/histogram":             model.ManagementKeyScopeRead,
	"GET /api/org/:id/logs/export":                model.ManagementKeyScopeRead,
	"GET /api/org/:id/analytics":                  model.ManagementKeyScopeRead,
	"GET /api/org/:id/analytics/export":           model.ManagementKeyScopeRead,
	"POST /api/org/:id/token":                     model.ManagementKeyScopeWrite,
	"PUT /api/org/:id/token":                      model.ManagementKeyScopeWrite,
	"DELETE /api/org/:id/token/:token_id":         model.ManagementKeyScopeWrite,
	"POST /api/org/:id/token/:token_id/reset_key": model.ManagementKeyScopeWrite,

	// MCP (its tools only read).
	"POST /mcp":         model.ManagementKeyScopeRead,
	"GET /mcp/sse":      model.ManagementKeyScopeRead,
	"POST /mcp/message": model.ManagementKeyScopeRead,
}

// managementKeyAllows reports whether a management key of this scope may call the route.
func managementKeyAllows(scope string, method string, fullPath string) bool {
	need, ok := managementKeyRoutes[method+" "+fullPath]
	if !ok {
		return false
	}
	switch scope {
	case model.ManagementKeyScopeWrite:
		return need == model.ManagementKeyScopeRead || need == model.ManagementKeyScopeWrite
	case model.ManagementKeyScopeRead:
		return need == model.ManagementKeyScopeRead
	default:
		return false
	}
}

// authenticateByKey handles a management API request that carries no session: credential is the
// Authorization header. It answers and aborts the request itself when the credential is refused,
// and otherwise returns the user the request acts for.
func authenticateByKey(c *gin.Context, credential string) (*model.User, bool) {
	if common.LooksLikeAPIKey(credential) {
		common.APIRespondWithCode(c, http.StatusUnauthorized, common.APICodeAPIKeyNotAllowed,
			"API keys can only call model APIs; use a management key for the management API", nil)
		c.Abort()
		return nil, false
	}
	key, user, err := model.ValidateManagementKey(credential)
	if err == model.ErrManagementKeyExpired {
		common.APIRespondWithCode(c, http.StatusUnauthorized, common.APICodeManagementKeyExpired, "Permission denied: the management key has expired", nil)
		c.Abort()
		return nil, false
	}
	if err != nil {
		common.APIRespondWithCode(c, http.StatusUnauthorized, common.APICodeInvalidAccessToken, "Permission denied: invalid management key or access token", nil)
		c.Abort()
		return nil, false
	}
	if key.Scope == model.ManagementKeyScopeLegacy {
		warnLegacyAccessToken(c, user.Id)
	} else if !managementKeyAllows(key.Scope, c.Request.Method, c.FullPath()) {
		common.APIRespondWithCode(c, http.StatusForbidden, common.APICodeManagementKeyForbidden,
			"This management key is not allowed to call this endpoint", map[string]any{"scope": key.Scope})
		c.Abort()
		return nil, false
	}
	c.Set(ManagementKeyIdContextKey, key.Id)
	c.Set(ManagementKeyScopeContextKey, key.Scope)
	return user, true
}

var legacyAccessTokenWarned sync.Map // user id -> last warning (Unix seconds)

// warnLegacyAccessToken marks a response served for a legacy access token as deprecated and logs
// it, at most once an hour per user so the log stays readable.
func warnLegacyAccessToken(c *gin.Context, userId int) {
	c.Header("Deprecation", "true")
	c.Header("Warning", `299 - "Access tokens are deprecated and will stop working in a future release; create a management key instead"`)
	now := time.Now().Unix()
	if last, ok := legacyAccessTokenWarned.Load(userId); ok && now-last.(int64) < 3600 {
		return
	}
	legacyAccessTokenWarned.Store(userId, now)
	logger.SysLog(fmt.Sprintf("deprecated access token used by user %d on %s %s; it will stop working in a future release, create a management key instead",
		userId, c.Request.Method, c.FullPath()))
}

// SessionOnly refuses requests authenticated by a management key or a legacy access token: the
// endpoints behind it (managing management keys) need a console session.
func SessionOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, byKey := c.Get(ManagementKeyScopeContextKey); byKey {
			common.APIRespondWithCode(c, http.StatusForbidden, common.APICodeManagementKeyForbidden,
				"Sign in to the console to manage management keys", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}
