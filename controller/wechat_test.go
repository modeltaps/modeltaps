package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func setupWeChatBindRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	r.GET("/api/oauth/state", GenerateOAuthCode)
	r.GET("/api/oauth/wechat/bind", WeChatBind)

	oldEnabled := config.WeChatAuthEnabled
	config.WeChatAuthEnabled = true
	t.Cleanup(func() { config.WeChatAuthEnabled = oldEnabled })
	return r
}

// 直连 /api/oauth/wechat/bind 而 session 中无 oauth_state（CSRF 伪造场景）应被 403 拒绝。
func TestWeChatBindRejectsMissingState(t *testing.T) {
	r := setupWeChatBindRouter(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/oauth/wechat/bind?code=abc", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("无 state 直连应返回 403，实际 %d", w.Code)
	}
}

// 携带的 state 与 session 中不一致时同样应被 403 拒绝。
func TestWeChatBindRejectsMismatchedState(t *testing.T) {
	r := setupWeChatBindRouter(t)

	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest("GET", "/api/oauth/state", nil))
	sessionCookie := w1.Header().Get("Set-Cookie")

	req := httptest.NewRequest("GET", "/api/oauth/wechat/bind?code=abc&state=wrong", nil)
	req.Header.Set("Cookie", sessionCookie)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("state 不一致应返回 403，实际 %d", w2.Code)
	}
}

// 正常前端流程：先 /api/oauth/state 种 state，再携带同一 state 请求绑定，应通过 state 门
// （随后因 code 为空在业务校验处返回 200 + 错误消息，而非 403）。
func TestWeChatBindAcceptsValidState(t *testing.T) {
	r := setupWeChatBindRouter(t)

	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest("GET", "/api/oauth/state", nil))
	sessionCookie := w1.Header().Get("Set-Cookie")
	var stateResp map[string]interface{}
	if err := json.Unmarshal(w1.Body.Bytes(), &stateResp); err != nil {
		t.Fatalf("解析 state 响应失败: %v", err)
	}
	state, _ := stateResp["data"].(string)
	if state == "" {
		t.Fatal("未获取到 state")
	}

	req := httptest.NewRequest("GET", "/api/oauth/wechat/bind?state="+state, nil)
	req.Header.Set("Cookie", sessionCookie)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if w2.Code == http.StatusForbidden {
		t.Fatalf("合法 state 不应被 403 拒绝")
	}
	var bindResp map[string]interface{}
	if err := json.Unmarshal(w2.Body.Bytes(), &bindResp); err != nil {
		t.Fatalf("解析绑定响应失败: %v", err)
	}
	if msg, _ := bindResp["message"].(string); msg == "state is empty or not same" {
		t.Fatalf("合法 state 不应触发 state 校验失败")
	}
}
