package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Management keys (AGENT-4): what API keys, read and write management keys and legacy access
// tokens may each call, through the real routers.

type mkFixture struct {
	t      *testing.T
	engine *gin.Engine
	user   *model.User
	admin  *model.User
}

func setupManagementKeyRouter(t *testing.T, withMCP bool) *mkFixture {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := testDB.AutoMigrate(&model.User{}, &model.Token{}, &model.ManagementKey{}, &model.UserSession{},
		&model.PlaygroundKey{}, &model.Log{}, &model.LogDetail{}, &model.Organization{}, &model.OrganizationMember{},
		&model.OrganizationInvitation{}, &model.OrganizationAuditLog{}, &model.ModelAccessList{}, &model.Channel{},
		&model.UserGroup{}, &model.AgentPairingCode{}, &model.AgentKeyClaim{}, &model.AgentDeviceAuthorization{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB := model.DB
	model.DB = testDB
	oldOrg := config.OrganizationEnabled
	config.OrganizationEnabled = true
	t.Cleanup(func() {
		model.DB = oldDB
		config.OrganizationEnabled = oldOrg
	})

	f := &mkFixture{t: t}
	f.user = f.createUser("mk-user", config.RoleCommonUser)
	f.admin = f.createUser("mk-admin", config.RoleAdminUser)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	// Signs the given user in, the way the console login does (a session row plus the cookie).
	engine.GET("/test/login/:id", func(c *gin.Context) {
		var user model.User
		if err := model.DB.First(&user, c.Param("id")).Error; err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		row, err := model.CreateUserSession(user.Id, model.SessionMethodPassword, "test", "127.0.0.1", 0, "")
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		session := sessions.Default(c)
		session.Set("sid", row.SessionKey)
		session.Set("id", user.Id)
		session.Set("username", user.Username)
		session.Set("role", user.Role)
		session.Set("status", user.Status)
		_ = session.Save()
		c.Status(http.StatusOK)
	})
	SetApiRouter(engine)
	SetDashboardRouter(engine)
	SetRelayRouter(engine)
	if withMCP {
		SetMcpRouter(engine)
		SetAdminMcpRouter(engine)
	}
	f.engine = engine
	return f
}

func (f *mkFixture) createUser(username string, role int) *model.User {
	f.t.Helper()
	user := &model.User{Username: username, DisplayName: username, Role: role, Status: config.UserStatusEnabled,
		AffCode: "aff-" + username, Group: "default", Quota: 1000, Email: model.NullableEmail(username + "@example.com")}
	if err := model.DB.Create(user).Error; err != nil {
		f.t.Fatalf("create user %s: %v", username, err)
	}
	return user
}

func (f *mkFixture) managementKey(userId int, scope string) string {
	f.t.Helper()
	key, err := model.CreateManagementKey(userId, scope+"-key", scope, 0)
	if err != nil {
		f.t.Fatalf("create %s management key: %v", scope, err)
	}
	return key.Key
}

func (f *mkFixture) apiKey(userId int, name string) (*model.Token, string) {
	f.t.Helper()
	token := &model.Token{UserId: userId, Name: name, ExpiredTime: -1, UnlimitedQuota: true, Status: config.TokenStatusEnabled}
	if err := token.Insert(); err != nil {
		f.t.Fatalf("create API key: %v", err)
	}
	return token, "sk-" + token.Key
}

func (f *mkFixture) login(userId int) []*http.Cookie {
	f.t.Helper()
	w := httptest.NewRecorder()
	f.engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test/login/"+itoa(userId), nil))
	if w.Code != http.StatusOK {
		f.t.Fatalf("login failed: %d", w.Code)
	}
	return w.Result().Cookies()
}

type mkResponse struct {
	status int
	header http.Header
	raw    string
	body   map[string]any
}

func (r mkResponse) code() string {
	code, _ := r.body["code"].(string)
	return code
}

func (r mkResponse) success() bool {
	ok, _ := r.body["success"].(bool)
	return ok
}

// do sends a request with an Authorization header (credential) or session cookies.
func (f *mkFixture) do(method, target, credential string, payload any, cookies ...*http.Cookie) mkResponse {
	f.t.Helper()
	var reader *bytes.Reader
	if payload != nil {
		raw, _ := json.Marshal(payload)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if credential != "" {
		req.Header.Set("Authorization", "Bearer "+credential)
	}
	addConsoleCookies(req, cookies)
	w := httptest.NewRecorder()
	f.engine.ServeHTTP(w, req)
	resp := mkResponse{status: w.Code, header: w.Header(), raw: w.Body.String()}
	_ = json.Unmarshal(w.Body.Bytes(), &resp.body)
	return resp
}

// addConsoleCookies attaches session cookies the way the console's own pages send them: with the
// site's Origin (httptest requests go to example.com), which CrossSiteRequestGuard requires of
// signed-in writes.
func addConsoleCookies(req *http.Request, cookies []*http.Cookie) {
	if len(cookies) > 0 {
		req.Header.Set("Origin", "http://"+req.Host)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func expectDenied(t *testing.T, r mkResponse, status int, code string, what string) {
	t.Helper()
	if r.status != status || r.code() != code || r.success() {
		t.Fatalf("%s: want %d %s, got %d %s", what, status, code, r.status, r.raw)
	}
}

func expectAllowed(t *testing.T, r mkResponse, what string) {
	t.Helper()
	if r.status != http.StatusOK || !r.success() {
		t.Fatalf("%s: want success, got %d %s", what, r.status, r.raw)
	}
}

// An API key can only call model APIs: on the management API it gets 401 with a stable code,
// with or without its "sk-" prefix.
func TestAPIKeyRefusedOnManagementAPI(t *testing.T) {
	f := setupManagementKeyRouter(t, false)
	token, key := f.apiKey(f.user.Id, "app")

	for _, credential := range []string{key, token.Key} {
		expectDenied(t, f.do(http.MethodGet, "/api/user/self", credential, nil), http.StatusUnauthorized, common.APICodeAPIKeyNotAllowed, "API key on /api/user/self")
		expectDenied(t, f.do(http.MethodGet, "/api/token/", credential, nil), http.StatusUnauthorized, common.APICodeAPIKeyNotAllowed, "API key on /api/token/")
	}
	// It still reads its own balance through the relay-side dashboard.
	w := f.do(http.MethodGet, "/v1/dashboard/billing/subscription", key, nil)
	if w.status != http.StatusOK {
		t.Fatalf("API key should read its own balance, got %d %s", w.status, w.raw)
	}
}

// Management keys never call model APIs (or the API key's self-service billing endpoints).
func TestManagementKeyRefusedOnRelay(t *testing.T) {
	f := setupManagementKeyRouter(t, false)
	key := f.managementKey(f.user.Id, model.ManagementKeyScopeWrite)

	for _, target := range []struct{ method, path string }{
		{http.MethodPost, "/v1/chat/completions"},
		{http.MethodGet, "/v1/models"},
		{http.MethodGet, "/v1/dashboard/billing/subscription"},
		{http.MethodPost, "/v1/embeddings"},
	} {
		w := f.do(target.method, target.path, key, map[string]any{"model": "gpt-a"})
		if w.status != http.StatusUnauthorized || !strings.Contains(w.raw, "Management keys cannot call model APIs") {
			t.Fatalf("management key on %s %s: want 401, got %d %s", target.method, target.path, w.status, w.raw)
		}
	}
}

// Read-only keys read balance, usage, logs and the masked API key list; read-write keys also
// create, change, disable and delete API keys.
func TestManagementKeyReadAndWriteScopes(t *testing.T) {
	f := setupManagementKeyRouter(t, false)
	existing, existingKey := f.apiKey(f.user.Id, "existing")
	read := f.managementKey(f.user.Id, model.ManagementKeyScopeRead)
	write := f.managementKey(f.user.Id, model.ManagementKeyScopeWrite)

	for _, target := range []string{"/api/user/self", "/api/log/self", "/api/log/self/stat", "/api/token/", "/api/token/" + itoa(existing.Id)} {
		expectAllowed(t, f.do(http.MethodGet, target, read, nil), "read key on "+target)
	}
	list := f.do(http.MethodGet, "/api/token/", read, nil)
	if strings.Contains(list.raw, strings.TrimPrefix(existingKey, "sk-")) || !strings.Contains(list.raw, existing.KeyHint) {
		t.Fatalf("the API key list must hold only masked keys: %s", list.raw)
	}

	newKey := map[string]any{"name": "from-agent", "expired_time": -1, "unlimited_quota": true}
	expectDenied(t, f.do(http.MethodPost, "/api/token/", read, newKey), http.StatusForbidden, common.APICodeManagementKeyForbidden, "read key creating an API key")
	expectDenied(t, f.do(http.MethodPut, "/api/token/?status_only=true", read, map[string]any{"id": existing.Id, "status": 2}), http.StatusForbidden, common.APICodeManagementKeyForbidden, "read key disabling an API key")
	expectDenied(t, f.do(http.MethodDelete, "/api/token/"+itoa(existing.Id), read, nil), http.StatusForbidden, common.APICodeManagementKeyForbidden, "read key deleting an API key")

	created := f.do(http.MethodPost, "/api/token/", write, newKey)
	expectAllowed(t, created, "write key creating an API key")
	data, _ := created.body["data"].(map[string]any)
	if key, _ := data["key"].(string); key == "" {
		t.Fatalf("the new API key should be returned once: %s", created.raw)
	}
	expectAllowed(t, f.do(http.MethodPut, "/api/token/?status_only=true", write, map[string]any{"id": existing.Id, "status": 2}), "write key disabling an API key")
	expectAllowed(t, f.do(http.MethodDelete, "/api/token/"+itoa(existing.Id), write, nil), "write key deleting an API key")
}

// No management key creates management keys, touches password, login methods, 2FA, sessions or
// payments, or reaches the admin console, whatever its user's role.
func TestManagementKeyCannotEscalate(t *testing.T) {
	f := setupManagementKeyRouter(t, false)
	userKey := f.managementKey(f.user.Id, model.ManagementKeyScopeWrite)
	adminKey := f.managementKey(f.admin.Id, model.ManagementKeyScopeWrite)

	denied := []struct{ method, path string }{
		{http.MethodGet, "/api/management_key/"},
		{http.MethodPost, "/api/management_key/"},
		{http.MethodPut, "/api/user/self"},
		{http.MethodPost, "/api/user/unbind"},
		{http.MethodPost, "/api/user/token"},
		{http.MethodGet, "/api/user/sessions"},
		{http.MethodPost, "/api/user/sessions/logout_others"},
		{http.MethodPost, "/api/webauthn/registration/begin"},
		{http.MethodDelete, "/api/webauthn/credentials/1"},
		{http.MethodPost, "/api/user/topup"},
		{http.MethodPost, "/api/user/order"},
		{http.MethodPut, "/api/user/setting"},
		{http.MethodPost, "/api/token/playground"},
		{http.MethodGet, "/api/log/self/detail/1"},
	}
	for _, target := range denied {
		expectDenied(t, f.do(target.method, target.path, userKey, map[string]any{}), http.StatusForbidden, common.APICodeManagementKeyForbidden, "management key on "+target.method+" "+target.path)
	}
	adminOnly := []struct{ method, path string }{
		{http.MethodGet, "/api/channel/"},
		{http.MethodGet, "/api/option/"},
		{http.MethodGet, "/api/user/"},
		{http.MethodPost, "/api/user/manage"},
		{http.MethodGet, "/api/log/"},
		{http.MethodGet, "/api/token/admin/search"},
		{http.MethodGet, "/api/organization/"},
	}
	for _, target := range adminOnly {
		expectDenied(t, f.do(target.method, target.path, adminKey, nil), http.StatusForbidden, common.APICodeManagementKeyForbidden, "admin's management key on "+target.method+" "+target.path)
	}
}

// In an organization a management key acts within its creator's role, and the API keys it
// creates are the creator's (created_by), so member budgets and model lists apply to them.
func TestManagementKeyOrganizationCeiling(t *testing.T) {
	f := setupManagementKeyRouter(t, false)
	org, err := model.CreateOrganizationWithOwner("Agents", "", f.admin.Id)
	if err != nil {
		t.Fatalf("create organization: %v", err)
	}
	member := &model.OrganizationMember{OrganizationId: org.Id, UserId: f.user.Id, Role: model.OrgRoleMember}
	if err := member.Insert(); err != nil {
		t.Fatalf("add member: %v", err)
	}
	memberKey := f.managementKey(f.user.Id, model.ManagementKeyScopeWrite)
	base := "/api/org/" + itoa(org.Id)

	expectAllowed(t, f.do(http.MethodGet, base+"/token", memberKey, nil), "member key listing organization keys")
	created := f.do(http.MethodPost, base+"/token", memberKey, map[string]any{"name": "org-agent", "expired_time": -1, "unlimited_quota": true})
	expectAllowed(t, created, "member key creating an organization key")
	data, _ := created.body["data"].(map[string]any)
	var stored model.Token
	if err := model.DB.First(&stored, int(data["id"].(float64))).Error; err != nil {
		t.Fatalf("load created key: %v", err)
	}
	if stored.UserId != org.ShadowUserId || stored.CreatedBy != f.user.Id {
		t.Fatalf("organization key should charge the organization and belong to the member: %+v", stored)
	}

	// The member's role still applies: their key neither sees nor deletes the owner's key.
	ownerToken := &model.Token{UserId: org.ShadowUserId, CreatedBy: f.admin.Id, Name: "owner-key", ExpiredTime: -1, UnlimitedQuota: true, Status: config.TokenStatusEnabled}
	if err := ownerToken.Insert(); err != nil {
		t.Fatalf("create owner key: %v", err)
	}
	listed := f.do(http.MethodGet, base+"/token", memberKey, nil)
	expectAllowed(t, listed, "member key listing organization keys")
	if strings.Contains(listed.raw, "owner-key") || !strings.Contains(listed.raw, "org-agent") {
		t.Fatalf("a member's key should list only the member's keys: %s", listed.raw)
	}
	if w := f.do(http.MethodDelete, base+"/token/"+itoa(ownerToken.Id), memberKey, nil); w.success() {
		t.Fatalf("a member's key must not delete the owner's key: %s", w.raw)
	}
	// A user outside the organization gets nothing from it.
	outsider := f.createUser("mk-outsider", config.RoleCommonUser)
	outsiderKey := f.managementKey(outsider.Id, model.ManagementKeyScopeWrite)
	expectDenied(t, f.do(http.MethodGet, base+"/token", outsiderKey, nil), http.StatusForbidden, common.APICodeOrgNotMember, "outsider key on the organization")
	// Even an owner's key cannot change the organization, manage members or move money.
	ownerKey := f.managementKey(f.admin.Id, model.ManagementKeyScopeWrite)
	expectDenied(t, f.do(http.MethodPut, base+"/", ownerKey, map[string]any{"name": "Renamed"}), http.StatusForbidden, common.APICodeManagementKeyForbidden, "owner key updating the organization")
	expectDenied(t, f.do(http.MethodPut, base+"/members/"+itoa(f.user.Id), ownerKey, map[string]any{"role": "admin"}), http.StatusForbidden, common.APICodeManagementKeyForbidden, "owner key changing a member role")
	expectDenied(t, f.do(http.MethodPost, base+"/quota/transfer", ownerKey, map[string]any{"quota": 1}), http.StatusForbidden, common.APICodeManagementKeyForbidden, "owner key moving quota")
}

// Management keys need an expiry: 90 days by default, at most a year; an expired key is refused.
// They are managed from a console session, stored as a hash only and shown once.
func TestManagementKeyLifecycle(t *testing.T) {
	f := setupManagementKeyRouter(t, false)
	cookies := f.login(f.user.Id)

	expectDenied(t, f.do(http.MethodPost, "/api/management_key/", "", map[string]any{"name": "x", "scope": "read", "expired_time": -1}, cookies...), http.StatusOK, common.APICodeManagementKeyExpiryRequired, "never-expiring key")
	tooLong := time.Now().Add(400 * 24 * time.Hour).Unix()
	expectDenied(t, f.do(http.MethodPost, "/api/management_key/", "", map[string]any{"name": "x", "scope": "read", "expired_time": tooLong}, cookies...), http.StatusOK, common.APICodeManagementKeyExpiryTooLong, "key valid over a year")
	expectDenied(t, f.do(http.MethodPost, "/api/management_key/", "", map[string]any{"name": "x", "scope": "admin"}, cookies...), http.StatusOK, common.APICodeManagementKeyScopeInvalid, "unknown scope")
	expectDenied(t, f.do(http.MethodPost, "/api/management_key/", "", map[string]any{"name": " ", "scope": "read"}, cookies...), http.StatusOK, common.APICodeManagementKeyNameRequired, "empty name")

	before := time.Now().Unix()
	created := f.do(http.MethodPost, "/api/management_key/", "", map[string]any{"name": "agent", "scope": "read"}, cookies...)
	expectAllowed(t, created, "create a management key")
	data, _ := created.body["data"].(map[string]any)
	key, _ := data["key"].(string)
	if !common.IsManagementKeyFormat(key) {
		t.Fatalf("new key has the wrong format: %q", key)
	}
	expiry := int64(data["expired_time"].(float64))
	if want := before + int64(model.ManagementKeyDefaultLifetime/time.Second); expiry < want || expiry > want+5 {
		t.Fatalf("default expiry should be 90 days, got %d (want about %d)", expiry, want)
	}

	// Only the hash and the hint are stored, and the list never shows the key again.
	var stored model.ManagementKey
	if err := model.DB.First(&stored, int(data["id"].(float64))).Error; err != nil {
		t.Fatalf("load key: %v", err)
	}
	if stored.KeyHash != common.HashManagementKey(key) || stored.KeyHint != common.ManagementKeyHint(key) || stored.KeyHint == key {
		t.Fatalf("stored key is not hash + hint: %+v", stored)
	}
	var row map[string]any
	model.DB.Table("management_keys").Where("id = ?", stored.Id).Take(&row)
	for column, value := range row {
		if s, ok := value.(string); ok && strings.Contains(s, key[3:]) {
			t.Fatalf("column %s holds the key", column)
		}
	}
	list := f.do(http.MethodGet, "/api/management_key/", "", nil, cookies...)
	expectAllowed(t, list, "list management keys")
	if strings.Contains(list.raw, key) || !strings.Contains(list.raw, stored.KeyHint) {
		t.Fatalf("the list must show the masked key only: %s", list.raw)
	}

	expectAllowed(t, f.do(http.MethodGet, "/api/token/", key, nil), "fresh key")

	// Expired keys are refused with their own code.
	model.DB.Model(&model.ManagementKey{}).Where("id = ?", stored.Id).Update("expired_time", time.Now().Unix()-1)
	expectDenied(t, f.do(http.MethodGet, "/api/token/", key, nil), http.StatusUnauthorized, common.APICodeManagementKeyExpired, "expired key")

	// Deleting revokes at once.
	model.DB.Model(&model.ManagementKey{}).Where("id = ?", stored.Id).Update("expired_time", time.Now().Add(time.Hour).Unix())
	expectAllowed(t, f.do(http.MethodDelete, "/api/management_key/"+itoa(stored.Id), "", nil, cookies...), "delete management key")
	expectDenied(t, f.do(http.MethodGet, "/api/token/", key, nil), http.StatusUnauthorized, common.APICodeInvalidAccessToken, "deleted key")
}

// A legacy access token keeps its old access for one release, marked deprecated; regenerating it
// shows the new one once and stores only its hash.
func TestLegacyAccessTokenStillWorks(t *testing.T) {
	f := setupManagementKeyRouter(t, false)
	legacy := "0123456789abcdef0123456789abcdef"
	if err := model.DB.Create(&model.ManagementKey{UserId: f.admin.Id, Name: model.LegacyAccessTokenName, Scope: model.ManagementKeyScopeLegacy, Key: legacy, ExpiredTime: -1}).Error; err != nil {
		t.Fatalf("create legacy token: %v", err)
	}

	w := f.do(http.MethodGet, "/api/channel/", legacy, nil)
	expectAllowed(t, w, "legacy access token on an admin route")
	if w.header.Get("Deprecation") != "true" || !strings.Contains(w.header.Get("Warning"), "deprecated") {
		t.Fatalf("legacy access token responses should be marked deprecated: %v", w.header)
	}
	// It cannot manage management keys.
	expectDenied(t, f.do(http.MethodGet, "/api/management_key/", legacy, nil), http.StatusForbidden, common.APICodeManagementKeyForbidden, "legacy token on management keys")

	cookies := f.login(f.admin.Id)
	regenerated := f.do(http.MethodPost, "/api/user/token", "", nil, cookies...)
	expectAllowed(t, regenerated, "regenerate access token")
	token, _ := regenerated.body["data"].(string)
	if len(token) != 32 || regenerated.body["deprecated"] != true {
		t.Fatalf("unexpected access token answer: %s", regenerated.raw)
	}
	var count int64
	model.DB.Model(&model.ManagementKey{}).Where("user_id = ? AND scope = ?", f.admin.Id, model.ManagementKeyScopeLegacy).Count(&count)
	var stored model.ManagementKey
	model.DB.Where("user_id = ? AND scope = ?", f.admin.Id, model.ManagementKeyScopeLegacy).First(&stored)
	if count != 1 || stored.KeyHash != common.HashManagementKey(token) {
		t.Fatalf("the regenerated token should replace the old one as a hash: count=%d %+v", count, stored)
	}
	expectDenied(t, f.do(http.MethodGet, "/api/channel/", legacy, nil), http.StatusUnauthorized, common.APICodeInvalidAccessToken, "replaced legacy token")
	expectAllowed(t, f.do(http.MethodGet, "/api/channel/", token, nil), "regenerated legacy token")
}

// MCP takes a management key in the Authorization header, and refuses API keys.
func TestMCPAcceptsManagementKey(t *testing.T) {
	f := setupManagementKeyRouter(t, true)
	read := f.managementKey(f.user.Id, model.ManagementKeyScopeRead)
	_, apiKey := f.apiKey(f.user.Id, "app")
	initialize := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "test", "version": "1"}},
	}

	w := f.do(http.MethodPost, "/mcp", read, initialize)
	if w.status != http.StatusOK || w.code() != "" {
		t.Fatalf("MCP should accept a read management key, got %d %s", w.status, w.raw)
	}
	expectDenied(t, f.do(http.MethodPost, "/mcp", apiKey, initialize), http.StatusUnauthorized, common.APICodeAPIKeyNotAllowed, "API key on MCP")
	expectDenied(t, f.do(http.MethodPost, "/mcp", "", initialize), http.StatusUnauthorized, common.APICodeNotSignedIn, "MCP without a key")
}

// The admin MCP endpoint answers root only: a read management key of root works, an admin's or
// a user's is refused. Its tool list holds the settings tools.
func TestAdminMCPIsRootOnly(t *testing.T) {
	f := setupManagementKeyRouter(t, true)
	root := f.createUser("mk-root", config.RoleRootUser)
	rootKey := f.managementKey(root.Id, model.ManagementKeyScopeRead)
	adminKey := f.managementKey(f.admin.Id, model.ManagementKeyScopeRead)
	userKey := f.managementKey(f.user.Id, model.ManagementKeyScopeRead)
	initialize := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "test", "version": "1"}},
	}
	if w := f.do(http.MethodPost, "/mcp/admin", rootKey, initialize); w.status != http.StatusOK || w.code() != "" {
		t.Fatalf("admin MCP should accept root's read key, got %d %s", w.status, w.raw)
	}
	for name, key := range map[string]string{"admin": adminKey, "user": userKey} {
		w := f.do(http.MethodPost, "/mcp/admin", key, initialize)
		if w.status == http.StatusOK && w.code() == "" {
			t.Errorf("admin MCP accepted the %s's key: %s", name, w.raw)
		}
	}
}
