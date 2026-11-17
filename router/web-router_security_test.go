package router

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// SEC-86: every page of the web app refuses to be framed, the SPA fallback (which serves
// /panel/device) included.
func TestWebPagesRefuseFraming(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// An empty build: every page is answered by the SPA fallback.
	SetWebRouter(r, embed.FS{}, []byte("<html></html>"))
	for _, path := range []string{"/panel/device?user_code=ABCD-EFGH", "/login", "/", "/skill.md"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "198.51.100.30:1234"
		r.ServeHTTP(w, req)
		if w.Header().Get("X-Frame-Options") != "DENY" {
			t.Errorf("%s: X-Frame-Options = %q, want DENY", path, w.Header().Get("X-Frame-Options"))
		}
		if !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
			t.Errorf("%s: Content-Security-Policy = %q, want frame-ancestors 'none'", path, w.Header().Get("Content-Security-Policy"))
		}
	}
}

// SEC-91: static files are not rate limited, so a Range header listing many ranges is dropped
// and the file is sent whole; a single range still works.
func TestStaticFilesServeSingleRangeOnly(t *testing.T) {
	r := setupStaticRateLimit(t)
	get := func(rangeHeader string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
		req.RemoteAddr = "198.51.100.40:1234"
		req.Header.Set("Range", rangeHeader)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	if w := get("bytes=0-1"); w.Code != http.StatusPartialContent || w.Body.String() != "co" {
		t.Fatalf("single range: got %d %q, want 206 \"co\"", w.Code, w.Body.String())
	}
	// Ranges that add up to less than the file, which net/http would otherwise answer as multipart.
	w := get("bytes=0-0,2-2,4-4")
	if w.Code != http.StatusOK || w.Body.String() != "console.log(1)" {
		t.Fatalf("multiple ranges: got %d %q, want 200 with the whole file", w.Code, w.Body.String())
	}
	if strings.Contains(w.Header().Get("Content-Type"), "multipart") {
		t.Fatalf("multiple ranges must not produce a multipart response")
	}
}
