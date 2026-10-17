package controller

import (
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type LinuxDoUser struct {
	Id         int    `json:"id"`
	Username   string `json:"username"`
	Name       string `json:"name"`
	Active     bool   `json:"active"`
	TrustLevel int    `json:"trust_level"`
	Silenced   bool   `json:"silenced"`
}

func LinuxDoBind(c *gin.Context) {
	if !config.EffectiveSocialLogin(config.LinuxDoOAuthEnabled) {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeOauthProviderDisabled, "The admin has not enabled sign-in and sign-up via LINUX DO", map[string]any{"provider": "LINUX DO"})
		return
	}

	// CSRF 防护：本处理器也挂在直连路由 /oauth/linuxdo/bind 上（绕过 LinuxDoOAuth 的 state 门），
	// 故必须自校验 oauth_state；否则攻击者可诱导已登录用户把攻击者的 LinuxDo 身份绑到其账号，
	// 进而以受害者身份登录。经 LinuxDoOAuth 进入时 state 已在 session/query 中，复检自然通过。
	session := sessions.Default(c)
	state := c.Query("state")
	if state == "" || session.Get("oauth_state") == nil || state != session.Get("oauth_state").(string) {
		common.APIRespondWithCode(c, http.StatusForbidden, common.APICodeOauthStateInvalid, "state is empty or not same", nil)
		return
	}

	// state 一次性使用：校验通过后立即失效，防止重放
	session.Delete("oauth_state")
	_ = session.Save()

	code := c.Query("code")
	linuxDoUser, err := getLinuxDoUserInfoByCode(code, c)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	if config.LinuxDoOAuthTrustLevelEnabled {
		if linuxDoUser.TrustLevel < config.LinuxDoOAuthLowestTrustLevel {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeLinuxdoTrustTooLow, "This LINUX DO account's trust level is too low to access", nil)
			return
		}
	}

	user := model.User{
		LinuxDoId: linuxDoUser.Id,
	}

	if model.IsLinuxDOIdAlreadyTaken(user.LinuxDoId) {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeOauthAccountLinked, "This LINUX DO account is already linked", map[string]any{"provider": "LINUX DO"})
		return
	}

	// UserAuth also accepts access tokens, which leave no id in the session, so do not assert the type.
	userId, ok := session.Get("id").(int)
	if !ok || userId == 0 {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeInvalidSession, "Session is invalid, please sign in again", nil)
		return
	}
	user.Id = userId

	err = user.FillUserById()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	// An account already linked to LINUX DO must be unlinked before linking another one.
	if user.LinuxDoId != 0 {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeLinuxdoAlreadyBound, "This account is already linked to LINUX DO, unlink it first to change", nil)
		return
	}

	user.LinuxDoId = linuxDoUser.Id
	user.LinuxDoUsername = linuxDoUser.Username
	user.LinuxDoTrustLevel = linuxDoUser.TrustLevel
	err = user.Update(false)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "bind",
	})
}

func getLinuxDoUserInfoByCode(code string, c *gin.Context) (*LinuxDoUser, error) {
	if code == "" {
		return nil, common.NewAPIError(common.APICodeOauthCodeInvalid, "invalid code", nil)
	}

	// Get access token using Basic auth
	tokenEndpoint := "https://connect.linux.do/oauth2/token"
	credentials := config.LinuxDoClientId + ":" + config.LinuxDoClientSecret
	basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(credentials))

	// Get redirect URI from request
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	redirectURI := fmt.Sprintf("%s://%s/api/oauth/linuxdo", scheme, c.Request.Host)

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)

	req, err := http.NewRequest("POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", basicAuth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, common.NewAPIError(common.APICodeOauthProviderUnreachable, "failed to connect to Linux DO server", map[string]any{"provider": "LINUX DO"})
	}
	defer res.Body.Close()

	var tokenRes struct {
		AccessToken string `json:"access_token"`
		Message     string `json:"message"`
	}
	if err := json.NewDecoder(res.Body).Decode(&tokenRes); err != nil {
		return nil, err
	}

	if tokenRes.AccessToken == "" {
		return nil, fmt.Errorf("failed to get access token: %s", tokenRes.Message)
	}

	// Get user info
	userEndpoint := "https://connect.linux.do/api/user"
	req, err = http.NewRequest("GET", userEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tokenRes.AccessToken)
	req.Header.Set("Accept", "application/json")

	res2, err := client.Do(req)
	if err != nil {
		return nil, common.NewAPIError(common.APICodeOauthInvalidResponse, "failed to get user info from Linux DO", nil)
	}
	defer res2.Body.Close()

	var linuxDoUser LinuxDoUser
	if err := json.NewDecoder(res2.Body).Decode(&linuxDoUser); err != nil {
		return nil, err
	}

	if linuxDoUser.Id == 0 {
		return nil, common.NewAPIError(common.APICodeOauthInvalidResponse, "invalid user info returned", nil)
	}

	return &linuxDoUser, nil
}

func LinuxDoOAuth(c *gin.Context) {
	session := sessions.Default(c)

	errorCode := c.Query("error")
	if errorCode != "" {
		errorDescription := c.Query("error_description")
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": errorDescription,
		})
		return
	}

	state := c.Query("state")
	if state == "" || session.Get("oauth_state") == nil || state != session.Get("oauth_state").(string) {
		common.APIRespondWithCode(c, http.StatusForbidden, common.APICodeOauthStateInvalid, "state is empty or not same", nil)
		return
	}

	username := session.Get("username")
	if username != nil {
		// 绑定分支的 state 由 LinuxDoBind 复检后再删除
		LinuxDoBind(c)
		return
	}

	// state 一次性使用：校验通过后立即失效，防止重放
	session.Delete("oauth_state")
	_ = session.Save()

	if !config.EffectiveSocialLogin(config.LinuxDoOAuthEnabled) {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeOauthProviderDisabled, "The admin has not enabled sign-in and sign-up via LINUX DO", map[string]any{"provider": "LINUX DO"})
		return
	}

	code := c.Query("code")
	linuxDoUser, err := getLinuxDoUserInfoByCode(code, c)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	user := model.User{
		LinuxDoId:         linuxDoUser.Id,
		LinuxDoUsername:   linuxDoUser.Username,
		LinuxDoTrustLevel: linuxDoUser.TrustLevel,
	}

	// 判断是否为已注册用户
	isExistingUser := model.IsLinuxDOIdAlreadyTaken(user.LinuxDoId)

	// 信任等级检查：动态限制开启或新用户注册时检查
	if config.LinuxDoOAuthTrustLevelEnabled {
		if config.LinuxDoOAuthDynamicTrustLevel || !isExistingUser {
			if linuxDoUser.TrustLevel < config.LinuxDoOAuthLowestTrustLevel {
				common.APIRespondWithCode(c, http.StatusOK, common.APICodeLinuxdoTrustTooLow, "This LINUX DO account's trust level is too low to access", nil)
				return
			}
		}
	}

	// Check if user exists
	if isExistingUser {
		err := user.FillUserByLinuxDOId()
		if err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}

		// update linux do user trust level
		if user.LinuxDoTrustLevel != linuxDoUser.TrustLevel {
			updateTrustLevelUser := model.User{
				Id:                user.Id,
				LinuxDoTrustLevel: linuxDoUser.TrustLevel,
			}

			err = updateTrustLevelUser.Update(false)
			if err != nil {
				common.APIRespondWithError(c, http.StatusOK, err)
				return
			}
		}

	} else {
		if config.RegisterEnabled {
			user.Username = "linuxdo_" + strconv.Itoa(model.GetMaxUserId()+1)
			if strings.TrimSpace(linuxDoUser.Name) == "" {
				user.DisplayName = linuxDoUser.Username
			} else {
				user.DisplayName = linuxDoUser.Name
			}
			user.Role = config.RoleCommonUser
			user.Status = config.UserStatusEnabled

			// 获取推荐码
			affCode := session.Get("aff")
			var affInviterId int
			if affCode != nil {
				affInviterId, _ = model.GetUserIdByAffCode(affCode.(string))
			}

			// 使用事务创建用户并处理邀请码
			err := model.DB.Transaction(func(tx *gorm.DB) error {
				// 验证和使用邀请码（如果启用）
				usedInviteCode, err := validateAndUseInviteCodeForOAuth(c, tx)
				if err != nil {
					return err
				}

				// 设置邀请人ID（使用原有推荐码逻辑）
				if affInviterId > 0 {
					user.InviterId = affInviterId
				}

				// 设置使用的邀请码
				if usedInviteCode != "" {
					user.UsedInviteCode = usedInviteCode
				}

				// 在事务中创建用户
				return user.InsertWithTx(tx, user.InviterId)
			})

			if err != nil {
				common.APIRespondWithError(c, http.StatusOK, err)
				return
			}
		} else {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeRegisterDisabled, "The admin has disabled new user sign-up", nil)
			return
		}
	}

	if user.Status != config.UserStatusEnabled {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeUserBanned, "User has been banned", nil)
		return
	}

	setupLoginSession(&user, c, loginSessionInfo{method: model.LoginMethodLinuxDo})
}
