package controller

import (
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/stmp"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/safty"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetOptions(c *gin.Context) {
	var options []*model.Option
	for k, v := range config.GlobalOption.GetAllNonSecret() {
		options = append(options, &model.Option{
			Key:   k,
			Value: v,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    options,
	})
	return
}

func GetSafeTools(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    safty.GetAllSafeToolsName(),
	})
	return
}

func UpdateOption(c *gin.Context) {
	var option model.Option
	err := json.NewDecoder(c.Request.Body).Decode(&option)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid parameters",
		})
		return
	}
	switch option.Key {
	case "AccountSystem":
		if option.Value == config.AccountSystemExternal {
			provider, err := model.GetFirstPartyOidcProvider()
			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": "Failed to read identity providers: " + err.Error(),
				})
				return
			}
			if provider == nil {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": "Enable an identity provider and mark it as \"site identity\" before switching to external identity providers",
				})
				return
			}
		} else if !config.ValidAccountSystem(option.Value) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Account system must be builtin or external",
			})
			return
		}
	case "GitHubOAuthEnabled":
		if option.Value == "true" && config.GitHubClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Cannot enable GitHub OAuth: enter the GitHub Client Id and GitHub Client Secret first",
			})
			return
		}
	case "LinuxDoOAuthEnabled":
		if option.Value == "true" && (config.LinuxDoClientId == "" || config.LinuxDoClientSecret == "") {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Cannot enable LINUX DO OAuth: enter the LINUX DO Client Id and LINUX DO Client Secret first",
			})
			return
		}
	case "LinuxDoOAuthTrustLevelEnabled":
		if option.Value == "true" && config.LinuxDoOAuthEnabled == false {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Cannot enable the LINUX DO trust level limit: enable LINUX DO OAuth first",
			})
			return
		}
	case "LinuxDoOAuthDynamicTrustLevel":
		if option.Value == "true" && config.LinuxDoOAuthTrustLevelEnabled == false {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Cannot enable the dynamic trust level limit: enable the LINUX DO trust level limit first",
			})
			return
		}
	case "LinuxDoOAuthLowestTrustLevel":
		lowestTrustLevel, err := strconv.Atoi(option.Value)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "LINUX DO trust level must be a number",
			})
			return
		}
		if lowestTrustLevel < config.Basic || lowestTrustLevel > config.Leader {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "LINUX DO trust level must be between 1 and 4",
			})
			return
		}

	case "SMTPTLSMode":
		if !stmp.IsValidTLSMode(option.Value) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Invalid SMTP encryption method",
			})
			return
		}
	case "EmailDomainRestrictionEnabled":
		if option.Value == "true" && len(config.EmailDomainWhitelist) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Cannot enable the email domain restriction: enter the allowed email domains first",
			})
			return
		}
	case "WeChatAuthEnabled":
		if option.Value == "true" && config.WeChatServerAddress == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Cannot enable WeChat login: fill in the WeChat login settings first",
			})
			return
		}
	case "TurnstileCheckEnabled":
		if option.Value == "true" && config.TurnstileSiteKey == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Cannot enable Turnstile verification: fill in the Turnstile settings first",
			})
			return
		}
	case "QuotaForNewUser":
		value, err := strconv.Atoi(option.Value)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Initial quota for new users must be an integer",
			})
			return
		}
		if value < 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Initial quota for new users cannot be negative",
			})
			return
		}
	case "QuotaForInviter":
		value, err := strconv.Atoi(option.Value)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Inviter quota reward must be an integer",
			})
			return
		}
		if value < 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Inviter quota reward cannot be negative",
			})
			return
		}
	case "QuotaForInvitee":
		value, err := strconv.Atoi(option.Value)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Invitee quota reward must be an integer",
			})
			return
		}
		if value < 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Invitee quota reward cannot be negative",
			})
			return
		}
	case "InviterRewardValue":
		value, err := strconv.Atoi(option.Value)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "Top-up bonus value must be a valid number",
			})
			return
		}

		// 验证充值返利值的范围
		if config.InviterRewardType == "percentage" {
			// 百分比类型：值应在0-100之间
			if value < 0 || value > 100 {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": "When the top-up bonus type is percentage, the bonus value must be between 0 and 100",
				})
				return
			}
		} else {
			// 固定类型：值应>=0
			if value < 0 {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": "When the top-up bonus type is fixed, the bonus value must be greater than or equal to 0",
				})
				return
			}
		}
	}
	err = model.UpdateOption(option.Key, option.Value)
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
	return
}
