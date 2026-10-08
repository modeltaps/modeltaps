package controller

import (
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type wechatLoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

func getWeChatIdByCode(code string) (string, error) {
	if code == "" {
		return "", errors.New("Invalid parameters")
	}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/wechat/user?code=%s", config.WeChatServerAddress, code), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", config.WeChatServerToken)
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	httpResponse, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer httpResponse.Body.Close()
	var res wechatLoginResponse
	err = json.NewDecoder(httpResponse.Body).Decode(&res)
	if err != nil {
		return "", err
	}
	if !res.Success {
		return "", errors.New(res.Message)
	}
	if res.Data == "" {
		return "", errors.New("Verification code is incorrect or has expired")
	}
	return res.Data, nil
}

func WeChatAuth(c *gin.Context) {
	if !config.EffectiveSocialLogin(config.WeChatAuthEnabled) {
		c.JSON(http.StatusOK, gin.H{
			"message": "The admin has not enabled sign-in and sign-up via WeChat",
			"success": false,
		})
		return
	}
	code := c.Query("code")
	wechatId, err := getWeChatIdByCode(code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	user := model.User{
		WeChatId: wechatId,
	}
	if model.IsWeChatIdAlreadyTaken(wechatId) {
		err := user.FillUserByWeChatId()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	} else {
		if config.RegisterEnabled {
			user.Username = "wechat_" + strconv.Itoa(model.GetMaxUserId()+1)
			user.DisplayName = "WeChat User"
			user.Role = config.RoleCommonUser
			user.Status = config.UserStatusEnabled

			// 使用事务创建用户并处理邀请码
			err := model.DB.Transaction(func(tx *gorm.DB) error {
				// 验证和使用邀请码（如果启用）
				usedInviteCode, err := validateAndUseInviteCodeForOAuth(c, tx)
				if err != nil {
					return err
				}

				// 设置使用的邀请码
				if usedInviteCode != "" {
					user.UsedInviteCode = usedInviteCode
				}

				// 在事务中创建用户
				return user.InsertWithTx(tx, 0)
			})

			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": err.Error(),
				})
				return
			}
		} else {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "The admin has disabled new user sign-up",
			})
			return
		}
	}

	if user.Status != config.UserStatusEnabled {
		c.JSON(http.StatusOK, gin.H{
			"message": "User has been banned",
			"success": false,
		})
		return
	}
	setupLoginSession(&user, c, loginSessionInfo{method: model.LoginMethodWeChat})
}

func WeChatBind(c *gin.Context) {
	if !config.EffectiveSocialLogin(config.WeChatAuthEnabled) {
		c.JSON(http.StatusOK, gin.H{
			"message": "The admin has not enabled sign-in and sign-up via WeChat",
			"success": false,
		})
		return
	}

	// CSRF 防护：本处理器挂在直连路由 /api/oauth/wechat/bind 上（无标准 OAuth redirect 流程做 state 门），
	// 故必须自校验 oauth_state；否则攻击者可诱导已登录用户把攻击者的微信身份绑到其账号，
	// 进而以受害者身份登录。前端绑定流程先调 /api/oauth/state 种下 state 再携带请求，复检自然通过。
	session := sessions.Default(c)
	state := c.Query("state")
	if state == "" || session.Get("oauth_state") == nil || state != session.Get("oauth_state").(string) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "state is empty or not same",
		})
		return
	}

	// state 一次性使用：校验通过后立即失效，防止重放
	session.Delete("oauth_state")
	_ = session.Save()

	code := c.Query("code")
	wechatId, err := getWeChatIdByCode(code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	if model.IsWeChatIdAlreadyTaken(wechatId) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "This WeChat account is already linked",
		})
		return
	}
	id := c.GetInt("id")
	user := model.User{
		Id: id,
	}
	err = user.FillUserById()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	user.WeChatId = wechatId
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
