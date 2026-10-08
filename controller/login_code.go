package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/stmp"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// 邮箱验证码登录（内置账号模式）：先向已注册且已验证的邮箱发一枚验证码，再凭验证码建立会话。
// 发码接口对「邮箱不存在 / 用户关闭了该方式 / 账号被锁」一律返回成功，不暴露账号是否存在。

// loginCodeResendInterval 同一邮箱两次发码的最小间隔。
const loginCodeResendInterval = 60 * time.Second

// loginCodeLength 验证码长度（十六进制字符，与注册验证码一致）。
const loginCodeLength = 6

// errLoginCodeFailed 验证失败的统一文案：验证码错误、过期、账号不可用共用同一句。
const errLoginCodeFailed = "Verification code is incorrect or has expired"

// emailCodeLoginUnavailable 站点未开放邮箱验证码登录时直接拒绝。
func emailCodeLoginUnavailable(c *gin.Context) bool {
	if config.EffectiveEmailCodeLogin(stmp.SystemStmpConfigured()) {
		return false
	}
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "Email verification code login is not enabled on this site",
	})
	return true
}

// userAllowsEmailCodeLogin 用户自己是否关闭了邮箱验证码登录（默认开启；读设置失败按开启处理）。
func userAllowsEmailCodeLogin(userId int) bool {
	setting, err := model.GetUserSettingById(userId)
	if err != nil {
		return true
	}
	return setting.EmailCodeLoginAllowed()
}

// resolveEmailCodeUser 按邮箱找可用邮箱验证码登录的账号；找不到、被封禁或用户关闭了该方式都返回 nil。
func resolveEmailCodeUser(email string) *model.User {
	user := model.User{Email: model.NullableEmail(email)}
	if err := user.FillUserByEmail(); err != nil {
		return nil
	}
	if user.Status != config.UserStatusEnabled || !userAllowsEmailCodeLogin(user.Id) {
		return nil
	}
	return &user
}

type sendLoginCodeRequest struct {
	Email string `json:"email"`
}

// SendLoginCode POST /api/user/login_code：给邮箱发登录验证码。
func SendLoginCode(c *gin.Context) {
	if emailCodeLoginUnavailable(c) {
		return
	}
	var req sendLoginCodeRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid parameters",
		})
		return
	}
	email := common.NormalizeEmail(req.Email)
	if err := common.ValidateEmailStrict(email); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid email format",
		})
		return
	}
	// 冷却期内重复请求静默成功：既防刷邮件，也不暴露邮箱是否存在
	if common.VerificationCodeSentWithin(email, common.LoginCodePurpose, loginCodeResendInterval) {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
		return
	}
	user := resolveEmailCodeUser(email)
	if user == nil || common.IsLoginLocked(common.LoginFailureKeyForUser(user.Id)) {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
		return
	}
	code := common.GenerateVerificationCode(loginCodeLength)
	common.RegisterVerificationCodeWithKey(email, code, common.LoginCodePurpose)
	if err := stmp.SendVerificationCodeEmail(email, code); err != nil {
		// 发信失败要让用户知道，否则会一直等；邮箱明文不入日志
		logger.SysError(fmt.Sprintf("Failed to send login verification code: user_id=%d err=%s", user.Id, err.Error()))
		common.DeleteKey(email, common.LoginCodePurpose)
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Failed to send verification code, please try again later",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

type loginWithCodeRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

// LoginWithCode POST /api/user/login/code：凭邮箱验证码登录。失败计数与密码登录共用同一把账号锁。
func LoginWithCode(c *gin.Context) {
	if emailCodeLoginUnavailable(c) {
		return
	}
	var req loginWithCodeRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid parameters",
		})
		return
	}
	email := common.NormalizeEmail(req.Email)
	if email == "" || req.Code == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid parameters",
		})
		return
	}
	user := resolveEmailCodeUser(email)
	failureKey := common.LoginFailureKeyForIdentifier(email)
	if user != nil {
		failureKey = common.LoginFailureKeyForUser(user.Id)
	}
	if common.IsLoginLocked(failureKey) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": errLoginCodeFailed,
		})
		return
	}
	// 先验码再看账号：验证码本身校验不过与账号不可用返回同一句，且都记一次失败
	if !common.VerifyCodeWithKey(email, req.Code, common.LoginCodePurpose) || user == nil {
		common.RecordLoginFailure(failureKey)
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": errLoginCodeFailed,
		})
		return
	}
	common.DeleteKey(email, common.LoginCodePurpose)
	common.ClearLoginFailures(failureKey)
	setupLoginSession(user, c, loginSessionInfo{method: model.SessionMethodEmailCode})
}
