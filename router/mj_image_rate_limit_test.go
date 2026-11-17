package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/middleware"

	"github.com/gin-gonic/gin"
)

// SEC-89: /mj/image needs no key, so it is limited per IP before the image is fetched.
func TestMJImageRouteIsRateLimited(t *testing.T) {
	if config.RedisEnabled {
		t.Skip("requires memory rate limit path (redis disabled)")
	}
	old := middleware.MJImageRateLimitNum
	middleware.MJImageRateLimitNum = 1
	t.Cleanup(func() { middleware.MJImageRateLimitNum = old })

	engine := gin.New()
	// The first request reaches the handler, which has no database here.
	engine.Use(gin.RecoveryWithWriter(io.Discard))
	setMJRouter(engine)
	for i, path := range []string{"/mj/image/123", "/fast/mj/image/123"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "198.51.100.50:1234"
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if i == 1 && w.Code != http.StatusTooManyRequests {
			t.Fatalf("%s: got %d, want 429 from the rate limit", path, w.Code)
		}
	}
}
