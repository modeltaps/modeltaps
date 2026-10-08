package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/middleware"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// 密码登录建立会话行；受保护接口靠会话行放行，行被吊销后同一 cookie 立即失效。
func TestSessionRevocationInvalidatesCookie(t *testing.T) {
	setupOidcTestDB(t)
	oldSystem, oldPassword := config.AccountSystem, config.PasswordLoginEnabled
	config.AccountSystem, config.PasswordLoginEnabled = config.AccountSystemBuiltin, true
	t.Cleanup(func() { config.AccountSystem, config.PasswordLoginEnabled = oldSystem, oldPassword })

	hash, err := common.Password2Hash("correct horse battery")
	if err != nil {
		t.Fatalf("生成密码哈希失败: %v", err)
	}
	user := seedOidcUser(t, &model.User{
		Username: "carol", Password: hash, Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		AccessToken: "tok-carol", AffCode: "aff-carol",
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	r.POST("/login", Login)
	r.GET("/me", middleware.UserAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true, "id": c.GetInt("id")})
	})

	loginReq := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"username":"carol","password":"correct horse battery"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRecorder := httptest.NewRecorder()
	r.ServeHTTP(loginRecorder, loginReq)
	cookies := loginRecorder.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("登录应下发会话 cookie，实际 %s", loginRecorder.Body.String())
	}

	me := func() map[string]interface{} {
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		for _, ck := range cookies {
			req.AddCookie(ck)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}
		return resp
	}

	if resp := me(); resp["success"] != true || int(resp["id"].(float64)) != user.Id {
		t.Fatalf("登录后应通过鉴权，实际 %v", resp)
	}

	if err := model.DeleteUserSessionsExcept(nil, user.Id, ""); err != nil {
		t.Fatalf("吊销会话失败: %v", err)
	}
	if resp := me(); resp["success"] != false || resp["message"] != model.ErrUserSessionInvalid.Error() {
		t.Fatalf("会话行吊销后同一 cookie 应失效，实际 %v", resp)
	}
}
