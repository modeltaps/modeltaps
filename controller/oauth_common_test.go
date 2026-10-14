package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func renderOAuthPage(t *testing.T, render func(c *gin.Context)) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/geminicli/oauth/callback", nil)
	render(c)
	return w
}

// The callback routes need no sign-in, so input such as error_description must never become script on our origin.
func TestOAuthResultPageEscapesUntrustedInput(t *testing.T) {
	payload := `<img src=x onerror=alert(1)></script><script>alert(2)</script>`
	renders := map[string]func(c *gin.Context){
		"geminicli": func(c *gin.Context) {
			renderOAuthResult(c, true, "Authorization failed: "+payload, payload, payload, "https://admin.example.com")
		},
		"antigravity": func(c *gin.Context) {
			renderAntigravityOAuthResult(c, true, "Authorization failed: "+payload, payload, payload, "https://admin.example.com")
		},
	}
	for name, render := range renders {
		t.Run(name, func(t *testing.T) {
			w := renderOAuthPage(t, render)
			body := w.Body.String()
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d", w.Code)
			}
			if strings.Contains(body, "<img src=x") || strings.Contains(body, "<script>alert") || strings.Contains(body, "</script><script>") {
				t.Fatalf("page contains unescaped injected content:\n%s", body)
			}
			if strings.Contains(body, "onclick=") {
				t.Fatal("page must not use inline event handlers under the CSP")
			}
			if strings.Count(body, "<script") != 1 {
				t.Fatalf("page should have exactly one script block, got %d", strings.Count(body, "<script"))
			}
			csp := w.Header().Get("Content-Security-Policy")
			if !strings.Contains(csp, "script-src 'nonce-") || !strings.Contains(csp, "default-src 'none'") {
				t.Fatalf("missing or wrong CSP header: %q", csp)
			}
			if w.Header().Get("X-Frame-Options") != "DENY" {
				t.Fatal("missing X-Frame-Options: DENY")
			}
			if strings.Contains(body, "'*'") || !strings.Contains(body, `"https://admin.example.com"`) {
				t.Fatal("postMessage must target the origin that started the flow, not '*'")
			}
		})
	}
}

func TestOAuthOpenerOrigin(t *testing.T) {
	cases := []struct {
		name    string
		origin  string
		referer string
		want    string
	}{
		{"Origin wins", "https://admin.example.com", "https://other.example.com/x", "https://admin.example.com"},
		{"Referer fallback drops the path", "", "http://localhost:3000/panel/channel?id=1", "http://localhost:3000"},
		{"null origin is not trusted", "null", "", ""},
		{"non-http(s) scheme is rejected", "javascript:alert(1)", "", ""},
		{"neither header", "", "", ""},
	}
	gin.SetMode(gin.TestMode)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/api/geminicli/oauth/start", nil)
			if tc.origin != "" {
				c.Request.Header.Set("Origin", tc.origin)
			}
			if tc.referer != "" {
				c.Request.Header.Set("Referer", tc.referer)
			}
			if got := oauthOpenerOrigin(c); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTruncateErrorDescription(t *testing.T) {
	long := strings.Repeat("错", oauthErrorDescriptionMaxRunes+50)
	if got := []rune(truncateErrorDescription(long)); len(got) != oauthErrorDescriptionMaxRunes {
		t.Fatalf("length after truncation = %d", len(got))
	}
	if got := truncateErrorDescription("short"); got != "short" {
		t.Fatalf("short input should be unchanged: %q", got)
	}
}
