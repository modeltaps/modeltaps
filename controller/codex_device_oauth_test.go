package controller

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/cache"
	"github.com/modeltaps/modeltaps/providers/codex"

	"github.com/gin-gonic/gin"
)

const testAdminID = 7

type oauthTestResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	ErrorCode string `json:"code"`
	Params    struct {
		Reason string `json:"reason"`
	} `json:"params"`
	Data struct {
		SessionID       string `json:"session_id"`
		UserCode        string `json:"user_code"`
		VerificationURL string `json:"verification_url"`
		Interval        int    `json:"interval"`
		ExpiresIn       int    `json:"expires_in"`
		Status          string `json:"status"`
		Credentials     string `json:"credentials"`
	} `json:"data"`
}

func callOAuthHandler(t *testing.T, handler gin.HandlerFunc, userID int, body any) (oauthTestResponse, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	raw, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", userID)
	handler(c)
	var resp oauthTestResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response %q: %v", w.Body.String(), err)
	}
	return resp, w.Body.String()
}

func testIDToken(accountID string) string {
	enc := base64.RawURLEncoding
	header := enc.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, _ := json.Marshal(map[string]any{
		"https://api.openai.com/auth": map[string]any{"chatgpt_account_id": accountID},
	})
	return header + "." + enc.EncodeToString(payload) + "." + enc.EncodeToString([]byte("sig"))
}

// fakeCodexAuth stands in for auth.openai.com. tokenPolls counts device token polls; the first
// `pendingPolls` answer 403 like OpenAI does before the code is approved.
type fakeCodexAuth struct {
	server       *httptest.Server
	tokenPolls   atomic.Int32
	pendingPolls int32
	usercodeCode int
	tokenStatus  int
	tokenBody    string
	lastExchange url.Values
}

func newFakeCodexAuth(t *testing.T) *fakeCodexAuth {
	t.Helper()
	cache.InitCacheManager()
	f := &fakeCodexAuth{pendingPolls: 1, usercodeCode: http.StatusOK, tokenStatus: http.StatusOK}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/accounts/deviceauth/usercode", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]string
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["client_id"] != CodexClientID {
			http.Error(w, "bad client", http.StatusBadRequest)
			return
		}
		if f.usercodeCode != http.StatusOK {
			w.WriteHeader(f.usercodeCode)
			return
		}
		_, _ = io.WriteString(w, `{"device_auth_id":"deviceauth_secret","user_code":"ABCD-12345","interval":"5","expires_at":"`+
			time.Now().Add(15*time.Minute).UTC().Format(time.RFC3339Nano)+`"}`)
	})
	mux.HandleFunc("/api/accounts/deviceauth/token", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]string
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["device_auth_id"] != "deviceauth_secret" || req["user_code"] != "ABCD-12345" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":{"message":"Failed to process device authorization.","code":"deviceauth_user_code_mismatch"}}`)
			return
		}
		if f.tokenPolls.Add(1) <= f.pendingPolls {
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"error":{"message":"Device authorization is pending.","code":"deviceauth_authorization_pending"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"authorization_code":"device-code-1","code_challenge":"ch","code_verifier":"device-verifier-1"}`)
	})
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.lastExchange = r.PostForm
		if f.tokenStatus != http.StatusOK {
			w.WriteHeader(f.tokenStatus)
			_, _ = io.WriteString(w, f.tokenBody)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id_token":      testIDToken("acct-42"),
			"access_token":  "access-1",
			"refresh_token": "refresh-1",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	})
	f.server = httptest.NewServer(mux)
	prev := codexAuthBaseURL
	codexAuthBaseURL = f.server.URL
	t.Cleanup(func() {
		codexAuthBaseURL = prev
		f.server.Close()
	})
	return f
}

// allowNextPoll moves the session's next poll time into the past, standing in for the interval.
func allowNextPoll(t *testing.T, sessionID string) {
	t.Helper()
	key := CodexDeviceSessionCachePrefix + sessionID
	s, err := cache.GetCache[CodexDeviceSession](key)
	if err != nil {
		t.Fatalf("session missing: %v", err)
	}
	s.NextPollAt = 0
	if err := cache.SetCache(key, s, time.Minute); err != nil {
		t.Fatal(err)
	}
}

func TestCodexDeviceLoginFlow(t *testing.T) {
	f := newFakeCodexAuth(t)

	start, raw := callOAuthHandler(t, StartCodexDeviceOAuth, testAdminID, map[string]any{"channel_id": 0})
	if !start.Success || start.Data.UserCode != "ABCD-12345" || start.Data.SessionID == "" {
		t.Fatalf("start failed: %s", raw)
	}
	if start.Data.VerificationURL != f.server.URL+"/codex/device" || start.Data.Interval != 5 {
		t.Fatalf("unexpected start data: %s", raw)
	}
	if start.Data.ExpiresIn <= 0 || start.Data.ExpiresIn > int(CodexDeviceMaxLifetime.Seconds()) {
		t.Fatalf("expires_in = %d", start.Data.ExpiresIn)
	}
	if strings.Contains(raw, "deviceauth_secret") {
		t.Fatal("device_auth_id must stay on the server")
	}
	poll := map[string]string{"session_id": start.Data.SessionID}

	// Polling before the interval is answered locally.
	resp, raw := callOAuthHandler(t, PollCodexDeviceOAuth, testAdminID, poll)
	if !resp.Success || resp.Data.Status != "pending" || f.tokenPolls.Load() != 0 {
		t.Fatalf("early poll: %s (upstream polls %d)", raw, f.tokenPolls.Load())
	}

	// Another admin cannot pick up the credentials.
	resp, _ = callOAuthHandler(t, PollCodexDeviceOAuth, testAdminID+1, poll)
	if resp.Success || resp.ErrorCode != common.APICodeChannelOAuthSessionExpired {
		t.Fatalf("other admin poll: %+v", resp)
	}

	allowNextPoll(t, start.Data.SessionID)
	resp, raw = callOAuthHandler(t, PollCodexDeviceOAuth, testAdminID, poll)
	if !resp.Success || resp.Data.Status != "pending" || f.tokenPolls.Load() != 1 {
		t.Fatalf("pending poll: %s", raw)
	}

	allowNextPoll(t, start.Data.SessionID)
	resp, raw = callOAuthHandler(t, PollCodexDeviceOAuth, testAdminID, poll)
	if !resp.Success || resp.Data.Status != "completed" {
		t.Fatalf("approved poll: %s", raw)
	}
	creds, err := codex.FromJSON(resp.Data.Credentials)
	if err != nil {
		t.Fatal(err)
	}
	if creds.AccessToken != "access-1" || creds.RefreshToken != "refresh-1" || creds.AccountID != "acct-42" || creds.ClientID != CodexClientID {
		t.Fatalf("credentials = %+v", creds)
	}
	if got := f.lastExchange.Get("redirect_uri"); got != f.server.URL+"/deviceauth/callback" {
		t.Fatalf("redirect_uri = %q", got)
	}
	if f.lastExchange.Get("code") != "device-code-1" || f.lastExchange.Get("code_verifier") != "device-verifier-1" {
		t.Fatalf("exchange form = %v", f.lastExchange)
	}

	// Credentials are handed out once.
	resp, _ = callOAuthHandler(t, PollCodexDeviceOAuth, testAdminID, poll)
	if resp.Success || resp.ErrorCode != common.APICodeChannelOAuthSessionExpired {
		t.Fatalf("poll after completion: %+v", resp)
	}
}

func TestCodexDeviceLoginUnavailable(t *testing.T) {
	f := newFakeCodexAuth(t)
	f.usercodeCode = http.StatusNotFound
	resp, raw := callOAuthHandler(t, StartCodexDeviceOAuth, testAdminID, map[string]any{})
	if resp.Success || resp.ErrorCode != common.APICodeChannelOAuthDeviceAuthOff {
		t.Fatalf("start: %s", raw)
	}
}

func TestCodexDeviceLoginExpires(t *testing.T) {
	newFakeCodexAuth(t)
	start, _ := callOAuthHandler(t, StartCodexDeviceOAuth, testAdminID, map[string]any{})
	key := CodexDeviceSessionCachePrefix + start.Data.SessionID
	s, _ := cache.GetCache[CodexDeviceSession](key)
	s.ExpiresAt = time.Now().Add(-time.Second).Unix()
	_ = cache.SetCache(key, s, time.Minute)

	resp, _ := callOAuthHandler(t, PollCodexDeviceOAuth, testAdminID, map[string]string{"session_id": start.Data.SessionID})
	if resp.Success || resp.ErrorCode != common.APICodeChannelOAuthDeviceCodeExpired {
		t.Fatalf("expired poll: %+v", resp)
	}
	if _, err := cache.GetCache[CodexDeviceSession](key); err == nil {
		t.Fatal("expired session should be removed")
	}
}

func TestCodexDeviceLoginRefused(t *testing.T) {
	f := newFakeCodexAuth(t)
	f.pendingPolls = 0
	start, _ := callOAuthHandler(t, StartCodexDeviceOAuth, testAdminID, map[string]any{})
	key := CodexDeviceSessionCachePrefix + start.Data.SessionID
	s, _ := cache.GetCache[CodexDeviceSession](key)
	s.UserCode = "WRONG-CODE"
	s.NextPollAt = 0
	_ = cache.SetCache(key, s, time.Minute)

	resp, raw := callOAuthHandler(t, PollCodexDeviceOAuth, testAdminID, map[string]string{"session_id": start.Data.SessionID})
	if resp.Success || resp.ErrorCode != common.APICodeChannelOAuthDeviceAuthFailed || !strings.Contains(resp.Params.Reason, "deviceauth_user_code_mismatch") {
		t.Fatalf("refused poll: %s", raw)
	}
}

func TestParseCodexDeviceInterval(t *testing.T) {
	for raw, want := range map[string]int{`"5"`: 5, `7`: 7, `"0"`: 5, `""`: 5, `"abc"`: 5, `"600"`: codexDeviceMaxInterval, ``: 5} {
		if got := parseCodexDeviceInterval(json.RawMessage(raw)); got != want {
			t.Errorf("parseCodexDeviceInterval(%s) = %d, want %d", raw, got, want)
		}
	}
}

// The paste flow keeps its session until tokens are issued, so a wrong paste or a used code can
// be retried without starting over.
func TestCodexPasteExchangeErrors(t *testing.T) {
	f := newFakeCodexAuth(t)
	start, raw := callOAuthHandler(t, StartCodexOAuth, testAdminID, map[string]any{})
	if !start.Success {
		t.Fatalf("start: %s", raw)
	}
	var startData struct {
		Data struct {
			SessionID string `json:"session_id"`
		} `json:"data"`
	}
	_ = json.Unmarshal([]byte(raw), &startData)
	session := startData.Data.SessionID
	callback := func(state string) string {
		return "http://localhost:1455/auth/callback?code=ac_abcdefghijkl&state=" + url.QueryEscape(state)
	}
	exchange := func(userID int, input string) oauthTestResponse {
		resp, _ := callOAuthHandler(t, CodexOAuthCallback, userID, map[string]string{"session_id": session, "callback_url": input})
		return resp
	}

	if resp := exchange(testAdminID, callback("another-attempt")); resp.ErrorCode != common.APICodeChannelOAuthStateMismatch {
		t.Fatalf("state mismatch: %+v", resp)
	}
	if resp := exchange(testAdminID, "https://auth.openai.com/oauth/authorize?client_id=x"); resp.ErrorCode != common.APICodeChannelOAuthInvalidInput {
		t.Fatalf("auth link pasted: %+v", resp)
	}
	if resp := exchange(testAdminID+1, callback(session)); resp.ErrorCode != common.APICodeChannelOAuthSessionExpired {
		t.Fatalf("other admin: %+v", resp)
	}

	f.tokenStatus = http.StatusBadRequest
	f.tokenBody = `{"error":"invalid_grant","error_description":"Invalid authorization code"}`
	if resp := exchange(testAdminID, callback(session)); resp.ErrorCode != common.APICodeChannelOAuthCodeRejected {
		t.Fatalf("used code: %+v", resp)
	}

	f.tokenStatus = http.StatusOK
	resp := exchange(testAdminID, callback(session))
	if !resp.Success || resp.Data.Credentials == "" {
		t.Fatalf("retry after errors: %+v", resp)
	}
	if f.lastExchange.Get("redirect_uri") != CodexRedirectURI {
		t.Fatalf("paste flow redirect_uri = %q", f.lastExchange.Get("redirect_uri"))
	}
	if resp := exchange(testAdminID, callback(session)); resp.ErrorCode != common.APICodeChannelOAuthSessionExpired {
		t.Fatalf("session must be single use: %+v", resp)
	}
}
