package config

// 账号体系二选一：本站内置账号，或交给外部身份提供方（Authgear / Keycloak / Auth0 …）。
// 形态决定登录页长什么样、哪些登录方式生效；第三方登录只在管账号的那一方配置。
const (
	AccountSystemBuiltin  = "builtin"
	AccountSystemExternal = "external"
)

// AccountSystem 当前账号体系，由后台「登录方式」页设置；只接受上面两个取值。
var AccountSystem = AccountSystemBuiltin

// AdminLoginEnabled 外部身份提供方模式下是否保留 /login/admin 管理员应急登录（仅 root 可通过）。
var AdminLoginEnabled = true

// EmailCodeLoginEnabled 内置账号模式下是否允许邮箱验证码登录（还需要 SMTP 已配置）。
var EmailCodeLoginEnabled = true

// PasskeyLoginEnabled 内置账号模式下是否允许通行密钥（WebAuthn）注册与登录。
var PasskeyLoginEnabled = true

// ValidAccountSystem 判断取值是否合法。
func ValidAccountSystem(value string) bool {
	return value == AccountSystemBuiltin || value == AccountSystemExternal
}

// IsExternalAccountSystem 账号体系是否交给外部身份提供方。
func IsExternalAccountSystem() bool {
	return AccountSystem == AccountSystemExternal
}

// EffectivePasswordLogin 密码登录对普通用户是否生效：外部模式下一律关闭（root 走 /login/admin，见 AdminLoginAvailable）。
func EffectivePasswordLogin() bool {
	return !IsExternalAccountSystem() && PasswordLoginEnabled
}

// EffectivePasswordRegister 密码注册是否生效：外部模式下注册在提供方完成。
func EffectivePasswordRegister() bool {
	return !IsExternalAccountSystem() && PasswordRegisterEnabled
}

// EffectiveSocialLogin 某个社交登录（GitHub / 微信 / 飞书 / LinuxDo）是否生效：
// 外部模式下社交登录在提供方后台配置，本站开关一律视为关闭。
func EffectiveSocialLogin(enabled bool) bool {
	return !IsExternalAccountSystem() && enabled
}

// EffectiveEmailCodeLogin 邮箱验证码登录是否生效：内置模式、开关开启、且 SMTP 已配置。
func EffectiveEmailCodeLogin(smtpConfigured bool) bool {
	return !IsExternalAccountSystem() && EmailCodeLoginEnabled && smtpConfigured
}

// EffectivePasskeyLogin 通行密钥对普通用户是否生效：外部模式下只有 root 能用（应急登录），见 PasskeyAllowedForRole。
func EffectivePasskeyLogin() bool {
	return !IsExternalAccountSystem() && PasskeyLoginEnabled
}

// PasskeyAllowedForRole 某个角色能否注册 / 使用通行密钥：
// 内置模式看站点开关；外部模式只放行 root（应急登录的第二条路径，与 AdminLoginEnabled 同进退）。
func PasskeyAllowedForRole(role int) bool {
	if IsExternalAccountSystem() {
		return AdminLoginEnabled && role == RoleRootUser
	}
	return PasskeyLoginEnabled
}

// AdminLoginAvailable /login/admin 是否可用：只在外部模式下存在，内置模式下管理员走普通登录。
func AdminLoginAvailable() bool {
	return IsExternalAccountSystem() && AdminLoginEnabled
}

// LocalPasswordAllowedForRole 某个角色现在能否用密码登录 / 设置密码：
// 内置模式看密码登录开关；外部模式只放行 root 且应急登录未关闭。
func LocalPasswordAllowedForRole(role int) bool {
	if IsExternalAccountSystem() {
		return AdminLoginEnabled && role == RoleRootUser
	}
	// 内置模式下关闭密码登录时仍给 root 留一条路，避免邮箱 / 通行密钥都不可用时无人能进后台
	return PasswordLoginEnabled || role == RoleRootUser
}
