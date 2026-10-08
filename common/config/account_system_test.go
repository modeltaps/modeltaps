package config

import "testing"

// 账号体系推导出的有效开关：外部模式下密码 / 注册 / 社交 / 通行密钥（非 root）一律关闭，
// 只给 root 留应急登录；内置模式按各自开关，且关闭密码登录时 root 仍可用密码。
func TestAccountSystemEffectiveFlags(t *testing.T) {
	old := struct {
		system                                    string
		password, register, github, code, passkey bool
		admin                                     bool
	}{AccountSystem, PasswordLoginEnabled, PasswordRegisterEnabled, GitHubOAuthEnabled, EmailCodeLoginEnabled, PasskeyLoginEnabled, AdminLoginEnabled}
	t.Cleanup(func() {
		AccountSystem, PasswordLoginEnabled, PasswordRegisterEnabled = old.system, old.password, old.register
		GitHubOAuthEnabled, EmailCodeLoginEnabled, PasskeyLoginEnabled, AdminLoginEnabled = old.github, old.code, old.passkey, old.admin
	})

	PasswordLoginEnabled, PasswordRegisterEnabled, GitHubOAuthEnabled = true, true, true
	EmailCodeLoginEnabled, PasskeyLoginEnabled, AdminLoginEnabled = true, true, true

	AccountSystem = AccountSystemBuiltin
	if !EffectivePasswordLogin() || !EffectivePasswordRegister() || !EffectiveSocialLogin(GitHubOAuthEnabled) {
		t.Fatal("内置模式下各开关应原样生效")
	}
	if !EffectiveEmailCodeLogin(true) || EffectiveEmailCodeLogin(false) {
		t.Fatal("邮箱验证码登录还要看 SMTP 是否配置")
	}
	if !EffectivePasskeyLogin() || !PasskeyAllowedForRole(RoleCommonUser) {
		t.Fatal("内置模式下通行密钥对普通用户开放")
	}
	if AdminLoginAvailable() {
		t.Fatal("内置模式下没有 /login/admin")
	}
	PasswordLoginEnabled = false
	if LocalPasswordAllowedForRole(RoleCommonUser) || !LocalPasswordAllowedForRole(RoleRootUser) {
		t.Fatal("内置模式关闭密码登录后只给 root 留密码")
	}
	PasswordLoginEnabled = true

	AccountSystem = AccountSystemExternal
	if EffectivePasswordLogin() || EffectivePasswordRegister() || EffectiveSocialLogin(GitHubOAuthEnabled) {
		t.Fatal("外部模式下密码 / 注册 / 社交登录一律关闭")
	}
	if EffectiveEmailCodeLogin(true) || EffectivePasskeyLogin() {
		t.Fatal("外部模式下邮箱验证码与通行密钥对普通用户关闭")
	}
	if PasskeyAllowedForRole(RoleCommonUser) || !PasskeyAllowedForRole(RoleRootUser) {
		t.Fatal("外部模式下通行密钥只给 root")
	}
	if !AdminLoginAvailable() || !LocalPasswordAllowedForRole(RoleRootUser) || LocalPasswordAllowedForRole(RoleAdminUser) {
		t.Fatal("外部模式下只有 root 能走应急登录")
	}
	AdminLoginEnabled = false
	if AdminLoginAvailable() || LocalPasswordAllowedForRole(RoleRootUser) || PasskeyAllowedForRole(RoleRootUser) {
		t.Fatal("关闭应急登录后 root 也进不来")
	}
}

func TestValidAccountSystem(t *testing.T) {
	if !ValidAccountSystem(AccountSystemBuiltin) || !ValidAccountSystem(AccountSystemExternal) {
		t.Fatal("两个合法取值应通过")
	}
	if ValidAccountSystem("") || ValidAccountSystem("mixed") {
		t.Fatal("空值与未知值应拒绝")
	}
}
