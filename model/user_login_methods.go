package model

import (
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/stmp"

	"gorm.io/gorm"
)

// 解绑请求里的绑定类型，同时用于登录方式盘点的目标标识。
const (
	LoginMethodGitHub  = "github"
	LoginMethodWeChat  = "wechat"
	LoginMethodLark    = "lark"
	LoginMethodLinuxDo = "linuxdo"
	LoginMethodOidc    = "oidc"
)

// UserLoginMethods 一个账号当前可用的登录 / 找回方式盘点结果。
// 每个 bool 与 OidcProviderIds 中的每个提供方各算一种方式。
type UserLoginMethods struct {
	// Password 密码登录：users.password 非空，且站点开启密码登录或本人是 root。
	// root 在站点关闭密码登录时仍可走 /login?local=1 逃生登录，故密码仍算一种方式。
	// 三方注册用户 password 为空串，bcrypt 校验必失败，故不算一种方式。
	Password bool
	// EmailRecovery 邮箱找回：站点开启密码登录、站点 SMTP 已配置齐全且 email 非空。
	// SMTP 未配置时发不出重置邮件，这条路走不通，不能算一种方式。
	EmailRecovery bool
	GitHub        bool
	WeChat        bool
	Lark          bool
	LinuxDo       bool
	// OidcProviderIds 该用户已绑定、且提供方处于启用状态的 provider id。
	OidcProviderIds []int
}

// Total 可用方式总数。
func (m UserLoginMethods) Total() int {
	total := len(m.OidcProviderIds)
	for _, ok := range []bool{m.Password, m.EmailRecovery, m.GitHub, m.WeChat, m.Lark, m.LinuxDo} {
		if ok {
			total++
		}
	}
	return total
}

// HasOidcProvider 该用户是否有此提供方下的可用身份。
func (m UserLoginMethods) HasOidcProvider(providerId int) bool {
	for _, id := range m.OidcProviderIds {
		if id == providerId {
			return true
		}
	}
	return false
}

// RemainingAfterUnbind 移除本次解绑目标后剩余的可用方式数。
// unbindType = oidc 时 oidcProviderId 为 0 表示移除该用户全部 OIDC 身份（旧前端的全量解绑）。
func (m UserLoginMethods) RemainingAfterUnbind(unbindType string, oidcProviderId int) int {
	remaining := m.Total()
	switch unbindType {
	case LoginMethodGitHub:
		if m.GitHub {
			remaining--
		}
	case LoginMethodWeChat:
		if m.WeChat {
			remaining--
		}
	case LoginMethodLark:
		if m.Lark {
			remaining--
		}
	case LoginMethodLinuxDo:
		if m.LinuxDo {
			remaining--
		}
	case LoginMethodOidc:
		if oidcProviderId == 0 {
			remaining -= len(m.OidcProviderIds)
		} else if m.HasOidcProvider(oidcProviderId) {
			remaining--
		}
	}
	return remaining
}

// InventoryUserLoginMethods 按站点开关与用户各绑定列盘点可用登录 / 找回方式。
// user 必须是读了全列（含 password）的用户行；enabledOidcProviderIds 只应包含已启用提供方。
func InventoryUserLoginMethods(user *User, enabledOidcProviderIds []int) UserLoginMethods {
	if user == nil {
		return UserLoginMethods{}
	}
	return UserLoginMethods{
		Password:        config.LocalPasswordAllowedForRole(user.Role) && user.Password != "",
		EmailRecovery:   config.EffectivePasswordLogin() && stmp.SystemStmpConfigured() && user.Email != "",
		GitHub:          config.EffectiveSocialLogin(config.GitHubOAuthEnabled) && (user.GitHubId != "" || user.GitHubIdNew != 0),
		WeChat:          config.EffectiveSocialLogin(config.WeChatAuthEnabled) && user.WeChatId != "",
		Lark:            config.EffectiveSocialLogin(config.LarkAuthEnabled) && user.LarkId != "",
		LinuxDo:         config.EffectiveSocialLogin(config.LinuxDoOAuthEnabled) && user.LinuxDoId != 0,
		OidcProviderIds: enabledOidcProviderIds,
	}
}

// ListUserEnabledOidcProviderIds 列出该用户已绑定、且提供方 enabled 的 provider id。
// 同一提供方下的多个 subject 只返回一次，避免被当成多种登录方式。
// tx 为空时走全局 DB，便于在事务内复用同一连接。
func ListUserEnabledOidcProviderIds(tx *gorm.DB, userId int) ([]int, error) {
	if tx == nil {
		tx = DB
	}
	var ids []int
	err := tx.Model(&UserOidcIdentity{}).
		Joins("JOIN oidc_providers ON oidc_providers.id = user_oidc_identities.provider_id").
		Where("user_oidc_identities.user_id = ? AND oidc_providers.enabled = ?", userId, true).
		Distinct().
		Order("user_oidc_identities.provider_id asc").
		Pluck("user_oidc_identities.provider_id", &ids).Error
	return ids, err
}
