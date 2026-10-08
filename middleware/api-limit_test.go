package middleware

// DynamicRedisRateLimiter 在 redis 未启用时的行为锁定：
// limiter 来自 limit.NewAPILimiter（RedisEnabled=false 时退化为内存限流器），
// 组不存在时返回 403，超限返回 429。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/limit"
	"github.com/modeltaps/modeltaps/model"
)

func newAPILimitRouter(userID int, group string) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("id", userID)
		c.Set("group", group)
	})
	r.Use(DynamicRedisRateLimiter())
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return r
}

func doAPILimitRequest(r *gin.Engine) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)
	return w
}

// withAPILimiters 临时替换全局限流器映射，测试结束后恢复
func withAPILimiters(t *testing.T, limiters map[string]limit.RateLimiter) {
	t.Helper()
	model.GlobalUserGroupRatio.Lock()
	old := model.GlobalUserGroupRatio.APILimiter
	model.GlobalUserGroupRatio.APILimiter = limiters
	model.GlobalUserGroupRatio.Unlock()
	t.Cleanup(func() {
		model.GlobalUserGroupRatio.Lock()
		model.GlobalUserGroupRatio.APILimiter = old
		model.GlobalUserGroupRatio.Unlock()
	})
}

func TestDynamicRedisRateLimiterUnknownGroupForbidden(t *testing.T) {
	withAPILimiters(t, map[string]limit.RateLimiter{})
	w := doAPILimitRequest(newAPILimitRouter(1, "no-such-group"))
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
	if !strings.Contains(w.Body.String(), "API requests are not allowed") {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestDynamicRedisRateLimiterMemoryPath(t *testing.T) {
	if config.RedisEnabled {
		t.Skip("requires memory limiter path (redis disabled)")
	}
	limiter := limit.NewAPILimiter(3) // rpm < RPMThreshold → 固定窗口内存限流器
	if _, ok := limiter.(*limit.MemoryLimiter); !ok {
		t.Fatalf("expected *limit.MemoryLimiter when redis disabled, got %T", limiter)
	}
	withAPILimiters(t, map[string]limit.RateLimiter{"testg": limiter})

	r := newAPILimitRouter(4242, "testg")
	for i := 0; i < 3; i++ {
		if w := doAPILimitRequest(r); w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}
	w := doAPILimitRequest(r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("request 4: expected %d, got %d", http.StatusTooManyRequests, w.Code)
	}
	if !strings.Contains(w.Body.String(), RATE_LIMIT_EXCEEDED_MSG) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}

	// 限流 key 按 userID 隔离（api-limiter:%d），另一用户不受影响
	if w := doAPILimitRequest(newAPILimitRouter(4243, "testg")); w.Code != http.StatusOK {
		t.Fatalf("other user: expected 200, got %d", w.Code)
	}
}
