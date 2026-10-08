package controller

import (
	"net/http"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// loginDirectTarget 判定本次 /login 请求是否应直接重定向到身份提供方，返回目标提供方；
// 不成立返回 nil，调用方按原样返回前端页面。
// 只有外部账号体系才直达，目标是承担本站身份的那一个提供方（见 model.PickFirstPartyOidcProvider）；
// 另加一条只有服务端才需要判断的前置：仅浏览器导航请求（Accept 含 text/html）。
// /login/admin 是另一条路由，不经过这里。
func loginDirectTarget(c *gin.Context, providers []*model.OidcProvider) *model.OidcProvider {
	if !strings.Contains(c.GetHeader("Accept"), "text/html") {
		return nil
	}
	if !config.IsExternalAccountSystem() {
		return nil
	}
	return model.PickFirstPartyOidcProvider(providers)
}

// LoginDirectRedirect 浏览器导航到 /login 时，外部账号体系下直接 302 到提供方授权页，
// 不再返回本站登录页。返回 false 表示未重定向，调用方落回前端页面（内置模式的登录表单，
// 或外部模式下提供方尚未配好时的提示页）。
// 授权 URL 与 /api/oauth/endpoint 共用同一份构造逻辑（含一次性 state、PKCE 与 nonce）；
// ui_locales 取 Accept-Language 首选项——这条路径上没有前端，读不到界面语言。
func LoginDirectRedirect(c *gin.Context) bool {
	if !config.IsExternalAccountSystem() {
		return false
	}
	providers, err := model.GetEnabledOidcProviders()
	if err != nil {
		logger.SysError("Failed to read enabled OIDC providers: " + err.Error())
		return false
	}
	provider := loginDirectTarget(c, providers)
	if provider == nil {
		return false
	}
	loginURL, err := oidcAuthorizeURL(c, provider, acceptLanguageUILocale(c.GetHeader("Accept-Language")))
	if err != nil {
		logger.SysError("Failed to build direct login URL: provider=" + provider.Slug + " err=" + err.Error())
		return false
	}
	c.Redirect(http.StatusFound, loginURL)
	return true
}

// AdminLoginPath 管理员应急登录页地址；旧的 /login?local=1 永久跳转到这里。
const AdminLoginPath = "/login/admin"
