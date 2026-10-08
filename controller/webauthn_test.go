package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// 通行密钥门控（账号体系）：内置模式看站点开关 PasskeyLoginEnabled，对所有角色一视同仁；
// 外部模式只给 root 留应急入口，且随 AdminLoginEnabled。四个接口被拒时一律 403 passkey_unavailable。

const passkeyUnavailableCode = "passkey_unavailable"

// setupWebauthnRouter 注册四个受门控约束的接口，并按给定身份注入 UserAuth 会设的 id / role。
func setupWebauthnRouter(t *testing.T, userId, role int) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	r.Use(func(c *gin.Context) {
		c.Set("id", userId)
		c.Set("role", role)
	})
	r.POST("/api/webauthn/registration/begin", WebauthnBeginRegistration)
	r.POST("/api/webauthn/registration/finish", WebauthnFinishRegistration)
	r.POST("/api/webauthn/login/begin", WebauthnBeginLogin)
	r.POST("/api/webauthn/login/finish", WebauthnFinishLogin)
	return r
}

func postWebauthn(t *testing.T, r *gin.Engine, path, body string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	return w.Code, resp
}

func assertPasskeyUnavailable(t *testing.T, status int, resp map[string]interface{}) {
	t.Helper()
	if status != http.StatusForbidden || resp["code"] != passkeyUnavailableCode {
		t.Fatalf("应返回 403 %s，实际 %d %v", passkeyUnavailableCode, status, resp)
	}
}

func assertPasskeyAllowed(t *testing.T, status int, resp map[string]interface{}) {
	t.Helper()
	if status == http.StatusForbidden || resp["code"] == passkeyUnavailableCode {
		t.Fatalf("不应被门控拦下，实际 %d %v", status, resp)
	}
}

// 测试表包含凭据表：登录 begin 过闸后要查用户有无凭据。
func setupWebauthnTestDB(t *testing.T) {
	t.Helper()
	setupGetUserTestDB(t)
	if err := model.DB.AutoMigrate(&model.WebAuthnCredential{}); err != nil {
		t.Fatalf("迁移凭据表失败: %v", err)
	}
}

// setPasskeyGate 设定账号体系与两个相关开关，用例结束后复原。
func setPasskeyGate(t *testing.T, system string, passkeyEnabled bool, adminLoginEnabled bool) {
	t.Helper()
	oldSystem, oldPasskey, oldAdmin := config.AccountSystem, config.PasskeyLoginEnabled, config.AdminLoginEnabled
	t.Cleanup(func() {
		config.AccountSystem, config.PasskeyLoginEnabled, config.AdminLoginEnabled = oldSystem, oldPasskey, oldAdmin
	})
	config.AccountSystem, config.PasskeyLoginEnabled, config.AdminLoginEnabled = system, passkeyEnabled, adminLoginEnabled
}

var webauthnPaths = []struct{ path, body string }{
	{"/api/webauthn/registration/begin", `{}`},
	{"/api/webauthn/registration/finish", `{}`},
	{"/api/webauthn/login/begin", `{"username":"memberkey"}`},
	{"/api/webauthn/login/finish", `{}`},
}

// 内置模式 + 站点开关关闭：所有角色、四个接口都被拒，root 也不例外（内置模式没有应急语义）。
func TestPasskeyUnavailableWhenSwitchOff(t *testing.T) {
	setupWebauthnTestDB(t)
	setPasskeyGate(t, config.AccountSystemBuiltin, false, true)

	member := createTestUser(t, "memberkey", config.RoleCommonUser, config.UserTypeNormal)
	root := createTestUser(t, "rootkey", config.RoleRootUser, config.UserTypeNormal)

	memberRouter := setupWebauthnRouter(t, member.Id, member.Role)
	for _, tc := range webauthnPaths {
		status, resp := postWebauthn(t, memberRouter, tc.path, tc.body)
		assertPasskeyUnavailable(t, status, resp)
	}

	t.Run("普通用户名、未知用户名与无用户名登录响应一致", func(t *testing.T) {
		for _, body := range []string{`{"username":"memberkey"}`, `{"username":"ghost"}`, `{"username":""}`, `{}`} {
			status, resp := postWebauthn(t, memberRouter, "/api/webauthn/login/begin", body)
			assertPasskeyUnavailable(t, status, resp)
		}
	})

	rootRouter := setupWebauthnRouter(t, root.Id, root.Role)
	status, resp := postWebauthn(t, rootRouter, "/api/webauthn/registration/begin", `{}`)
	assertPasskeyUnavailable(t, status, resp)
}

// 内置模式 + 开关开启：普通用户四个接口都不受门控影响。
func TestPasskeyAllowedWhenSwitchOn(t *testing.T) {
	setupWebauthnTestDB(t)
	setPasskeyGate(t, config.AccountSystemBuiltin, true, true)

	member := createTestUser(t, "memberkey", config.RoleCommonUser, config.UserTypeNormal)
	r := setupWebauthnRouter(t, member.Id, member.Role)
	for _, tc := range webauthnPaths {
		status, resp := postWebauthn(t, r, tc.path, tc.body)
		assertPasskeyAllowed(t, status, resp)
	}
}

// 外部模式：注册侧只有 root 能用，且随管理员应急登录开关；登录侧 begin 不按角色拒绝
// （防用户名枚举，角色到 finish 才判定），普通用户没有凭据时与不存在的用户一样得到 login_unavailable。
func TestPasskeyExternalModeRootOnly(t *testing.T) {
	setupWebauthnTestDB(t)
	setPasskeyGate(t, config.AccountSystemExternal, true, true)

	member := createTestUser(t, "memberkey", config.RoleCommonUser, config.UserTypeNormal)
	root := createTestUser(t, "rootkey", config.RoleRootUser, config.UserTypeNormal)

	memberRouter := setupWebauthnRouter(t, member.Id, member.Role)
	for _, path := range []string{"/api/webauthn/registration/begin", "/api/webauthn/registration/finish"} {
		status, resp := postWebauthn(t, memberRouter, path, `{}`)
		assertPasskeyUnavailable(t, status, resp)
	}
	for _, body := range []string{`{"username":"memberkey"}`, `{"username":"ghost"}`} {
		status, resp := postWebauthn(t, memberRouter, "/api/webauthn/login/begin", body)
		assertPasskeyAllowed(t, status, resp)
		if resp["code"] != "login_unavailable" {
			t.Fatalf("外部模式登录 begin 不应暴露用户是否存在，应一律 login_unavailable，实际 %v", resp)
		}
	}

	rootRouter := setupWebauthnRouter(t, root.Id, root.Role)
	status, resp := postWebauthn(t, rootRouter, "/api/webauthn/registration/begin", `{}`)
	assertPasskeyAllowed(t, status, resp)

	// root 尚未注册凭据，过闸后走防枚举的 login_unavailable，而不是门控拒绝。
	status, resp = postWebauthn(t, rootRouter, "/api/webauthn/login/begin", `{"username":"rootkey"}`)
	assertPasskeyAllowed(t, status, resp)
	if resp["code"] != "login_unavailable" {
		t.Fatalf("root 无凭据时应返回 login_unavailable，实际 %v", resp)
	}

	t.Run("关闭管理员应急登录后 root 也不能再注册", func(t *testing.T) {
		config.AdminLoginEnabled = false
		status, resp := postWebauthn(t, rootRouter, "/api/webauthn/registration/begin", `{}`)
		assertPasskeyUnavailable(t, status, resp)
	})
}
