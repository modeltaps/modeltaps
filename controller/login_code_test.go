package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// setupEmailCodeLogin 内置账号 + 已配置 SMTP（假值，只让开关生效，用例不真正发信）。
func setupEmailCodeLogin(t *testing.T) *gin.Engine {
	t.Helper()
	setupOidcTestDB(t)
	old := struct {
		system, server, account, token string
		port                           int
		enabled                        bool
	}{config.AccountSystem, config.SMTPServer, config.SMTPAccount, config.SMTPToken, config.SMTPPort, config.EmailCodeLoginEnabled}
	config.AccountSystem = config.AccountSystemBuiltin
	config.SMTPServer, config.SMTPAccount, config.SMTPToken, config.SMTPPort = "smtp.invalid", "noreply", "secret", 465
	config.EmailCodeLoginEnabled = true
	t.Cleanup(func() {
		config.AccountSystem, config.SMTPServer, config.SMTPAccount, config.SMTPToken, config.SMTPPort = old.system, old.server, old.account, old.token, old.port
		config.EmailCodeLoginEnabled = old.enabled
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	r.POST("/send", SendLoginCode)
	r.POST("/login", LoginWithCode)
	return r
}

func postJSON(t *testing.T, r *gin.Engine, path string, body string) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v (body=%s)", err, w.Body.String())
	}
	return w, resp
}

// 发码对不存在的邮箱也返回成功（防枚举）；验码错误、正确各走一遍，正确后验证码即失效。
func TestEmailCodeLoginFlow(t *testing.T) {
	r := setupEmailCodeLogin(t)
	user := seedOidcUser(t, &model.User{
		Username: "alice", Email: "alice@example.test", Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		AccessToken: "tok-alice", AffCode: "aff-alice",
	})

	if _, resp := postJSON(t, r, "/send", `{"email":"nobody@example.test"}`); resp["success"] != true {
		t.Fatalf("不存在的邮箱也应返回成功，实际 %v", resp)
	}

	common.RegisterVerificationCodeWithKey("alice@example.test", "abc123", common.LoginCodePurpose)
	t.Cleanup(func() { common.DeleteKey("alice@example.test", common.LoginCodePurpose) })

	if _, resp := postJSON(t, r, "/login", `{"email":"alice@example.test","code":"wrong"}`); resp["success"] != false || resp["message"] != errLoginCodeFailed {
		t.Fatalf("错误验证码应失败，实际 %v", resp)
	}

	w, resp := postJSON(t, r, "/login", `{"email":"Alice@Example.test","code":"abc123"}`)
	if resp["success"] != true {
		t.Fatalf("正确验证码应登录成功，实际 %v", resp)
	}
	if len(w.Result().Cookies()) == 0 {
		t.Fatal("登录应下发会话 cookie")
	}
	sessionList, err := model.ListUserSessions(user.Id)
	if err != nil || len(sessionList) != 1 || sessionList[0].LoginMethod != model.SessionMethodEmailCode {
		t.Fatalf("应建立一行 email_code 会话，实际 %+v err=%v", sessionList, err)
	}

	if _, resp := postJSON(t, r, "/login", `{"email":"alice@example.test","code":"abc123"}`); resp["success"] != false {
		t.Fatalf("验证码用过即失效，实际 %v", resp)
	}
}

// 用户自己关闭邮箱验证码登录后，即便验证码正确也拒绝；外部账号体系下整个入口关闭。
func TestEmailCodeLoginRespectsUserSettingAndAccountSystem(t *testing.T) {
	r := setupEmailCodeLogin(t)
	user := seedOidcUser(t, &model.User{
		Username: "bob", Email: "bob@example.test", Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		AccessToken: "tok-bob", AffCode: "aff-bob",
	})
	off := false
	if err := model.UpdateUserSettingById(user.Id, model.UserSetting{EmailCodeLogin: &off}); err != nil {
		t.Fatalf("写用户设置失败: %v", err)
	}
	common.RegisterVerificationCodeWithKey("bob@example.test", "abc123", common.LoginCodePurpose)
	t.Cleanup(func() { common.DeleteKey("bob@example.test", common.LoginCodePurpose) })

	if _, resp := postJSON(t, r, "/login", `{"email":"bob@example.test","code":"abc123"}`); resp["success"] != false {
		t.Fatalf("用户关闭该方式后应拒绝，实际 %v", resp)
	}

	config.AccountSystem = config.AccountSystemExternal
	if _, resp := postJSON(t, r, "/send", `{"email":"bob@example.test"}`); resp["success"] != false {
		t.Fatalf("外部账号体系下发码应拒绝，实际 %v", resp)
	}
}
