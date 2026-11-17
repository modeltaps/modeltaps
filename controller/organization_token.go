package controller

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// getOrgTokenForOperator 取组织令牌并校验操作权限:
// 令牌必须归属该组织影子账户;Member 仅能操作自己创建的令牌,Owner/Admin 可操作全部(规格 §3.4)。
func getOrgTokenForOperator(c *gin.Context, tokenId int) (*model.Token, error) {
	org, role := getOrgFromContext(c)
	token, err := model.GetTokenByIds(tokenId, org.ShadowUserId)
	if err != nil {
		return nil, common.NewAPIError(common.APICodeTokenNotFound, "API key not found", nil)
	}
	if role == model.OrgRoleMember && token.CreatedBy != c.GetInt("id") {
		return nil, common.NewAPIError(common.APICodeTokenAccessDenied, "Not allowed to operate on this API key", nil)
	}
	return token, nil
}

// orgLogIODisclosure tells the members of an organization whether the request and response bodies
// of its API keys are being stored, who turned that on and when (SEC-51). The organization default
// is the only switch (the key-level log_io is not read), so the answer is the same for every key.
type orgLogIODisclosure struct {
	Stored bool `json:"stored"`
	// Source is "organization" when the organization turned it on, "site" when the organization
	// follows the site default and the site stores bodies.
	Source      string `json:"source,omitempty"`
	ChangedBy   string `json:"changed_by,omitempty"`
	ChangedById int    `json:"changed_by_id,omitempty"`
	ChangedAt   int64  `json:"changed_at,omitempty"`
	// RetentionDays is how long stored bodies are kept.
	RetentionDays int `json:"retention_days,omitempty"`
}

// Values of orgLogIODisclosure.Source.
const (
	orgLogIOSourceOrganization = "organization"
	orgLogIOSourceSite         = "site"
)

// orgLogIOActive reports whether request and response bodies of the organization's API keys are
// stored now (the site switch and the organization default, as model.ResolveTokenLogIO resolves it).
func orgLogIOActive(org *model.Organization) bool {
	return config.LogIOEnabled && org.ResolveLogIODefault()
}

// orgLogIOStatus is the disclosure shown with the organization's API keys.
func orgLogIOStatus(org *model.Organization) orgLogIODisclosure {
	if !orgLogIOActive(org) {
		return orgLogIODisclosure{}
	}
	status := orgLogIODisclosure{Stored: true, Source: orgLogIOSourceSite, RetentionDays: model.LogIORetentionDays}
	if org.Setting.Data().LogIODefault == nil {
		return status
	}
	status.Source = orgLogIOSourceOrganization
	entry, err := model.GetLatestOrgLogIOUpdate(org.Id)
	if err != nil {
		logger.SysError(fmt.Sprintf("failed to read the LogIO change of organization %d: %v", org.Id, err))
		return status
	}
	if entry != nil {
		status.ChangedBy = entry.ActorName
		status.ChangedById = entry.ActorId
		status.ChangedAt = entry.CreatedTime
	}
	return status
}

// recordOrgLogIOUpdate records a change of the organization's LogIO default in the audit log under
// its own action, so the disclosure can name who changed it and when. Turning storage on is also
// written to the system log.
func recordOrgLogIOUpdate(org *model.Organization, actorId int, value string, wasStored bool) {
	stored := orgLogIOActive(org)
	model.RecordOrgAudit(org.Id, actorId, model.OrgAuditActionLogIOUpdate,
		fmt.Sprintf("Set request and response retention of organization API keys to %s (bodies stored: %t)", value, stored))
	if stored && !wasStored {
		logger.SysLog(fmt.Sprintf("organization %d: user %d turned on storing request and response bodies of organization API keys", org.Id, actorId))
	}
}

// GetOrgTokensList 组织令牌列表:Member 仅自己创建的,Owner/Admin 全部
func GetOrgTokensList(c *gin.Context) {
	org, role := getOrgFromContext(c)
	var params model.TokenListParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	createdBy := 0
	if role == model.OrgRoleMember {
		createdBy = c.GetInt("id")
	}
	// created_by=-1 lists the keys the organization owns (ORG-16); the shadow account id stays
	// on the server.
	if params.CreatedBy == -1 {
		params.CreatedBy = org.ShadowUserId
	}
	tokens, statusCounts, err := model.ListOrgTokens(org.ShadowUserId, createdBy, &params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	// 对于非可信用户，隐藏 BillingTag 字段
	if c.GetInt("role") < config.RoleReliableUser {
		for _, token := range *tokens.Data {
			setting := token.Setting.Data()
			setting.BillingTag = nil
			token.Setting.Set(setting)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "",
		"data":          tokens,
		"status_counts": statusCounts,
		"log_io":        orgLogIOStatus(org),
	})
}

// GetOrgToken 组织令牌详情(Member 仅自己创建的)
func GetOrgToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("token_id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	token, err := getOrgTokenForOperator(c, id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	// 对于非可信用户，隐藏 BillingTag 字段
	if c.GetInt("role") < config.RoleReliableUser {
		setting := token.Setting.Data()
		setting.BillingTag = nil
		token.Setting.Set(setting)
	}
	model.AttachAgentInfo(token)

	org, _ := getOrgFromContext(c)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    token,
		"log_io":  orgLogIOStatus(org),
	})
}

// GetOrgTokenModels 组织令牌实际可调用的模型(含成员白名单),可见范围同 GetOrgToken
func GetOrgTokenModels(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("token_id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	token, err := getOrgTokenForOperator(c, id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	respondTokenModels(c, token)
}

// Owners of an organization API key, as the create request names them (ORG-16).
const (
	orgTokenOwnerMember       = "member"
	orgTokenOwnerOrganization = "organization"
)

// addOrgTokenRequest is a new organization API key plus who owns it: the member creating it (the
// default) or the organization, whose keys keep working when that member leaves.
type addOrgTokenRequest struct {
	model.Token
	Owner string `json:"owner"`
}

// AddOrgToken 组织上下文创建令牌:user_id 指向影子账户,created_by 记录实际创建成员;
// 组织所有的 key(ORG-16)created_by 为影子账户,仅 Owner/Admin 可创建
func AddOrgToken(c *gin.Context) {
	org, operatorRole := getOrgFromContext(c)
	userId := c.GetInt("id")
	req := addOrgTokenRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	orgOwned := false
	switch req.Owner {
	case "", orgTokenOwnerMember:
	case orgTokenOwnerOrganization:
		if !isOrgAdmin(operatorRole) {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeTokenOrgOwnedAdminOnly, "Only organization owners and admins can manage API keys owned by the organization", nil)
			return
		}
		orgOwned = true
	default:
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeInvalidParams, "owner must be member or organization", nil)
		return
	}
	creatorId := userId
	if orgOwned {
		creatorId = org.ShadowUserId
	}
	cleanToken, err := newOrgToken(org, creatorId, operatorRole, c.GetInt("role"), &req.Token)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if err := cleanToken.Insert(); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	detail := fmt.Sprintf("Created organization API key %s (ID: %d)", cleanToken.Name, cleanToken.Id)
	if orgOwned {
		detail = fmt.Sprintf("Created organization API key %s (ID: %d), owned by the organization", cleanToken.Name, cleanToken.Id)
	}
	model.RecordOrgAudit(org.Id, userId, "token.create", detail)
	// Same shape as the personal endpoint: the console shows the new key once, right away.
	respondCreatedTokenKey(c, cleanToken, userId)
}

// newOrgToken checks what a member asked for in a new organization API key and returns the key to
// insert: owned by the organization's shadow account, created_by the member, within the member's
// model access (ORG-15). Shared by the console's create and the device flow's approval (AGENT-7).
// userId is the shadow account for a key the organization owns (ORG-16): only the organization
// layers of model access apply to it.
func newOrgToken(org *model.Organization, userId int, operatorRole string, userRole int, token *model.Token) (*model.Token, error) {
	if len(token.Name) > 30 {
		return nil, common.NewAPIError(common.APICodeTokenNameTooLong, "API key name is too long", nil)
	}
	if err := checkTokenName(token.Name, org.ShadowUserId, 0); err != nil {
		return nil, err
	}

	// 分组校验以影子账户(组织)的用户组为准
	if token.Group != "" {
		if err := validateTokenGroupForUser(token.Group, org.ShadowUserId); err != nil {
			return nil, err
		}
	}
	if token.BackupGroup != "" {
		if err := validateTokenGroupForUser(token.BackupGroup, org.ShadowUserId); err != nil {
			return nil, err
		}
	}

	setting := token.Setting.Data()
	if err := validateTokenSetting(&setting); err != nil {
		return nil, err
	}
	if err := model.ValidateOrgTokenModelAccess(org, userId, operatorRole, &setting.Limits.LimitModelSetting); err != nil {
		return nil, err
	}

	// 非可信用户不能设置 BillingTag
	if userRole < config.RoleReliableUser {
		setting.BillingTag = nil
	}

	cleanToken := &model.Token{
		UserId:         org.ShadowUserId,
		CreatedBy:      userId,
		Name:           token.Name,
		CreatedTime:    utils.GetTimestamp(),
		AccessedTime:   utils.GetTimestamp(),
		ExpiredTime:    token.ExpiredTime,
		RemainQuota:    token.RemainQuota,
		UnlimitedQuota: token.UnlimitedQuota,
		Group:          token.Group,
		BackupGroup:    token.BackupGroup,
	}
	cleanToken.Setting.Set(setting)
	return cleanToken, nil
}

// ResetOrgTokenKey gives an organization API key a new secret and returns it once. Only the
// member who created the key may reset it, owners and admins included: resetting hands over a
// working key, and nobody gets to see another member's key (SEC-47). Others can still disable
// or delete it.
func ResetOrgTokenKey(c *gin.Context) {
	org, _ := getOrgFromContext(c)
	id, err := strconv.Atoi(c.Param("token_id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	token, err := getOrgTokenForOperator(c, id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	userId := c.GetInt("id")
	if !canHandOverOrgToken(c, token) {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeTokenKeyResetNotCreator, "Only the member who created this API key can reset it", nil)
		return
	}
	if _, err := token.ResetKey(); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	model.RecordOrgAudit(org.Id, userId, "token.reset_key", fmt.Sprintf("Reset the secret of organization API key %s (ID: %d)", token.Name, token.Id))
	respondNewTokenKey(c, token)
}

// UpdateOrgToken 更新组织令牌(Member 仅自己创建的;分组校验以影子账户为准)
func UpdateOrgToken(c *gin.Context) {
	org, _ := getOrgFromContext(c)
	userRole := c.GetInt("role")
	statusOnly := c.Query("status_only")
	token := model.Token{}
	if err := c.ShouldBindJSON(&token); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if len(token.Name) > 30 {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeTokenNameTooLong, "API key name is too long", nil)
		return
	}

	newSetting := token.Setting.Data()
	if err := validateTokenSetting(&newSetting); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	cleanToken, err := getOrgTokenForOperator(c, token.Id)
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
		if err := validateTokenGroupForUser(token.Group, org.ShadowUserId); err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
	}
	if cleanToken.BackupGroup != token.BackupGroup && token.BackupGroup != "" {
		if err := validateTokenGroupForUser(token.BackupGroup, org.ShadowUserId); err != nil {
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
		cleanToken.RemainQuota = token.RemainQuota
		cleanToken.UnlimitedQuota = token.UnlimitedQuota
		cleanToken.Group = token.Group
		cleanToken.BackupGroup = token.BackupGroup
		// log_io is not taken from the request: retention follows the organization default only
		// (SEC-51), so nobody can switch it on for one member's key without the members being told.

		// 处理 BillingTag: 非可信用户保持原值不变
		oldSetting := cleanToken.Setting.Data()

		// The model limit must stay inside the approved models and the creator's member layer (ORG-15).
		// Only checked when the limit changed, so after a layer above is narrowed other edits (name, quota)
		// still save; entries outside are refused at request time and shown as not in effect.
		if model.LimitModelSettingChanged(oldSetting.Limits.LimitModelSetting, newSetting.Limits.LimitModelSetting) {
			_, operatorRole := getOrgFromContext(c)
			if err := model.ValidateOrgTokenModelAccess(org, cleanToken.CreatedBy, operatorRole, &newSetting.Limits.LimitModelSetting); err != nil {
				common.APIRespondWithError(c, http.StatusOK, err)
				return
			}
		}
		if userRole < config.RoleReliableUser {
			newSetting.BillingTag = oldSetting.BillingTag
		}

		// 周期计数为服务端维护的专用列(普通更新不触碰故自动保留);仅 period 配置变化时清零
		if model.PeriodConfigChanged(&newSetting, &oldSetting) {
			if err := model.ResetTokenPeriodCounters(cleanToken.Id); err != nil {
				common.APIRespondWithError(c, http.StatusOK, err)
				return
			}
		}

		cleanToken.Setting.Set(newSetting)
	}
	if err := cleanToken.Update(); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	model.RecordOrgAudit(org.Id, c.GetInt("id"), "token.update", fmt.Sprintf("Updated organization API key %s (ID: %d)", cleanToken.Name, cleanToken.Id))

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

// DeleteOrgToken 删除组织令牌(Member 仅自己创建的;产生审计记录)
func DeleteOrgToken(c *gin.Context) {
	org, _ := getOrgFromContext(c)
	id, err := strconv.Atoi(c.Param("token_id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	token, err := getOrgTokenForOperator(c, id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if err := model.DeleteTokenById(token.Id, org.ShadowUserId); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	model.RecordOrgAudit(org.Id, c.GetInt("id"), "token.delete", fmt.Sprintf("Deleted organization API key %s (ID: %d)", token.Name, token.Id))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// canHandOverOrgToken reports whether the operator may get a working secret of the key (reset it,
// hand it to an agent): the member who created it, or for a key the organization owns, its owners
// and admins (ORG-16). Nobody gets to see another member's key (SEC-47).
func canHandOverOrgToken(c *gin.Context, token *model.Token) bool {
	if model.TokenIsOrgOwned(token) {
		_, role := getOrgFromContext(c)
		return isOrgAdmin(role)
	}
	return token.CreatedBy == c.GetInt("id")
}

// TransferOrgTokenToOrganization makes a member's organization API key owned by the organization
// (ORG-16, owners and admins): it no longer follows the member, so removing them, their leaving or
// a ban leaves it working, and no member budget or model list applies. The key keeps its status
// and secret: whoever already holds the secret can still use it until the key is reset.
func TransferOrgTokenToOrganization(c *gin.Context) {
	org, role := getOrgFromContext(c)
	if !isOrgAdmin(role) {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeTokenOrgOwnedAdminOnly, "Only organization owners and admins can manage API keys owned by the organization", nil)
		return
	}
	id, err := strconv.Atoi(c.Param("token_id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	token, err := getOrgTokenForOperator(c, id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if !model.TokenIsOrgOwned(token) {
		formerMember := token.CreatedBy
		if err := model.TransferOrgTokensToOrganization(org, []int{token.Id}); err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
		model.RecordOrgAudit(org.Id, c.GetInt("id"), "token.transfer",
			fmt.Sprintf("Made organization API key %s (ID: %d) owned by the organization (created by member %d)", token.Name, token.Id, formerMember))
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
