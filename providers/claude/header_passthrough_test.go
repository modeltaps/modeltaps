package claude

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/modeltaps/modeltaps/model"
)

// SEC-80: the client's fingerprint headers reach the upstream, the address and identity headers
// a reverse proxy or CDN adds about the end user do not.
func TestGetRequestHeadersDropsProxyIdentityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
	for name, value := range map[string]string{
		"User-Agent":                      "claude-cli/2.0.0",
		"X-App":                           "cli",
		"X-Stainless-Lang":                "js",
		"X-Forwarded-For":                 "203.0.113.7",
		"X-Forwarded-User":                "alice",
		"X-Forwarded-Email":               "alice@example.com",
		"Forwarded":                       "for=203.0.113.7",
		"X-Real-IP":                       "203.0.113.7",
		"CF-Connecting-IP":                "203.0.113.7",
		"CF-IPCountry":                    "NL",
		"True-Client-IP":                  "203.0.113.7",
		"Via":                             "1.1 nginx",
		"X-Auth-Request-Email":            "alice@example.com",
		"Remote-User":                     "alice",
		"X-Amzn-Oidc-Accesstoken":         "eyJ",
		"Tailscale-User-Login":            "alice@example.com",
		"X-Goog-Authenticated-User-Email": "accounts.google.com:alice@example.com",
	} {
		c.Request.Header.Set(name, value)
	}
	p := ClaudeProviderFactory{}.Create(&model.Channel{Key: "sk-ant"}).(*ClaudeProvider)
	p.SetContext(c)
	headers := p.GetRequestHeaders()

	for _, kept := range []string{"user-agent", "x-app", "x-stainless-lang"} {
		if headers[kept] == "" {
			t.Errorf("fingerprint header %s was dropped", kept)
		}
	}
	for _, dropped := range []string{"x-forwarded-for", "x-forwarded-user", "x-forwarded-email", "forwarded", "x-real-ip",
		"cf-connecting-ip", "cf-ipcountry", "true-client-ip", "via", "x-auth-request-email", "remote-user",
		"x-amzn-oidc-accesstoken", "tailscale-user-login", "x-goog-authenticated-user-email"} {
		if v, ok := headers[dropped]; ok {
			t.Errorf("header %s forwarded upstream (%q)", dropped, v)
		}
	}
	if headers["x-api-key"] != "sk-ant" {
		t.Errorf("x-api-key %q", headers["x-api-key"])
	}
}
