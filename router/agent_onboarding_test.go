package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/middleware"
	"github.com/modeltaps/modeltaps/model"
)

// AGENT-7 end to end through the real routers: a key created in the console is handed to an agent
// with its pairing code, or an agent asks for one with the device flow and the user approves it.

// agentCall sends a request as coming from ip with a User-Agent, with session cookies if given.
func (f *mkFixture) agentCall(method, target, ip, credential string, payload any, cookies ...*http.Cookie) mkResponse {
	f.t.Helper()
	raw, _ := json.Marshal(payload)
	req := httptest.NewRequest(method, target, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "test-agent/1.0")
	req.Header.Set("X-Forwarded-For", ip)
	if credential != "" {
		req.Header.Set("Authorization", "Bearer "+credential)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	f.engine.ServeHTTP(w, req)
	resp := mkResponse{status: w.Code, header: w.Header(), raw: w.Body.String()}
	_ = json.Unmarshal(w.Body.Bytes(), &resp.body)
	return resp
}

func (r mkResponse) agentError() string {
	code, _ := r.body["error"].(string)
	return code
}

func (r mkResponse) data() map[string]any {
	data, _ := r.body["data"].(map[string]any)
	return data
}

// keyCalls reports whether key reads its balance on the relay side, as skill.md's check does.
func (f *mkFixture) keyCalls(key string) bool {
	return f.do(http.MethodGet, "/v1/dashboard/billing/subscription", key, nil).status == http.StatusOK
}

func TestPairingCodeEndToEnd(t *testing.T) {
	f := setupManagementKeyRouter(t, false)
	cookies := f.login(f.user.Id)
	ip := "198.51.100.71"

	created := f.do(http.MethodPost, "/api/token/", "", map[string]any{"name": "for-agent", "expired_time": -1, "unlimited_quota": true}, cookies...)
	expectAllowed(t, created, "create an API key")
	shown := "sk-" + created.data()["key"].(string)
	code, _ := created.data()["pairing_code"].(string)
	if !strings.HasPrefix(code, common.PairingCodePrefix) || created.data()["pairing_expires_at"] == nil {
		t.Fatalf("creating a key should also issue a pairing code: %s", created.raw)
	}
	id := itoa(int(created.data()["id"].(float64)))

	pending := f.do(http.MethodGet, "/api/token/"+id, "", nil, cookies...)
	if pending.data()["pairing_expires_at"] == nil || pending.data()["agent_claim"] != nil {
		t.Fatalf("details should show the pending code and no claim yet: %s", pending.raw)
	}

	redeemed := f.agentCall(http.MethodPost, "/api/agent/pair", ip, "", map[string]any{"code": code})
	if redeemed.status != http.StatusOK || redeemed.body["token_type"] != "Bearer" || redeemed.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("redeem: %d %s", redeemed.status, redeemed.raw)
	}
	agentKey, _ := redeemed.body["access_token"].(string)
	if !strings.HasPrefix(agentKey, "sk-") || agentKey == shown || redeemed.body["key_name"] != "for-agent" {
		t.Fatalf("the agent should get the key with a fresh secret: %s", redeemed.raw)
	}
	if !f.keyCalls(agentKey) || f.keyCalls(shown) {
		t.Fatal("after pairing only the agent's copy of the key works")
	}

	replay := f.agentCall(http.MethodPost, "/api/agent/pair", ip, "", map[string]any{"code": code})
	if replay.status != http.StatusBadRequest || replay.agentError() != common.APICodePairingCodeUsed || replay.code() != common.APICodePairingCodeUsed {
		t.Fatalf("replay: want 400 pairing_code_used, got %d %s", replay.status, replay.raw)
	}

	details := f.do(http.MethodGet, "/api/token/"+id, "", nil, cookies...)
	claim, _ := details.data()["agent_claim"].(map[string]any)
	if claim["via"] != model.AgentClaimViaPairingCode || claim["ip"] != ip || claim["user_agent"] != "test-agent/1.0" || claim["claimed_at"] == nil {
		t.Fatalf("details should show who claimed the key: %s", details.raw)
	}
	if strings.Contains(details.raw, strings.TrimPrefix(agentKey, "sk-")) {
		t.Fatal("details must not hold the key")
	}

	// A new code from the details hands the key over again; a reset voids a pending one.
	again := f.do(http.MethodPost, "/api/token/"+id+"/pairing_code", "", nil, cookies...)
	expectAllowed(t, again, "issue a new pairing code")
	newCode, _ := again.data()["pairing_code"].(string)
	expectAllowed(t, f.do(http.MethodPost, "/api/token/"+id+"/reset_key", "", nil, cookies...), "reset the key")
	void := f.agentCall(http.MethodPost, "/api/agent/pair", ip, "", map[string]any{"code": newCode})
	if void.agentError() != common.APICodePairingCodeInvalid {
		t.Fatalf("a reset key's pending code must be void: %s", void.raw)
	}
	afterReset := f.do(http.MethodGet, "/api/token/"+id, "", nil, cookies...)
	if afterReset.data()["agent_claim"] != nil {
		t.Fatalf("after a reset the agent no longer holds the key: %s", afterReset.raw)
	}
}

// Neither path hands out a management key, and neither can be driven with a key: issuing pairing
// codes and answering device requests take a console session.
func TestAgentHandoverRefusesManagementKeys(t *testing.T) {
	f := setupManagementKeyRouter(t, false)
	token, apiKey := f.apiKey(f.user.Id, "app")
	write := f.managementKey(f.user.Id, model.ManagementKeyScopeWrite)
	legacy := "0123456789abcdef0123456789abcdef"
	if err := model.DB.Create(&model.ManagementKey{UserId: f.user.Id, Name: model.LegacyAccessTokenName, Scope: model.ManagementKeyScopeLegacy, Key: legacy, ExpiredTime: -1}).Error; err != nil {
		t.Fatal(err)
	}
	pairingPath := "/api/token/" + itoa(token.Id) + "/pairing_code"

	expectDenied(t, f.do(http.MethodPost, pairingPath, write, nil), http.StatusForbidden, common.APICodeManagementKeyForbidden, "write management key issuing a pairing code")
	expectDenied(t, f.do(http.MethodPost, pairingPath, legacy, nil), http.StatusForbidden, common.APICodeManagementKeyForbidden, "legacy access token issuing a pairing code")
	expectDenied(t, f.do(http.MethodPost, pairingPath, apiKey, nil), http.StatusUnauthorized, common.APICodeAPIKeyNotAllowed, "API key issuing a pairing code")

	start := f.agentCall(http.MethodPost, "/api/agent/device", "198.51.100.72", "", map[string]any{"client_name": "x"})
	userCode, _ := start.body["user_code"].(string)
	approve := "/api/agent/device/" + userCode + "/approve"
	expectDenied(t, f.do(http.MethodPost, approve, write, map[string]any{"name": "mk"}), http.StatusForbidden, common.APICodeManagementKeyForbidden, "management key approving a device request")
	expectDenied(t, f.do(http.MethodPost, approve, legacy, map[string]any{"name": "mk"}), http.StatusForbidden, common.APICodeManagementKeyForbidden, "legacy token approving a device request")
	expectDenied(t, f.do(http.MethodPost, approve, apiKey, map[string]any{"name": "mk"}), http.StatusUnauthorized, common.APICodeAPIKeyNotAllowed, "API key approving a device request")

	// A management key is not a pairing code either.
	wrong := f.agentCall(http.MethodPost, "/api/agent/pair", "198.51.100.72", "", map[string]any{"code": write})
	if wrong.agentError() != common.APICodePairingCodeInvalid {
		t.Fatalf("a management key as a pairing code: %s", wrong.raw)
	}
}

func TestAgentPairRateLimit(t *testing.T) {
	prev := middleware.AgentPairRateLimitNum
	middleware.AgentPairRateLimitNum = 3
	t.Cleanup(func() { middleware.AgentPairRateLimitNum = prev })
	f := setupManagementKeyRouter(t, false)
	ip := "198.51.100.73"
	for i := 0; i < 3; i++ {
		if w := f.agentCall(http.MethodPost, "/api/agent/pair", ip, "", map[string]any{"code": "pair-wrong"}); w.status != http.StatusBadRequest {
			t.Fatalf("attempt %d: want 400, got %d %s", i, w.status, w.raw)
		}
	}
	limited := f.agentCall(http.MethodPost, "/api/agent/pair", ip, "", map[string]any{"code": "pair-wrong"})
	if limited.status != http.StatusTooManyRequests || limited.header.Get("Retry-After") == "" {
		t.Fatalf("want 429 with Retry-After, got %d %s", limited.status, limited.raw)
	}
	// Other addresses keep their own budget.
	if w := f.agentCall(http.MethodPost, "/api/agent/pair", "198.51.100.74", "", map[string]any{"code": "pair-wrong"}); w.status != http.StatusBadRequest {
		t.Fatalf("another address: want 400, got %d", w.status)
	}
}

// resetPollClock lets the next poll through the interval check (the model's clock is real time).
func resetPollClock(t *testing.T) {
	t.Helper()
	if err := model.DB.Model(&model.AgentDeviceAuthorization{}).Where("id > 0").Update("last_polled_at", 0).Error; err != nil {
		t.Fatal(err)
	}
}

func TestDeviceFlowEndToEnd(t *testing.T) {
	f := setupManagementKeyRouter(t, false)
	cookies := f.login(f.user.Id)
	ip := "198.51.100.75"

	start := f.agentCall(http.MethodPost, "/api/agent/device", ip, "", map[string]any{"client_name": "Hermes on build box"})
	deviceCode, _ := start.body["device_code"].(string)
	userCode, _ := start.body["user_code"].(string)
	uri, _ := start.body["verification_uri"].(string)
	complete, _ := start.body["verification_uri_complete"].(string)
	if start.status != http.StatusOK || deviceCode == "" || len(userCode) != 9 || !strings.HasSuffix(uri, "/device") ||
		complete != uri+"?user_code="+userCode || start.body["interval"] != float64(model.AgentDevicePollInterval) || start.body["expires_in"] == nil {
		t.Fatalf("start: %d %s", start.status, start.raw)
	}

	poll := func() mkResponse {
		return f.agentCall(http.MethodPost, "/api/agent/device/token", ip, "", map[string]any{
			"grant_type": "urn:ietf:params:oauth:grant-type:device_code", "device_code": deviceCode,
		})
	}
	if w := poll(); w.status != http.StatusBadRequest || w.agentError() != common.APICodeDeviceAuthPending {
		t.Fatalf("before approval: want authorization_pending, got %d %s", w.status, w.raw)
	}
	if w := poll(); w.agentError() != common.APICodeDeviceSlowDown {
		t.Fatalf("polling too fast: want slow_down, got %s", w.raw)
	}
	if w := f.agentCall(http.MethodPost, "/api/agent/device/token", ip, "", map[string]any{"grant_type": "password", "device_code": deviceCode}); w.agentError() != common.APICodeDeviceUnsupportedGrant {
		t.Fatalf("wrong grant type: %s", w.raw)
	}

	// The confirmation page needs a session; it shows who is asking.
	if w := f.do(http.MethodGet, "/api/agent/device/"+userCode, "", nil); w.success() {
		t.Fatal("looking up a device request needs a session")
	}
	shown := f.do(http.MethodGet, "/api/agent/device/"+strings.ToLower(userCode), "", nil, cookies...)
	expectAllowed(t, shown, "look up the device request")
	if shown.data()["client_name"] != "Hermes on build box" || shown.data()["request_ip"] != ip || shown.data()["request_user_agent"] != "test-agent/1.0" {
		t.Fatalf("the request should show the client: %s", shown.raw)
	}
	if shown.data()["same_network"] != false {
		t.Fatalf("a request from another address is not on the user's network: %s", shown.raw)
	}
	status := func() mkResponse { return f.do(http.MethodGet, "/api/agent/device/"+userCode+"/status", "", nil, cookies...) }
	if w := status(); w.success() {
		t.Fatalf("an unanswered request has no status for the page to follow: %s", w.raw)
	}

	approved := f.do(http.MethodPost, "/api/agent/device/"+userCode+"/approve", "", map[string]any{
		"name": "hermes", "expired_time": -1, "remain_quota": 5000, "unlimited_quota": false,
		"setting": map[string]any{"limits": map[string]any{"limit_model_setting": map[string]any{"enabled": true, "models": []string{"gpt-a"}}}},
	}, cookies...)
	expectAllowed(t, approved, "approve the device request")
	if _, ok := approved.data()["key"]; ok {
		t.Fatal("the approval answer must not carry the key")
	}
	again := f.do(http.MethodPost, "/api/agent/device/"+userCode+"/approve", "", map[string]any{"name": "twice"}, cookies...)
	expectDenied(t, again, http.StatusOK, common.APICodeDeviceUserCodeInvalid, "second approval")
	if w := status(); !w.success() || w.data()["status"] != model.AgentDeviceStatusApproved || w.data()["claim"] != nil {
		t.Fatalf("after approval the page sees it waiting for the agent: %s", w.raw)
	}

	resetPollClock(t)
	got := poll()
	key, _ := got.body["access_token"].(string)
	if got.status != http.StatusOK || !strings.HasPrefix(key, "sk-") || got.body["key_name"] != "hermes" {
		t.Fatalf("after approval: want the key, got %d %s", got.status, got.raw)
	}
	if !f.keyCalls(key) {
		t.Fatal("the issued key should work")
	}
	claimed := status()
	claim, _ := claimed.data()["claim"].(map[string]any)
	if claimed.data()["status"] != model.AgentDeviceStatusClaimed || claim == nil || claim["ip"] != ip || claim["claimed_at"] == nil {
		t.Fatalf("once the agent has its key the page shows when and from where: %s", claimed.raw)
	}
	var stored model.Token
	model.DB.First(&stored, int(approved.data()["id"].(float64)))
	limit := stored.Setting.Data().Limits.LimitModelSetting
	if stored.UserId != f.user.Id || stored.RemainQuota != 5000 || stored.UnlimitedQuota || !limit.Enabled || len(limit.Models) != 1 {
		t.Fatalf("the key should carry the user's choices: %+v", stored)
	}

	resetPollClock(t)
	if w := poll(); w.agentError() != common.APICodeDeviceInvalidGrant {
		t.Fatalf("the device code is single use: %s", w.raw)
	}
}

func TestDeviceFlowDenied(t *testing.T) {
	f := setupManagementKeyRouter(t, false)
	cookies := f.login(f.user.Id)
	ip := "198.51.100.76"
	start := f.agentCall(http.MethodPost, "/api/agent/device", ip, "", map[string]any{"client_name": "unknown"})
	userCode, _ := start.body["user_code"].(string)
	expectAllowed(t, f.do(http.MethodPost, "/api/agent/device/"+userCode+"/deny", "", nil, cookies...), "deny")
	w := f.agentCall(http.MethodPost, "/api/agent/device/token", ip, "", map[string]any{"device_code": start.body["device_code"]})
	if w.agentError() != common.APICodeDeviceAccessDenied {
		t.Fatalf("want access_denied, got %s", w.raw)
	}
	expectDenied(t, f.do(http.MethodGet, "/api/agent/device/"+userCode, "", nil, cookies...), http.StatusOK, common.APICodeDeviceUserCodeInvalid, "answered request")
	if st := f.do(http.MethodGet, "/api/agent/device/"+userCode+"/status", "", nil, cookies...); st.data()["status"] != model.AgentDeviceStatusDenied {
		t.Fatalf("the page follows a denial too: %s", st.raw)
	}
}

// Organization keys: the paired key is the organization's key, only its creator can issue a code,
// and a device request can be answered with an organization key.
func TestAgentHandoverOrganizationKeys(t *testing.T) {
	f := setupManagementKeyRouter(t, false)
	org, err := model.CreateOrganizationWithOwner("Agents", "", f.admin.Id)
	if err != nil {
		t.Fatal(err)
	}
	member := &model.OrganizationMember{OrganizationId: org.Id, UserId: f.user.Id, Role: model.OrgRoleMember}
	if err := member.Insert(); err != nil {
		t.Fatal(err)
	}
	memberCookies := f.login(f.user.Id)
	ownerCookies := f.login(f.admin.Id)
	base := "/api/org/" + itoa(org.Id)
	ip := "198.51.100.77"

	created := f.do(http.MethodPost, base+"/token", "", map[string]any{"name": "org-agent", "expired_time": -1, "unlimited_quota": true}, memberCookies...)
	expectAllowed(t, created, "member creating an organization key")
	code, _ := created.data()["pairing_code"].(string)
	tokenId := int(created.data()["id"].(float64))

	expectDenied(t, f.do(http.MethodPost, base+"/token/"+itoa(tokenId)+"/pairing_code", "", nil, ownerCookies...), http.StatusOK,
		common.APICodePairingCodeNotCreator, "owner issuing a code for a member's key")

	redeemed := f.agentCall(http.MethodPost, "/api/agent/pair", ip, "", map[string]any{"code": code})
	key, _ := redeemed.body["access_token"].(string)
	if redeemed.status != http.StatusOK || !f.keyCalls(key) {
		t.Fatalf("redeem the organization key: %d %s", redeemed.status, redeemed.raw)
	}
	claimed, err := model.GetTokenByKeyHash(common.HashUserTokenKey(key))
	if err != nil || claimed.Id != tokenId || claimed.UserId != org.ShadowUserId || claimed.CreatedBy != f.user.Id {
		t.Fatalf("the paired key must be the organization key: %+v %v", claimed, err)
	}
	var audits int64
	model.DB.Model(&model.OrganizationAuditLog{}).Where("organization_id = ? AND action = ?", org.Id, "token.agent_claim").Count(&audits)
	if audits != 1 {
		t.Fatalf("the claim should be in the organization's audit log, found %d", audits)
	}

	// Device flow answered with an organization key; outsiders cannot.
	start := f.agentCall(http.MethodPost, "/api/agent/device", ip, "", map[string]any{"client_name": "team agent"})
	userCode, _ := start.body["user_code"].(string)
	outsider := f.createUser("agent-outsider", 1)
	outsiderCookies := f.login(outsider.Id)
	expectDenied(t, f.do(http.MethodPost, base+"/agent/device/"+userCode+"/approve", "", map[string]any{"name": "x"}, outsiderCookies...),
		http.StatusForbidden, common.APICodeOrgNotMember, "outsider approving with an organization key")
	approved := f.do(http.MethodPost, base+"/agent/device/"+userCode+"/approve", "", map[string]any{"expired_time": -1, "unlimited_quota": true}, memberCookies...)
	expectAllowed(t, approved, "member approving with an organization key")
	if approved.data()["name"] != "team agent" {
		t.Fatalf("a key approved without a name is named after the client: %s", approved.raw)
	}
	resetPollClock(t)
	got := f.agentCall(http.MethodPost, "/api/agent/device/token", ip, "", map[string]any{"device_code": start.body["device_code"]})
	orgKey, _ := got.body["access_token"].(string)
	issued, err := model.GetTokenByKeyHash(common.HashUserTokenKey(orgKey))
	if err != nil || issued.UserId != org.ShadowUserId || issued.CreatedBy != f.user.Id {
		t.Fatalf("the device flow key must be the organization key: %+v %v (%s)", issued, err, got.raw)
	}
}
