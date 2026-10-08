package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// runSecurityHeaders 在给定 https / trusted_header 配置下执行一次请求，返回响应
func runSecurityHeaders(t *testing.T, https bool, trustedHeader string) *httptest.ResponseRecorder {
	t.Helper()

	viper.Set("https", https)
	viper.Set("trusted_header", trustedHeader)
	t.Cleanup(func() {
		viper.Set("https", false)
		viper.Set("trusted_header", "")
	})

	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
	return w
}

func TestSecurityHeadersCommon(t *testing.T) {
	w := runSecurityHeaders(t, false, "")

	for header, want := range map[string]string{
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
		"X-XSS-Protection":       "1; mode=block",
	} {
		if got := w.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestSecurityHeadersHSTS(t *testing.T) {
	const wantHSTS = "max-age=31536000; includeSubDomains"

	cases := []struct {
		name          string
		https         bool
		trustedHeader string
		want          string
	}{
		{"纯 HTTP 不注入", false, "", ""},
		{"https 开启时注入", true, "", wantHSTS},
		{"Cloudflare 回源时注入", false, "CF-Connecting-IP", wantHSTS},
		{"其它 trusted_header 不注入", false, "X-Forwarded-For", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := runSecurityHeaders(t, tc.https, tc.trustedHeader)
			if got := w.Header().Get("Strict-Transport-Security"); got != tc.want {
				t.Errorf("Strict-Transport-Security = %q, want %q", got, tc.want)
			}
		})
	}
}
