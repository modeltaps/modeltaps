package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// runTurnstile 在 session 中预置给定的 turnstile 标记后执行一次无 token 的请求。
// 标记有效时应放行（200 + reached=true），过期或缺失时应被中间件拦截。
func runTurnstile(t *testing.T, marker interface{}) map[string]interface{} {
	t.Helper()
	gin.SetMode(gin.TestMode)

	oldEnabled := config.TurnstileCheckEnabled
	config.TurnstileCheckEnabled = true
	t.Cleanup(func() { config.TurnstileCheckEnabled = oldEnabled })

	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	r.GET("/seed", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("turnstile", marker)
		_ = session.Save()
		c.Status(http.StatusOK)
	})
	r.GET("/guarded", TurnstileCheck(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true, "reached": true})
	})

	seed := httptest.NewRecorder()
	r.ServeHTTP(seed, httptest.NewRequest(http.MethodGet, "/seed", nil))

	req := httptest.NewRequest(http.MethodGet, "/guarded", nil)
	for _, c := range seed.Result().Cookies() {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	return resp
}

func TestTurnstileSessionMarkerWithinTTL(t *testing.T) {
	resp := runTurnstile(t, time.Now().Unix())
	if resp["reached"] != true {
		t.Errorf("TTL 内的会话标记应免检放行，实际 %v", resp)
	}
}

func TestTurnstileSessionMarkerExpired(t *testing.T) {
	stale := time.Now().Add(-config.TurnstileSessionTTL - time.Minute).Unix()
	resp := runTurnstile(t, stale)
	if resp["reached"] == true {
		t.Error("过期的会话标记应要求重新校验")
	}
	if resp["success"] != false {
		t.Errorf("过期后无 token 应返回失败，实际 %v", resp)
	}
}

// 旧版本在 session 中存的是布尔值，无法判断时效，应视为过期并强制重新校验。
func TestTurnstileLegacyBoolMarkerRejected(t *testing.T) {
	resp := runTurnstile(t, true)
	if resp["reached"] == true {
		t.Error("旧布尔标记不应继续免检")
	}
}
