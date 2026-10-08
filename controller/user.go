package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/limit"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"

	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// getFriendlyValidationMessage 将验证错误转换为友好的中文提示
func getFriendlyValidationMessage(err error) string {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, fieldError := range validationErrors {
			field := fieldError.Field()
			tag := fieldError.Tag()

			switch field {
			case "Username":
				switch tag {
				case "required":
					return "Username must not be empty"
				case "max":
					return "Username must not exceed 12 characters"
				case "excludes":
					return "Username must not contain the @ symbol"
				}
			case "Password":
				switch tag {
				case "required":
					return "Password must not be empty"
				case "min":
					return "Password must be at least 8 characters"
				case "max":
					return "Password must not exceed 20 characters"
				}
			case "DisplayName":
				switch tag {
				case "max":
					return "Display name must not exceed 20 characters"
				}
			case "Email":
				switch tag {
				case "email":
					return "Invalid email format"
				case "max":
					return "Email must not exceed 50 characters"
				}
			}
		}
	}
	return "Invalid input parameters"
}

func Login(c *gin.Context) {
	var loginRequest LoginRequest
	decodeErr := json.NewDecoder(c.Request.Body).Decode(&loginRequest)
	username := loginRequest.Username
	password := loginRequest.Password
	invalidParams := decodeErr != nil || strings.TrimSpace(username) == "" || strings.TrimSpace(password) == ""

	// 先按标识符类型显式解析用户（含 @ 只查邮箱，否则只查用户名），再查锁定状态：
	// 这样计数 key 能在解析成功时收敛到 user.Id，使同一账号的用户名/邮箱两种写法共享计数。
	var resolvedUser *model.User
	var failureKey string
	if !invalidParams {
		user, resolveErr := model.ResolveLoginUser(username)
		failureKey = common.LoginFailureKeyForIdentifier(username)
		if resolveErr == nil {
			resolvedUser = user
			failureKey = common.LoginFailureKeyForUser(user.Id)
		}
	}

	// 密码登录关闭时只给 root 留一条不依赖身份提供方的逃生口（IdP 不可用时仍能进后台改配置），
	// 其余情形（含请求非法、账号不存在）响应完全一致且不记失败计数，避免给不可登录的账号累积锁定。
	if !config.EffectivePasswordLogin() && (resolvedUser == nil || !config.LocalPasswordAllowedForRole(resolvedUser.Role)) {
		c.JSON(http.StatusOK, gin.H{
			"message": "The admin has disabled password sign-in",
			"success": false,
		})
		return
	}
	if invalidParams {
		c.JSON(http.StatusOK, gin.H{
			"message": "Invalid parameters",
			"success": false,
		})
		return
	}

	// 账号级锁定：与 IP 级限流互补，遏制跨 IP 的分布式撞库。
	// 提示语与状态码需与普通认证失败完全一致，并同样走一次等价开销的密码比较，
	// 避免通过响应文案或耗时暴露账号是否存在/是否被锁。
	if common.IsLoginLocked(failureKey) {
		_ = model.ValidateLoginPassword(nil, password)
		c.JSON(http.StatusOK, gin.H{
			"message": model.ErrLoginFailed.Error(),
			"success": false,
		})
		return
	}

	if err := model.ValidateLoginPassword(resolvedUser, password); err != nil {
		common.RecordLoginFailure(failureKey)
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	common.ClearLoginFailures(failureKey)
	setupLogin(resolvedUser, c)
}

// setup session & cookies and then return user info
// loginSessionInfo 建立会话时记录的登录方式；经身份提供方登录时带上提供方与原始 id_token
// （退出时作 RP-Initiated Logout 的 id_token_hint）。
type loginSessionInfo struct {
	method     string
	provider   *model.OidcProvider
	rawIdToken string
}

// setupLogin 密码登录建立会话。
func setupLogin(user *model.User, c *gin.Context) {
	setupLoginSession(user, c, loginSessionInfo{method: model.SessionMethodPassword})
}

// setupLoginSession 建一行 user_sessions 并把会话键写进 cookie 会话：cookie 只是索引，
// 会话是否有效以行为准（改密踢出、登出其它设备、封禁、空闲 / 绝对过期都靠它）。
func setupLoginSession(user *model.User, c *gin.Context, info loginSessionInfo) {
	session := sessions.Default(c)
	// 先清空旧会话再写入，避免残留字段(如 turnstile 标记)被新会话继承。
	session.Clear()
	providerId, idToken := 0, ""
	if info.provider != nil {
		providerId = info.provider.Id
		idToken = info.rawIdToken
	}
	userSession, err := model.CreateUserSession(user.Id, info.method, c.Request.UserAgent(), c.ClientIP(), providerId, idToken)
	if err != nil {
		logger.SysError("failed to create login session: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"message": "Unable to save session, please try again",
			"success": false,
		})
		return
	}
	session.Set("sid", userSession.SessionKey)
	session.Set("id", user.Id)
	session.Set("username", user.Username)
	session.Set("role", user.Role)
	session.Set("status", user.Status)
	err = session.Save()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "Unable to save session, please try again",
			"success": false,
		})
		return
	}
	user.LastLoginTime = time.Now().Unix()
	user.LastLoginIp = c.ClientIP()

	user.Update(false)

	cleanUser := model.User{
		Id:          user.Id,
		AvatarUrl:   user.AvatarUrl,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		Status:      user.Status,
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "",
		"success": true,
		"data":    cleanUser,
	})
}

// Logout 结束本次会话：删掉会话行并清 cookie。若本次会话经身份提供方登录且提供方支持
// RP-Initiated Logout，额外下发 redirect_url：前端整页跳过去结束 IdP 会话，再由 IdP 回到本站 /signed-out。
// 其余情形 redirect_url 为空串，前端直接落 /signed-out。
func Logout(c *gin.Context) {
	session := sessions.Default(c)
	// 必须在 Clear 之前取：清完就读不到本次会话的键了
	redirectURL := oidcEndSessionURL(c, session)
	if key, _ := session.Get("sid").(string); key != "" {
		if err := model.DeleteUserSessionByKey(key); err != nil {
			logger.SysError("failed to delete session row: " + err.Error())
		}
	}
	session.Clear()
	err := session.Save()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "",
		"success": true,
		"data": gin.H{
			"redirect_url": redirectURL,
		},
	})
}

func Register(c *gin.Context) {
	if !config.RegisterEnabled {
		c.JSON(http.StatusOK, gin.H{
			"message": "The admin has disabled new user sign-up",
			"success": false,
		})
		return
	}
	if !config.EffectivePasswordRegister() {
		c.JSON(http.StatusOK, gin.H{
			"message": "The admin has disabled password sign-up. Please sign up with a third-party account.",
			"success": false,
		})
		return
	}
	var user model.User
	err := c.ShouldBindJSON(&user)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	// 密码注册特定验证
	if strings.TrimSpace(user.Password) == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Password must not be empty",
		})
		return
	}

	if err := common.Validate.Struct(&user); err != nil {
		// 友好的验证错误提示
		friendlyMessage := getFriendlyValidationMessage(err)
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": friendlyMessage,
		})
		return
	}
	user.Email = model.NullableEmail(common.NormalizeEmail(string(user.Email)))
	if config.EmailVerificationEnabled {
		if user.Email == "" || user.VerificationCode == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "The admin has enabled email verification. Please enter your email address and verification code.",
			})
			return
		}

		// 严格验证邮箱格式
		if err := common.ValidateEmailStrict(string(user.Email)); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Invalid email format",
			})
			return
		}

		if !common.VerifyCodeWithKey(string(user.Email), user.VerificationCode, common.EmailVerificationPurpose) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Verification code is incorrect or has expired",
			})
			return
		}
	}

	// 邀请码基本验证（仅适用于密码注册，三方登录注册不需要邀请码）
	if config.InviteCodeRegisterEnabled {
		if user.InviteCode == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "The admin has enabled invite-code sign-up. Please enter an invite code.",
			})
			return
		}
	}

	affCode := user.AffCode // this code is the inviter's code, not the user's own code
	inviterId, _ := model.GetUserIdByAffCode(affCode)
	cleanUser := model.User{
		Username:    user.Username,
		Password:    user.Password,
		DisplayName: user.Username,
		InviterId:   inviterId,
	}

	// 只有启用邀请码注册时才保存使用的邀请码
	if config.InviteCodeRegisterEnabled && user.InviteCode != "" {
		cleanUser.UsedInviteCode = user.InviteCode
	}

	if config.EmailVerificationEnabled {
		// 走到这里说明验证码已核验通过，这个邮箱是本人的
		cleanUser.Email = user.Email
		cleanUser.EmailVerified = true
	}

	// 如果需要使用邀请码，先获取锁（按照order.go的模式）
	if config.InviteCodeRegisterEnabled && user.InviteCode != "" {
		// 优先使用Redis分布式锁，失败时使用内存锁
		if config.RedisEnabled {
			mutex, lockErr := model.AcquireInviteCodeLock(user.InviteCode)
			if lockErr == nil && mutex != nil {
				defer func() {
					unlockOk, unlockErr := mutex.Unlock()
					if unlockErr != nil || !unlockOk {
						// 注册过程中的解锁失败不应该影响用户注册结果，只记录日志
						// logger.SysError(fmt.Sprintf("failed to unlock invite code %s: ok=%v, err=%v", user.InviteCode, unlockOk, unlockErr))
					}
				}()
			} else {
				// Redis锁失败，降级到内存锁
				model.LockInviteCode(user.InviteCode)
				defer model.UnlockInviteCode(user.InviteCode)
			}
		} else {
			// 无Redis时使用内存锁
			model.LockInviteCode(user.InviteCode)
			defer model.UnlockInviteCode(user.InviteCode)
		}

		// 在锁保护下验证邀请码（防止TOCTOU攻击）
		err := model.CheckInviteCode(user.InviteCode)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}

	// 使用事务确保用户创建和邀请码使用的原子性
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		// 在事务中创建用户
		if err := cleanUser.InsertWithTx(tx, inviterId); err != nil {
			return err
		}

		// 在事务中增加邀请码使用次数
		if config.InviteCodeRegisterEnabled && user.InviteCode != "" {
			if err := model.UseInviteCodeWithTx(tx, user.InviteCode); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 事务提交成功后，刷新相关缓存
	if config.RedisEnabled {
		// 刷新用户配额缓存（如果有邀请奖励）
		if inviterId != 0 && config.QuotaForInviter > 0 {
			model.CacheUpdateUserQuota(inviterId)
		}
		if config.QuotaForInvitee > 0 {
			model.CacheUpdateUserQuota(cleanUser.Id)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func GetUsersList(c *gin.Context) {
	var params model.SearchUserParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	users, err := model.GetUsersList(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    users,
	})
}

// GetUserStats 用户列表页头统计：分页下前端无法自行算出的总数与按状态 / 角色的计数。
func GetUserStats(c *gin.Context) {
	stats, err := model.GetUserStats()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    stats,
	})
}

// GetOidcCoverage 管理员核查 OIDC 迁移进度：活跃用户中已绑 / 未绑启用提供方身份的人数。
func GetOidcCoverage(c *gin.Context) {
	coverage, err := model.GetOidcCoverage()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    coverage,
	})
}

func GetUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	user, err := model.GetUserById(id, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	// 影子记账账户对后台直查不可见,与列表/搜索的 ExcludeShadowUsers 语义一致
	if model.IsShadowUser(user) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "User not found",
		})
		return
	}
	myRole := c.GetInt("role")
	if myRole <= user.Role && myRole != config.RoleRootUser {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "You are not allowed to view users of the same or higher role",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    user,
	})
}

func GetRateRealtime(c *gin.Context) {
	id := c.GetInt("id")
	user, err := model.GetUserById(id, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 与 /panel/analytics 实时流量同一口径：logs 表最近60秒滑动窗口，仅统计消费日志
	rpmTpmStats, err := model.GetRpmTpmStatistics(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// maxRPM 仍取自分组限流配置，仅用于展示使用率
	maxRPM := 0
	if limiter := model.GlobalUserGroupRatio.GetAPILimiter(user.Group); limiter != nil {
		maxRPM = limit.GetMaxRate(limiter)
	}
	var usageRpmRate float64 = 0
	if maxRPM > 0 {
		usageRpmRate = math.Floor(float64(rpmTpmStats.RPM)/float64(maxRPM)*100*100) / 100
	}

	data := map[string]interface{}{
		"rpm":          rpmTpmStats.RPM,
		"maxRPM":       maxRPM,
		"usageRpmRate": usageRpmRate,
		"tpm":          rpmTpmStats.TPM,
		"maxTPM":       0,
		"usageTpmRate": 0,
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

func GetUserDashboard(c *gin.Context) {
	id := c.GetInt("id")

	// 使用 TZ 环境变量的时区，与 UpdateStatistics 保持一致
	location := time.Local
	if tzEnv := os.Getenv("TZ"); tzEnv != "" {
		if loc, err := time.LoadLocation(tzEnv); err == nil {
			location = loc
		}
	}
	now := time.Now().In(location)
	toDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	endOfDay := toDay.Add(-time.Second).Add(time.Hour * 24).Format("2006-01-02")

	// range 查询参数(7d/30d/90d),默认 7d 与历史行为一致;天数 clamp 到 MaxLogQuerySpanDays
	days := 7
	switch c.Query("range") {
	case "30d":
		days = 30
	case "90d":
		days = 90
	case "7d", "":
		days = 7
	}
	if days > MaxLogQuerySpanDays {
		days = MaxLogQuerySpanDays
	}
	startOfDay := toDay.AddDate(0, 0, -days).Format("2006-01-02")

	dashboards, err := model.GetUserModelStatisticsByPeriod(id, startOfDay, endOfDay)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Unable to get statistics.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    dashboards,
	})
}

func GenerateAccessToken(c *gin.Context) {
	id := c.GetInt("id")
	user, err := model.GetUserById(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	user.AccessToken = utils.GetUUID()

	if model.DB.Where("access_token = ?", user.AccessToken).First(user).RowsAffected != 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Please try again: the generated UUID was a duplicate.",
		})
		return
	}

	if err := user.Update(false); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    user.AccessToken,
	})
}

func GetAffCode(c *gin.Context) {
	id := c.GetInt("id")
	user, err := model.GetUserById(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if user.AffCode == "" {
		user.AffCode = utils.GetRandomString(4)
		if err := user.Update(false); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    user.AffCode,
	})
}

func GetSelf(c *gin.Context) {
	id := c.GetInt("id")
	user, err := model.GetUserById(id, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 实时计算邀请人数
	affCount, err := model.GetUserInviteCount(id)
	if err == nil {
		user.AffCount = int(affCount)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": selfResponse{
			User:           user,
			HasPassword:    selfHasPassword(id),
			OidcIdentities: selfOidcIdentities(id),
		},
	})
}

// selfResponse GET /api/user/self 的响应体：在用户对象上追加已绑定的 OIDC 身份。
// user.oidc_id 仍原样保留一个版本，供尚未升级的前端读取。
type selfResponse struct {
	*model.User
	// Password 影子字段：同名字段层级更浅，压掉内嵌 model.User 的 password 标签；
	// 永远为 nil 配合 omitempty，响应体里彻底没有这个键。
	// （不能写 json:"-"，那样字段会被直接丢弃、压不住内嵌的同名键。）
	Password *string `json:"password,omitempty"`
	// has_password 本站是否给该账号存了密码：三方首登建号的用户为空串，前端据此区分
	// 「设置密码」与「修改密码」。只下发布尔，哈希永不进响应体。
	HasPassword    bool               `json:"has_password"`
	OidcIdentities []selfOidcIdentity `json:"oidc_identities"`
}

// selfHasPassword 单独问一次密码列是否非空：GetUserById(id,false) Omit 了 password。
// 查询失败按「有密码」处理，保持现状渲染（改密表单）而不是误报为无密码。
func selfHasPassword(userId int) bool {
	hasPassword, err := model.HasPasswordById(userId)
	if err != nil {
		return true
	}
	return hasPassword
}

type selfOidcIdentity struct {
	ProviderSlug string `json:"provider_slug"`
	DisplayName  string `json:"display_name"`
	// first_party 提供方即本站账号体系，前端据此用站点名而非供应商名做行标签
	FirstParty bool `json:"first_party"`
	// account_settings_url 提供方侧账号设置页，空串表示没有；已停用的提供方也照样下发
	AccountSettingsUrl string `json:"account_settings_url"`
	// 四个设置页深链：账号安全页按行跳转，留空的行不渲染
	PasswordUrl string `json:"password_url"`
	MfaUrl      string `json:"mfa_url"`
	PasskeyUrl  string `json:"passkey_url"`
	IdentityUrl string `json:"identity_url"`
	// idp_username / idp_email 是 IdP 侧账户的展示快照，仅用于告诉用户这条绑定对应对方的
	// 哪个账户；存量绑定为空串，下次经该提供方登录后自动补齐。subject 不下发。
	IdpUsername string `json:"idp_username"`
	IdpEmail    string `json:"idp_email"`
	// created_time 绑定时间（秒级时间戳）
	CreatedTime int64 `json:"created_time"`
}

// selfOidcIdentities 列出该用户已绑定的 OIDC 身份。读取失败时返回空列表：
// 个人信息接口不应因为绑定列表查询失败而整体失败。
func selfOidcIdentities(userId int) []selfOidcIdentity {
	identities, err := model.ListUserOidcIdentities(userId)
	if err != nil {
		return []selfOidcIdentity{}
	}
	providers, err := model.GetOidcProviders()
	if err != nil {
		return []selfOidcIdentity{}
	}
	byId := make(map[int]*model.OidcProvider, len(providers))
	for _, provider := range providers {
		byId[provider.Id] = provider
	}
	result := make([]selfOidcIdentity, 0, len(identities))
	for _, identity := range identities {
		provider, ok := byId[identity.ProviderId]
		if !ok {
			continue
		}
		displayName := provider.DisplayName
		if displayName == "" {
			displayName = provider.Slug
		}
		result = append(result, selfOidcIdentity{
			ProviderSlug:       provider.Slug,
			DisplayName:        displayName,
			FirstParty:         provider.FirstParty,
			AccountSettingsUrl: provider.AccountSettingsUrl,
			PasswordUrl:        provider.PasswordUrl,
			MfaUrl:             provider.MfaUrl,
			PasskeyUrl:         provider.PasskeyUrl,
			IdentityUrl:        provider.IdentityUrl,
			IdpUsername:        identity.IdpUsername,
			IdpEmail:           identity.IdpEmail,
			CreatedTime:        identity.CreatedTime,
		})
	}
	return result
}

func UpdateUser(c *gin.Context) {
	var updatedUser model.User
	err := json.NewDecoder(c.Request.Body).Decode(&updatedUser)
	if err != nil || updatedUser.Id == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid parameters",
		})
		return
	}
	if updatedUser.Password == "" {
		updatedUser.Password = "$I_LOVE_U" // make Validator happy :)
	}
	if err := common.Validate.Struct(&updatedUser); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": getFriendlyValidationMessage(err),
		})
		return
	}

	// 如果更新了邮箱，先归一化再进行严格验证
	updatedUser.Email = model.NullableEmail(common.NormalizeEmail(string(updatedUser.Email)))
	if updatedUser.Email != "" {
		if err := common.ValidateEmailStrict(string(updatedUser.Email)); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Invalid email format",
			})
			return
		}
	}
	originUser, err := model.GetUserById(updatedUser.Id, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	// 影子记账账户不接受后台管理操作,与 GetUser/GetUsersList 的排除语义一致
	if model.IsShadowUser(originUser) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Cannot operate on an organization shadow account",
		})
		return
	}
	myRole := c.GetInt("role")
	if myRole <= originUser.Role && myRole != config.RoleRootUser {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "You are not allowed to update users of the same or higher role",
		})
		return
	}
	if myRole <= updatedUser.Role && myRole != config.RoleRootUser {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "You are not allowed to raise another user's role to equal or above your own",
		})
		return
	}
	if updatedUser.Password == "$I_LOVE_U" {
		updatedUser.Password = "" // rollback to what it should be
	}
	updatePassword := updatedUser.Password != ""
	// 「已验证」只由验证码绑定与 IdP 已验证 claim 授予，请求体里的 email_verified 一律不采信：
	// 先复位成库里的值，再按服务端比对决定是否降级。后台改邮箱不发验证码（见 userPage.emailEditHint），
	// 换成了另一个地址就不再有任何来源可以证明它属于本人，必须降级为未验证——否则管理员
	// 误填的邮箱会挂着「已验证」角标，还能被 link_by_verified_email 当成别人的登录入口。
	// 提交空邮箱时库里的邮箱不会被清掉（Update 走 Updates(struct)，空串是零值会被跳过），
	// 故也不动这个标记。
	updatedUser.EmailVerified = originUser.EmailVerified
	emailReplaced := updatedUser.Email != "" && updatedUser.Email != originUser.Email
	if emailReplaced {
		updatedUser.EmailVerified = false
	}
	if err := updatedUser.Update(updatePassword); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	// false 是零值，Updates(struct) 会跳过，须显式写列
	if emailReplaced {
		if err := model.UpdateUser(updatedUser.Id, map[string]interface{}{"email_verified": false}); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}
	if originUser.Quota != updatedUser.Quota {
		model.RecordLog(originUser.Id, model.LogTypeManage, fmt.Sprintf("Admin changed user quota from %s to %s", common.LogQuota(originUser.Quota), common.LogQuota(updatedUser.Quota)))
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// selfUpdateRequest holds the fields a user may change on their own account. The username is not
// self-editable, so it is neither required nor validated here.
type selfUpdateRequest struct {
	Password         string `json:"password" validate:"omitempty,min=8,max=64"`
	OriginalPassword string `json:"original_password"`
	DisplayName      string `json:"display_name" validate:"max=20"`
}

func UpdateSelf(c *gin.Context) {
	var user selfUpdateRequest
	err := json.NewDecoder(c.Request.Body).Decode(&user)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid parameters",
		})
		return
	}
	if err := common.Validate.Struct(&user); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": getFriendlyValidationMessage(err),
		})
		return
	}

	cleanUser := model.User{
		Id:          c.GetInt("id"),
		Password:    user.Password,
		DisplayName: user.DisplayName,
	}
	updatePassword := user.Password != ""
	// 密码登录关闭时本站不再持有第二套凭据：自助改密一律拒绝（管理员重置密码不受影响），
	// 其余字段照常更新需由前端另发一次不带 password 的请求。
	// 例外：root 的逃生口密码必须可轮换，否则关闭密码登录后就再也改不动了。
	if updatePassword && !config.LocalPasswordAllowedForRole(c.GetInt("role")) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Password sign-in is not enabled on this site; passwords are managed by the identity provider",
		})
		return
	}
	if updatePassword {
		if msg := checkOriginalPassword(cleanUser.Id, user.OriginalPassword); msg != "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": msg,
			})
			return
		}
	}
	if err := cleanUser.Update(updatePassword); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	// 改密后踢掉其它设备：只保留发起改密的这一个会话
	if updatePassword {
		if err := model.DeleteUserSessionsExcept(nil, cleanUser.Id, currentSessionKey(c)); err != nil {
			logger.SysError("failed to clear other sessions after password change: " + err.Error())
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// checkOriginalPassword returns an error message when the current password does not authorize a
// password change, or "" when it does. Accounts without a stored password (created through a
// third-party sign-in) set their first password without one.
func checkOriginalPassword(userId int, originalPassword string) string {
	stored, err := model.GetUserById(userId, true)
	if err != nil {
		return "Failed to load the current account"
	}
	if stored.Password == "" {
		return ""
	}
	if originalPassword == "" {
		return "Current password is required to change the password"
	}
	if !common.ValidatePasswordAndHash(originalPassword, stored.Password) {
		return "Current password is incorrect"
	}
	return ""
}

// GetUserSetting 获取当前用户的偏好设置（仅 self）
func GetUserSetting(c *gin.Context) {
	id := c.GetInt("id")
	setting, err := model.GetUserSettingById(id)
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
		"data":    setting.APIView(),
	})
}

// UpdateUserSetting 部分更新当前用户的偏好设置（仅 self；未提供的字段保持不变）
func UpdateUserSetting(c *gin.Context) {
	var update model.UserSettingUpdate
	if err := json.NewDecoder(c.Request.Body).Decode(&update); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid parameters",
		})
		return
	}
	id := c.GetInt("id")
	setting, err := model.GetUserSettingById(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if err := setting.ApplyUpdate(update); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if err := setting.Validate(); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if err := model.UpdateUserSettingById(id, *setting); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    setting.APIView(),
	})
}

func DeleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	originUser, err := model.GetUserById(id, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	// 影子记账账户不接受后台管理操作,与 GetUser/GetUsersList 的排除语义一致
	if model.IsShadowUser(originUser) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Cannot operate on an organization shadow account",
		})
		return
	}
	myRole := c.GetInt("role")
	if myRole <= originUser.Role {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "You are not allowed to delete users of the same or higher role",
		})
		return
	}
	if err := model.DeleteUserById(id); err != nil {
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

func CreateUser(c *gin.Context) {
	var user model.User
	err := c.ShouldBindJSON(&user)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 管理员创建用户时的特定验证
	if strings.TrimSpace(user.Username) == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Username must not be empty",
		})
		return
	}

	if strings.TrimSpace(user.Password) == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Password must not be empty",
		})
		return
	}
	if err := common.Validate.Struct(&user); err != nil {
		// 友好的验证错误提示
		friendlyMessage := getFriendlyValidationMessage(err)
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": friendlyMessage,
		})
		return
	}

	// 如果提供了邮箱，先归一化再进行严格验证
	user.Email = model.NullableEmail(common.NormalizeEmail(string(user.Email)))
	if user.Email != "" {
		if err := common.ValidateEmailStrict(string(user.Email)); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Invalid email format",
			})
			return
		}
	}

	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}
	myRole := c.GetInt("role")
	if user.Role >= myRole {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Cannot create a user with a role equal to or above your own",
		})
		return
	}
	// Even for admin users, we cannot fully trust them!
	cleanUser := model.User{
		Username:    user.Username,
		Password:    user.Password,
		DisplayName: user.DisplayName,
	}
	if err := cleanUser.Insert(0); err != nil {
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

type ManageRequest struct {
	UserId int    `json:"user_id"`
	Action string `json:"action"`
}

// ManageUser Only admin user can do this
func ManageUser(c *gin.Context) {
	var req ManageRequest
	err := json.NewDecoder(c.Request.Body).Decode(&req)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid parameters",
		})
		return
	}

	if req.UserId == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "User ID must not be empty",
		})
		return
	}

	user, err := model.GetUserById(req.UserId, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "User not found",
		})
		return
	}
	// 影子记账账户不接受后台管理操作,与 GetUser/GetUsersList 的排除语义一致
	if model.IsShadowUser(user) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Cannot operate on an organization shadow account",
		})
		return
	}
	myRole := c.GetInt("role")
	if myRole <= user.Role && myRole != config.RoleRootUser {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "You are not allowed to update users of the same or higher role",
		})
		return
	}
	switch req.Action {
	case "disable":
		user.Status = config.UserStatusDisabled
		if user.Role == config.RoleRootUser {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Cannot disable the root user",
			})
			return
		}
		// 封禁即结束他的全部会话；鉴权中间件本就每次核对状态，这里只是不留残行
		if err := model.DeleteUserSessionsExcept(nil, user.Id, ""); err != nil {
			logger.SysError("failed to clear sessions after banning user: " + err.Error())
		}
	case "enable":
		user.Status = config.UserStatusEnabled
	case "delete":
		if user.Role == config.RoleRootUser {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Cannot delete the root user",
			})
			return
		}
		if err := user.Delete(); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "promote":
		// 设置为管理员：只有超级管理员能操作
		if myRole != config.RoleRootUser {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Only root can promote other users to admin",
			})
			return
		}
		if user.Role == config.RoleRootUser {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Cannot change the root user's role",
			})
			return
		}
		user.Role = config.RoleAdminUser
	case "demote":
		// 设置为普通用户：不能操作超级管理员
		if user.Role == config.RoleRootUser {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Cannot change the root user's role",
			})
			return
		}
		user.Role = config.RoleCommonUser
	case "set_reliable":
		// 设置为可信内部员工：管理员及以上能操作
		if myRole < config.RoleAdminUser {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Only admin or root can mark users as trusted internal staff",
			})
			return
		}
		if user.Role == config.RoleRootUser {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Cannot change the root user's role",
			})
			return
		}
		user.Role = config.RoleReliableUser
	}

	if err := user.Update(false); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	clearUser := model.User{
		Role:   user.Role,
		Status: user.Status,
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    clearUser,
	})
}

func EmailBind(c *gin.Context) {
	email := common.NormalizeEmail(c.Query("email"))
	code := c.Query("code")

	// 严格验证邮箱格式
	if err := common.ValidateEmailStrict(email); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid email format",
		})
		return
	}

	if !common.VerifyCodeWithKey(email, code, common.EmailVerificationPurpose) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Verification code is incorrect or has expired",
		})
		return
	}
	id := c.GetInt("id")
	user := model.User{
		Id: id,
	}
	err := user.FillUserById()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	user.Email = model.NullableEmail(email)
	// 验证码是本人收到并回填的，这个邮箱按已验证记账
	user.EmailVerified = true
	// no need to check if this email already taken, because we have used verification code to check it
	err = user.Update(false)
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

type topUpRequest struct {
	Key string `json:"key"`
}

func TopUp(c *gin.Context) {
	req := topUpRequest{}
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	id := c.GetInt("id")
	quota, err := model.Redeem(req.Key, id, c.ClientIP())
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
		"data":    quota,
	})
}

type ChangeUserQuotaRequest struct {
	Quota  int    `json:"quota" form:"quota"`
	Remark string `json:"remark" form:"remark"`
}

func ChangeUserQuota(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	var req ChangeUserQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	if req.Quota == 0 {
		common.APIRespondWithError(c, http.StatusOK, errors.New("must not be 0"))
		return
	}

	targetUser, err := model.GetUserById(userId, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	// 影子记账账户不接受后台管理操作,与 GetUser/GetUsersList 的排除语义一致
	if model.IsShadowUser(targetUser) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Cannot operate on an organization shadow account",
		})
		return
	}
	myRole := c.GetInt("role")
	if myRole <= targetUser.Role && myRole != config.RoleRootUser {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "You are not allowed to update users of the same or higher role",
		})
		return
	}

	err = model.ChangeUserQuota(userId, req.Quota, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	remark := fmt.Sprintf("Admin adjusted user quota by %s", common.LogQuota(req.Quota))

	if req.Remark != "" {
		remark = fmt.Sprintf("%s, remark: %s", remark, req.Remark)
	}

	model.RecordQuotaLog(userId, model.LogTypeManage, req.Quota, c.ClientIP(), remark)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

type UnbindRequest struct {
	Type string `json:"type"`
	// Provider 仅对 type=oidc 生效：指定要解绑的提供方 slug。
	// 旧前端不带该字段，此时解绑当前账号下全部 OIDC 身份。
	Provider string `json:"provider"`
}

// unbindLastLoginMethodMessage 被解绑的方式确实是账号仅剩一种可用方式时的提示。
const unbindLastLoginMethodMessage = "This is the only sign-in method for this account; unlinking it would lock you out. Set a password or link another sign-in method first."

// unbindNoLoginMethodMessage 被解绑的身份本就不计入可用方式（站点开关关闭或提供方停用），
// 且账号已无任何可用方式时的提示——此时称它「唯一的登录方式」并不准确。
const unbindNoLoginMethodMessage = "This account has no other available sign-in method; unlinking would lock you out. Set a password or link another sign-in method first."

// unbindUnknownProviderMessage 待解绑的登录方式查不到时的对外提示：不露 slug 与「OIDC」等技术词，
// 具体 slug 只进日志。
const unbindUnknownProviderMessage = "This sign-in method does not exist or has been disabled"

// unbindClearedColumns 各绑定类型解绑时要清空的 users 列。
// OIDC 绑定关系存在 user_oidc_identities，users.oidc_id 只是迁移期遗留列，不再改写。
var unbindClearedColumns = map[string]map[string]interface{}{
	model.LoginMethodGitHub: {"github_id": "", "github_id_new": nil},
	model.LoginMethodWeChat: {"wechat_id": ""},
	model.LoginMethodLark:   {"lark_id": ""},
	model.LoginMethodLinuxDo: {
		"linuxdo_id": 0, "linuxdo_username": "", "linuxdo_trust_level": 0,
	},
}

func Unbind(c *gin.Context) {
	var req UnbindRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid parameters",
		})
		return
	}
	updates, known := unbindClearedColumns[req.Type]
	if !known && req.Type != model.LoginMethodOidc {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Unknown link type",
		})
		return
	}
	id := c.GetInt("id")
	// 「盘点剩余登录方式 → 写库」必须在一个事务的行锁内串行，否则并发解绑两种方式会双双通过检查（SEC-23）。
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		// 判定要读 password 列，故不能复用 Omit 了密码的 GetUserById；哈希只在服务端参与判定，不进任何响应。
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", id).Error; err != nil {
			return err
		}
		providerIds, err := model.ListUserEnabledOidcProviderIds(tx, id)
		if err != nil {
			return err
		}
		targetProviderId := 0
		if req.Type == model.LoginMethodOidc && req.Provider != "" {
			var provider model.OidcProvider
			if err := tx.First(&provider, "slug = ?", req.Provider).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					logger.SysLog("unlink failed: identity provider " + req.Provider + " not found")
					return errors.New(unbindUnknownProviderMessage)
				}
				return err
			}
			targetProviderId = provider.Id
		}
		methods := model.InventoryUserLoginMethods(&user, providerIds)
		remaining := methods.RemainingAfterUnbind(req.Type, targetProviderId)
		if remaining < 1 {
			// remaining 未减少说明本次解绑的身份本就不计入可用方式，账号是已经零方式而非「解绑掉最后一种」。
			if remaining == methods.Total() {
				return errors.New(unbindNoLoginMethodMessage)
			}
			return errors.New(unbindLastLoginMethodMessage)
		}
		if req.Type == model.LoginMethodOidc {
			// provider 为空即旧前端的全量解绑，删该用户全部 OIDC 身份。
			query := tx.Where("user_id = ?", id)
			if targetProviderId != 0 {
				query = query.Where("provider_id = ?", targetProviderId)
			}
			return query.Delete(&model.UserOidcIdentity{}).Error
		}
		return tx.Model(&model.User{}).Where("id = ?", id).Updates(updates).Error
	})
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
