package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// Two-factor authentication (TOTP) for password and email-code sign-in; see model/user_totp.go.
//
// Sign-in: after a correct password (or email code) a user with two-factor on gets
// {success: true, data: {totp_required: true}} instead of a session. The cookie then carries only
// the user id and a random challenge token whose hash, expiry and failure count are on the
// server; POST /api/user/login/totp with a code from the app or a recovery code finishes the
// sign-in. Wrong codes count against the same account / IP locks as wrong passwords (login_guard.go)
// and the account counters are only cleared once the second factor passed, so knowing the
// password does not buy more guesses. Passkey and identity provider sign-ins never ask.

// Cookie session keys of a pending second step.
const (
	totpSessionUserKey   = "totp_uid"
	totpSessionTokenKey  = "totp_token"
	totpSessionMethodKey = "totp_method"
)

// totpManageFailureKey counts wrong codes on the account page (disable, new recovery codes). It
// is kept apart from the sign-in counters so that it cannot lock the account out of sign-in.
func totpManageFailureKey(userId int) string {
	return "totp-manage:uid:" + strconv.Itoa(userId)
}

type totpCodeRequest struct {
	Code string `json:"code"`
}

func readTotpCode(c *gin.Context) (string, bool) {
	var req totpCodeRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil || req.Code == "" {
		common.APIRespondWithError(c, http.StatusOK, errInvalidParams)
		return "", false
	}
	return req.Code, true
}

// beginSecondFactor is called after the first factor passed. It returns false when the user has
// no two-factor (the caller signs them in); otherwise it has answered the request, either with
// the second step or with an error (it fails closed when the setting cannot be read).
func beginSecondFactor(c *gin.Context, user *model.User, method string) bool {
	enabled, err := model.UserTotpEnabled(user.Id)
	if err != nil {
		logger.SysError(fmt.Sprintf("failed to read two-factor setting of user %d: %s", user.Id, err.Error()))
		common.APIRespondWithError(c, http.StatusOK, errSessionSaveFailed)
		return true
	}
	if !enabled {
		return false
	}
	token, err := model.StartUserTotpChallenge(user.Id, time.Now())
	if err != nil {
		logger.SysError(fmt.Sprintf("failed to start two-factor sign-in of user %d: %s", user.Id, err.Error()))
		common.APIRespondWithError(c, http.StatusOK, errSessionSaveFailed)
		return true
	}
	session := sessions.Default(c)
	session.Clear()
	session.Set(totpSessionUserKey, user.Id)
	session.Set(totpSessionTokenKey, token)
	session.Set(totpSessionMethodKey, method)
	if err := session.Save(); err != nil {
		common.APIRespondWithError(c, http.StatusOK, errSessionSaveFailed)
		return true
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"totp_required": true}})
	return true
}

func clearTotpSession(c *gin.Context) {
	session := sessions.Default(c)
	session.Delete(totpSessionUserKey)
	session.Delete(totpSessionTokenKey)
	session.Delete(totpSessionMethodKey)
	if err := session.Save(); err != nil {
		logger.SysError("failed to clear two-factor sign-in: " + err.Error())
	}
}

// LoginWithTotp POST /api/user/login/totp: the second sign-in step.
func LoginWithTotp(c *gin.Context) {
	code, ok := readTotpCode(c)
	if !ok {
		return
	}
	session := sessions.Default(c)
	userId, _ := session.Get(totpSessionUserKey).(int)
	token, _ := session.Get(totpSessionTokenKey).(string)
	method, _ := session.Get(totpSessionMethodKey).(string)
	now := time.Now()
	if !model.CheckUserTotpChallenge(userId, token, now) {
		clearTotpSession(c)
		common.APIRespondWithError(c, http.StatusOK, model.ErrTotpChallengeExpired)
		return
	}
	user, err := model.GetUserById(userId, false)
	if err != nil || user.Status != config.UserStatusEnabled {
		_ = model.ClearUserTotpChallenge(userId)
		clearTotpSession(c)
		common.APIRespondWithError(c, http.StatusOK, model.ErrTotpChallengeExpired)
		return
	}
	// The first factor may have been turned off by the admin meanwhile.
	if method == model.SessionMethodPassword && !config.EffectivePasswordLogin() && !config.LocalPasswordAllowedForRole(user.Role) {
		clearTotpSession(c)
		common.APIRespondWithCode(c, http.StatusOK, common.APICodePasswordLoginDisabled, "The admin has disabled password sign-in", nil)
		return
	}
	if method == model.SessionMethodEmailCode && emailCodeLoginUnavailable(c) {
		clearTotpSession(c)
		return
	}

	guard := newLoginGuard(c, common.LoginFailureKeyForUser(userId))
	if guard.locked() {
		common.APIRespondWithError(c, http.StatusOK, model.ErrTotpCodeInvalid)
		return
	}
	recovery, err := model.VerifyUserTotp(userId, code, now)
	if err != nil {
		if errors.Is(err, model.ErrTotpCodeInvalid) {
			guard.recordFailure()
			if recordErr := model.RecordUserTotpChallengeFailure(userId); recordErr != nil {
				logger.SysError("failed to count a wrong two-factor code: " + recordErr.Error())
			}
			common.APIRespondWithError(c, http.StatusOK, model.ErrTotpCodeInvalid)
			return
		}
		logger.SysError(fmt.Sprintf("failed to check two-factor code of user %d: %s", userId, err.Error()))
		common.APIRespondWithError(c, http.StatusOK, errSessionSaveFailed)
		return
	}
	if err := model.ClearUserTotpChallenge(userId); err != nil {
		logger.SysError("failed to close two-factor sign-in: " + err.Error())
	}
	guard.clear()
	if recovery {
		left, _ := model.UserTotpRecoveryCodesLeft(userId)
		model.RecordLog(userId, model.LogTypeSystem, fmt.Sprintf("Signed in with a two-factor recovery code from %s; %d recovery codes left", c.ClientIP(), left))
	}
	if method == "" {
		method = model.SessionMethodPassword
	}
	setupLoginSession(user, c, loginSessionInfo{method: method})
}

// GetTotpStatus GET /api/user/totp
func GetTotpStatus(c *gin.Context) {
	userId := c.GetInt("id")
	row, err := model.GetUserTotp(userId)
	enabled := err == nil && row.Enabled
	data := gin.H{"enabled": enabled, "enabled_time": int64(0), "recovery_codes_left": int64(0)}
	if enabled {
		left, _ := model.UserTotpRecoveryCodesLeft(userId)
		data["enabled_time"] = row.EnabledTime
		data["recovery_codes_left"] = left
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
}

// totpAuthURL is the otpauth:// URI the QR code holds (Key Uri Format, as authenticator apps read it).
func totpAuthURL(issuer, account, secret string) string {
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)
	query := url.Values{}
	query.Set("secret", secret)
	query.Set("issuer", issuer)
	query.Set("algorithm", "SHA1")
	query.Set("digits", strconv.Itoa(model.TotpDigits))
	query.Set("period", strconv.Itoa(model.TotpPeriod))
	return "otpauth://totp/" + label + "?" + query.Encode()
}

// BeginTotpSetup POST /api/user/totp/setup: a new secret to scan; nothing changes at sign-in until
// a code confirms it.
func BeginTotpSetup(c *gin.Context) {
	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	secret, err := model.BeginUserTotpSetup(userId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	issuer := config.SystemName
	if issuer == "" {
		issuer = "Modeltaps"
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{
		"secret":      secret,
		"otpauth_url": totpAuthURL(issuer, user.Username, secret),
	}})
}

// EnableTotp POST /api/user/totp/enable: confirm the setup with a code; returns the recovery codes,
// shown this once.
func EnableTotp(c *gin.Context) {
	code, ok := readTotpCode(c)
	if !ok {
		return
	}
	userId := c.GetInt("id")
	codes, err := model.EnableUserTotp(userId, code, time.Now())
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	logger.SysLog(fmt.Sprintf("user %d turned on two-factor authentication", userId))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"recovery_codes": codes}})
}

// verifyTotpForManage checks the code the account page asks for before a change. It has answered
// the request when it returns false.
func verifyTotpForManage(c *gin.Context, userId int, code string) bool {
	key := totpManageFailureKey(userId)
	if common.IsLoginLocked(key) {
		common.APIRespondWithError(c, http.StatusOK, model.ErrTotpCodeInvalid)
		return false
	}
	if _, err := model.VerifyUserTotp(userId, code, time.Now()); err != nil {
		if errors.Is(err, model.ErrTotpCodeInvalid) {
			common.RecordLoginFailure(key)
		}
		common.APIRespondWithError(c, http.StatusOK, err)
		return false
	}
	common.ClearLoginFailures(key)
	return true
}

// DisableTotp POST /api/user/totp/disable: needs a current code or a recovery code.
func DisableTotp(c *gin.Context) {
	code, ok := readTotpCode(c)
	if !ok {
		return
	}
	userId := c.GetInt("id")
	if !verifyTotpForManage(c, userId, code) {
		return
	}
	if err := model.DeleteUserTotp(userId); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	logger.SysLog(fmt.Sprintf("user %d turned off two-factor authentication", userId))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

// RegenerateTotpRecoveryCodes POST /api/user/totp/recovery_codes: needs a current code; the old
// recovery codes stop working.
func RegenerateTotpRecoveryCodes(c *gin.Context) {
	code, ok := readTotpCode(c)
	if !ok {
		return
	}
	userId := c.GetInt("id")
	if !verifyTotpForManage(c, userId, code) {
		return
	}
	codes, err := model.RegenerateUserTotpRecoveryCodes(userId, time.Now())
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"recovery_codes": codes}})
}

// ResetUserTotp DELETE /api/user/:id/totp (root): turns another user's two-factor off, for a user
// who lost both the app and the recovery codes. Root's own goes through DisableTotp with a code.
func ResetUserTotp(c *gin.Context) {
	targetId, err := strconv.Atoi(c.Param("id"))
	if err != nil || targetId <= 0 {
		common.APIRespondWithError(c, http.StatusOK, errInvalidParams)
		return
	}
	operatorId := c.GetInt("id")
	if targetId == operatorId {
		common.APIRespondWithError(c, http.StatusOK, errInvalidParams)
		return
	}
	target, err := model.GetUserById(targetId, false)
	if err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeUserNotFound, "User not found", nil)
		return
	}
	enabled, err := model.UserTotpEnabled(targetId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if !enabled {
		common.APIRespondWithError(c, http.StatusOK, model.ErrTotpNotEnabled)
		return
	}
	if err := model.DeleteUserTotp(targetId); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	logger.SysLog(fmt.Sprintf("root user %d reset two-factor authentication of user %d (%s) from %s", operatorId, targetId, target.Username, c.ClientIP()))
	model.RecordLog(targetId, model.LogTypeManage, fmt.Sprintf("Root user %d reset two-factor authentication", operatorId))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}
