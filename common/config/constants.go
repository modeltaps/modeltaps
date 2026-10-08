package config

import (
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"
)

var StartTime = time.Now().Unix() // unit: second
var Version = "v0.0.0"            // this hard coding will be replaced automatically when building, no need to manually change
var Commit = "unknown"
var BuildTime = "unknown"
var SystemName = "Modeltaps"
var ServerAddress = "http://localhost:3000"
var PaymentCallbackAddress = ""
var Debug = false

var OldTokenMaxId = 0

var Footer = ""

// UnpricedModelPolicy 控制「价格表查无此模型」时的计费行为:
// block(默认)=拒绝请求零费用; zero=放行不计费; default=按 UnpricedModelDefaultRatio 计费。
var UnpricedModelPolicy = "block"
var UnpricedModelDefaultRatio = 30.0
var Logo = ""
var TopUpLink = ""
var DocsLink = "https://docs.modeltaps.com"
var ChatLink = ""
var ChatLinks = ""
var AnalyticsCode = ""
var QuotaPerUnit = 500 * 1000.0 // $0.002 / 1K tokens
var DisplayInCurrencyEnabled = true
var DisplayTokenStatEnabled = true

// 是否开启用户月账单功能
var UserInvoiceMonth = false

// Any options with "Secret", "Token" in its key won't be return by GetOptions

var SessionSecret = uuid.New().String()

// SessionSecretExplicit 标记 session_secret 是否由部署方显式配置；
// 未配置时每次启动都会生成随机值，导致重启后会话失效。
var SessionSecretExplicit = false

var ItemsPerPage = 10
var MaxRecentItems = 100

// 账号级登录失败计数与锁定：同一账号(不区分来源 IP)连续失败达到阈值后，
// 在锁定时长内直接拒绝密码登录，用于遏制分布式撞库。
var LoginMaxFailures = 5
var LoginLockoutDuration = 15 * time.Minute

// TurnstileSessionTTL 会话内 Turnstile 校验结果的有效期。
// 需覆盖「发送验证码 → 注册」两步流程，同时避免一次校验长期免检导致登录接口被绕过。
var TurnstileSessionTTL = 10 * time.Minute

// 登录会话生命周期：SessionIdleDuration 内没有任何请求即失效；无论是否活跃，
// 距登录时刻超过 SessionMaxDuration 也失效（config.yaml 的 session.idle_days / session.max_days）。
var SessionIdleDuration = 7 * 24 * time.Hour
var SessionMaxDuration = 30 * 24 * time.Hour

var PasswordLoginEnabled = true
var PasswordRegisterEnabled = true

// 登录直达 OIDC(UX-34):开启后前端在需要登录时直接跳到唯一启用的 OIDC 提供方,不再展示本站登录页。
// 前端只在密码登录已关闭、恰好一个 OIDC 提供方且无其他第三方登录时才真正跳转;/login?local=1 永远显示本站登录页。
var OidcAutoRedirect = false
var EmailVerificationEnabled = false
var GitHubOAuthEnabled = false
var WeChatAuthEnabled = false
var LarkAuthEnabled = false
var TurnstileCheckEnabled = false
var RegisterEnabled = true
var InviteCodeRegisterEnabled = false
var OIDCAuthEnabled = false
var LinuxDoOAuthEnabled = false
var LinuxDoOAuthTrustLevelEnabled = false
var LinuxDoOAuthDynamicTrustLevel = true // 动态限制已注册用户的信任等级，关闭后已注册用户不受新等级限制影响

// 是否开启内容审查
var EnableSafe = false

// 站点级「完整请求/响应留存」总开关(opt-in,默认关闭,ZDR 心智)。
// 写入明细需站点级总开关与归属默认(用户/组织)同时开启,见 T50e。
var LogIOEnabled = false

// LogIO 分层继承的站点级默认值(均默认关闭,ZDR);令牌为「继承」(nil)且上级未配置时回退到这两个值。
var LogIODefaultUser = false         // 个人令牌「继承」且用户未配置个人默认时的站点默认
var OrganizationLogIODefault = false // 组织令牌「继承」且组织未配置默认时的站点默认

// 默认使用系统自带关键词审查工具
var SafeToolName = "Keyword"

// 系统自带关键词审查默认字典
var SafeKeyWords = []string{
	"fuck",
	"shit",
	"bitch",
	"pussy",
	"cunt",
	"dick",
	"asshole",
	"bastard",
	"slut",
	"whore",
	"nigger",
	"nigga",
	"nazi",
	"gay",
	"lesbian",
	"transgender",
	"queer",
	"homosexual",
	"incest",
	"rape",
	"rapist",
	"raped",
	"raping",
	"raped",
	"raping",
	"rapist",
	"rape",
	"sex",
	"sexual",
	"sexually",
	"sexualize",
	"sexualized",
	"sexualizes",
	"sexualizing",
	"sexually",
	"sex",
	"porn",
	"pornography",
	"prostitute",
	"prostitution",
	"masturbate",
	"masturbation",
	"pedophile",
	"pedophilia",
	"hentai",
	"explicit",
	"obscene",
	"obscenity",
	"erotic",
	"erotica",
	"fetish",
	"NSFW",
	"nude",
	"nudity",
	"harassment",
	"abuse",
	"violent",
	"violence",
	"suicide",
	"racist",
	"racism",
	"discrimination",
	"hate",
	"terrorism",
	"terrorist",
	"drugs",
	"cocaine",
	"heroin",
	"methamphetamine",
}

// mj
var MjNotifyEnabled = false

// 内置聊天功能开关
var BuiltinChatEnabled = true

var EmailDomainRestrictionEnabled = false
var EmailDomainWhitelist = []string{
	"gmail.com",
	"163.com",
	"126.com",
	"qq.com",
	"outlook.com",
	"hotmail.com",
	"icloud.com",
	"yahoo.com",
	"foxmail.com",
}

var MemoryCacheEnabled = false

var LogConsumeEnabled = true

var LogAutoDeleteEnabled = false // 是否启用消费日志自动清理
var LogAutoDeleteDays = 30       // 保留天数，默认30天

var SMTPServer = ""
var SMTPPort = 587
var SMTPAccount = ""
var SMTPFrom = ""
var SMTPToken = ""

// SMTPTLSMode 连接加密方式：auto|ssl|starttls|starttls_opportunistic|none，
// auto 维持按端口推断的历史行为。
var SMTPTLSMode = "auto"

var ChatImageRequestProxy = ""

var GitHubProxy = ""
var GitHubClientId = ""
var GitHubClientSecret = ""
var GitHubOldIdCloseEnabled = false

var LarkClientId = ""
var LarkClientSecret = ""

var WeChatServerAddress = ""
var WeChatServerToken = ""
var WeChatAccountQRCodeImageURL = ""

var TurnstileSiteKey = ""
var TurnstileSecretKey = ""

var OIDCClientId = ""
var OIDCClientSecret = ""
var OIDCIssuer = ""
var OIDCScopes = ""
var OIDCUsernameClaims = ""
var OIDCDisplayName = ""

var LinuxDoClientId = ""
var LinuxDoClientSecret = ""
var LinuxDoOAuthLowestTrustLevel = 1

var QuotaForNewUser = 0

// 邀请奖励总开关：关闭后前端不再展示邀请奖励入口；注册时的邀请码/aff 行为不受影响。
var InviteRewardEnabled = true
var QuotaForInviter = 0
var QuotaForInvitee = 0
var InviterRewardType = "fixed" // "fixed" 或 "percentage"
var InviterRewardValue = 0
var ChannelDisableThreshold = 5.0
var AutomaticDisableChannelEnabled = false
var AutomaticEnableChannelEnabled = false
var AutomaticDisableChannelNotifyEnabled = true
var QuotaRemindThreshold = 1000
var PreConsumedQuota = 500
var ApproximateTokenEnabled = false
var EmptyResponseBillingEnabled = true

// MaxPromptTokens 输入 token 上限的粗粒度守卫（仅对 AWS/Bedrock 渠道生效）。
// AWS 对超过模型上下文窗口（尤其 >1M）的请求既不快速报错也不拒绝，会一直挂起直到
// 墙钟超时才被砍掉（表现为长时间等待后中断、计费 $0）。在发送上游前用本值预拦截，
// 直接返回明确的 400。有效上限优先取 model_info.ContextLength(>0)，否则回落到本值。
// 注意 ContextLength 是整个上下文窗口（含输出侧），这里直接当输入上限使用，不为输出
// 预留 headroom——作为"防挂死"守卫偏宽松、不会误杀，足够。设为 0 可禁用该守卫。
var MaxPromptTokens = 1000000
var DisableTokenEncoders = false
var RetryTimes = 0
var RetryTimeOut = 10

// ChannelFailErrorWrapEnabled 是否启用"渠道失败统一封装"。
// 开启（默认）：FilterOpenAIErr 把所有非 400 上游错误坍缩为 503 + ChannelFailErrorMessage，
//
//	对客户端隐藏上游身份、key 状态等内部信息。
//
// 关闭：跳过坍缩，上游错误原样透传（仍走 request id 拼接 / Type 隐藏等轻度规整）。
//
//	给运维一个"临时关掉看上游真实错误"的口子，便于调试。
var ChannelFailErrorWrapEnabled = true

// ChannelFailErrorMessage 返回给客户端的统一上游错误文案。
// 仅在 ChannelFailErrorWrapEnabled 为 true 时生效；留空时回退到 DefaultChannelFailErrorMessage。
// 通过 GetChannelFailErrorMessage() 取值。
const DefaultChannelFailErrorMessage = "Upstream for the current group is at capacity, please try again later"

var ChannelFailErrorMessage = DefaultChannelFailErrorMessage

// UpstreamAuthFailedErrorMessage 上游返回 401/403（渠道 key 缺失、失效或无权限）时返回给客户端的文案。
// 与 ChannelFailErrorMessage 同样只在 ChannelFailErrorWrapEnabled 为 true 时生效，不含上游原文。
const UpstreamAuthFailedErrorMessage = "Upstream channel authentication failed, please ask the admin to check the channel key"

// GetChannelFailErrorMessage 返回当前配置的统一错误文案；
// 运维在管理后台清空（空串或纯空白）时回退到默认值，避免客户端收到空 message。
func GetChannelFailErrorMessage() string {
	if strings.TrimSpace(ChannelFailErrorMessage) == "" {
		return DefaultChannelFailErrorMessage
	}
	return ChannelFailErrorMessage
}

// CatalogEnforceHidden 读取模型目录的隐藏约束开关（catalog.enforce_hidden，默认开）：
// 开启时被管理员隐藏的模型不进公开列表，经 relay 请求返回 404；
// 关闭后隐藏不再生效，列表与 relay 退回「所有可路由模型」，仅用于应急回滚。
// 未设置时沿用旧开关 catalog.enforce_published 的值，两者都未设置时默认开。
func CatalogEnforceHidden() bool {
	if viper.IsSet("catalog.enforce_hidden") {
		return viper.GetBool("catalog.enforce_hidden")
	}
	if viper.IsSet("catalog.enforce_published") {
		return viper.GetBool("catalog.enforce_published")
	}
	return true
}

// 统一请求响应模型（响应中显示用户请求的原始模型名称）
var UnifiedRequestResponseModelEnabled = false

// FingerprintPassThroughEnabled 让中转响应尽量保留上游的响应指纹：Claude / Bedrock 的
// 非流式原始字节透传、流式跳过 model 改写，以及上游响应头透传（Bedrock x-amzn-* /
// Claude anthropic-ratelimit-* / OpenAI x-ratelimit-* 等）。
// 默认开启；关闭后回退到与其它渠道一致的结构体序列化行为。
var FingerprintPassThroughEnabled = true

// 模型名称大小写不敏感匹配
var ModelNameCaseInsensitiveEnabled = false

var DefaultChannelWeight = uint(1)
var RetryCooldownSeconds = 5

// ModelNotFoundCooldownSeconds 是上游对某模型返回"模型不存在 / 无可用端点"类错误时，
// 该 (渠道, 模型) 对的自动隔离时长（秒）。默认 6 小时；设为 0 关闭自动隔离。
var ModelNotFoundCooldownSeconds = 21600

// RetryCooldownPerStatus stores the JSON source of per-status cooldown overrides,
// e.g. {"503":120,"502":60}. The parsed map is held in retryCooldownPerStatusMap
// and accessed via GetRetryCooldownForStatus.
var RetryCooldownPerStatus = ""

var (
	retryCooldownPerStatusMap  = map[int]int{}
	retryCooldownPerStatusLock sync.RWMutex
)

// SetRetryCooldownPerStatusMap replaces the in-memory map. Called by the option
// setter after parsing the JSON payload.
func SetRetryCooldownPerStatusMap(m map[int]int) {
	retryCooldownPerStatusLock.Lock()
	defer retryCooldownPerStatusLock.Unlock()
	retryCooldownPerStatusMap = m
}

// GetRetryCooldownForStatus returns (seconds, configured). configured=false means
// the caller should fall through to RetryCooldownSeconds (or skip cooldown entirely
// depending on the caller's policy).
func GetRetryCooldownForStatus(statusCode int) (int, bool) {
	retryCooldownPerStatusLock.RLock()
	defer retryCooldownPerStatusLock.RUnlock()
	v, ok := retryCooldownPerStatusMap[statusCode]
	return v, ok
}

var CFWorkerImageUrl = ""
var CFWorkerImageKey = ""

var RootUserEmail = ""

var IsMasterNode = true

var RequestInterval time.Duration

var BatchUpdateEnabled = false
var BatchUpdateInterval = 5

var MCP_ENABLE = false

var UPTIMEKUMA_ENABLE = false
var UPTIMEKUMA_DOMAIN = ""
var UPTIMEKUMA_STATUS_PAGE_NAME = ""

// Gemini
var GeminiAPIEnabled = true

// Claude
var ClaudeAPIEnabled = true

// Claude Prompt Caching 自动注入全局兜底开关（渠道 Plugin 可按 on/off 覆盖）。
// 默认关闭：全局关 + 渠道 inherit 时零注入、零行为变化。
var ClaudePromptCachingEnabled = false

const (
	RoleGuestUser    = 0
	RoleCommonUser   = 1
	RoleReliableUser = 3 // 可信的内部员工
	RoleAdminUser    = 10
	RoleRootUser     = 100
)

var RateLimitKeyExpirationDuration = 20 * time.Minute

const (
	UserStatusEnabled  = 1 // don't use 0, 0 is the default value!
	UserStatusDisabled = 2 // also don't use 0
)

// 用户类型(方案 B:组织 = 影子记账账户)
const (
	UserTypeNormal    = 0 // 普通用户
	UserTypeOrgShadow = 1 // 组织影子记账账户,禁止登录,仅作组织积分池记账主体
)

// 组织功能站点级配置
var OrganizationEnabled = true                // 组织功能全局开关,默认开启(未显式设置过该选项的部署升级后即开启)
var OrganizationMaxPerUser = 3                // 每用户可创建的组织数量
var OrganizationDefaultMaxMembers = 10        // 组织默认成员上限,0 表示不限制
var OrganizationDissolveQuotaRefund = "owner" // 解散时剩余积分处理策略:owner=退回 Owner 个人账户,discard=不退回
var OrganizationUsageVisibleDefault = true    // 成员用量可见性默认值(Member 是否可见全员用量)
var OrganizationQuotaTransferEnabled = true   // 个人积分转入组织池开关(单向,默认开启)

// 令牌周期配额重置的站点级时间设置(TK8a)
var QuotaResetTimezone = "UTC"     // 周期边界计算所用时区(IANA 名,默认 UTC);无效值回退 UTC 并记一次 SysError
var QuotaResetWeekStart = "monday" // weekly 周期的一周起始日:monday(默认)或 sunday

// 品牌图标站点级配置
var BrandIconSyncEnabled = true                          // 后台定时从 npm registry 同步 lobehub 图标库
var BrandIconRegistry = "https://registry.npmmirror.com" // 同步所用 npm registry
var BrandIconFaviconFetchEnabled = true                  // 为未知厂商抓取站点 favicon

const (
	TokenStatusEnabled   = 1 // don't use 0, 0 is the default value!
	TokenStatusDisabled  = 2 // also don't use 0
	TokenStatusExpired   = 3
	TokenStatusExhausted = 4
)

const (
	RedemptionCodeStatusEnabled  = 1 // don't use 0, 0 is the default value!
	RedemptionCodeStatusDisabled = 2 // also don't use 0
	RedemptionCodeStatusUsed     = 3 // also don't use 0
)

const (
	ChannelStatusUnknown          = 0
	ChannelStatusEnabled          = 1 // don't use 0, 0 is the default value!
	ChannelStatusManuallyDisabled = 2 // also don't use 0
	ChannelStatusAutoDisabled     = 3
)

const (
	ChannelTypeUnknown = 0
	ChannelTypeOpenAI  = 1
	// ChannelTypeAPI2D          = 2
	ChannelTypeAzure = 3
	// ChannelTypeCloseAI = 4
	// ChannelTypeOpenAISB       = 5
	// ChannelTypeOpenAIMax      = 6
	// ChannelTypeOhMyGPT        = 7
	ChannelTypeCustom = 8
	// ChannelTypeAILS           = 9
	// ChannelTypeAIProxy        = 10
	ChannelTypePaLM = 11
	// ChannelTypeAPI2GPT        = 12
	// ChannelTypeAIGC2D         = 13
	ChannelTypeAnthropic  = 14
	ChannelTypeBaidu      = 15
	ChannelTypeZhipu      = 16
	ChannelTypeAli        = 17
	ChannelTypeXunfei     = 18
	ChannelType360        = 19
	ChannelTypeOpenRouter = 20
	// ChannelTypeAIProxyLibrary = 21
	// ChannelTypeFastGPT        = 22
	ChannelTypeTencent         = 23
	ChannelTypeAzureSpeech     = 24
	ChannelTypeGemini          = 25
	ChannelTypeBaichuan        = 26
	ChannelTypeMiniMax         = 27
	ChannelTypeDeepseek        = 28
	ChannelTypeMoonshot        = 29
	ChannelTypeMistral         = 30
	ChannelTypeGroq            = 31
	ChannelTypeBedrock         = 32
	ChannelTypeLingyi          = 33
	ChannelTypeMidjourney      = 34
	ChannelTypeCloudflareAI    = 35
	ChannelTypeCohere          = 36
	ChannelTypeStabilityAI     = 37
	ChannelTypeCoze            = 38
	ChannelTypeOllama          = 39
	ChannelTypeHunyuan         = 40
	ChannelTypeSuno            = 41
	ChannelTypeVertexAI        = 42
	ChannelTypeLLAMA           = 43
	ChannelTypeIdeogram        = 44
	ChannelTypeSiliconflow     = 45
	ChannelTypeFlux            = 46
	ChannelTypeJina            = 47
	ChannelTypeRerank          = 48
	ChannelTypeGithub          = 49
	ChannelTypeRecraft         = 51
	ChannelTypeReplicate       = 52
	ChannelTypeKling           = 53
	ChannelTypeAzureDatabricks = 54
	ChannelTypeAzureV1         = 55
	ChannelTypeXAI             = 56
	ChannelTypeGeminiCli       = 57
	ChannelTypeClaudeCode      = 58
	ChannelTypeCodex           = 59
	ChannelTypeAntigravity     = 60
	ChannelTypeVertexAIExpress = 61
)

const (
	RelayModeUnknown = iota
	RelayModeChatCompletions
	RelayModeCompletions
	RelayModeEmbeddings
	RelayModeModerations
	RelayModeImagesGenerations
	RelayModeImagesEdits
	RelayModeImagesVariations
	RelayModeEdits
	RelayModeAudioSpeech
	RelayModeAudioTranscription
	RelayModeAudioTranslation
	RelayModeSuno
	RelayModeRerank
	RelayModeChatRealtime
	RelayModeKling
	RelayModeResponses
)

type ContextKey string

// linux do 用户信任等级
const (
	Basic   = 1 // 基础用户
	Member  = 2 // 会员
	Regular = 3 // 活跃用户
	Leader  = 4 // 领导者
)
