package model

import (
	"encoding/json"
	"fmt"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/brandicon"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"strconv"
	"strings"
	"time"
)

type Option struct {
	Key   string `json:"key" gorm:"primaryKey"`
	Value string `json:"value"`
}

func AllOption() ([]*Option, error) {
	var options []*Option
	err := DB.Find(&options).Error
	return options, err
}

func GetOption(key string) (option Option, err error) {
	err = DB.First(&option, Option{Key: key}).Error
	return
}

func InitOptionMap() {
	registerOptions()
	loadOptionsFromDatabase()
}

func registerOptions() {

	// 登录页要在提交前就知道密码登录 / 密码注册是否开放,据此隐藏密码表单与注册入口(UX-33);
	// 两个布尔本就能通过提交探知,公开下发不泄露敏感信息。
	config.GlobalOption.RegisterBool("PasswordLoginEnabled", &config.PasswordLoginEnabled, config.ScopePublic, config.PublicSpec{StatusKey: "password_login"})
	config.GlobalOption.RegisterBool("PasswordRegisterEnabled", &config.PasswordRegisterEnabled, config.ScopePublic, config.PublicSpec{StatusKey: "password_register"})
	// 账号体系二选一（builtin / external）：登录页、账号安全页与各登录方式的有效性都由它推导，
	// 需公开下发；取值只有两个词，不含敏感信息。非法取值在 Set 时拒绝，避免把站点锁进未知形态。
	config.GlobalOption.RegisterCustom("AccountSystem", func() string {
		return config.AccountSystem
	}, func(value string) error {
		if value == "" {
			config.AccountSystem = config.AccountSystemBuiltin
			return nil
		}
		if !config.ValidAccountSystem(value) {
			return fmt.Errorf("AccountSystem must be %s or %s", config.AccountSystemBuiltin, config.AccountSystemExternal)
		}
		config.AccountSystem = value
		return nil
	}, "", config.ScopePublic, config.PublicSpec{StatusKey: "account_system", StatusValue: func() any { return config.AccountSystem }})
	// 外部模式下的管理员应急登录开关、内置模式下的邮箱验证码 / 通行密钥开关：登录页与账号页据此渲染入口。
	config.GlobalOption.RegisterBool("AdminLoginEnabled", &config.AdminLoginEnabled, config.ScopePublic, config.PublicSpec{StatusKey: "admin_login_enabled"})
	config.GlobalOption.RegisterBool("EmailCodeLoginEnabled", &config.EmailCodeLoginEnabled, config.ScopeAdmin)
	config.GlobalOption.RegisterBool("PasskeyLoginEnabled", &config.PasskeyLoginEnabled, config.ScopeAdmin)
	// 登录直达 OIDC(UX-34):登录页据此决定是否直接跳去 IdP,需公开下发;只是一个布尔,不含敏感信息。
	config.GlobalOption.RegisterBool("OidcAutoRedirect", &config.OidcAutoRedirect, config.ScopePublic, config.PublicSpec{StatusKey: "oidc_auto_redirect"})
	config.GlobalOption.RegisterBool("EmailVerificationEnabled", &config.EmailVerificationEnabled, config.ScopePublic, config.PublicSpec{StatusKey: "email_verification"})
	config.GlobalOption.RegisterBool("GitHubOAuthEnabled", &config.GitHubOAuthEnabled, config.ScopePublic, config.PublicSpec{StatusKey: "github_oauth"})
	config.GlobalOption.RegisterBool("WeChatAuthEnabled", &config.WeChatAuthEnabled, config.ScopePublic, config.PublicSpec{StatusKey: "wechat_login"})
	config.GlobalOption.RegisterBool("LarkAuthEnabled", &config.LarkAuthEnabled, config.ScopePublic, config.PublicSpec{StatusKey: "lark_login"})
	// OIDCAuthEnabled 已由 oidc_providers 表取代（/api/status 的 oidc_auth 由启用行数算出）。
	// 仍注册是为了让存量 options 行有归属、迁移可重跑；scope 降为 admin，不再对外下发。
	config.GlobalOption.RegisterBool("OIDCAuthEnabled", &config.OIDCAuthEnabled, config.ScopeAdmin)
	config.GlobalOption.RegisterBool("LinuxDoOAuthEnabled", &config.LinuxDoOAuthEnabled, config.ScopePublic, config.PublicSpec{StatusKey: "linuxDo_oauth"})
	config.GlobalOption.RegisterBool("InviteCodeRegisterEnabled", &config.InviteCodeRegisterEnabled, config.ScopePublic, config.PublicSpec{StatusKey: "invite_code_register"})
	config.GlobalOption.RegisterBool("LinuxDoOAuthTrustLevelEnabled", &config.LinuxDoOAuthTrustLevelEnabled, config.ScopeAdmin)
	config.GlobalOption.RegisterBool("LinuxDoOAuthDynamicTrustLevel", &config.LinuxDoOAuthDynamicTrustLevel, config.ScopeAdmin)
	config.GlobalOption.RegisterBool("TurnstileCheckEnabled", &config.TurnstileCheckEnabled, config.ScopePublic, config.PublicSpec{StatusKey: "turnstile_check"})
	config.GlobalOption.RegisterBool("RegisterEnabled", &config.RegisterEnabled, config.ScopeAdmin)
	config.GlobalOption.RegisterBool("AutomaticDisableChannelEnabled", &config.AutomaticDisableChannelEnabled, config.ScopeAdmin)
	config.GlobalOption.RegisterBool("AutomaticEnableChannelEnabled", &config.AutomaticEnableChannelEnabled, config.ScopeAdmin)
	config.GlobalOption.RegisterBool("AutomaticDisableChannelNotifyEnabled", &config.AutomaticDisableChannelNotifyEnabled, config.ScopeAdmin)
	config.GlobalOption.RegisterBool("ApproximateTokenEnabled", &config.ApproximateTokenEnabled, config.ScopeAdmin)
	config.GlobalOption.RegisterBool("LogConsumeEnabled", &config.LogConsumeEnabled, config.ScopeAdmin)
	config.GlobalOption.RegisterBool("LogAutoDeleteEnabled", &config.LogAutoDeleteEnabled, config.ScopeAdmin)
	config.GlobalOption.RegisterInt("LogAutoDeleteDays", &config.LogAutoDeleteDays, config.ScopeAdmin)
	config.GlobalOption.RegisterBool("EmptyResponseBillingEnabled", &config.EmptyResponseBillingEnabled, config.ScopeAdmin)
	config.GlobalOption.RegisterInt("MaxPromptTokens", &config.MaxPromptTokens, config.ScopeAdmin)
	config.GlobalOption.RegisterBool("DisplayInCurrencyEnabled", &config.DisplayInCurrencyEnabled, config.ScopePublic, config.PublicSpec{StatusKey: "display_in_currency"})
	config.GlobalOption.RegisterBool("DisplayTokenStatEnabled", &config.DisplayTokenStatEnabled, config.ScopeAdmin)
	config.GlobalOption.RegisterFloat("ChannelDisableThreshold", &config.ChannelDisableThreshold, config.ScopeAdmin)
	config.GlobalOption.RegisterBool("EmailDomainRestrictionEnabled", &config.EmailDomainRestrictionEnabled, config.ScopeAdmin)

	config.GlobalOption.RegisterCustom("EmailDomainWhitelist", func() string {
		return strings.Join(config.EmailDomainWhitelist, ",")
	}, func(value string) error {
		config.EmailDomainWhitelist = strings.Split(value, ",")
		return nil
	}, "", config.ScopeAdmin)

	config.GlobalOption.RegisterString("SMTPServer", &config.SMTPServer, config.ScopeAdmin)
	config.GlobalOption.RegisterString("SMTPFrom", &config.SMTPFrom, config.ScopeAdmin)
	config.GlobalOption.RegisterInt("SMTPPort", &config.SMTPPort, config.ScopeAdmin)
	config.GlobalOption.RegisterString("SMTPAccount", &config.SMTPAccount, config.ScopeAdmin)
	config.GlobalOption.RegisterString("SMTPToken", &config.SMTPToken, config.ScopeSecret)
	config.GlobalOption.RegisterString("SMTPTLSMode", &config.SMTPTLSMode, config.ScopeAdmin)
	config.GlobalOption.RegisterValue("Notice", config.ScopeAdmin)
	config.GlobalOption.RegisterValue("About", config.ScopeAdmin)
	config.GlobalOption.RegisterValue("HomePageContent", config.ScopeAdmin)
	config.GlobalOption.RegisterString("Footer", &config.Footer, config.ScopePublic, config.PublicSpec{StatusKey: "footer_html"})
	config.GlobalOption.RegisterString("SystemName", &config.SystemName, config.ScopePublic, config.PublicSpec{StatusKey: "system_name"})
	config.GlobalOption.RegisterString("Logo", &config.Logo, config.ScopePublic, config.PublicSpec{StatusKey: "logo"})
	config.GlobalOption.RegisterString("AnalyticsCode", &config.AnalyticsCode, config.ScopePublic, config.PublicSpec{StatusKey: "analytics_code"})
	config.GlobalOption.RegisterString("ServerAddress", &config.ServerAddress, config.ScopePublic, config.PublicSpec{StatusKey: "server_address"})
	config.GlobalOption.RegisterString("PaymentCallbackAddress", &config.PaymentCallbackAddress, config.ScopeAdmin)
	config.GlobalOption.RegisterString("GitHubClientId", &config.GitHubClientId, config.ScopePublic, config.PublicSpec{StatusKey: "github_client_id"})
	config.GlobalOption.RegisterString("GitHubClientSecret", &config.GitHubClientSecret, config.ScopeSecret)

	// 以下 OIDC* 选项为只读兼容：多提供方配置已迁到 oidc_providers 表（迁移 202609030001），
	// 认证流程不再读取它们。保留注册只为存量 options 行仍能加载、迁移可重跑。
	config.GlobalOption.RegisterString("OIDCClientId", &config.OIDCClientId, config.ScopeAdmin)
	config.GlobalOption.RegisterString("OIDCClientSecret", &config.OIDCClientSecret, config.ScopeSecret)
	config.GlobalOption.RegisterString("OIDCIssuer", &config.OIDCIssuer, config.ScopeAdmin)
	config.GlobalOption.RegisterString("OIDCScopes", &config.OIDCScopes, config.ScopeAdmin)
	config.GlobalOption.RegisterString("OIDCUsernameClaims", &config.OIDCUsernameClaims, config.ScopeAdmin)
	config.GlobalOption.RegisterString("OIDCDisplayName", &config.OIDCDisplayName, config.ScopeAdmin)

	config.GlobalOption.RegisterString("LinuxDoClientId", &config.LinuxDoClientId, config.ScopePublic, config.PublicSpec{StatusKey: "linuxDo_client_id"})
	config.GlobalOption.RegisterString("LinuxDoClientSecret", &config.LinuxDoClientSecret, config.ScopeSecret)
	config.GlobalOption.RegisterInt("LinuxDoOAuthLowestTrustLevel", &config.LinuxDoOAuthLowestTrustLevel, config.ScopeAdmin)

	config.GlobalOption.RegisterString("WeChatServerAddress", &config.WeChatServerAddress, config.ScopeAdmin)
	config.GlobalOption.RegisterString("WeChatServerToken", &config.WeChatServerToken, config.ScopeSecret)
	config.GlobalOption.RegisterString("WeChatAccountQRCodeImageURL", &config.WeChatAccountQRCodeImageURL, config.ScopePublic, config.PublicSpec{StatusKey: "wechat_qrcode"})
	config.GlobalOption.RegisterString("TurnstileSiteKey", &config.TurnstileSiteKey, config.ScopePublic, config.PublicSpec{StatusKey: "turnstile_site_key"})
	config.GlobalOption.RegisterString("TurnstileSecretKey", &config.TurnstileSecretKey, config.ScopeSecret)
	config.GlobalOption.RegisterInt("QuotaForNewUser", &config.QuotaForNewUser, config.ScopeAdmin)
	// 邀请奖励总开关：前端未登录/登录态都要据此决定是否展示邀请奖励入口，故公开下发。
	config.GlobalOption.RegisterBool("InviteRewardEnabled", &config.InviteRewardEnabled, config.ScopePublic, config.PublicSpec{StatusKey: "invite_reward_enabled"})
	config.GlobalOption.RegisterInt("QuotaForInviter", &config.QuotaForInviter, config.ScopeAdmin)
	config.GlobalOption.RegisterInt("QuotaForInvitee", &config.QuotaForInvitee, config.ScopeAdmin)
	config.GlobalOption.RegisterString("InviterRewardType", &config.InviterRewardType, config.ScopeAdmin)
	config.GlobalOption.RegisterInt("InviterRewardValue", &config.InviterRewardValue, config.ScopeAdmin)
	config.GlobalOption.RegisterInt("QuotaRemindThreshold", &config.QuotaRemindThreshold, config.ScopeAdmin)
	config.GlobalOption.RegisterInt("PreConsumedQuota", &config.PreConsumedQuota, config.ScopeAdmin)

	config.GlobalOption.RegisterString("TopUpLink", &config.TopUpLink, config.ScopePublic, config.PublicSpec{StatusKey: "top_up_link"})
	config.GlobalOption.RegisterString("DocsLink", &config.DocsLink, config.ScopePublic, config.PublicSpec{StatusKey: "docs_link"})
	config.GlobalOption.RegisterString("ChatLink", &config.ChatLink, config.ScopePublic, config.PublicSpec{StatusKey: "chat_link"})
	config.GlobalOption.RegisterString("ChatLinks", &config.ChatLinks, config.ScopePublic, config.PublicSpec{StatusKey: "chat_links"})
	config.GlobalOption.RegisterFloat("QuotaPerUnit", &config.QuotaPerUnit, config.ScopePublic, config.PublicSpec{StatusKey: "quota_per_unit"})
	config.GlobalOption.RegisterInt("RetryTimes", &config.RetryTimes, config.ScopeAdmin)
	config.GlobalOption.RegisterInt("RetryCooldownSeconds", &config.RetryCooldownSeconds, config.ScopeAdmin)
	config.GlobalOption.RegisterInt("ModelNotFoundCooldownSeconds", &config.ModelNotFoundCooldownSeconds, config.ScopeAdmin)
	config.GlobalOption.RegisterBool("ChannelFailErrorWrapEnabled", &config.ChannelFailErrorWrapEnabled, config.ScopeAdmin)
	config.GlobalOption.RegisterString("ChannelFailErrorMessage", &config.ChannelFailErrorMessage, config.ScopeAdmin)
	config.GlobalOption.RegisterCustom("RetryCooldownPerStatus", func() string {
		return config.RetryCooldownPerStatus
	}, func(value string) error {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			config.RetryCooldownPerStatus = ""
			config.SetRetryCooldownPerStatusMap(map[int]int{})
			return nil
		}
		// Accept both `{"503":120}` and `{"503":"120"}` forms; reject anything else.
		raw := map[string]json.Number{}
		if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
			return fmt.Errorf("RetryCooldownPerStatus must be a JSON object of status->seconds: %w", err)
		}
		parsed := make(map[int]int, len(raw))
		for k, v := range raw {
			code, err := strconv.Atoi(strings.TrimSpace(k))
			if err != nil || code < 100 || code > 599 {
				return fmt.Errorf("invalid status code %q (must be 100-599)", k)
			}
			secs, err := v.Int64()
			if err != nil || secs < 0 {
				return fmt.Errorf("invalid cooldown for status %s: %q (must be non-negative integer)", k, v.String())
			}
			parsed[code] = int(secs)
		}
		config.RetryCooldownPerStatus = trimmed
		config.SetRetryCooldownPerStatusMap(parsed)
		return nil
	}, "", config.ScopeAdmin)

	config.GlobalOption.RegisterBool("MjNotifyEnabled", &config.MjNotifyEnabled, config.ScopePublic, config.PublicSpec{StatusKey: "mj_notify_enabled"})
	config.GlobalOption.RegisterBool("BuiltinChatEnabled", &config.BuiltinChatEnabled, config.ScopePublic, config.PublicSpec{StatusKey: "builtin_chat_enabled"})
	config.GlobalOption.RegisterString("ChatImageRequestProxy", &config.ChatImageRequestProxy, config.ScopeAdmin)
	config.GlobalOption.RegisterFloat("PaymentUSDRate", &config.PaymentUSDRate, config.ScopePublic)
	config.GlobalOption.RegisterInt("PaymentMinAmount", &config.PaymentMinAmount, config.ScopePublic)

	config.GlobalOption.RegisterCustom("RechargeDiscount", func() string {
		return common.RechargeDiscount2JSONString()
	}, func(value string) error {
		config.RechargeDiscount = value
		common.UpdateRechargeDiscountByJSONString(value)
		return nil
	}, "", config.ScopePublic, config.PublicSpec{StatusValue: func() any { return config.RechargeDiscount }})

	config.GlobalOption.RegisterString("CFWorkerImageUrl", &config.CFWorkerImageUrl, config.ScopeAdmin)
	config.GlobalOption.RegisterString("CFWorkerImageKey", &config.CFWorkerImageKey, config.ScopeSecret)
	config.GlobalOption.RegisterInt("OldTokenMaxId", &config.OldTokenMaxId, config.ScopeAdmin)
	config.GlobalOption.RegisterBool("GitHubOldIdCloseEnabled", &config.GitHubOldIdCloseEnabled, config.ScopeAdmin)

	config.GlobalOption.RegisterBool("GeminiAPIEnabled", &config.GeminiAPIEnabled, config.ScopePublic)
	config.GlobalOption.RegisterBool("ClaudeAPIEnabled", &config.ClaudeAPIEnabled, config.ScopePublic)
	config.GlobalOption.RegisterBool("ClaudePromptCachingEnabled", &config.ClaudePromptCachingEnabled, config.ScopeAdmin)

	config.GlobalOption.RegisterCustom("DisableChannelKeywords", func() string {
		return common.DisableChannelKeywordsInstance.GetKeywords()
	}, func(value string) error {
		common.DisableChannelKeywordsInstance.Load(value)
		return nil
	}, common.GetDefaultDisableChannelKeywords(), config.ScopeAdmin)

	config.GlobalOption.RegisterInt("RetryTimeOut", &config.RetryTimeOut, config.ScopeAdmin)

	config.GlobalOption.RegisterBool("LogIOEnabled", &config.LogIOEnabled, config.ScopePublic, config.PublicSpec{StatusKey: "log_io_enabled"})
	// 个人/组织设置页的留存开关要显示「生效值」(用户/组织未设置时即站点默认),
	// 故两个站点默认值公开下发;它们只描述默认策略,不含敏感信息。
	config.GlobalOption.RegisterBool("LogIODefaultUser", &config.LogIODefaultUser, config.ScopePublic, config.PublicSpec{StatusKey: "log_io_default_user"})
	config.GlobalOption.RegisterBool("OrganizationLogIODefault", &config.OrganizationLogIODefault, config.ScopePublic, config.PublicSpec{StatusKey: "organization_log_io_default"})

	config.GlobalOption.RegisterBool("EnableSafe", &config.EnableSafe, config.ScopePublic)
	config.GlobalOption.RegisterString("SafeToolName", &config.SafeToolName, config.ScopePublic)
	config.GlobalOption.RegisterCustom("SafeKeyWords", func() string {
		return strings.Join(config.SafeKeyWords, "\n")
	}, func(value string) error {
		config.SafeKeyWords = strings.Split(value, "\n")
		return nil
	}, "", config.ScopePublic, config.PublicSpec{StatusValue: func() any { return config.SafeKeyWords }})

	// 注册统一请求响应模型配置项
	config.GlobalOption.RegisterBool("UnifiedRequestResponseModelEnabled", &config.UnifiedRequestResponseModelEnabled, config.ScopeAdmin)

	// 注册响应指纹透传配置项
	config.GlobalOption.RegisterBool("FingerprintPassThroughEnabled", &config.FingerprintPassThroughEnabled, config.ScopeAdmin)

	// 注册模型名称大小写不敏感配置项
	config.GlobalOption.RegisterBool("ModelNameCaseInsensitiveEnabled", &config.ModelNameCaseInsensitiveEnabled, config.ScopeAdmin)

	// 组织功能配置项
	config.GlobalOption.RegisterBool("OrganizationEnabled", &config.OrganizationEnabled, config.ScopePublic, config.PublicSpec{StatusKey: "organization_enabled"})
	config.GlobalOption.RegisterInt("OrganizationMaxPerUser", &config.OrganizationMaxPerUser, config.ScopeAdmin)
	config.GlobalOption.RegisterInt("OrganizationDefaultMaxMembers", &config.OrganizationDefaultMaxMembers, config.ScopeAdmin)
	config.GlobalOption.RegisterString("OrganizationDissolveQuotaRefund", &config.OrganizationDissolveQuotaRefund, config.ScopeAdmin)
	config.GlobalOption.RegisterBool("OrganizationUsageVisibleDefault", &config.OrganizationUsageVisibleDefault, config.ScopeAdmin)
	config.GlobalOption.RegisterBool("OrganizationQuotaTransferEnabled", &config.OrganizationQuotaTransferEnabled, config.ScopeAdmin)

	// 令牌周期配额重置的时间设置(TK8a)
	config.GlobalOption.RegisterString("QuotaResetTimezone", &config.QuotaResetTimezone, config.ScopePublic, config.PublicSpec{StatusKey: "quota_reset_timezone"})
	config.GlobalOption.RegisterString("QuotaResetWeekStart", &config.QuotaResetWeekStart, config.ScopePublic, config.PublicSpec{StatusKey: "quota_reset_week_start"})

	// 品牌图标：同步层开关、registry 与 favicon 抓取开关
	config.GlobalOption.RegisterBool("BrandIconSyncEnabled", &config.BrandIconSyncEnabled, config.ScopeAdmin)
	config.GlobalOption.RegisterCustom("BrandIconRegistry", func() string {
		return config.BrandIconRegistry
	}, func(value string) error {
		registry, err := brandicon.NormalizeRegistry(value)
		if err != nil {
			return err
		}
		config.BrandIconRegistry = registry
		return nil
	}, "", config.ScopeAdmin)
	config.GlobalOption.RegisterBool("BrandIconFaviconFetchEnabled", &config.BrandIconFaviconFetchEnabled, config.ScopeAdmin)
}

func loadOptionsFromDatabase() {
	options, _ := AllOption()
	for _, option := range options {
		err := config.GlobalOption.Set(option.Key, option.Value)
		if err != nil {
			logger.SysError("failed to update option map: " + err.Error())
		}
	}
}

func SyncOptions(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		logger.SysLog("syncing options from database")
		loadOptionsFromDatabase()
	}
}

func UpdateOption(key string, value string) error {
	// Save to database first
	option := Option{
		Key: key,
	}
	// https://gorm.io/docs/update.html#Save-All-Fields
	DB.FirstOrCreate(&option, Option{Key: key})
	option.Value = value
	// Save is a combination function.
	// If save value does not contain primary key, it will execute Create,
	// otherwise it will execute Update (with all fields).
	DB.Save(&option)
	// Update OptionMap
	return config.GlobalOption.Set(key, value)
}
