package controller

import (
	"encoding/json"
	"fmt"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/stmp"
	"github.com/modeltaps/modeltaps/common/telegram"
	"github.com/modeltaps/modeltaps/model"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetStatus(c *gin.Context) {
	telegramBot := ""
	if telegram.TGEnabled {
		telegramBot = telegram.TGBot.User.Username
	}
	// 非 option 配置(版本/启动信息、viper/env 配置、运行态)仍手工下发;
	// 其余 key 由注册表按 scope=public 自动生成。
	data := gin.H{
		"version":               config.Version,
		"start_time":            config.StartTime,
		"lark_client_id":        config.LarkClientId,
		"telegram_bot":          telegramBot,
		"UserInvoiceMonth":      config.UserInvoiceMonth,
		"UptimeDomain":          config.UPTIMEKUMA_DOMAIN,
		"UptimePageName":        config.UPTIMEKUMA_STATUS_PAGE_NAME,
		"UptimeEnabled":         config.UPTIMEKUMA_ENABLE,
		"max_log_query_days":    MaxLogQuerySpanDays,
		"max_log_lookback_days": MaxLogLookbackDays,
		// 仅下发「SMTP 是否可用」这一布尔，供前端决定邮箱验证码入口是否渲染
		"smtp_configured": stmp.SystemStmpConfigured(),
		// 请求/响应留存的单条正文上限（KB），来自配置文件 log_io_max_body_kb，
		// 供前端设置页文案显示真实上限
		"log_io_max_body_kb": model.LogIOMaxBodyBytes() / 1024,
	}
	for k, v := range config.GlobalOption.PublicOptions() {
		data[k] = v
	}
	// 账号体系推导出的有效开关覆盖注册表自动下发的原始值：外部身份提供方模式下密码、注册、
	// 社交登录、通行密钥（非 root）一律关闭。前端只读这里的有效值，不再自己拼判定。
	smtpConfigured := stmp.SystemStmpConfigured()
	data["password_login"] = config.EffectivePasswordLogin()
	data["password_register"] = config.EffectivePasswordRegister()
	data["github_oauth"] = config.EffectiveSocialLogin(config.GitHubOAuthEnabled)
	data["wechat_login"] = config.EffectiveSocialLogin(config.WeChatAuthEnabled)
	data["lark_login"] = config.EffectiveSocialLogin(config.LarkAuthEnabled)
	data["linuxDo_oauth"] = config.EffectiveSocialLogin(config.LinuxDoOAuthEnabled)
	data["email_code_login"] = config.EffectiveEmailCodeLogin(smtpConfigured)
	data["passkey_login"] = config.EffectivePasskeyLogin()
	data["admin_login_enabled"] = config.AdminLoginAvailable()
	// 登录用的提供方：外部模式下只有承担本站身份的那一个；内置模式下不下发（提供方不是登录按钮）。
	// 旧的 OIDCAuthEnabled / OIDCDisplayName / OidcAutoRedirect 选项仅作兼容保留，这里覆盖掉自动下发的值。
	providers := loginOidcProviderStatus()
	data["oidc_providers"] = providers
	data["oidc_auth"] = len(providers) > 0
	data["oidc_auto_redirect"] = len(providers) > 0
	data["oidc_display_name"] = ""
	if len(providers) > 0 {
		data["oidc_display_name"] = providers[0].DisplayName
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

// oidcProviderStatus /api/status 下发的单个 OIDC 登录入口，只含前端渲染按钮所需字段。
type oidcProviderStatus struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	FirstParty  bool   `json:"first_party"`
	// issuer 提供方的签发者地址，原样下发（未知时为空串）：深链全空时前端据此取登录服务站点根地址做兜底出口
	Issuer string `json:"issuer"`
	// account_settings_url 提供方侧账号设置页，空串表示没有
	AccountSettingsUrl string `json:"account_settings_url"`
	// 四个设置页深链：账号安全页按行跳转，留空的行不渲染
	PasswordUrl string `json:"password_url"`
	MfaUrl      string `json:"mfa_url"`
	PasskeyUrl  string `json:"passkey_url"`
	IdentityUrl string `json:"identity_url"`
}

// loginOidcProviderStatus 下发登录用的提供方：外部账号体系下只有承担本站身份的那一个（没有则空），
// 内置账号下恒为空。查询失败时记日志并返回空列表：状态接口是匿名入口，不能因为一次 DB 抖动整体失败。
func loginOidcProviderStatus() []oidcProviderStatus {
	if !config.IsExternalAccountSystem() {
		return []oidcProviderStatus{}
	}
	provider, err := model.GetFirstPartyOidcProvider()
	if err != nil {
		logger.SysError("failed to read site identity providers: " + err.Error())
		return []oidcProviderStatus{}
	}
	var providers []*model.OidcProvider
	if provider != nil {
		providers = append(providers, provider)
	}
	result := make([]oidcProviderStatus, 0, len(providers))
	for _, provider := range providers {
		displayName := provider.DisplayName
		if displayName == "" {
			displayName = provider.Slug
		}
		result = append(result, oidcProviderStatus{
			Slug:               provider.Slug,
			DisplayName:        displayName,
			FirstParty:         provider.FirstParty,
			Issuer:             provider.Issuer,
			AccountSettingsUrl: provider.AccountSettingsUrl,
			PasswordUrl:        provider.PasswordUrl,
			MfaUrl:             provider.MfaUrl,
			PasskeyUrl:         provider.PasskeyUrl,
			IdentityUrl:        provider.IdentityUrl,
		})
	}
	return result
}

func GetNotice(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    config.GlobalOption.Get("Notice"),
	})
}

func GetAbout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    config.GlobalOption.Get("About"),
	})
}

func GetHomePageContent(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    config.GlobalOption.Get("HomePageContent"),
	})
}

func SendEmailVerification(c *gin.Context) {
	email := common.NormalizeEmail(c.Query("email"))
	if err := common.ValidateEmailStrict(email); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid email format",
		})
		return
	}
	if config.EmailDomainRestrictionEnabled {
		allowed := false
		for _, domain := range config.EmailDomainWhitelist {
			if strings.HasSuffix(email, "@"+domain) {
				allowed = true
				break
			}
		}
		if !allowed {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "The admin has enabled an email domain allowlist, and your email domain is not on it",
			})
			return
		}
	}
	// 防枚举：邮箱已被占用时不发送验证码，但返回与正常发送一致的响应
	if model.IsEmailAlreadyTaken(email) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
		})
		return
	}
	code := common.GenerateVerificationCode(6)
	common.RegisterVerificationCodeWithKey(email, code, common.EmailVerificationPurpose)
	err := stmp.SendVerificationCodeEmail(email, code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// passwordManagedExternally 外部账号体系下密码在提供方那里找回，本站的重置链路整体关闭。
func passwordManagedExternally(c *gin.Context) bool {
	if !config.IsExternalAccountSystem() {
		return false
	}
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "Accounts on this site are managed by single sign-on. Please reset your password from the sign-in page.",
	})
	return true
}

func SendPasswordResetEmail(c *gin.Context) {
	if passwordManagedExternally(c) {
		return
	}
	email := common.NormalizeEmail(c.Query("email"))
	if err := common.Validate.Var(email, "required,email"); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid parameters",
		})
		return
	}

	user := &model.User{
		Email: model.NullableEmail(email),
	}

	// 防枚举：邮箱未注册时不发信，但返回与正常发送一致的响应
	if err := user.FillUserByEmail(); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
		})
		return
	}

	userName := user.DisplayName
	if userName == "" {
		userName = user.Username
	}

	code := common.GenerateVerificationCode(0)
	common.RegisterVerificationCodeWithKey(email, code, common.PasswordResetPurpose)
	link := fmt.Sprintf("%s/user/reset?email=%s&token=%s", config.ServerAddress, email, code)
	err := stmp.SendPasswordResetEmail(userName, email, link)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

type PasswordResetRequest struct {
	Email string `json:"email"`
	Token string `json:"token"`
}

func ResetPassword(c *gin.Context) {
	if passwordManagedExternally(c) {
		return
	}
	var req PasswordResetRequest
	err := json.NewDecoder(c.Request.Body).Decode(&req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid parameters",
		})
		return
	}

	req.Email = common.NormalizeEmail(req.Email)
	if req.Email == "" || req.Token == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid parameters",
		})
		return
	}
	if !common.VerifyCodeWithKey(req.Email, req.Token, common.PasswordResetPurpose) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Reset link is invalid or has expired",
		})
		return
	}
	password := common.GenerateVerificationCode(12)
	err = model.ResetUserPasswordByEmail(req.Email, password)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	common.DeleteKey(req.Email, common.PasswordResetPurpose)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    password,
	})
}
