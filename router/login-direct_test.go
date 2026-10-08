package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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

const browserAccept = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"

// fakeIdP 起一个只提供 discovery 文档的假 IdP：/login 直达要走真实的 oidc.Get 发现流程。
func fakeIdP(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q,"jwks_uri":%q,"id_token_signing_alg_values_supported":["RS256"]}`,
			server.URL, server.URL+"/authorize", server.URL+"/token", server.URL+"/jwks")
	})
	return server.URL
}

// setupLoginDirect 准备内存库 + 一个已启用的提供方，并返回挂好 /login 直达路由的引擎。
// fallback 与生产一致地返回 200 HTML，便于区分「直达」与「落回前端页面」。
func setupLoginDirect(t *testing.T, issuer string) *gin.Engine {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&model.OidcProvider{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })

	provider := &model.OidcProvider{
		Slug: "keycloak", DisplayName: "Keycloak", Issuer: issuer,
		ClientId: "modeltaps", Scopes: "openid,profile,email", Enabled: true,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("创建提供方失败: %v", err)
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	RegisterLoginDirect(engine, func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte("<html>spa</html>"))
	})
	return engine
}

// 外部账号体系下浏览器导航到 /login 应直接 302 到 IdP 授权页（带 state / PKCE / nonce / ui_locales，
// 不再申请 access_type=offline）；`?local=1` 永久跳转到 /login/admin；非 HTML 请求照常 200 返回前端页面。
func TestLoginDirectRoute(t *testing.T) {
	issuer := fakeIdP(t)
	engine := setupLoginDirect(t, issuer)

	oldSystem := config.AccountSystem
	config.AccountSystem = config.AccountSystemExternal
	t.Cleanup(func() { config.AccountSystem = oldSystem })

	do := func(target, accept, acceptLanguage string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		if acceptLanguage != "" {
			req.Header.Set("Accept-Language", acceptLanguage)
		}
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		return w
	}

	t.Run("浏览器导航 302 到 IdP", func(t *testing.T) {
		w := do("/login", browserAccept, "zh-CN,zh;q=0.9")
		if w.Code != http.StatusFound {
			t.Fatalf("期望 302，实际 %d", w.Code)
		}
		location, err := url.Parse(w.Header().Get("Location"))
		if err != nil {
			t.Fatalf("Location 不是合法 URL: %v", err)
		}
		if got := location.Scheme + "://" + location.Host + location.Path; got != issuer+"/authorize" {
			t.Fatalf("期望跳到 %s/authorize，实际 %s", issuer, got)
		}
		q := location.Query()
		for key, want := range map[string]string{
			"client_id":             "modeltaps",
			"response_type":         "code",
			"scope":                 "openid profile email",
			"code_challenge_method": "S256",
			"ui_locales":            "zh-CN",
		} {
			if q.Get(key) != want {
				t.Fatalf("授权 URL 的 %s = %q，期望 %q", key, q.Get(key), want)
			}
		}
		if q.Get("state") == "" || q.Get("code_challenge") == "" || q.Get("nonce") == "" {
			t.Fatalf("授权 URL 缺少 state / code_challenge / nonce: %s", location.RawQuery)
		}
		if q.Get("access_type") != "" {
			t.Fatal("不应再申请 access_type=offline")
		}
		if len(w.Result().Cookies()) == 0 {
			t.Fatal("未下发会话 Cookie：state 没有落到会话里，回调会校验失败")
		}
	})

	t.Run("local=1 永久跳转到 /login/admin", func(t *testing.T) {
		w := do("/login?local=1", browserAccept, "")
		if w.Code != http.StatusMovedPermanently || w.Header().Get("Location") != "/login/admin" {
			t.Fatalf("期望 301 到 /login/admin，实际 %d（Location=%q）", w.Code, w.Header().Get("Location"))
		}
	})

	t.Run("内置账号模式落回前端页面", func(t *testing.T) {
		config.AccountSystem = config.AccountSystemBuiltin
		defer func() { config.AccountSystem = config.AccountSystemExternal }()
		if w := do("/login", browserAccept, ""); w.Code != http.StatusOK {
			t.Fatalf("期望 200，实际 %d（Location=%q）", w.Code, w.Header().Get("Location"))
		}
	})

	t.Run("非 HTML 请求落回前端页面", func(t *testing.T) {
		if w := do("/login", "*/*", ""); w.Code != http.StatusOK {
			t.Fatalf("期望 200，实际 %d（Location=%q）", w.Code, w.Header().Get("Location"))
		}
	})
}
