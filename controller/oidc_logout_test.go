package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// newFakeIdPForLogout 起一个只提供 discovery 文档的假 IdP。withEndSession 决定文档里是否声明
// end_session_endpoint（该字段在规范里是可选的）。
func newFakeIdPForLogout(t *testing.T, withEndSession bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		doc := map[string]interface{}{
			"issuer":                                server.URL,
			"authorization_endpoint":                server.URL + "/oauth2/authorize",
			"token_endpoint":                        server.URL + "/oauth2/token",
			"jwks_uri":                              server.URL + "/oauth2/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		}
		if withEndSession {
			doc["end_session_endpoint"] = server.URL + "/oauth2/end_session"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	})
	return server
}

// newLogoutRouter 提供「建立会话」与「退出」两个端点：前者模拟一次登录（带 provider 参数即模拟
// 经身份提供方登录，会话行上记下 id_token），后者就是真实的 Logout。
func newLogoutRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	r.GET("/login", func(c *gin.Context) {
		user := &model.User{Id: 1, Username: "u1"}
		if providerId, _ := strconv.Atoi(c.Query("provider")); providerId > 0 {
			provider, err := model.GetOidcProviderById(providerId)
			if err != nil {
				t.Fatalf("读取提供方失败: %v", err)
			}
			oidcSetupLogin(c, provider, user, "raw-id-token")
			return
		}
		setupLogin(user, c)
	})
	r.GET("/logout", Logout)
	return r
}

// logoutRedirectURL 先登录拿到会话 cookie，再带着它调退出，返回 data.redirect_url。
func logoutRedirectURL(t *testing.T, r *gin.Engine, providerId int) string {
	t.Helper()
	return logoutRedirectURLWithQuery(t, r, providerId, "")
}

// logoutRedirectURLWithQuery 同上，query 为退出请求上附带的查询串（含 `?`），用于覆盖 ui_locales。
func logoutRedirectURLWithQuery(t *testing.T, r *gin.Engine, providerId int, query string) string {
	t.Helper()
	loginRecorder := httptest.NewRecorder()
	r.ServeHTTP(loginRecorder, httptest.NewRequest(http.MethodGet, "/login?provider="+strconv.Itoa(providerId), nil))
	cookies := loginRecorder.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("登录应下发会话 cookie，实际 %s", loginRecorder.Body.String())
	}
	if n := countUserSessions(t, 1); n != 1 {
		t.Fatalf("登录后应有 1 行会话，实际 %d", n)
	}

	req := httptest.NewRequest(http.MethodGet, "/logout"+query, nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			RedirectURL string `json:"redirect_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析退出响应失败: %v (body=%s)", err, w.Body.String())
	}
	if !resp.Success {
		t.Fatalf("退出应成功，实际 %s", w.Body.String())
	}
	if n := countUserSessions(t, 1); n != 0 {
		t.Fatalf("退出后会话行（含 id_token）应已删除，实际还剩 %d 行", n)
	}
	return resp.Data.RedirectURL
}

func countUserSessions(t *testing.T, userId int) int64 {
	t.Helper()
	var count int64
	if err := model.DB.Model(&model.UserSession{}).Where("user_id = ?", userId).Count(&count).Error; err != nil {
		t.Fatalf("统计会话行失败: %v", err)
	}
	return count
}

func setServerAddress(t *testing.T, address string) {
	t.Helper()
	old := config.ServerAddress
	config.ServerAddress = address
	t.Cleanup(func() { config.ServerAddress = old })
}

// 经身份提供方登录的会话退出时应拿到该提供方的 end_session 地址，带 id_token_hint 与本站 /signed-out 回跳。
func TestLogoutReturnsEndSessionURLForOidcSession(t *testing.T) {
	setupOidcTestDB(t)
	setServerAddress(t, "https://modeltaps.test/")
	idp := newFakeIdPForLogout(t, true)

	provider := seedOidcProvider(t, "authgear", false)
	provider.Issuer = idp.URL
	if err := provider.Update(); err != nil {
		t.Fatalf("更新提供方失败: %v", err)
	}

	redirectURL := logoutRedirectURL(t, newLogoutRouter(t), provider.Id)
	parsed, err := url.Parse(redirectURL)
	if err != nil {
		t.Fatalf("redirect_url 应可解析: %v (%s)", err, redirectURL)
	}
	if got, want := parsed.Scheme+"://"+parsed.Host+parsed.Path, idp.URL+"/oauth2/end_session"; got != want {
		t.Fatalf("redirect_url 应指向 end_session_endpoint，期望 %s 实际 %s", want, got)
	}
	if got := parsed.Query().Get("id_token_hint"); got != "raw-id-token" {
		t.Fatalf("应携带 id_token_hint，实际 %q", got)
	}
	if got := parsed.Query().Get("post_logout_redirect_uri"); got != "https://modeltaps.test/signed-out" {
		t.Fatalf("post_logout_redirect_uri 不正确，实际 %q", got)
	}
	if got := parsed.Query().Get("client_id"); got != provider.ClientId {
		t.Fatalf("应携带 client_id，实际 %q", got)
	}
}

// 退出跳转只在 ui_locales 合法时携带该参数：缺省与非法值下 URL 与不带时一致。
func TestLogoutEndSessionURLUILocales(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"合法", "?ui_locales=zh-HK", "zh-HK"},
		{"缺省", "", ""},
		{"非法", "?ui_locales=fr", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupOidcTestDB(t)
			setServerAddress(t, "https://modeltaps.test")
			idp := newFakeIdPForLogout(t, true)

			provider := seedOidcProvider(t, "authgear", false)
			provider.Issuer = idp.URL
			if err := provider.Update(); err != nil {
				t.Fatalf("更新提供方失败: %v", err)
			}

			redirectURL := logoutRedirectURLWithQuery(t, newLogoutRouter(t), provider.Id, tc.query)
			parsed, err := url.Parse(redirectURL)
			if err != nil {
				t.Fatalf("redirect_url 应可解析: %v (%s)", err, redirectURL)
			}
			if got := parsed.Query().Get("ui_locales"); got != tc.want {
				t.Fatalf("ui_locales 期望 %q 实际 %q", tc.want, got)
			}
		})
	}
}

// 密码登录的会话行上没有提供方，退出时 redirect_url 为空，前端走纯本地退出。
func TestLogoutReturnsEmptyRedirectForPasswordSession(t *testing.T) {
	setupOidcTestDB(t)
	setServerAddress(t, "https://modeltaps.test")

	if got := logoutRedirectURL(t, newLogoutRouter(t), 0); got != "" {
		t.Fatalf("密码登录退出不应下发 redirect_url，实际 %q", got)
	}
}

// 提供方的 discovery 未声明 end_session_endpoint 时无法结束 IdP 会话，redirect_url 为空。
func TestLogoutReturnsEmptyRedirectWhenProviderHasNoEndSession(t *testing.T) {
	setupOidcTestDB(t)
	setServerAddress(t, "https://modeltaps.test")
	idp := newFakeIdPForLogout(t, false)

	provider := seedOidcProvider(t, "authgear", false)
	provider.Issuer = idp.URL
	if err := provider.Update(); err != nil {
		t.Fatalf("更新提供方失败: %v", err)
	}

	if got := logoutRedirectURL(t, newLogoutRouter(t), provider.Id); got != "" {
		t.Fatalf("提供方无 end_session_endpoint 时不应下发 redirect_url，实际 %q", got)
	}
}
