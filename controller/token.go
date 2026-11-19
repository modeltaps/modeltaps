package controller

import (
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetUserTokensList(c *gin.Context) {
	userId := c.GetInt("id")
	userRole := c.GetInt("role")
	var params model.TokenListParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	tokens, statusCounts, err := model.ListUserTokens(userId, &params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	// 对于非可信用户，隐藏 BillingTag 字段
	if userRole < config.RoleReliableUser {
		for _, token := range *tokens.Data {
			setting := token.Setting.Data()
			setting.BillingTag = nil
			token.Setting.Set(setting)
		}
	}

	// 附带当日(TZ 零点起)按令牌名聚合的已用额度,与列表平级。
	// 聚合失败不阻断列表返回(additive,避免 used_quota/remain_quota 回归),仅记录日志并返回空映射。
	todayUsage, err := model.GetUserTodayUsageByToken(userId)
	if err != nil {
		logger.SysError("failed to aggregate today token usage: " + err.Error())
		todayUsage = map[string]int64{}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "",
		"data":          tokens,
		"today_usage":   todayUsage,
		"status_counts": statusCounts,
	})
}

// GetTokensListByAdmin 管理员查询令牌列表（可按用户ID或令牌ID查询）
func GetTokensListByAdmin(c *gin.Context) {
	var params model.AdminSearchTokensParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	tokens, err := model.GetTokensListByAdmin(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    tokens,
	})
}

// GetTokenModels 返回令牌实际可调用的模型(口径同 /v1/models),供 Key 详情展示
func GetTokenModels(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	token, err := model.GetTokenByIds(id, c.GetInt("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	respondTokenModels(c, token)
}

func respondTokenModels(c *gin.Context, token *model.Token) {
	models, err := model.TokenAvailableModels(token)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    models,
	})
}

func GetToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	userId := c.GetInt("id")
	userRole := c.GetInt("role")
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	token, err := model.GetTokenByIds(id, userId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	// 对于非可信用户，隐藏 BillingTag 字段
	if userRole < config.RoleReliableUser {
		setting := token.Setting.Data()
		setting.BillingTag = nil
		token.Setting.Set(setting)
	}
	model.AttachAgentInfo(token)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    token,
	})
}

// IssuePlaygroundKey gives the signed-in user a short-lived key for the console playground (see
// model.PlaygroundKey): it spends on the user's playground API key and expires after an hour.
// The long-lived API keys themselves cannot be read back.
func IssuePlaygroundKey(c *gin.Context) {
	key, expiresAt, err := model.IssuePlaygroundKey(c.GetInt("id"))
	if err != nil {
		logger.SysError("failed to issue a playground key: " + err.Error())
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeTokenChatCreateFailed, "Failed to create the chat API key, please try again later or contact the admin", nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"key":        key,
			"expires_at": expiresAt,
		},
	})
}

// respondNewTokenKey answers a request that created or reset an API key: the only response that
// carries the key itself (without "sk-"). It cannot be read back later.
func respondNewTokenKey(c *gin.Context, token *model.Token) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"id":       token.Id,
			"key":      token.Key,
			"key_hint": token.KeyHint,
		},
	})
}

// ResetTokenKey gives one of the user's API keys a new secret and returns it once; the old key
// stops working at once. The playground's key is managed by the system and cannot be reset.
func ResetTokenKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	token, err := model.GetTokenByIds(id, c.GetInt("id"))
	if err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeTokenNotFound, "API key not found", nil)
		return
	}
	resetTokenKey(c, token)
}

func resetTokenKey(c *gin.Context, token *model.Token) {
	if token.Name == model.PlaygroundTokenName {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeTokenKeyResetManaged, "The playground API key is managed by the system and cannot be reset", nil)
		return
	}
	if _, err := token.ResetKey(); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	respondNewTokenKey(c, token)
}

// reservedTokenNamePrefix marks names the system gives its own keys (the playground key); users
// cannot take them.
const reservedTokenNamePrefix = "sys_"

// checkTokenName returns why name cannot be stored for an API key of ownerId, or nil: it uses the
// reserved prefix or another key of the same owner already has it. Usage, logs and today's spend
// group keys by name, so names are unique per owner.
func checkTokenName(name string, ownerId, selfId int) error {
	if strings.HasPrefix(name, reservedTokenNamePrefix) {
		return common.NewAPIError(common.APICodeTokenNameReserved, "API key names starting with sys_ are reserved", nil)
	}
	taken, err := model.TokenNameTaken(ownerId, name, selfId)
	if err != nil {
		return err
	}
	if taken {
		return common.NewAPIError(common.APICodeTokenNameTaken, "An API key with this name already exists", nil)
	}
	return nil
}

// rejectTokenName answers the request and returns true when name cannot be stored for an API key
// of ownerId (checkTokenName). Saving a key whose name and owner did not change is always allowed,
// so keys that already share a name keep working.
func rejectTokenName(c *gin.Context, name string, ownerId, selfId int, unchanged bool) bool {
	if unchanged {
		return false
	}
	if err := checkTokenName(name, ownerId, selfId); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return true
	}
	return false
}

func AddToken(c *gin.Context) {
	userId := c.GetInt("id")
	token := model.Token{}
	if err := c.ShouldBindJSON(&token); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	cleanToken, err := newPersonalToken(userId, c.GetInt("role"), &token)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if err := cleanToken.Insert(); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	respondCreatedTokenKey(c, cleanToken, userId)
}

// newPersonalToken checks what a user asked for in a new API key of their own and returns the key
// to insert. Shared by the console's create and the device flow's approval (AGENT-7).
func newPersonalToken(userId, userRole int, token *model.Token) (*model.Token, error) {
	if len(token.Name) > 30 {
		return nil, common.NewAPIError(common.APICodeTokenNameTooLong, "API key name is too long", nil)
	}
	if err := checkTokenName(token.Name, userId, 0); err != nil {
		return nil, err
	}
	if token.Group != "" {
		if err := validateTokenGroup(token.Group, userId); err != nil {
			return nil, err
		}
	}
	if token.BackupGroup != "" {
		if err := validateTokenGroup(token.BackupGroup, userId); err != nil {
			return nil, err
		}
	}

	setting := token.Setting.Data()
	if err := validatePersonalTokenSetting(&setting); err != nil {
		return nil, err
	}
	// 非可信用户不能设置 BillingTag
	if userRole < config.RoleReliableUser {
		setting.BillingTag = nil
	}

	cleanToken := &model.Token{
		UserId:         userId,
		Name:           token.Name,
		CreatedTime:    utils.GetTimestamp(),
		AccessedTime:   utils.GetTimestamp(),
		ExpiredTime:    token.ExpiredTime,
		RemainQuota:    token.RemainQuota,
		UnlimitedQuota: token.UnlimitedQuota,
		Group:          token.Group,
		BackupGroup:    token.BackupGroup,
		LogIO:          personalTokenLogIO(userId, token.LogIO),
	}
	cleanToken.Setting.Set(setting)
	return cleanToken, nil
}

func DeleteToken(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userId := c.GetInt("id")
	err := model.DeleteTokenById(id, userId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func UpdateToken(c *gin.Context) {
	userId := c.GetInt("id")
	userRole := c.GetInt("role")
	statusOnly := c.Query("status_only")
	token := model.Token{}
	err := c.ShouldBindJSON(&token)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if len(token.Name) > 30 {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeTokenNameTooLong, "API key name is too long", nil)
		return
	}

	newSetting := token.Setting.Data()
	err = validatePersonalTokenSetting(&newSetting)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	cleanToken, err := model.GetTokenByIds(token.Id, userId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if token.Status == config.TokenStatusEnabled {
		if cleanToken.Status == config.TokenStatusExpired && cleanToken.ExpiredTime <= utils.GetTimestamp() && cleanToken.ExpiredTime != -1 {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeTokenExpiredCannotEnable, "API key has expired and cannot be enabled. Change its expiration time or set it to never expire first.", nil)
			return
		}
		if cleanToken.Status == config.TokenStatusExhausted && cleanToken.RemainQuota <= 0 && !cleanToken.UnlimitedQuota {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeTokenQuotaExhaustedCannotEnable, "API key quota is exhausted and it cannot be enabled. Change its remaining quota or set it to unlimited first.", nil)
			return
		}
	}

	if cleanToken.Group != token.Group && token.Group != "" {
		err = validateTokenGroup(token.Group, userId)
		if err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
	}
	if cleanToken.BackupGroup != token.BackupGroup && token.BackupGroup != "" {
		err = validateTokenGroup(token.BackupGroup, userId)
		if err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
	}

	if statusOnly == "" && rejectTokenName(c, token.Name, cleanToken.UserId, cleanToken.Id, token.Name == cleanToken.Name) {
		return
	}

	if statusOnly != "" {
		cleanToken.Status = token.Status
	} else {
		// If you add more fields, please also update token.Update()
		cleanToken.Name = token.Name
		cleanToken.ExpiredTime = token.ExpiredTime
		cleanToken.UnlimitedQuota = token.UnlimitedQuota
		// 无限额度令牌没有上限概念，保留原 remain_quota，避免误填值覆盖掉用户真实额度（再切回有限额时仍可用）
		if !token.UnlimitedQuota {
			cleanToken.RemainQuota = token.RemainQuota
		}
		cleanToken.Group = token.Group
		cleanToken.BackupGroup = token.BackupGroup
		// The retention switch is kept when the request does not send it (ORG-10).
		if token.LogIO != nil {
			cleanToken.LogIO = token.LogIO
		}

		// 处理 BillingTag: 非可信用户保持原值不变
		oldSetting := cleanToken.Setting.Data()
		if userRole < config.RoleReliableUser {
			// 非可信用户：保持原来的 BillingTag，忽略前端传入的值
			newSetting.BillingTag = oldSetting.BillingTag
		}
		// 可信用户：直接使用前端传入的值（包括空值，用于清除 BillingTag）

		// 周期计数为服务端维护的专用列(普通更新不触碰故自动保留);仅 period 配置变化时清零
		if model.PeriodConfigChanged(&newSetting, &oldSetting) {
			if err := model.ResetTokenPeriodCounters(cleanToken.Id); err != nil {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
				return
			}
		}

		cleanToken.Setting.Set(newSetting)
	}
	err = cleanToken.Update()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	// 对于非可信用户，返回数据时隐藏 BillingTag 字段
	if userRole < config.RoleReliableUser {
		responseSetting := cleanToken.Setting.Data()
		responseSetting.BillingTag = nil
		cleanToken.Setting.Set(responseSetting)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    cleanToken,
	})
}

// DeleteTokenByAdmin 管理员删除任意token
func DeleteTokenByAdmin(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	err := model.DeleteTokenByIdAdmin(id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// UpdateTokenByAdmin 管理员更新任意token（支持转移用户）
func UpdateTokenByAdmin(c *gin.Context) {
	statusOnly := c.Query("status_only")
	token := model.Token{}
	err := c.ShouldBindJSON(&token)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if len(token.Name) > 30 {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeTokenNameTooLong, "API key name is too long", nil)
		return
	}

	newSetting := token.Setting.Data()
	err = validateTokenSetting(&newSetting)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	cleanToken, err := model.GetTokenById(token.Id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	// Named access lists belong to organizations (ORG-15); the site console cannot change the attachment
	newSetting.Limits.LimitModelSetting.AccessListId = cleanToken.Setting.Data().Limits.LimitModelSetting.AccessListId

	if token.Status == config.TokenStatusEnabled {
		if cleanToken.Status == config.TokenStatusExpired && cleanToken.ExpiredTime <= utils.GetTimestamp() && cleanToken.ExpiredTime != -1 {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeTokenExpiredCannotEnable, "API key has expired and cannot be enabled. Change its expiration time or set it to never expire first.", nil)
			return
		}
		if cleanToken.Status == config.TokenStatusExhausted && cleanToken.RemainQuota <= 0 && !cleanToken.UnlimitedQuota {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeTokenQuotaExhaustedCannotEnable, "API key quota is exhausted and it cannot be enabled. Change its remaining quota or set it to unlimited first.", nil)
			return
		}
	}

	// 验证目标用户是否存在（如果要转移token）
	if token.UserId > 0 && token.UserId != cleanToken.UserId {
		targetUser, err := model.GetUserById(token.UserId, false)
		if err != nil || targetUser == nil {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeUserNotFound, "Target user not found", nil)
			return
		}
	}

	// 验证用户组（使用目标用户ID）
	targetUserId := cleanToken.UserId
	if token.UserId > 0 {
		targetUserId = token.UserId
	}

	if cleanToken.Group != token.Group && token.Group != "" {
		err = validateTokenGroupForUser(token.Group, targetUserId)
		if err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
	}
	if cleanToken.BackupGroup != token.BackupGroup && token.BackupGroup != "" {
		err = validateTokenGroupForUser(token.BackupGroup, targetUserId)
		if err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
	}

	if statusOnly == "" && rejectTokenName(c, token.Name, targetUserId, cleanToken.Id, token.Name == cleanToken.Name && targetUserId == cleanToken.UserId) {
		return
	}

	if statusOnly != "" {
		cleanToken.Status = token.Status
	} else {
		cleanToken.Name = token.Name
		cleanToken.ExpiredTime = token.ExpiredTime
		cleanToken.UnlimitedQuota = token.UnlimitedQuota
		// 无限额度令牌没有上限概念，保留原 remain_quota，避免误填值覆盖掉用户真实额度（再切回有限额时仍可用）
		if !token.UnlimitedQuota {
			cleanToken.RemainQuota = token.RemainQuota
		}
		cleanToken.Group = token.Group
		cleanToken.BackupGroup = token.BackupGroup
		// The retention switch is kept when the request does not send it (ORG-10). An organization
		// key's switch belongs to the organization's policy and audit, so it is not changed here.
		if token.LogIO != nil && cleanToken.CreatedBy == 0 {
			cleanToken.LogIO = token.LogIO
		}

		// 周期计数为服务端维护的专用列(普通更新不触碰故自动保留);仅 period 配置变化时清零
		oldSetting := cleanToken.Setting.Data()
		if model.PeriodConfigChanged(&newSetting, &oldSetting) {
			if err := model.ResetTokenPeriodCounters(cleanToken.Id); err != nil {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
				return
			}
		}
		cleanToken.Setting.Set(newSetting)

		// 管理员可以转移token给其他用户
		if token.UserId > 0 {
			cleanToken.UserId = token.UserId
		}
	}

	err = cleanToken.UpdateByAdmin()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    cleanToken,
	})
}

// validateTokenGroupForUser 验证用户组是否对指定用户有效
func validateTokenGroupForUser(tokenGroup string, userId int) error {
	userGroup, _ := model.CacheGetUserGroup(userId)
	if userGroup == "" {
		return common.NewAPIError(common.APICodeUserGroupLoadFailed, "Failed to get user group info", nil)
	}

	groupRatio := model.GlobalUserGroupRatio.GetBySymbol(tokenGroup)
	if groupRatio == nil {
		return common.NewAPIError(common.APICodeUserGroupInvalid, "Invalid user group", nil)
	}

	if !groupRatio.Public && userGroup != tokenGroup {
		return common.NewAPIError(common.APICodeUserGroupNotAllowed, "Target user is not allowed to use the specified group", nil)
	}

	return nil
}

func validateTokenGroup(tokenGroup string, userId int) error {
	userGroup, _ := model.CacheGetUserGroup(userId)
	if userGroup == "" {
		return common.NewAPIError(common.APICodeUserGroupLoadFailed, "Failed to get user group info", nil)
	}

	groupRatio := model.GlobalUserGroupRatio.GetBySymbol(tokenGroup)
	if groupRatio == nil {
		return common.NewAPIError(common.APICodeUserGroupInvalid, "Invalid user group", nil)
	}

	if !groupRatio.Public && userGroup != tokenGroup {
		return common.NewAPIError(common.APICodeUserGroupNotAllowed, "Current user group is not allowed to use the specified group", nil)
	}

	return nil
}

// validatePersonalTokenSetting: named access lists belong to organizations, personal keys cannot use them.
func validatePersonalTokenSetting(setting *model.TokenSetting) error {
	if setting != nil && setting.Limits.LimitModelSetting.AccessListId > 0 {
		return common.NewAPIError(common.APICodeModelAccessPersonalToken, "named access lists are only available to organization API keys", nil)
	}
	return validateTokenSetting(setting)
}

func validateTokenSetting(setting *model.TokenSetting) error {
	if setting == nil {
		return nil
	}

	if setting.Heartbeat.Enabled {
		if setting.Heartbeat.TimeoutSeconds < 30 || setting.Heartbeat.TimeoutSeconds > 90 {
			return common.NewAPIError(common.APICodeTokenHeartbeatRange, "heartbeat timeout seconds must be between 30 and 90", nil)
		}
	}

	mode, ok := config.NormalizePromptCacheMode(setting.PromptCache)
	if !ok {
		return common.NewAPIError(common.APICodeTokenPromptCacheInvalid, "prompt_cache must be auto, on or off", nil)
	}
	// auto is stored as the empty string, the value keys made before the setting existed carry.
	if mode == config.PromptCacheModeAuto {
		mode = ""
	}
	setting.PromptCache = mode

	if rs := setting.QuotaReset; rs != nil {
		switch rs.Period {
		case model.TokenQuotaResetPeriodDaily, model.TokenQuotaResetPeriodWeekly, model.TokenQuotaResetPeriodMonthly:
		default:
			return common.NewAPIError(common.APICodeTokenQuotaResetPeriodInvalid, "quota_reset period must be daily, weekly or monthly", nil)
		}
		if rs.Limit <= 0 {
			return common.NewAPIError(common.APICodeTokenQuotaResetLimitInvalid, "quota_reset limit must be greater than 0", nil)
		}
	}

	return nil
}

// personalTokenLogIO is the retention switch a new personal API key is saved with: the requested
// one, or the user's default for new keys (ORG-10).
func personalTokenLogIO(userId int, requested *bool) *bool {
	if requested != nil {
		value := *requested
		return &value
	}
	value := false
	if setting, err := model.GetUserSettingById(userId); err == nil {
		value = setting.NewKeyLogIO()
	}
	return &value
}
