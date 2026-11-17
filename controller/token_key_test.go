package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/middleware"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// SEC-47: the key is returned once, by create and reset; lists and details (personal,
// organization and site admin) carry only the masked hint; organization owners and admins cannot
// get a member's key; the playground gets short-lived keys.

type keyTestEnv struct {
	r      *gin.Engine
	org    *model.Organization
	owner  *model.User
	member *model.User
}

func setupKeyTest(t *testing.T) *keyTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if viper.GetString("user_token_secret") == "" {
		viper.Set("user_token_secret", "token-key-test-secret")
		if err := common.InitUserToken(); err != nil {
			t.Fatal(err)
		}
	}
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&model.User{}, &model.Organization{}, &model.OrganizationAuditLog{}, &model.Token{}, &model.PlaygroundKey{}, &model.Log{}); err != nil {
		t.Fatal(err)
	}
	prevDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = prevDB })

	owner := &model.User{Id: 1, Username: "owner", AffCode: "a-owner", Group: "default", Status: config.UserStatusEnabled}
	member := &model.User{Id: 2, Username: "member", AffCode: "a-member", Group: "default", Status: config.UserStatusEnabled}
	shadow := &model.User{Id: 11, Username: "shadow", AffCode: "a-shadow", Group: "default", Status: config.UserStatusEnabled, Type: config.UserTypeOrgShadow}
	for _, u := range []*model.User{owner, member, shadow} {
		if err := testDB.Create(u).Error; err != nil {
			t.Fatal(err)
		}
	}
	org := &model.Organization{Id: 1, Name: "Org", Slug: "org1", Status: model.OrganizationStatusEnabled, ShadowUserId: shadow.Id, CreatedBy: owner.Id}
	if err := testDB.Create(org).Error; err != nil {
		t.Fatal(err)
	}

	// The caller is chosen per request with the X-Test-User / X-Test-Role / X-Test-Org-Role headers.
	as := func(c *gin.Context) {
		id := 1
		if c.GetHeader("X-Test-User") == "2" {
			id = 2
		}
		role := config.RoleCommonUser
		if c.GetHeader("X-Test-Role") == "admin" {
			role = config.RoleAdminUser
		}
		c.Set("id", id)
		c.Set("role", role)
		c.Next()
	}
	asOrg := func(c *gin.Context) {
		c.Set(middleware.OrgContextKey, org)
		orgRole := c.GetHeader("X-Test-Org-Role")
		if orgRole == "" {
			orgRole = model.OrgRoleMember
		}
		c.Set(middleware.OrgRoleContextKey, orgRole)
		c.Next()
	}
	r := gin.New()
	r.Use(as)
	r.POST("/api/token/", AddToken)
	r.GET("/api/token/", GetUserTokensList)
	r.GET("/api/token/:id", GetToken)
	r.PUT("/api/token/", UpdateToken)
	r.POST("/api/token/:id/reset_key", ResetTokenKey)
	r.POST("/api/token/playground", IssuePlaygroundKey)
	r.GET("/api/token/admin/search", GetTokensListByAdmin)
	r.POST("/api/org/:id/token", asOrg, AddOrgToken)
	r.GET("/api/org/:id/token", asOrg, GetOrgTokensList)
	r.GET("/api/org/:id/token/:token_id", asOrg, GetOrgToken)
	r.POST("/api/org/:id/token/:token_id/reset_key", asOrg, ResetOrgTokenKey)
	return &keyTestEnv{r: r, org: org, owner: owner, member: member}
}

// call sends a request and returns the status, the decoded body and the raw body text.
func (e *keyTestEnv) call(t *testing.T, method, path string, body any, headers map[string]string) (int, map[string]any, string) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	resp := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return w.Code, resp, w.Body.String()
}

func newKeyFrom(t *testing.T, body map[string]any) (int, string, string) {
	t.Helper()
	if body["success"] != true {
		t.Fatalf("request failed: %v", body)
	}
	data, _ := body["data"].(map[string]any)
	key, _ := data["key"].(string)
	hint, _ := data["key_hint"].(string)
	id, _ := data["id"].(float64)
	if key == "" || hint == "" || id == 0 {
		t.Fatalf("response has no id/key/hint: %v", body)
	}
	return int(id), key, hint
}

// assertNoSecret fails when a response body carries the key or its stored hash.
func assertNoSecret(t *testing.T, what, raw, key string) {
	t.Helper()
	if strings.Contains(raw, key) {
		t.Fatalf("%s leaks the key: %s", what, raw)
	}
	if strings.Contains(raw, common.HashUserTokenKey(key)) {
		t.Fatalf("%s leaks the key hash: %s", what, raw)
	}
	if strings.Contains(raw, `"key":`) {
		t.Fatalf("%s has a key field: %s", what, raw)
	}
}

func TestPersonalKeyShownOnceAndMaskedEverywhere(t *testing.T) {
	env := setupKeyTest(t)

	_, body, _ := env.call(t, http.MethodPost, "/api/token/", map[string]any{"name": "ci", "expired_time": -1, "unlimited_quota": true}, nil)
	id, key, hint := newKeyFrom(t, body)
	if !common.IsUserTokenKeyFormat(key) || hint != common.UserTokenKeyHint(key) {
		t.Fatalf("create returned key %q hint %q", key, hint)
	}
	stored, err := model.GetTokenById(id)
	if err != nil || stored.KeyHash != common.HashUserTokenKey(key) {
		t.Fatalf("stored token %+v %v", stored, err)
	}

	paths := map[string]string{
		"list":   "/api/token/",
		"detail": "/api/token/" + itoa(id),
		"admin":  "/api/token/admin/search?user_id=1",
	}
	for what, path := range paths {
		_, resp, raw := env.call(t, http.MethodGet, path, nil, map[string]string{"X-Test-Role": "admin"})
		if resp["success"] != true {
			t.Fatalf("%s failed: %s", what, raw)
		}
		assertNoSecret(t, what, raw, key)
		if !strings.Contains(raw, hint) {
			t.Fatalf("%s has no hint: %s", what, raw)
		}
	}
	// An edit answers with the stored key, masked too.
	_, resp, raw := env.call(t, http.MethodPut, "/api/token/", map[string]any{"id": id, "name": "ci-2", "expired_time": -1, "unlimited_quota": true}, nil)
	if resp["success"] != true {
		t.Fatalf("update failed: %s", raw)
	}
	assertNoSecret(t, "update", raw, key)

	// The site admin finds a key by its full value (hashes cannot be searched by prefix).
	_, _, raw = env.call(t, http.MethodGet, "/api/token/admin/search?key=sk-"+key, nil, map[string]string{"X-Test-Role": "admin"})
	if !strings.Contains(raw, `"total_count":1`) || !strings.Contains(raw, hint) {
		t.Fatalf("admin search by full key: %s", raw)
	}
	_, _, raw = env.call(t, http.MethodGet, "/api/token/admin/search?key="+key[:10], nil, map[string]string{"X-Test-Role": "admin"})
	if !strings.Contains(raw, `"total_count":0`) {
		t.Fatalf("admin search by a prefix must find nothing: %s", raw)
	}

	// Reset: a new key, once; the old one stops working.
	_, body, _ = env.call(t, http.MethodPost, "/api/token/"+itoa(id)+"/reset_key", nil, nil)
	resetId, newKey, newHint := newKeyFrom(t, body)
	if resetId != id || newKey == key || newHint != common.UserTokenKeyHint(newKey) {
		t.Fatalf("reset returned id %d key %q hint %q", resetId, newKey, newHint)
	}
	if _, err := model.ValidateUserToken(key); err == nil {
		t.Fatal("the old key must stop working after a reset")
	}
	if got, err := model.ValidateUserToken(newKey); err != nil || got.Id != id {
		t.Fatalf("the new key must work: %v %v", got, err)
	}
	_, _, raw = env.call(t, http.MethodGet, "/api/token/"+itoa(id), nil, nil)
	assertNoSecret(t, "detail after reset", raw, newKey)

	// Someone else's key cannot be reset.
	_, body, _ = env.call(t, http.MethodPost, "/api/token/"+itoa(id)+"/reset_key", nil, map[string]string{"X-Test-User": "2"})
	if body["success"] == true || body["code"] != common.APICodeTokenNotFound {
		t.Fatalf("another user reset the key: %v", body)
	}
}

func TestOrgAdminCannotGetMemberKeys(t *testing.T) {
	env := setupKeyTest(t)
	member := map[string]string{"X-Test-User": "2"}
	ownerHeaders := map[string]string{"X-Test-Org-Role": model.OrgRoleOwner}

	_, body, _ := env.call(t, http.MethodPost, "/api/org/1/token", map[string]any{"name": "member-key", "expired_time": -1, "unlimited_quota": true}, member)
	id, key, _ := newKeyFrom(t, body)

	// The owner lists and opens the member's key: masked only.
	for what, path := range map[string]string{"org list": "/api/org/1/token", "org detail": "/api/org/1/token/" + itoa(id)} {
		_, resp, raw := env.call(t, http.MethodGet, path, nil, ownerHeaders)
		if resp["success"] != true {
			t.Fatalf("%s failed: %s", what, raw)
		}
		assertNoSecret(t, what, raw, key)
	}

	// Resetting would hand over a working key: only the creator may.
	for _, role := range []string{model.OrgRoleOwner, model.OrgRoleAdmin} {
		_, body, _ = env.call(t, http.MethodPost, "/api/org/1/token/"+itoa(id)+"/reset_key", nil, map[string]string{"X-Test-Org-Role": role})
		if body["success"] == true || body["code"] != common.APICodeTokenKeyResetNotCreator {
			t.Fatalf("%s reset a member's key: %v", role, body)
		}
	}
	if _, err := model.ValidateUserToken(key); err != nil {
		t.Fatalf("a refused reset must leave the key working: %v", err)
	}
	_, body, _ = env.call(t, http.MethodPost, "/api/org/1/token/"+itoa(id)+"/reset_key", nil, member)
	_, newKey, _ := newKeyFrom(t, body)
	if _, err := model.ValidateUserToken(newKey); err != nil {
		t.Fatalf("the creator's reset must work: %v", err)
	}
}

func TestPlaygroundKeyEndpoint(t *testing.T) {
	env := setupKeyTest(t)

	_, body, raw := env.call(t, http.MethodPost, "/api/token/playground", nil, nil)
	if body["success"] != true {
		t.Fatalf("playground key: %s", raw)
	}
	data, _ := body["data"].(map[string]any)
	key, _ := data["key"].(string)
	if !common.IsPlaygroundKeyFormat(key) || data["expires_at"] == nil {
		t.Fatalf("playground key response: %s", raw)
	}
	token, err := model.ValidateUserToken(key)
	if err != nil || token.Name != model.PlaygroundTokenName || token.UserId != env.owner.Id {
		t.Fatalf("playground key must open the user's playground API key: %+v %v", token, err)
	}

	// The playground API key is listed masked, and is managed by the system: no reset.
	_, _, raw = env.call(t, http.MethodGet, "/api/token/", nil, nil)
	assertNoSecret(t, "list with the playground key", raw, key)
	_, body, _ = env.call(t, http.MethodPost, "/api/token/"+itoa(token.Id)+"/reset_key", nil, nil)
	if body["success"] == true || body["code"] != common.APICodeTokenKeyResetManaged {
		t.Fatalf("the playground API key was reset: %v", body)
	}
}
