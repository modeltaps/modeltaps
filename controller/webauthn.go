package controller

import (
	"encoding/json"
	"errors"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/webauthn"
	"github.com/modeltaps/modeltaps/model"
	"net/http"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	wauth "github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthn 错误码(供前端 i18n 映射):
// config_failed / user_not_found / bad_request / login_unavailable / begin_login_failed /
// begin_register_failed / session_expired / session_error / verify_failed / save_failed /
// webauthn_disabled
// login_unavailable 统一覆盖"User not found"与"用户无凭据",避免登录侧用户名枚举
// message 字段保留兼容,裸错误信息移入 detail 字段(前端仅 console.warn,不进 toast)
func webauthnError(c *gin.Context, status int, code string, message string, detail string) {
	resp := gin.H{
		"success": false,
		"code":    code,
		"message": message,
	}
	if detail != "" {
		resp["detail"] = detail
	}
	c.JSON(status, resp)
}

// passkeyUnavailable 通行密钥对该角色不开放：内置模式看站点开关，外部模式只放行 root 的应急登录。
// 注册与登录两侧共用同一个错误码，前端映射成同一句文案。
func passkeyUnavailable(c *gin.Context, role int) bool {
	if config.PasskeyAllowedForRole(role) {
		return false
	}
	webauthnError(c, http.StatusForbidden, "passkey_unavailable", "Passkeys are not enabled on this site", "")
	return true
}

// WebAuthn 注册开始
func WebauthnBeginRegistration(c *gin.Context) {
	if passkeyUnavailable(c, c.GetInt("role")) {
		return
	}

	webauthnInstance, err := webauthn.GetWebAuthn()
	if err != nil {
		webauthnError(c, http.StatusInternalServerError, "config_failed", "Failed to get WebAuthn config", err.Error())
		return
	}

	userId := c.GetInt("id")

	user, err := model.GetUserById(userId, false)
	if err != nil {
		webauthnError(c, http.StatusNotFound, "user_not_found", "User not found", err.Error())
		return
	}
	if passkeyUnavailable(c, user.Role) {
		return
	}
	options, session, err := webauthnInstance.BeginRegistration(user)
	if err != nil {
		webauthnError(c, http.StatusInternalServerError, "begin_register_failed", "Unable to start registration", err.Error())
		return
	}

	// 读取可选别名，并将session与别名存储到用户会话中
	type BeginRegReq struct {
		Alias string `json:"alias"`
	}
	var req BeginRegReq
	_ = c.ShouldBindJSON(&req) // 忽略错误，别名为可选

	sess := sessions.Default(c)
	sessionData, err := json.Marshal(session)
	if err != nil {
		webauthnError(c, http.StatusInternalServerError, "session_error", "Failed to serialize session data", err.Error())
		return
	}
	sess.Set("webauthn_registration_session", string(sessionData))
	if req.Alias != "" {
		sess.Set("webauthn_alias", req.Alias)
	}
	sess.Save()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    options,
	})
}

// WebAuthn 注册完成
func WebauthnFinishRegistration(c *gin.Context) {
	if passkeyUnavailable(c, c.GetInt("role")) {
		return
	}

	webauthnInstance, err := webauthn.GetWebAuthn()
	if err != nil {
		webauthnError(c, http.StatusInternalServerError, "config_failed", "Failed to get WebAuthn config", err.Error())
		return
	}

	userId := c.GetInt("id")

	user, err := model.GetUserById(userId, false)
	if err != nil {
		webauthnError(c, http.StatusNotFound, "user_not_found", "User not found", err.Error())
		return
	}
	if passkeyUnavailable(c, user.Role) {
		return
	}

	// 从会话中获取session
	sess := sessions.Default(c)
	sessionDataStr := sess.Get("webauthn_registration_session")
	if sessionDataStr == nil {
		webauthnError(c, http.StatusBadRequest, "session_expired", "Session has expired", "")
		return
	}

	var sessionData wauth.SessionData
	err = json.Unmarshal([]byte(sessionDataStr.(string)), &sessionData)
	if err != nil {
		webauthnError(c, http.StatusInternalServerError, "session_error", "Failed to deserialize session data", err.Error())
		return
	}

	credential, err := webauthnInstance.FinishRegistration(user, sessionData, c.Request)
	if err != nil {
		webauthnError(c, http.StatusBadRequest, "verify_failed", "Registration verification failed", err.Error())
		return
	}

	// 保存凭据到数据库
	alias := ""
	if v := sess.Get("webauthn_alias"); v != nil {
		if s, ok := v.(string); ok {
			alias = s
		}
	}
	err = model.SaveWebAuthnCredential(userId, credential, alias)
	if err != nil {
		webauthnError(c, http.StatusInternalServerError, "save_failed", "Failed to save credential", err.Error())
		return
	}

	// 清除会话
	sess.Delete("webauthn_registration_session")
	sess.Delete("webauthn_alias")
	sess.Save()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "WebAuthn registration successful",
	})
}

// WebAuthn 登录开始
func WebauthnBeginLogin(c *gin.Context) {
	// 从请求中获取用户名
	type LoginRequest struct {
		Username string `json:"username"`
	}

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		webauthnError(c, http.StatusBadRequest, "bad_request", "Invalid request parameters", err.Error())
		return
	}

	webauthnInstance, err := webauthn.GetWebAuthn()
	if err != nil {
		webauthnError(c, http.StatusInternalServerError, "config_failed", "Failed to get WebAuthn config", err.Error())
		return
	}

	var options interface{}
	var session *wauth.SessionData

	sess := sessions.Default(c)

	// 内置模式关闭了通行密钥时，登录侧连挑战都不发；外部模式下要等确定用户后按角色判定（见 FinishLogin）。
	if !config.EffectivePasskeyLogin() && !config.IsExternalAccountSystem() {
		webauthnError(c, http.StatusForbidden, "passkey_unavailable", "Passkeys are not enabled on this site", "")
		return
	}

	if req.Username == "" {
		// 无用户名登录 (Discoverable Login)
		options, session, err = webauthnInstance.BeginDiscoverableLogin()
		if err != nil {
			webauthnError(c, http.StatusInternalServerError, "begin_login_failed", "Unable to start usernameless login", err.Error())
			return
		}
		sess.Delete("webauthn_user_id")
	} else {
		// 与密码登录共用同一套标识符解析:含 @ 只按归一化邮箱查,否则只按用户名查,
		// 两者均排除组织影子账户。
		// 防枚举:用户不存在与用户无凭据返回同一错误码与文案
		user, err := model.ResolveLoginUser(req.Username)
		if err != nil {
			webauthnError(c, http.StatusBadRequest, "login_unavailable", "Login failed", err.Error())
			return
		}

		// 防枚举：角色不允许用通行密钥时与「无凭据」返回同一错误码
		if len(model.GetUserWebAuthnCredentials(user.Id)) == 0 || !config.PasskeyAllowedForRole(user.Role) {
			webauthnError(c, http.StatusBadRequest, "login_unavailable", "Login failed", "")
			return
		}

		options, session, err = webauthnInstance.BeginLogin(user)
		if err != nil {
			webauthnError(c, http.StatusInternalServerError, "begin_login_failed", "Unable to start login", err.Error())
			return
		}
		sess.Set("webauthn_user_id", strconv.Itoa(user.Id))
	}

	// 将session存储到用户会话中
	sessionData, err := json.Marshal(session)
	if err != nil {
		webauthnError(c, http.StatusInternalServerError, "session_error", "Failed to serialize session data", err.Error())
		return
	}
	sess.Set("webauthn_login_session", string(sessionData))
	sess.Save()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    options,
	})
}

// WebAuthn 登录完成
func WebauthnFinishLogin(c *gin.Context) {
	// 内置模式关闭了通行密钥时直接拒绝；外部模式下要等确定用户后按角色判定（下方）。
	if !config.EffectivePasskeyLogin() && !config.IsExternalAccountSystem() {
		webauthnError(c, http.StatusForbidden, "passkey_unavailable", "Passkeys are not enabled on this site", "")
		return
	}
	webauthnInstance, err := webauthn.GetWebAuthn()
	if err != nil {
		webauthnError(c, http.StatusInternalServerError, "config_failed", "Failed to get WebAuthn config", err.Error())
		return
	}

	// 从会话中获取用户ID和session
	sess := sessions.Default(c)
	sessionDataStr := sess.Get("webauthn_login_session")
	userIdStr := sess.Get("webauthn_user_id")

	if sessionDataStr == nil {
		webauthnError(c, http.StatusBadRequest, "session_expired", "Session has expired", "")
		return
	}

	var sessionData wauth.SessionData
	err = json.Unmarshal([]byte(sessionDataStr.(string)), &sessionData)
	if err != nil {
		webauthnError(c, http.StatusInternalServerError, "session_error", "Failed to deserialize session data", err.Error())
		return
	}

	var user *model.User

	if userIdStr != nil {
		// 有用户名的登录
		userId, err := strconv.Atoi(userIdStr.(string))
		if err != nil {
			webauthnError(c, http.StatusInternalServerError, "session_error", "Login failed", err.Error())
			return
		}

		user, err = model.GetUserById(userId, false)
		if err != nil {
			webauthnError(c, http.StatusNotFound, "user_not_found", "Login failed", err.Error())
			return
		}

		_, err = webauthnInstance.FinishLogin(user, sessionData, c.Request)
		if err != nil {
			webauthnError(c, http.StatusBadRequest, "verify_failed", "Login verification failed", err.Error())
			return
		}
	} else {
		// 无用户名登录 (Discoverable Login)
		var foundUser wauth.User
		_, err := webauthnInstance.FinishDiscoverableLogin(func(rawID, userHandle []byte) (wauth.User, error) {
			if len(userHandle) > 0 {
				userId, err := strconv.Atoi(string(userHandle))
				if err == nil {
					u, err := model.GetUserById(userId, false)
					if err == nil {
						foundUser = u
						return u, nil
					}
				}
			}
			// Fallback to lookup by credential ID
			u, err := model.GetUserByWebAuthnCredentialId(rawID)
			if err == nil {
				foundUser = u
				return u, nil
			}
			return nil, errors.New("user not found")
		}, sessionData, c.Request)

		if err != nil {
			webauthnError(c, http.StatusBadRequest, "verify_failed", "Login verification failed", err.Error())
			return
		}

		if foundUser == nil {
			webauthnError(c, http.StatusBadRequest, "user_not_found", "Unable to find the corresponding user", "")
			return
		}

		var ok bool
		user, ok = foundUser.(*model.User)
		if !ok {
			webauthnError(c, http.StatusInternalServerError, "verify_failed", "User type assertion failed", "type assertion to *model.User failed")
			return
		}
	}

	// 安全密钥跟随密码登录开关:凭据本身有效也不放行非 root,否则密码登录关闭后
	// 它仍是一条绕过身份提供方的本站登录入口。
	if passkeyUnavailable(c, user.Role) {
		return
	}

	// 检查用户状态
	if user.Status != 1 {
		webauthnError(c, http.StatusForbidden, "verify_failed", "Login failed", "user status is not enabled")
		return
	}
	// 外部账号体系下通行密钥只给 root 应急登录；无用户名登录要到这里才知道是谁
	if !config.PasskeyAllowedForRole(user.Role) {
		webauthnError(c, http.StatusForbidden, "login_unavailable", "Login failed", "passkey not allowed for role")
		return
	}

	// 清除会话
	sess.Delete("webauthn_login_session")
	sess.Delete("webauthn_user_id")
	sess.Save()

	// 设置用户登录状态
	setupLoginSession(user, c, loginSessionInfo{method: model.SessionMethodPasskey})
}

// 获取用户的WebAuthn凭据列表
func GetUserWebAuthnCredentials(c *gin.Context) {
	userId := c.GetInt("id")

	var credentials []model.WebAuthnCredential
	model.DB.Where("user_id = ?", userId).Find(&credentials)

	// 只返回必要的信息，不包含私钥
	var result []gin.H
	for _, cred := range credentials {
		result = append(result, gin.H{
			"id":            cred.Id,
			"credential_id": cred.CredentialId,
			"alias":         cred.Alias,
			"created_time":  cred.CreatedTime,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// 删除WebAuthn凭据
func DeleteWebAuthnCredential(c *gin.Context) {
	userId := c.GetInt("id")

	credentialId := c.Param("id")
	if credentialId == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Credential ID is required",
			"success": false,
		})
		return
	}

	result := model.DB.Where("user_id = ? AND id = ?", userId, credentialId).Delete(&model.WebAuthnCredential{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to delete credential",
			"success": false,
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "Credential not found",
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Credential deleted",
	})
}
