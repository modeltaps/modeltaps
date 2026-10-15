package controller

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/cache"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/providers/codex"

	"github.com/gin-gonic/gin"
)

// Device code login for Codex channels, the flow `codex login --device-auth` uses. The server
// asks auth.openai.com for a one-time user code, the admin enters it on the OpenAI device page in
// any browser, and the console polls this server until OpenAI issues an authorization code, which
// is exchanged for tokens with the verifier OpenAI returned. Nothing has to reach localhost:1455.
//
// The console drives the polling (one poll request per interval); the server holds the device
// session, enforces the interval and lifetime, and returns the credentials exactly once, to the
// admin who started the login, in the API response. No page, popup or postMessage is involved.
const (
	CodexDeviceSessionCachePrefix = "codex_device_session:"
	// CodexDeviceMaxLifetime matches the 15 minutes the Codex CLI waits for approval.
	CodexDeviceMaxLifetime = 15 * time.Minute

	codexDeviceVerificationPath = "/codex/device"
	codexDeviceRedirectPath     = "/deviceauth/callback"
	codexDeviceDefaultInterval  = 5
	codexDeviceMaxInterval      = 30
	// The cache entry outlives the code a little, so a late poll learns the code expired
	// instead of finding no session.
	codexDeviceCacheGrace = time.Minute
)

// CodexDeviceSession is the server-side state of one device login. DeviceAuthID never leaves
// the server.
type CodexDeviceSession struct {
	UserID       int    `json:"user_id"`
	ChannelID    int    `json:"channel_id"`
	DeviceAuthID string `json:"device_auth_id"`
	UserCode     string `json:"user_code"`
	Interval     int    `json:"interval"`
	Proxy        string `json:"proxy"`
	ExpiresAt    int64  `json:"expires_at"`
	NextPollAt   int64  `json:"next_poll_at"`
}

// codexDevicePolls keeps two polls of the same session from both reaching OpenAI, which would
// make the second token exchange fail on an already used code.
var codexDevicePolls sync.Map

var errCodexDeviceSessionExpired = common.NewAPIError(common.APICodeChannelOAuthSessionExpired, "This login session has expired. Get a new device code.", nil)

type StartCodexDeviceOAuthRequest struct {
	ChannelID int    `json:"channel_id"`
	Proxy     string `json:"proxy"`
}

// StartCodexDeviceOAuth requests a user code.
// POST /api/codex/oauth/device/start
func StartCodexDeviceOAuth(c *gin.Context) {
	var req StartCodexDeviceOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	client, err := codexOAuthHTTPClient(req.Proxy)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	body, _ := json.Marshal(map[string]string{"client_id": CodexClientID})
	status, respBody, err := postCodexDeviceJSON(client, codexAuthBaseURL+"/api/accounts/deviceauth/usercode", body)
	if err != nil {
		logger.SysError("Codex device code request failed: " + err.Error())
		respondOAuthError(c, oauthAPIError(common.APICodeChannelOAuthDeviceAuthFailed, "Could not reach the OpenAI auth server", err.Error()))
		return
	}
	if status == http.StatusNotFound {
		respondOAuthError(c, common.NewAPIError(common.APICodeChannelOAuthDeviceAuthOff, "OpenAI does not offer device code login right now. Use the callback URL method instead.", nil))
		return
	}
	if status != http.StatusOK {
		respondOAuthError(c, oauthAPIError(common.APICodeChannelOAuthDeviceAuthFailed, "The device code request failed", fmt.Sprintf("status %d: %s", status, codexDeviceErrorMessage(respBody))))
		return
	}

	var uc struct {
		DeviceAuthID string          `json:"device_auth_id"`
		UserCode     string          `json:"user_code"`
		UserCodeAlt  string          `json:"usercode"`
		Interval     json.RawMessage `json:"interval"`
		ExpiresAt    string          `json:"expires_at"`
	}
	if err := json.Unmarshal(respBody, &uc); err != nil {
		respondOAuthError(c, oauthAPIError(common.APICodeChannelOAuthDeviceAuthFailed, "The device code request failed", "unexpected response"))
		return
	}
	if uc.UserCode == "" {
		uc.UserCode = uc.UserCodeAlt
	}
	if uc.DeviceAuthID == "" || uc.UserCode == "" {
		respondOAuthError(c, oauthAPIError(common.APICodeChannelOAuthDeviceAuthFailed, "The device code request failed", "response is missing device_auth_id or user_code"))
		return
	}

	now := time.Now()
	expiresAt := now.Add(CodexDeviceMaxLifetime)
	if t, err := time.Parse(time.RFC3339Nano, uc.ExpiresAt); err == nil && t.After(now) && t.Before(expiresAt) {
		expiresAt = t
	}
	interval := parseCodexDeviceInterval(uc.Interval)

	sessionID, err := randomCodexDeviceSessionID()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	session := CodexDeviceSession{
		UserID:       c.GetInt("id"),
		ChannelID:    req.ChannelID,
		DeviceAuthID: uc.DeviceAuthID,
		UserCode:     uc.UserCode,
		Interval:     interval,
		Proxy:        req.Proxy,
		ExpiresAt:    expiresAt.Unix(),
		NextPollAt:   now.Add(time.Duration(interval) * time.Second).Unix(),
	}
	if err := cache.SetCache(CodexDeviceSessionCachePrefix+sessionID, session, time.Until(expiresAt)+codexDeviceCacheGrace); err != nil {
		common.APIRespondWithError(c, http.StatusOK, fmt.Errorf("failed to store device session: %w", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"session_id":       sessionID,
			"user_code":        uc.UserCode,
			"verification_url": codexAuthBaseURL + codexDeviceVerificationPath,
			"interval":         interval,
			"expires_in":       int(time.Until(expiresAt).Seconds()),
		},
	})
}

type PollCodexDeviceOAuthRequest struct {
	SessionID string `json:"session_id"`
}

// PollCodexDeviceOAuth checks once whether the user code was approved. It answers
// `status: pending` (poll again after `interval` seconds) or `status: completed` with the
// credentials; failures carry a code (see common/api_error.go) and end the session.
// POST /api/codex/oauth/device/poll
func PollCodexDeviceOAuth(c *gin.Context) {
	var req PollCodexDeviceOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.SessionID == "" {
		common.APIRespondWithError(c, http.StatusOK, fmt.Errorf("session_id is required"))
		return
	}

	cacheKey := CodexDeviceSessionCachePrefix + req.SessionID
	session, err := cache.GetCache[CodexDeviceSession](cacheKey)
	if err != nil || session.UserID != c.GetInt("id") {
		respondOAuthError(c, errCodexDeviceSessionExpired)
		return
	}

	now := time.Now()
	if now.Unix() >= session.ExpiresAt {
		cache.DeleteCache(cacheKey)
		codexDevicePolls.Delete(req.SessionID)
		respondOAuthError(c, common.NewAPIError(common.APICodeChannelOAuthDeviceCodeExpired, "The device code expired before it was approved. Get a new code.", nil))
		return
	}
	pending := func() {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data":    gin.H{"status": "pending", "interval": session.Interval},
		})
	}
	// Polling faster than OpenAI asked for is answered locally.
	if now.Unix() < session.NextPollAt {
		pending()
		return
	}

	lock, _ := codexDevicePolls.LoadOrStore(req.SessionID, &sync.Mutex{})
	mu := lock.(*sync.Mutex)
	if !mu.TryLock() {
		pending()
		return
	}
	defer mu.Unlock()

	// Re-read under the lock: a poll that just finished may have consumed the session.
	session, err = cache.GetCache[CodexDeviceSession](cacheKey)
	if err != nil {
		respondOAuthError(c, errCodexDeviceSessionExpired)
		return
	}
	if now.Unix() < session.NextPollAt {
		pending()
		return
	}

	endSession := func() {
		cache.DeleteCache(cacheKey)
		codexDevicePolls.Delete(req.SessionID)
	}

	client, err := codexOAuthHTTPClient(session.Proxy)
	if err != nil {
		endSession()
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	body, _ := json.Marshal(map[string]string{"device_auth_id": session.DeviceAuthID, "user_code": session.UserCode})
	status, respBody, err := postCodexDeviceJSON(client, codexAuthBaseURL+"/api/accounts/deviceauth/token", body)

	session.NextPollAt = time.Now().Add(time.Duration(session.Interval) * time.Second).Unix()
	if err != nil || status == http.StatusForbidden || status == http.StatusNotFound || status == http.StatusTooManyRequests || status >= 500 {
		// Not approved yet (403/404, as the Codex CLI treats them) or a transient failure: keep
		// the session and let the console poll again.
		if err != nil {
			logger.SysError("Codex device token poll failed: " + err.Error())
		}
		if status == http.StatusTooManyRequests && session.Interval < codexDeviceMaxInterval {
			session.Interval += codexDeviceDefaultInterval
			session.NextPollAt = time.Now().Add(time.Duration(session.Interval) * time.Second).Unix()
		}
		cache.SetCache(cacheKey, session, time.Until(time.Unix(session.ExpiresAt, 0))+codexDeviceCacheGrace)
		pending()
		return
	}
	// From here on the device session is finished either way.
	endSession()

	if status != http.StatusOK {
		respondOAuthError(c, oauthAPIError(common.APICodeChannelOAuthDeviceAuthFailed, "OpenAI refused the device code login", fmt.Sprintf("status %d: %s", status, codexDeviceErrorMessage(respBody))))
		return
	}

	var grant struct {
		AuthorizationCode string `json:"authorization_code"`
		CodeVerifier      string `json:"code_verifier"`
	}
	if err := json.Unmarshal(respBody, &grant); err != nil || grant.AuthorizationCode == "" || grant.CodeVerifier == "" {
		respondOAuthError(c, oauthAPIError(common.APICodeChannelOAuthDeviceAuthFailed, "OpenAI refused the device code login", "response is missing authorization_code or code_verifier"))
		return
	}

	tokenResp, err := exchangeCodexCodeForToken(grant.AuthorizationCode, grant.CodeVerifier, codexAuthBaseURL+codexDeviceRedirectPath, session.Proxy)
	if err != nil {
		logger.SysError("Codex device code exchange failed: " + err.Error())
		respondOAuthError(c, oauthAPIError(common.APICodeChannelOAuthExchangeFailed, "Failed to exchange the authorization code for credentials", err.Error()))
		return
	}

	respondCodexCredentials(c, tokenResp)
}

// postCodexDeviceJSON posts a JSON body and returns the status and the (size-limited) body.
func postCodexDeviceJSON(client *http.Client, endpoint string, body []byte) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", codex.DefaultCodexUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, respBody, nil
}

// parseCodexDeviceInterval accepts the interval as a string ("5", what OpenAI sends) or a
// number, and keeps it within a sane range.
func parseCodexDeviceInterval(raw json.RawMessage) int {
	s := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return codexDeviceDefaultInterval
	}
	if n > codexDeviceMaxInterval {
		return codexDeviceMaxInterval
	}
	return n
}

// codexDeviceErrorMessage pulls error.message out of an OpenAI error body, truncated.
func codexDeviceErrorMessage(body []byte) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error.Message != "" {
		msg := e.Error.Message
		if e.Error.Code != "" {
			msg += " [" + e.Error.Code + "]"
		}
		return truncateErrorDescription(msg)
	}
	return truncateErrorDescription(strings.TrimSpace(string(body)))
}

func randomCodexDeviceSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate session id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
