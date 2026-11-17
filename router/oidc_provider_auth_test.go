package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const (
	oidcProviderRootToken  = "oidcproviderroottoken00000000001"
	oidcProviderAdminToken = "oidcprovideradmintoken0000000002"
)

// setupOidcProviderRouter mounts the real /api routes on an in-memory database with one root and
// one admin, each holding a legacy access token, and one provider to act on.
func setupOidcProviderRouter(t *testing.T) *gin.Engine {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open the in-memory database: %v", err)
	}
	if err := testDB.AutoMigrate(&model.User{}, &model.ManagementKey{}, &model.OidcProvider{}, &model.UserOidcIdentity{}, &model.UserSession{}); err != nil {
		t.Fatalf("failed to migrate test tables: %v", err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })

	users := []*model.User{
		{Username: "oidc-root", DisplayName: "root", Role: config.RoleRootUser, Status: config.UserStatusEnabled, AffCode: "affroot"},
		{Username: "oidc-admin", DisplayName: "admin", Role: config.RoleAdminUser, Status: config.UserStatusEnabled, AffCode: "affadmin"},
	}
	for i, u := range users {
		if err := testDB.Create(u).Error; err != nil {
			t.Fatalf("failed to create user %s: %v", u.Username, err)
		}
		token := []string{oidcProviderRootToken, oidcProviderAdminToken}[i]
		if err := testDB.Create(&model.ManagementKey{UserId: u.Id, Scope: model.ManagementKeyScopeLegacy, Key: token, ExpiredTime: -1}).Error; err != nil {
			t.Fatalf("failed to create access token: %v", err)
		}
	}
	provider := &model.OidcProvider{Slug: "idp", Issuer: "https://idp.example.com", ClientId: "cid", ClientSecret: "stored", Scopes: "openid"}
	if err := provider.Insert(); err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	SetApiRouter(engine)
	return engine
}

// SEC-53: every identity provider route is root only. An admin who could edit a provider could
// point it at an IdP of their own and sign in as root, or send the stored client secret there.
func TestOidcProviderRoutesRequireRoot(t *testing.T) {
	engine := setupOidcProviderRouter(t)
	routes := []struct{ method, target, body string }{
		{http.MethodGet, "/api/oidc_provider/", ""},
		{http.MethodGet, "/api/oidc_provider/1", ""},
		{http.MethodPost, "/api/oidc_provider/?force=true", `{"slug":"evil","issuer":"https://127.0.0.1:1","client_id":"x","scopes":"openid","enabled":true}`},
		{http.MethodPut, "/api/oidc_provider/1?force=true", `{"issuer":"https://127.0.0.1:1","client_id":"cid","scopes":"openid","enabled":true}`},
		{http.MethodPut, "/api/oidc_provider/1/status", `{"enabled":true}`},
		{http.MethodPost, "/api/oidc_provider/1/test", ""},
		{http.MethodDelete, "/api/oidc_provider/1", ""},
	}
	request := func(method, target, body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		return w
	}

	for _, route := range routes {
		for name, token := range map[string]string{"anonymous": "", "admin": oidcProviderAdminToken} {
			w := request(route.method, route.target, route.body, token)
			if modelInfoSuccess(t, w) || w.Code == http.StatusBadRequest || w.Code == http.StatusNotFound || w.Code == http.StatusConflict {
				t.Errorf("%s %s by %s should be refused before the handler, got %d (body=%s)", route.method, route.target, name, w.Code, w.Body.String())
			}
		}
	}

	stored, err := model.GetOidcProviderById(1)
	if err != nil {
		t.Fatalf("the provider should still exist: %v", err)
	}
	if stored.Issuer != "https://idp.example.com" || stored.ClientSecret != "stored" || stored.Enabled {
		t.Fatalf("refused requests must not change the provider: %+v", stored)
	}
	var count int64
	model.DB.Model(&model.OidcProvider{}).Count(&count)
	if count != 1 {
		t.Fatalf("refused requests must not create providers, got %d", count)
	}

	if w := request(http.MethodGet, "/api/oidc_provider/", "", oidcProviderRootToken); w.Code != http.StatusOK || !modelInfoSuccess(t, w) {
		t.Fatalf("root should list providers, got %d (body=%s)", w.Code, w.Body.String())
	}
}
