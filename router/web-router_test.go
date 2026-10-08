package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/modeltaps/modeltaps/middleware"

	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

// setupStaticRateLimit mirrors the middleware order of SetWebRouter on a temporary build folder,
// using the real web rate limiter.
func setupStaticRateLimit(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"index.html":     "<html></html>",
		"logo.png":       "png",
		"assets/app.js":  "console.log(1)",
		"assets/app.css": "body{}",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fs := static.LocalFile(dir, false)
	r := gin.New()
	r.Use(skipStaticFiles(fs, middleware.GlobalWebRateLimit()))
	r.Use(static.Serve("/", fs))
	r.NoRoute(func(c *gin.Context) { c.String(http.StatusOK, "spa") })
	return r
}

func doFrom(r *gin.Engine, ip, path string) int {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = ip + ":12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestWebRateLimitSkipsStaticFiles(t *testing.T) {
	r := setupStaticRateLimit(t)
	ip := "198.51.100.10"
	n := middleware.GlobalWebRateLimitNum + 50
	for i := 0; i < n; i++ {
		for _, p := range []string{"/assets/app.js", "/assets/app.css", "/logo.png"} {
			if code := doFrom(r, ip, p); code != http.StatusOK {
				t.Fatalf("request %d to %s: got %d, want 200", i, p, code)
			}
		}
	}
	// Static traffic must not have used up the budget for page requests.
	if code := doFrom(r, ip, "/console"); code != http.StatusOK {
		t.Fatalf("page request after static burst: got %d, want 200", code)
	}
}

func TestWebRateLimitStillAppliesToNonStaticPaths(t *testing.T) {
	r := setupStaticRateLimit(t)
	ip := "198.51.100.20"
	limited := false
	for i := 0; i < middleware.GlobalWebRateLimitNum+1; i++ {
		if doFrom(r, ip, "/console") == http.StatusTooManyRequests {
			limited = true
			break
		}
	}
	if !limited {
		t.Fatalf("expected 429 after %d page requests", middleware.GlobalWebRateLimitNum+1)
	}
	// A missing file under /assets falls through to the limiter as well.
	if code := doFrom(r, ip, "/assets/missing.js"); code != http.StatusTooManyRequests {
		t.Fatalf("missing asset: got %d, want 429", code)
	}
	// Existing files keep loading for the same client even when it is limited.
	if code := doFrom(r, ip, "/assets/app.js"); code != http.StatusOK {
		t.Fatalf("static file while limited: got %d, want 200", code)
	}
}
