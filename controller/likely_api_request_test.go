package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestIsLikelyAPIRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	browser := "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"
	cases := []struct {
		name, method, path, accept string
		want                       bool
	}{
		{"page reload with an API fragment in the path", http.MethodGet, "/panel/models", browser, false},
		{"page reload of another console page", http.MethodGet, "/panel/admin/models", browser, false},
		{"plain page", http.MethodGet, "/panel/dashboard", browser, false},
		{"client missing /v1", http.MethodGet, "/models", "application/json", true},
		{"client missing /v1 without Accept", http.MethodGet, "/models", "", true},
		{"POST without /v1", http.MethodPost, "/chat/completions", browser, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(tc.method, tc.path, nil)
			if tc.accept != "" {
				c.Request.Header.Set("Accept", tc.accept)
			}
			if got := IsLikelyAPIRequest(c); got != tc.want {
				t.Fatalf("IsLikelyAPIRequest(%s %s, Accept=%q) = %v, want %v", tc.method, tc.path, tc.accept, got, tc.want)
			}
		})
	}
}
