package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/oidc"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

// oidcAllowedUILocales 允许透传给 IdP 的 ui_locales 取值（OIDC Core 3.1.2.1 的 BCP47 语言标签），
// 与前端 web/src/i18n/uiLocale.js 的映射结果一一对应。其余取值一律忽略，不拼进跳转 URL。
var oidcAllowedUILocales = map[string]bool{
	"zh-CN": true,
	"zh-HK": true,
	"en":    true,
	"ja":    true,
}

// oidcUILocale 取请求里的 ui_locales，非法或缺省时返回空串（调用方据此不追加该参数）。
func oidcUILocale(c *gin.Context) string {
	locale := c.Query("ui_locales")
	if oidcAllowedUILocales[locale] {
		return locale
	}
	return ""
}

// acceptLanguageUILocale 取 Accept-Language 的首个语言标签，按与前端 web/src/i18n/uiLocale.js
// 相同的规则映射成 BCP47 值再过白名单；无法映射时返回空串（调用方据此不追加该参数）。
// 仅服务端直达 IdP 时使用：那条路径上没有前端，读不到界面语言，只能以浏览器偏好为准。
func acceptLanguageUILocale(header string) string {
	tag := strings.SplitN(header, ",", 2)[0]
	tag = strings.ToLower(strings.TrimSpace(strings.SplitN(tag, ";", 2)[0]))
	locale := ""
	switch {
	case tag == "zh-cn" || strings.HasPrefix(tag, "zh-hans"):
		locale = "zh-CN"
	case tag == "zh-hk" || tag == "zh-tw" || strings.HasPrefix(tag, "zh-hant"):
		locale = "zh-HK"
	case tag == "en" || strings.HasPrefix(tag, "en-"):
		locale = "en"
	case tag == "ja" || strings.HasPrefix(tag, "ja-"):
		locale = "ja"
	}
	if oidcAllowedUILocales[locale] {
		return locale
	}
	return ""
}

// oidcSlugFromContext 取路由上的 :slug；无 slug 路由（/oauth/oidc、/oauth/endpoint）
// 落到存量别名 slug=oidc。
func oidcSlugFromContext(c *gin.Context) string {
	slug := c.Param("slug")
	if slug == "" {
		return model.LegacyOidcProviderSlug
	}
	return slug
}

// oidcUnavailableMessage 配置类失败的对外文案：登录页不该出现「OIDC」/ slug 等技术词，
// 技术细节由调用处记日志。
const oidcUnavailableMessage = "This sign-in method is currently unavailable, please contact the admin"

// oidcUnavailable 记下技术细节后返回统一的对外错误。
func oidcUnavailable(detail string) error {
	logger.SysError("OIDC login unavailable: " + detail)
	return errors.New(oidcUnavailableMessage)
}

// resolveOidcProvider 按 slug 取出已启用的提供方。每次请求现查库，后台改配置立即生效。
// slug 不存在、未启用或配置不全都返回统一的对外错误，具体原因只进日志。
func resolveOidcProvider(c *gin.Context) (*model.OidcProvider, error) {
	// 内置账号模式下提供方不参与登录：即便表里还有启用的行，也不接受授权 / 回调请求。
	if !config.IsExternalAccountSystem() {
		return nil, oidcUnavailable("Account system is builtin, external identity providers are not enabled")
	}
	slug := oidcSlugFromContext(c)
	provider, err := model.GetOidcProviderBySlug(slug)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, oidcUnavailable("Identity provider " + slug + " does not exist")
		}
		return nil, err
	}
	if !provider.Enabled {
		return nil, oidcUnavailable("Identity provider " + slug + " is not enabled")
	}
	if provider.Issuer == "" || provider.ClientId == "" {
		return nil, oidcUnavailable("Identity provider " + slug + " is not fully configured")
	}
	return provider, nil
}

// oidcProviderLabel 取面向用户展示的提供方名称；未配置显示名时退回 slug。
func oidcProviderLabel(provider *model.OidcProvider) string {
	if provider.DisplayName != "" {
		return provider.DisplayName
	}
	return provider.Slug
}

// oidcFailureAdvice 失败提示的收尾建议。密码登录关闭时站内已无「原方式」可用，
// 再让用户去登录后手动绑定就是死路，改为引导联系管理员。
func oidcFailureAdvice() string {
	if config.EffectivePasswordLogin() {
		return "please sign in with your original method first, then link it manually on the \"Linked accounts\" page"
	}
	return "please contact the admin"
}

// oidcLinkConflictMessage 邮箱关联冲突的下发文案。first_party 提供方即本站账号体系，
// 文案里不出现提供方名称，避免向用户暴露背后的 IdP。
func oidcLinkConflictMessage(provider *model.OidcProvider) string {
	detail := "The account for this email is already linked to another sign-in identity"
	if !provider.FirstParty {
		detail = "The account for this email is already linked to another " + oidcProviderLabel(provider) + " identity"
	}
	return "OIDC_LINK_CONFLICT:" + detail + ", " + oidcFailureAdvice()
}

// oidcRegisterDisabledMessage 提供方禁止首登自动建号时的下发文案，同样按 first_party 隐藏提供方名称。
func oidcRegisterDisabledMessage(provider *model.OidcProvider) string {
	detail := "This sign-in identity is not linked to an account on this site"
	if !provider.FirstParty {
		detail = "This " + oidcProviderLabel(provider) + " account is not linked to an account on this site"
	}
	return "OIDC_REGISTER_DISABLED:" + detail + ", " + oidcFailureAdvice()
}

// oidcBindTakenMessage 登录态绑定时该身份已归属其它账号的下发文案，
// 同样按 first_party 隐藏提供方名称。
func oidcBindTakenMessage(provider *model.OidcProvider) string {
	if provider.FirstParty {
		return "This sign-in identity is already linked to another account"
	}
	return "This " + oidcProviderLabel(provider) + " account is already linked to another account"
}

// oidcStringClaim 安全取出字符串 claim：claim 名为空、claim 缺失或值不是字符串时返回空串。
// IdP 侧可下发任意类型，直接类型断言会 panic。
func oidcStringClaim(claims map[string]interface{}, name string) string {
	if name == "" {
		return ""
	}
	value, ok := claims[name]
	if !ok || value == nil {
		return ""
	}
	str, ok := value.(string)
	if !ok {
		return ""
	}
	return str
}

// oidcAuthorizeURL 构造该提供方的授权地址，并把一次性 state 写入会话（回调时校验）。
// uiLocales 非空时作为 ui_locales 参数带上；access_type=offline 由 oidc.Get 的 LoginURL 统一附加。
// 供 /api/oauth/endpoint 与服务端 /login 直达共用，两条路径的授权 URL 逐字节一致。
func oidcAuthorizeURL(c *gin.Context, provider *model.OidcProvider, uiLocales string) (string, error) {
	oidcConfig, err := oidc.Get(c.Request.Context(), provider)
	if err != nil {
		logger.SysError("Failed to get OIDC config, err: " + err.Error())
		return "", errors.New(oidcUnavailableMessage)
	}

	session := sessions.Default(c)
	state := utils.GetSecureRandomString(32)
	// PKCE(S256)与 nonce 与 state 一起存进会话：code_verifier 在换 token 时提交，nonce 在校验 id_token 时比对。
	// 不支持 PKCE 的 IdP 会忽略这两个授权参数，不影响兼容。
	verifier := oauth2.GenerateVerifier()
	nonce := utils.GetSecureRandomString(32)
	session.Set("oauth_state", state)
	session.Set(oidcPkceSessionKey, verifier)
	session.Set(oidcNonceSessionKey, nonce)
	authOptions := []oauth2.AuthCodeOption{oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("nonce", nonce)}
	if uiLocales != "" {
		authOptions = append(authOptions, oauth2.SetAuthURLParam("ui_locales", uiLocales))
	}
	loginURL := oidcConfig.LoginURL(state, authOptions...)
	if err = session.Save(); err != nil {
		return "", err
	}
	return loginURL, nil
}

func OIDCEndpoint(c *gin.Context) {
	provider, err := resolveOidcProvider(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}

	loginURL, err := oidcAuthorizeURL(c, provider, oidcUILocale(c))
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
		"data":    loginURL,
	})
}

// oidcEmailVerified 判定 email_verified claim 是否为真。部分 IdP 以字符串形式下发该 claim，
// 故同时接受布尔 true 与字符串 "true"；缺失或其他取值一律视为未验证。
func oidcEmailVerified(claims map[string]interface{}) bool {
	switch v := claims["email_verified"].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	default:
		return false
	}
}

// oidcTrustedEmail 取出可信邮箱：email claim 存在、格式合法且 email_verified 为 true 时返回
// 归一化后的邮箱，否则返回空串。只有可信邮箱才允许关联已有账号或回填到新注册账号。
func oidcTrustedEmail(claims map[string]interface{}) string {
	emailStr := common.NormalizeEmail(oidcStringClaim(claims, "email"))
	if emailStr == "" || common.ValidateEmailStrict(emailStr) != nil {
		return ""
	}
	if !oidcEmailVerified(claims) {
		return ""
	}
	return emailStr
}

// maxOidcPhoneNumberLength 与 users.phone_number 的 varchar(32) 对齐：超长值宁可不展示，
// 也不截断成错误号码或触发 MySQL 严格模式下的写入报错。
const maxOidcPhoneNumberLength = 32

// oidcPhoneNumberVerified 判定 phone_number_verified claim 是否为真，
// 与 oidcEmailVerified 同样宽松：同时接受布尔 true 与字符串 "true"。
func oidcPhoneNumberVerified(claims map[string]interface{}) bool {
	switch v := claims["phone_number_verified"].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	default:
		return false
	}
}

// oidcVerifiedPhoneNumber 取出可信手机号：phone_number claim 为非空字符串、长度不超过列宽
// 且 phone_number_verified 为真时返回，否则返回空串（不展示、不覆盖）。
func oidcVerifiedPhoneNumber(claims map[string]interface{}) string {
	phone := strings.TrimSpace(oidcStringClaim(claims, "phone_number"))
	if phone == "" || len([]rune(phone)) > maxOidcPhoneNumberLength {
		return ""
	}
	if !oidcPhoneNumberVerified(claims) {
		return ""
	}
	return phone
}

// syncOidcPhoneNumber 把 IdP 已验证手机号同步到本站账号，写入条件与 syncOidcEmail 逐条一致，
// 只差字段名与查重函数，随提供方身份权威程度分两档：
//   - first_party：该提供方即本站账号体系，手机号以 IdP 为准，本地为空或与 IdP 不同都写入；
//   - 第三方提供方：仅本地手机号为空时回填，本地已有手机号一律不覆盖（哪怕与 IdP 不同），
//     避免第三方静默改掉本站的可关联标识。
//
// 两档都不抢别人的手机号：目标号码已归属其它账号时跳过，否则既是账号接管路径，
// 也会撞 users.phone_number 的唯一索引。IdP 未下发该 claim（phone 为空）时直接返回，
// 不清空本地已有手机号。
// 跳过与写失败都只记 user_id 与 provider slug，手机号明文不入日志，且不影响登录 / 绑定本身。
func syncOidcPhoneNumber(user *model.User, phone string, provider *model.OidcProvider) {
	if user == nil || user.Id == 0 || provider == nil {
		return
	}
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return
	}
	if user.PhoneNumber != "" {
		if !provider.FirstParty {
			return
		}
		if strings.TrimSpace(string(user.PhoneNumber)) == phone {
			return
		}
	}
	if model.IsPhoneAlreadyTaken(phone) {
		logger.SysLog(fmt.Sprintf("OIDC phone number sync skipped: provider=%s user_id=%d phone number already in use by another account", provider.Slug, user.Id))
		return
	}
	if err := model.UpdateUser(user.Id, map[string]interface{}{"phone_number": model.NullablePhone(phone)}); err != nil {
		logger.SysError(fmt.Sprintf("Failed to sync user phone number: provider=%s user_id=%d err=%s", provider.Slug, user.Id, err.Error()))
		return
	}
	user.PhoneNumber = model.NullablePhone(phone)
}

// syncOidcEmail 把 IdP 已验证邮箱同步到本站账号，写入条件随提供方身份权威程度分两档：
//   - first_party：该提供方即本站账号体系，邮箱以 IdP 为准，本地为空或与 IdP 不同都写入；
//   - 第三方提供方：仅本地邮箱为空时回填，本地已有邮箱一律不覆盖（哪怕与 IdP 不同），
//     避免第三方静默改掉本站登录标识。
//
// 两档都不抢别人的邮箱：目标邮箱已归属其它账号（含影子账户）时跳过，否则既是账号接管路径，
// 也会撞 users.email 的唯一约束。
// 跳过与写失败都只记 user_id 与 provider slug，邮箱明文不入日志，且不影响登录 / 绑定本身。
func syncOidcEmail(user *model.User, email string, provider *model.OidcProvider) {
	if user == nil || user.Id == 0 || provider == nil {
		return
	}
	email = common.NormalizeEmail(email)
	if email == "" {
		return
	}
	if user.Email != "" {
		if !provider.FirstParty {
			return
		}
		if common.NormalizeEmail(string(user.Email)) == email {
			// 地址没变，但 IdP 刚刚证实它已验证：本地若还记着「未验证」（如这个邮箱当初是
			// 管理员代填的），就地升级，别让角标与自动关联一直停在错误的状态。
			if !user.EmailVerified {
				if err := model.UpdateUser(user.Id, map[string]interface{}{"email_verified": true}); err != nil {
					logger.SysError(fmt.Sprintf("Failed to mark user email as verified: provider=%s user_id=%d err=%s", provider.Slug, user.Id, err.Error()))
					return
				}
				user.EmailVerified = true
			}
			return
		}
	}
	if model.IsEmailAlreadyTaken(email) {
		logger.SysLog(fmt.Sprintf("OIDC email sync skipped: provider=%s user_id=%d email already in use by another account", provider.Slug, user.Id))
		return
	}
	// 写进来的是 IdP 的已验证邮箱（oidcTrustedEmail 已核过 email_verified claim），
	// 故与地址一并记为已验证。
	if err := model.UpdateUser(user.Id, map[string]interface{}{
		"email":          model.NullableEmail(email),
		"email_verified": true,
	}); err != nil {
		logger.SysError(fmt.Sprintf("Failed to sync user email: provider=%s user_id=%d err=%s", provider.Slug, user.Id, err.Error()))
		return
	}
	user.Email = model.NullableEmail(email)
	user.EmailVerified = true
}

// maxOidcSnapshotLength 与 user_oidc_identities 的 varchar(255) 对齐：超长值宁可不展示，
// 也不截断成错误信息或触发 MySQL 严格模式下的写入报错。
const maxOidcSnapshotLength = 255

// oidcIdentitySnapshot 取展示用的 IdP 账户信息：用户名按提供方配置的用户名 claim（默认
// preferred_username），缺失时退回显示名 claim（默认 name）；邮箱取 email claim 原样
// （不要求 verified——只用于展示，不参与任何匹配）。
func oidcIdentitySnapshot(provider *model.OidcProvider, claims map[string]interface{}) (string, string) {
	usernameClaim, displayNameClaim := "preferred_username", "name"
	if provider != nil && provider.UsernameClaim != "" {
		usernameClaim = provider.UsernameClaim
	}
	if provider != nil && provider.DisplayNameClaim != "" {
		displayNameClaim = provider.DisplayNameClaim
	}
	username := strings.TrimSpace(oidcStringClaim(claims, usernameClaim))
	if username == "" {
		username = strings.TrimSpace(oidcStringClaim(claims, displayNameClaim))
	}
	email := strings.TrimSpace(oidcStringClaim(claims, "email"))
	if len([]rune(username)) > maxOidcSnapshotLength {
		username = ""
	}
	if len([]rune(email)) > maxOidcSnapshotLength {
		email = ""
	}
	return username, email
}

// refreshOidcIdentitySnapshot 把最新 claim 写回身份行的展示快照。失败只记日志：
// 展示信息过期不该拖垮登录 / 绑定本身，用户名与邮箱明文不入日志。
func refreshOidcIdentitySnapshot(provider *model.OidcProvider, subject string, claims map[string]interface{}) {
	username, email := oidcIdentitySnapshot(provider, claims)
	if err := model.UpdateUserOidcIdentitySnapshot(provider.Id, subject, username, email); err != nil {
		logger.SysError(fmt.Sprintf("Failed to refresh OIDC identity display info: provider=%s subject=%s err=%s", provider.Slug, subject, err.Error()))
	}
}

// oidcLinkOutcome 描述按身份行 / 已验证邮箱 / 已验证手机号关联已有账号的结果。
type oidcLinkOutcome int

const (
	// oidcLinkLogin 命中可用账号，可直接登录。
	oidcLinkLogin oidcLinkOutcome = iota
	// oidcLinkDisabled 命中账号但已被封禁。
	oidcLinkDisabled
	// oidcLinkNone 未关联到任何账号，调用方可继续注册流程。
	oidcLinkNone
	// oidcLinkError 真实 DB 错误，须中止。
	oidcLinkError
	// oidcLinkConflict 已验证标识命中的账号在同一提供方下已绑定另一个 subject，拒绝自动关联。
	oidcLinkConflict
)

// resolveOidcUser 按 (provider, subject) 查身份行；未命中时依次尝试按已验证邮箱
// （link_by_verified_email）、按已验证手机号（link_by_verified_phone）关联已有账号
// （FillUserByEmail / FillUserByPhoneNumber 均已排除影子账户）并补写身份行。
// 都未命中返回 oidcLinkNone，由调用方走注册。身份行按 provider 隔离：同一 subject
// 在不同提供方是两个身份。
//
// 邮箱优先于手机号：邮箱是本站既有的主登录标识，两者都命中且指向不同账号时，
// 按邮箱关联的结果更贴近用户预期，也与升级前的行为一致。
//
// 关联分支额外要求目标账号在该提供方下尚无任何身份行：已有身份行说明同一
// IdP 下换了 subject（账号重建、租户迁移或 IdP 侧标识复用），自动补写第二条身份
// 会把陌生 subject 静默接入已有账号，故返回 oidcLinkConflict 并要求用户登录后
// 在「账号绑定」页显式绑定（oidcBind 仍允许同提供方多个 subject）。
func resolveOidcUser(provider *model.OidcProvider, subject string, trustedEmail string, trustedPhone string) (*model.User, oidcLinkOutcome, error) {
	identity, err := model.FindUserOidcIdentity(provider.Id, subject)
	if err == nil {
		user := model.User{Id: identity.UserId}
		if err := user.FillUserById(); err != nil {
			if errors.Is(err, model.ErrUserNotFound) {
				return nil, oidcLinkNone, nil
			}
			return nil, oidcLinkError, err
		}
		if user.Status != config.UserStatusEnabled {
			return &user, oidcLinkDisabled, nil
		}
		return &user, oidcLinkLogin, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, oidcLinkError, err
	}

	existingUser, err := lookupOidcLinkTarget(provider, trustedEmail, trustedPhone)
	if err != nil {
		return nil, oidcLinkError, err
	}
	if existingUser == nil {
		return nil, oidcLinkNone, nil
	}
	if existingUser.Status != config.UserStatusEnabled {
		return existingUser, oidcLinkDisabled, nil
	}
	if _, err := model.FindUserOidcIdentityByUserAndProvider(existingUser.Id, provider.Id); err == nil {
		return existingUser, oidcLinkConflict, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, oidcLinkError, err
	}
	newIdentity := model.UserOidcIdentity{
		UserId:     existingUser.Id,
		ProviderId: provider.Id,
		Subject:    subject,
	}
	if err := newIdentity.Insert(); err != nil {
		return nil, oidcLinkError, err
	}
	return existingUser, oidcLinkLogin, nil
}

// lookupOidcLinkTarget 按提供方开启的已验证标识查可关联账号：先邮箱后手机号，
// 任一命中即返回。两个开关都关闭、或标识为空、或查不到账号时返回 (nil, nil)。
// 手机号上有唯一索引，命中至多一个账号，故与邮箱一样不存在"关联到哪一个"的歧义。
//
// 「已验证」是双向要求：IdP 侧的 claim 要已验证（trustedEmail / trustedPhone 由调用方核过），
// 本站侧命中的账号也要已验证——邮箱看 users.email_verified，手机号只经 IdP 已验证 claim 写入，
// 恒为已验证。
func lookupOidcLinkTarget(provider *model.OidcProvider, trustedEmail string, trustedPhone string) (*model.User, error) {
	if provider.LinkByVerifiedEmail && trustedEmail != "" {
		user := model.User{Email: model.NullableEmail(trustedEmail)}
		err := user.FillUserByEmail()
		if err == nil {
			// 目标账号自己的邮箱也必须是已验证的：管理员代填或其它未经验证写入的邮箱
			// 不得成为登录入口——否则在某个账号上填一个别人的邮箱，就能凭该邮箱的 IdP
			// 身份登进这个账号。未验证按「未命中」处理，继续走后面的手机号规则 / 注册。
			if user.EmailVerified {
				return &user, nil
			}
			logger.SysLog(fmt.Sprintf("OIDC email linking skipped: provider=%s user_id=%d account email is not verified", provider.Slug, user.Id))
		} else if !errors.Is(err, model.ErrUserNotFound) {
			return nil, err
		}
	}
	if provider.LinkByVerifiedPhone && trustedPhone != "" {
		user := model.User{PhoneNumber: model.NullablePhone(trustedPhone)}
		err := user.FillUserByPhoneNumber()
		if err == nil {
			return &user, nil
		}
		if !errors.Is(err, model.ErrUserNotFound) {
			return nil, err
		}
	}
	return nil, nil
}

// signedOutPath IdP 结束会话后回跳的本站落地页，须在 IdP 侧登记为 Post Logout Redirect URI。
const signedOutPath = "/signed-out"

// oidcSetupLogin 建立本地会话，会话行上记下提供方与原始 id_token（供退出时结束 IdP 会话）。
func oidcSetupLogin(c *gin.Context, provider *model.OidcProvider, user *model.User, rawIdToken string) {
	setupLoginSession(user, c, loginSessionInfo{method: model.SessionMethodOidc, provider: provider, rawIdToken: rawIdToken})
}

// oidcEndSessionURL 按当前会话构造 IdP 结束会话地址；不是经身份提供方登录的会话、
// 提供方不支持或未配置时返回空串。会话行本身由 Logout 删除，id_token 随之消失。
func oidcEndSessionURL(c *gin.Context, session sessions.Session) string {
	key, _ := session.Get("sid").(string)
	if key == "" {
		return ""
	}
	record, err := model.GetUserSessionByKey(key)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.SysError("Failed to read session row: " + err.Error())
		}
		return ""
	}
	if record.ProviderId == 0 || record.IdToken == "" {
		return ""
	}
	provider, err := model.GetOidcProviderById(record.ProviderId)
	if err != nil || provider.Issuer == "" {
		return ""
	}
	endpoint, err := oidc.EndSessionEndpoint(c.Request.Context(), provider)
	if err != nil {
		logger.SysError(fmt.Sprintf("Failed to read end_session_endpoint: provider=%s err=%s", provider.Slug, err.Error()))
		return ""
	}
	if endpoint == "" {
		return ""
	}
	return buildEndSessionURL(endpoint, record.IdToken, provider.ClientId, oidcUILocale(c))
}

// buildEndSessionURL 按 RP-Initiated Logout 1.0 拼参数：id_token_hint 让 IdP 免确认页直接登出，
// post_logout_redirect_uri 指回本站已登出页（须在 IdP 侧预先登记）。
// uiLocales 非空时一并带上，让 IdP 侧若出现页面与本站界面语言一致。
// ServerAddress 未配置时无法给出合法回跳地址，直接放弃外跳。
func buildEndSessionURL(endpoint string, rawIdToken string, clientId string, uiLocales string) string {
	serverAddress := strings.TrimSuffix(strings.TrimSpace(config.ServerAddress), "/")
	if serverAddress == "" {
		return ""
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		logger.SysError("Failed to parse end_session_endpoint: " + err.Error())
		return ""
	}
	query := parsed.Query()
	query.Set("id_token_hint", rawIdToken)
	query.Set("post_logout_redirect_uri", serverAddress+signedOutPath)
	if clientId != "" {
		query.Set("client_id", clientId)
	}
	if uiLocales != "" {
		query.Set("ui_locales", uiLocales)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

// oidcIdToken 完成授权码换取与 id_token 校验，返回原始 id_token、subject 与全部 claim。
// 原始 id_token 供退出时作 id_token_hint 使用。失败时已写好响应，调用方直接返回。
// 会话里 PKCE code_verifier 与 nonce 的键名；与 oauth_state 同生命周期，回调校验后即删除。
const (
	oidcPkceSessionKey  = "oauth_pkce"
	oidcNonceSessionKey = "oauth_nonce"
)

// oidcTakeSessionString 取出并删除会话里的一次性字符串（state / verifier / nonce 都只能用一次）。
func oidcTakeSessionString(session sessions.Session, key string) string {
	value, _ := session.Get(key).(string)
	session.Delete(key)
	return value
}

func oidcIdToken(c *gin.Context, provider *model.OidcProvider, verifier string, nonce string) (string, string, map[string]interface{}, bool) {
	oidcConfig, err := oidc.Get(c.Request.Context(), provider)
	if err != nil {
		logger.SysError("Failed to get OIDC config, err: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"message": oidcUnavailableMessage,
			"success": false,
		})
		return "", "", nil, false
	}

	ctx := c.Request.Context()
	var exchangeOptions []oauth2.AuthCodeOption
	if verifier != "" {
		exchangeOptions = append(exchangeOptions, oauth2.VerifierOption(verifier))
	}
	token, err := oidcConfig.OAuth2Config.Exchange(ctx, c.Query("code"), exchangeOptions...)
	if err != nil {
		// 上游原文只进日志：换 token 失败的细节（client 配置、IdP 报错）不该回给浏览器
		logger.SysError(fmt.Sprintf("OIDC token exchange failed: provider=%s err=%s", provider.Slug, err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Login failed, please try again",
		})
		return "", "", nil, false
	}

	rawIdToken, ok := token.Extra("id_token").(string)
	if !ok || rawIdToken == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "IdP did not return an id_token",
		})
		return "", "", nil, false
	}
	idToken, err := oidcConfig.Verifier.Verify(ctx, rawIdToken)
	if err != nil {
		logger.SysError(fmt.Sprintf("OIDC id_token verification failed: provider=%s err=%s", provider.Slug, err.Error()))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Login failed, please try again",
		})
		return "", "", nil, false
	}
	if nonce != "" && idToken.Nonce != nonce {
		logger.SysError(fmt.Sprintf("OIDC nonce mismatch: provider=%s", provider.Slug))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Login failed, please try again",
		})
		return "", "", nil, false
	}
	if idToken.Subject == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "ID token has no subject",
		})
		return "", "", nil, false
	}

	claims := make(map[string]interface{})
	if err := idToken.Claims(&claims); err != nil {
		c.String(http.StatusBadRequest, "Failed to parse claims: %v", err)
		return "", "", nil, false
	}
	return rawIdToken, idToken.Subject, claims, true
}

// OIDCAuth 通过 OIDC 登录 / 注册 / 绑定，提供方由路由 :slug 决定（缺省 slug=oidc）。
// 关联顺序：先按 (provider_id, subject) 查身份行，命中即登录（遵循用户禁用条件）；
// 未命中且该提供方开启 link_by_verified_email 且 id_token 携带已验证邮箱时，按邮箱
// 关联已有账号并补写身份行；若该账号在同一提供方下已绑定别的 subject 则拒绝关联，
// 返回 OIDC_LINK_CONFLICT 让用户改走登录后手动绑定；未关联到账号且该提供方开启
// disable_auto_register 时返回 OIDC_REGISTER_DISABLED，否则注册新用户（站点关闭注册时返回
// OIDC_REGISTER_CLOSED）。
// 注意：绝不按 username claim 关联已有账号——IdP 侧可任意设置该 claim，会形成账号接管路径。
// 会话中已有登录用户时走绑定分支（与 GitHub 等既有 OAuth 一致）。
func OIDCAuth(c *gin.Context) {
	session := sessions.Default(c)
	state := c.Query("state")
	if state == "" || session.Get("oauth_state") == nil || state != session.Get("oauth_state").(string) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "state is empty or not same",
		})
		return
	}
	// state / code_verifier / nonce 一次性使用：校验通过后立即失效，防止重放
	session.Delete("oauth_state")
	verifier := oidcTakeSessionString(session, oidcPkceSessionKey)
	nonce := oidcTakeSessionString(session, oidcNonceSessionKey)
	_ = session.Save()

	provider, err := resolveOidcProvider(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}

	rawIdToken, subject, claims, ok := oidcIdToken(c, provider, verifier, nonce)
	if !ok {
		return
	}

	if session.Get("id") != nil {
		oidcBind(c, provider, subject, claims)
		return
	}

	trustedEmail := oidcTrustedEmail(claims)
	trustedPhone := oidcVerifiedPhoneNumber(claims)
	existingUser, outcome, err := resolveOidcUser(provider, subject, trustedEmail, trustedPhone)
	switch outcome {
	case oidcLinkLogin:
		// 覆盖身份行命中登录与按已验证邮箱 / 手机号关联三条路径
		refreshOidcIdentitySnapshot(provider, subject, claims)
		syncOidcPhoneNumber(existingUser, trustedPhone, provider)
		syncOidcEmail(existingUser, trustedEmail, provider)
		oidcSetupLogin(c, provider, existingUser, rawIdToken)
		return
	case oidcLinkDisabled:
		c.JSON(http.StatusOK, gin.H{
			"message": "User is banned or does not exist",
			"success": false,
		})
		return
	case oidcLinkConflict:
		// 只记 provider slug / 目标用户 id / subject，邮箱与手机号明文不入日志
		logger.SysError(fmt.Sprintf("OIDC verified identifier link conflict: provider=%s user_id=%d subject=%s account already linked to another identity under this provider", provider.Slug, existingUser.Id, subject))
		c.JSON(http.StatusOK, gin.H{
			"message": oidcLinkConflictMessage(provider),
			"success": false,
		})
		return
	case oidcLinkError:
		logger.SysError("Failed to query user: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}

	oidcRegisterOrReject(c, provider, rawIdToken, subject, claims, trustedEmail)
}

// oidcRegisterOrReject 未关联到任何已有账号时的收口：提供方开启
// disable_auto_register 则拒绝首登建号，否则照常注册。
// 只记 provider slug 与 subject，邮箱 / 手机号明文不入日志。
func oidcRegisterOrReject(c *gin.Context, provider *model.OidcProvider, rawIdToken string, subject string, claims map[string]interface{}, trustedEmail string) {
	if provider.DisableAutoRegister {
		logger.SysLog(fmt.Sprintf("OIDC auto sign-up on first login is disabled: provider=%s subject=%s", provider.Slug, subject))
		c.JSON(http.StatusOK, gin.H{
			"message": oidcRegisterDisabledMessage(provider),
			"success": false,
		})
		return
	}
	oidcRegister(c, provider, rawIdToken, subject, claims, trustedEmail)
}

// oidcBindOutcome 描述登录态绑定的判定结果。
type oidcBindOutcome int

const (
	// oidcBindOK 绑定成功（含已归属自己的幂等成功）。
	oidcBindOK oidcBindOutcome = iota
	// oidcBindDenied 会话用户已不存在（含软删）或被封禁。
	oidcBindDenied
	// oidcBindTaken 该身份已归属其它账号。
	oidcBindTaken
	// oidcBindError 真实 DB 错误，须中止。
	oidcBindError
)

// bindOidcIdentity 先校验会话用户在 users 表中仍存在（未软删）且已启用，再把
// (provider, subject) 绑定到该账号。校验不过直接拒绝，既不查身份行也不写行——
// 会话可能签发于封禁 / 删除之前，仅凭 session 里的 id 写身份行会给失效账号留下入口。
func bindOidcIdentity(providerId int, userId int, subject string) (oidcBindOutcome, error) {
	// IsUserEnabled 走 Find：软删用户在默认 scope 下查不到，status 为零值同样判为不启用
	enabled, err := model.IsUserEnabled(userId)
	if err != nil {
		return oidcBindError, err
	}
	if !enabled {
		return oidcBindDenied, nil
	}

	identity, err := model.FindUserOidcIdentity(providerId, subject)
	if err == nil {
		if identity.UserId != userId {
			return oidcBindTaken, nil
		}
		return oidcBindOK, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return oidcBindError, err
	}

	newIdentity := model.UserOidcIdentity{
		UserId:     userId,
		ProviderId: providerId,
		Subject:    subject,
	}
	if err := newIdentity.Insert(); err != nil {
		return oidcBindError, err
	}
	return oidcBindOK, nil
}

// oidcBind 登录态回调：把 (provider, subject) 绑定到当前账号。
// 会话用户已失效时拒绝；该身份已归属其它账号时拒绝；已归属当前账号则视作幂等成功。
func oidcBind(c *gin.Context, provider *model.OidcProvider, subject string, claims map[string]interface{}) {
	session := sessions.Default(c)
	userId, ok := session.Get("id").(int)
	if !ok || userId == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Unable to read the currently signed-in user",
		})
		return
	}

	outcome, err := bindOidcIdentity(provider.Id, userId, subject)
	switch outcome {
	case oidcBindDenied:
		// 只记 provider slug / user_id / subject，邮箱明文不入日志
		logger.SysError(fmt.Sprintf("OIDC link rejected: provider=%s user_id=%d subject=%s session user does not exist or is banned", provider.Slug, userId, subject))
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "User is banned or does not exist",
		})
		return
	case oidcBindTaken:
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": oidcBindTakenMessage(provider),
		})
		return
	case oidcBindError:
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	refreshOidcIdentitySnapshot(provider, subject, claims)

	// 绑定成功后同步 IdP 已验证标识：手机号与邮箱同一口径——对 first_party 提供方跟随 IdP 覆盖、
	// 对第三方提供方仅在本地为空时回填，两者都不抢已归属其它账号的值。
	// 两者复用同一次 GetUserById，读失败只是不同步，不影响绑定结果。
	phone := oidcVerifiedPhoneNumber(claims)
	trustedEmail := oidcTrustedEmail(claims)
	if phone != "" || trustedEmail != "" {
		if user, err := model.GetUserById(userId, false); err == nil {
			syncOidcPhoneNumber(user, phone, provider)
			syncOidcEmail(user, trustedEmail, provider)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "bind",
	})
}

// oidcRegister 注册新用户并写入身份行。username / display_name / avatar 按提供方配置的
// claim 名取值，取不到就退回兜底用户名。
func oidcRegister(c *gin.Context, provider *model.OidcProvider, rawIdToken string, subject string, claims map[string]interface{}, trustedEmail string) {
	if !config.RegisterEnabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "OIDC_REGISTER_CLOSED:New user sign-up is currently closed on this site, " + oidcFailureAdvice(),
		})
		return
	}

	var inviterId int
	if affCode := c.Query("aff"); affCode != "" {
		inviterId, _ = model.GetUserIdByAffCode(affCode)
	}

	user := model.User{
		Role:   config.RoleCommonUser,
		Status: config.UserStatusEnabled,
	}
	// username claim 常见形态就是邮箱，直接落库会造出无法用用户名登录的账号
	user.Username = oauthUsername(oidcStringClaim(claims, provider.UsernameClaim), "oidc")
	// 仅回填可信邮箱，且邮箱未被占用（含影子账户）；未验证或已占用时新账号不带邮箱。
	// 回填的就是 IdP 的已验证邮箱，故一并记为已验证。
	if trustedEmail != "" && !model.IsEmailAlreadyTaken(trustedEmail) {
		user.Email = model.NullableEmail(trustedEmail)
		user.EmailVerified = true
	}
	user.DisplayName = oidcStringClaim(claims, provider.DisplayNameClaim)
	user.AvatarUrl = oidcStringClaim(claims, provider.AvatarClaim)
	// 仅回填已验证且未被占用的手机号：它已是可关联标识，抢占会撞唯一索引
	if phone := oidcVerifiedPhoneNumber(claims); phone != "" && !model.IsPhoneAlreadyTaken(phone) {
		user.PhoneNumber = model.NullablePhone(phone)
	}

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		usedInviteCode, err := validateAndUseInviteCodeForOAuth(c, tx)
		if err != nil {
			return err
		}
		if inviterId > 0 {
			user.InviterId = inviterId
		}
		if usedInviteCode != "" {
			user.UsedInviteCode = usedInviteCode
		}
		if err := user.InsertWithTx(tx, user.InviterId); err != nil {
			return err
		}
		idpUsername, idpEmail := oidcIdentitySnapshot(provider, claims)
		identity := model.UserOidcIdentity{
			UserId:      user.Id,
			ProviderId:  provider.Id,
			Subject:     subject,
			IdpUsername: idpUsername,
			IdpEmail:    idpEmail,
			CreatedTime: utils.GetTimestamp(),
		}
		return tx.Create(&identity).Error
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	oidcSetupLogin(c, provider, &user, rawIdToken)
}
