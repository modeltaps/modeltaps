package controller

import (
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/stmp"
	"github.com/modeltaps/modeltaps/common/utils"
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
	// 密钥类配置不下发原值，只给回显：键齐全(未设置为空串)，前端据此识别密钥字段，
	// 留空保存表示不修改。
	secrets := make(map[string]string)
	for k, v := range config.GlobalOption.GetAllSecret() {
		secrets[k] = utils.MaskSecret(v)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    options,
		"secrets": secrets,
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
		common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeInvalidParams, "Invalid parameters", nil)
		return
	}
	switch option.Key {
	case "AccountSystem":
		if option.Value == config.AccountSystemExternal {
			provider, err := model.GetFirstPartyOidcProvider()
			if err != nil {
				common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionIdpReadFailed, "Failed to read identity providers: "+err.Error(), map[string]any{"reason": err.Error()})
				return
			}
			if provider == nil {
				common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionExternalRequiresSiteIdp, "Enable an identity provider and mark it as \"site identity\" before switching to external identity providers", nil)
				return
			}
		} else if !config.ValidAccountSystem(option.Value) {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionAccountSystemInvalid, "Account system must be builtin or external", nil)
			return
		}
	case "GitHubOAuthEnabled":
		if option.Value == "true" && config.GitHubClientId == "" {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionGithubOauthIncomplete, "Cannot enable GitHub OAuth: enter the GitHub Client Id and GitHub Client Secret first", nil)
			return
		}
	case "LinuxDoOAuthEnabled":
		if option.Value == "true" && (config.LinuxDoClientId == "" || config.LinuxDoClientSecret == "") {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionLinuxdoOauthIncomplete, "Cannot enable LINUX DO OAuth: enter the LINUX DO Client Id and LINUX DO Client Secret first", nil)
			return
		}
	case "LinuxDoOAuthTrustLevelEnabled":
		if option.Value == "true" && config.LinuxDoOAuthEnabled == false {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionLinuxdoTrustRequiresOauth, "Cannot enable the LINUX DO trust level limit: enable LINUX DO OAuth first", nil)
			return
		}
	case "LinuxDoOAuthDynamicTrustLevel":
		if option.Value == "true" && config.LinuxDoOAuthTrustLevelEnabled == false {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionLinuxdoDynamicRequiresLimit, "Cannot enable the dynamic trust level limit: enable the LINUX DO trust level limit first", nil)
			return
		}
	case "LinuxDoOAuthLowestTrustLevel":
		lowestTrustLevel, err := strconv.Atoi(option.Value)
		if err != nil {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionLinuxdoTrustNotNumber, "LINUX DO trust level must be a number", nil)
			return
		}
		if lowestTrustLevel < config.Basic || lowestTrustLevel > config.Leader {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionLinuxdoTrustOutOfRange, "LINUX DO trust level must be between 1 and 4", nil)
			return
		}

	case "SMTPTLSMode":
		if !stmp.IsValidTLSMode(option.Value) {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionSmtpEncryptionInvalid, "Invalid SMTP encryption method", nil)
			return
		}
	case "EmailDomainRestrictionEnabled":
		if option.Value == "true" && len(config.EmailDomainWhitelist) == 0 {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionEmailDomainsRequired, "Cannot enable the email domain restriction: enter the allowed email domains first", nil)
			return
		}
	case "WeChatAuthEnabled":
		if option.Value == "true" && config.WeChatServerAddress == "" {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionWechatIncomplete, "Cannot enable WeChat login: fill in the WeChat login settings first", nil)
			return
		}
	case "TurnstileCheckEnabled":
		if option.Value == "true" && config.TurnstileSiteKey == "" {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionTurnstileIncomplete, "Cannot enable Turnstile verification: fill in the Turnstile settings first", nil)
			return
		}
	case "QuotaForNewUser":
		value, err := strconv.Atoi(option.Value)
		if err != nil {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionQuotaNotInteger, "Initial quota for new users must be an integer", nil)
			return
		}
		if value < 0 {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeQuotaNegative, "Initial quota for new users cannot be negative", nil)
			return
		}
	case "QuotaForInviter":
		value, err := strconv.Atoi(option.Value)
		if err != nil {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionQuotaNotInteger, "Inviter quota reward must be an integer", nil)
			return
		}
		if value < 0 {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeQuotaNegative, "Inviter quota reward cannot be negative", nil)
			return
		}
	case "QuotaForInvitee":
		value, err := strconv.Atoi(option.Value)
		if err != nil {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionQuotaNotInteger, "Invitee quota reward must be an integer", nil)
			return
		}
		if value < 0 {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeQuotaNegative, "Invitee quota reward cannot be negative", nil)
			return
		}
	case "InviterRewardValue":
		value, err := strconv.Atoi(option.Value)
		if err != nil {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionTopupBonusNotNumber, "Top-up bonus value must be a valid number", nil)
			return
		}

		// 验证充值返利值的范围
		if config.InviterRewardType == "percentage" {
			// 百分比类型：值应在0-100之间
			if value < 0 || value > 100 {
				common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionTopupBonusPercentRange, "When the top-up bonus type is percentage, the bonus value must be between 0 and 100", nil)
				return
			}
		} else {
			// 固定类型：值应>=0
			if value < 0 {
				common.APIRespondWithCode(c, http.StatusOK, common.APICodeOptionTopupBonusFixedNegative, "When the top-up bonus type is fixed, the bonus value must be greater than or equal to 0", nil)
				return
			}
		}
	}
	err = model.UpdateOption(option.Key, option.Value)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}
