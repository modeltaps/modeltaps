package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common/config"
)

func TestRequestLanguage(t *testing.T) {
	old := config.DefaultLanguage
	t.Cleanup(func() { config.DefaultLanguage = old })
	config.DefaultLanguage = "zh_CN"

	cases := []struct {
		cookie, accept, want string
	}{
		{"", "ja,en;q=0.5", "ja_JP"}, // the browser's language
		{"zh_HK", "ja", "zh_HK"},     // the language picked in the console wins
		{"", "xx, yy", "zh_CN"},      // nothing supported: the site default
		{"", "", "zh_CN"},
	}
	for _, tc := range cases {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/api/verification", nil)
		if tc.accept != "" {
			c.Request.Header.Set("Accept-Language", tc.accept)
		}
		if tc.cookie != "" {
			c.Request.AddCookie(&http.Cookie{Name: "appLanguage", Value: tc.cookie})
		}
		if got := requestLanguage(c); got != tc.want {
			t.Errorf("cookie=%q accept=%q: got %q, want %q", tc.cookie, tc.accept, got, tc.want)
		}
	}
}
