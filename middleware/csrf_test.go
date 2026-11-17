package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// csrfTestEngine serves /login (signs the cookie session in), a guarded write route, a guarded
// read route and the exempt payment notification route.
func csrfTestEngine(t *testing.T) (*gin.Engine, []*http.Cookie) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("csrf-test-secret"))))
	engine.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", 7)
		session.Set("username", "alice")
		_ = session.Save()
		c.Status(http.StatusOK)
	})
	api := engine.Group("/api")
	api.Use(CrossSiteRequestGuard())
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	api.POST("/write", ok)
	api.DELETE("/write", ok)
	api.GET("/write", ok)
	api.Any("/payment/notify/:uuid", ok)

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/login", nil))
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login set no session cookie")
	}
	return engine, cookies
}

func setCSRFSiteConfig(t *testing.T, serverAddress, frontendBaseURL string) {
	t.Helper()
	oldAddress := config.ServerAddress
	oldFrontend := viper.GetString("frontend_base_url")
	config.ServerAddress = serverAddress
	viper.Set("frontend_base_url", frontendBaseURL)
	t.Cleanup(func() {
		config.ServerAddress = oldAddress
		viper.Set("frontend_base_url", oldFrontend)
	})
}

type csrfCase struct {
	name     string
	method   string
	path     string
	host     string
	peer     string
	headers  map[string]string
	signedIn bool
	want     int
}

func runCSRFCases(t *testing.T, engine *gin.Engine, cookies []*http.Cookie, cases []csrfCase) {
	t.Helper()
	for _, tc := range cases {
		method := tc.method
		if method == "" {
			method = http.MethodPost
		}
		path := tc.path
		if path == "" {
			path = "/api/write"
		}
		req := httptest.NewRequest(method, path, nil)
		if tc.host != "" {
			req.Host = tc.host
		}
		if tc.peer != "" {
			req.RemoteAddr = tc.peer + ":40000"
		}
		for k, v := range tc.headers {
			req.Header.Set(k, v)
		}
		if tc.signedIn {
			for _, c := range cookies {
				req.AddCookie(c)
			}
		}
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Errorf("%s: status %d, want %d (body %s)", tc.name, w.Code, tc.want, w.Body.String())
		}
	}
}

func TestCrossSiteRequestGuard(t *testing.T) {
	setCSRFSiteConfig(t, "https://gateway.example.com", "")
	engine, cookies := csrfTestEngine(t)
	runCSRFCases(t, engine, cookies, []csrfCase{
		// Refused: a signed-in write without proof that this site's page sent it.
		{name: "no Origin or Referer", signedIn: true, host: "gateway.example.com", want: http.StatusForbidden},
		{name: "foreign Origin", signedIn: true, host: "gateway.example.com", headers: map[string]string{"Origin": "https://evil.example"}, want: http.StatusForbidden},
		{name: "foreign Origin on DELETE", method: http.MethodDelete, signedIn: true, host: "gateway.example.com", headers: map[string]string{"Origin": "https://evil.example"}, want: http.StatusForbidden},
		{name: "sibling subdomain", signedIn: true, host: "gateway.example.com", headers: map[string]string{"Origin": "https://user-content.example.com", "Sec-Fetch-Site": "same-site"}, want: http.StatusForbidden},
		{name: "cross-site fetch metadata", signedIn: true, host: "gateway.example.com", headers: map[string]string{"Origin": "https://evil.example", "Sec-Fetch-Site": "cross-site"}, want: http.StatusForbidden},
		{name: "foreign Referer", signedIn: true, host: "gateway.example.com", headers: map[string]string{"Referer": "https://evil.example/page"}, want: http.StatusForbidden},
		{name: "null Origin", signedIn: true, host: "gateway.example.com", headers: map[string]string{"Origin": "null"}, want: http.StatusForbidden},
		{name: "configured address with another scheme", signedIn: true, host: "internal:3000", headers: map[string]string{"Origin": "http://gateway.example.com"}, want: http.StatusForbidden},
		{name: "host suffix is not the host", signedIn: true, host: "gateway.example.com", headers: map[string]string{"Origin": "https://gateway.example.com.evil.example"}, want: http.StatusForbidden},

		// Allowed: this site's own pages.
		{name: "Origin matches Host", signedIn: true, host: "gateway.example.com", headers: map[string]string{"Origin": "https://gateway.example.com"}, want: http.StatusOK},
		{name: "Origin with default port", signedIn: true, host: "gateway.example.com", headers: map[string]string{"Origin": "https://gateway.example.com:443"}, want: http.StatusOK},
		{name: "Origin matches configured address behind a Host-rewriting proxy", signedIn: true, host: "127.0.0.1:3000", peer: "10.0.0.5", headers: map[string]string{"Origin": "https://gateway.example.com"}, want: http.StatusOK},
		{name: "Referer matches Host", signedIn: true, host: "gateway.example.com", headers: map[string]string{"Referer": "https://gateway.example.com/panel/channel"}, want: http.StatusOK},
		{name: "same-origin fetch metadata", signedIn: true, host: "127.0.0.1:3050", peer: "10.0.0.5", headers: map[string]string{"Sec-Fetch-Site": "same-origin"}, want: http.StatusOK},

		// Not checked: reads, requests without a signed-in session, exempt callbacks.
		{name: "signed-in read", method: http.MethodGet, signedIn: true, host: "gateway.example.com", headers: map[string]string{"Origin": "https://evil.example"}, want: http.StatusOK},
		{name: "anonymous write", host: "gateway.example.com", headers: map[string]string{"Origin": "https://evil.example"}, want: http.StatusOK},
		{name: "management key write", host: "gateway.example.com", headers: map[string]string{"Authorization": "Bearer mk-xxx"}, want: http.StatusOK},
		{name: "payment notification", path: "/api/payment/notify/abc", signedIn: true, host: "gateway.example.com", headers: map[string]string{"Origin": "https://pay.example"}, want: http.StatusOK},
	})
}

// The Vite dev server (3010) proxies to the backend (3050) with changeOrigin, so the backend
// sees Host 127.0.0.1:3050 and Origin http://localhost:3010 on a loopback connection.
func TestCrossSiteRequestGuardDevProxy(t *testing.T) {
	setCSRFSiteConfig(t, "http://localhost:3000", "")
	engine, cookies := csrfTestEngine(t)
	runCSRFCases(t, engine, cookies, []csrfCase{
		{name: "Vite dev origin on loopback", signedIn: true, host: "127.0.0.1:3050", peer: "127.0.0.1", headers: map[string]string{"Origin": "http://localhost:3010"}, want: http.StatusOK},
		{name: "loopback origin from a remote peer", signedIn: true, host: "gateway.example.com", peer: "203.0.113.9", headers: map[string]string{"Origin": "http://localhost:3010"}, want: http.StatusForbidden},
		{name: "foreign origin on loopback", signedIn: true, host: "127.0.0.1:3050", peer: "127.0.0.1", headers: map[string]string{"Origin": "https://evil.example"}, want: http.StatusForbidden},
	})
}

func TestCrossSiteRequestGuardFrontendAndProxy(t *testing.T) {
	setCSRFSiteConfig(t, "", "https://console.example.com/")
	setTrustedProxies(t, "10.0.0.0/8")
	engine, cookies := csrfTestEngine(t)
	runCSRFCases(t, engine, cookies, []csrfCase{
		{name: "separate frontend", signedIn: true, host: "api.example.com", headers: map[string]string{"Origin": "https://console.example.com", "Sec-Fetch-Site": "same-site"}, want: http.StatusOK},
		{name: "X-Forwarded-Host from a trusted proxy", signedIn: true, host: "backend:3000", peer: "10.1.2.3", headers: map[string]string{"Origin": "https://gateway.example.com", "X-Forwarded-Host": "gateway.example.com"}, want: http.StatusOK},
		{name: "X-Forwarded-Host from an untrusted peer", signedIn: true, host: "backend:3000", peer: "203.0.113.9", headers: map[string]string{"Origin": "https://evil.example", "X-Forwarded-Host": "evil.example"}, want: http.StatusForbidden},
	})
}
