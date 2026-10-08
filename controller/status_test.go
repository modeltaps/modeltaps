package controller

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

func statusData(t *testing.T) map[string]any {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/api/status", nil)
	GetStatus(c)
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析 /api/status 失败: %v", err)
	}
	return body.Data
}

// /api/status 下发的是账号体系推导后的有效开关：外部模式下密码 / 注册 / 社交登录一律 false，
// 只下发承担本站身份的提供方并宣告直达；内置模式下提供方列表为空、各开关原样。
func TestGetStatusFollowsAccountSystem(t *testing.T) {
	setupOidcTestDB(t)
	model.InitOptionMap()
	oldPassword, oldRegister, oldGitHub := config.PasswordLoginEnabled, config.PasswordRegisterEnabled, config.GitHubOAuthEnabled
	t.Cleanup(func() {
		config.PasswordLoginEnabled, config.PasswordRegisterEnabled, config.GitHubOAuthEnabled = oldPassword, oldRegister, oldGitHub
	})
	config.PasswordLoginEnabled, config.PasswordRegisterEnabled, config.GitHubOAuthEnabled = true, true, true
	provider := seedOidcProvider(t, "authany", true)
	provider.FirstParty = true
	if err := provider.Update(); err != nil {
		t.Fatalf("更新提供方失败: %v", err)
	}

	config.AccountSystem = config.AccountSystemExternal
	data := statusData(t)
	for _, key := range []string{"password_login", "password_register", "github_oauth", "passkey_login", "email_code_login"} {
		if data[key] != false {
			t.Fatalf("外部模式下 %s 应为 false，实际 %v", key, data[key])
		}
	}
	if data["account_system"] != "external" || data["admin_login_enabled"] != true || data["oidc_auto_redirect"] != true {
		t.Fatalf("外部模式的账号体系字段不对: %v", data)
	}
	providers, _ := data["oidc_providers"].([]any)
	if len(providers) != 1 {
		t.Fatalf("外部模式应只下发本站身份提供方，实际 %v", data["oidc_providers"])
	}

	config.AccountSystem = config.AccountSystemBuiltin
	data = statusData(t)
	if data["password_login"] != true || data["password_register"] != true || data["github_oauth"] != true || data["passkey_login"] != true {
		t.Fatalf("内置模式下各开关应原样下发: %v", data)
	}
	if data["account_system"] != "builtin" || data["admin_login_enabled"] != false || data["oidc_auto_redirect"] != false {
		t.Fatalf("内置模式的账号体系字段不对: %v", data)
	}
	if providers, _ := data["oidc_providers"].([]any); len(providers) != 0 {
		t.Fatalf("内置模式不应下发提供方，实际 %v", data["oidc_providers"])
	}
}
