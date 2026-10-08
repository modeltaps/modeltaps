package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
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
		return nil, errors.New("API key not found")
	}
	if role == model.OrgRoleMember && token.CreatedBy != c.GetInt("id") {
		return nil, errors.New("Not allowed to operate on this API key")
	}
	return token, nil
}

// GetOrgTokensList 组织令牌列表:Member 仅自己创建的,Owner/Admin 全部
func GetOrgTokensList(c *gin.Context) {
	org, role := getOrgFromContext(c)
	var params model.GenericParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	createdBy := 0
	if role == model.OrgRoleMember {
		createdBy = c.GetInt("id")
	}
	tokens, err := model.GetOrgTokensList(org.ShadowUserId, createdBy, &params)
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
		"success": true,
		"message": "",
		"data":    tokens,
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

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    token,
	})
}

// AddOrgToken 组织上下文创建令牌:user_id 指向影子账户,created_by 记录实际创建成员
func AddOrgToken(c *gin.Context) {
	org, _ := getOrgFromContext(c)
	userId := c.GetInt("id")
	userRole := c.GetInt("role")
	token := model.Token{}
	if err := c.ShouldBindJSON(&token); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if len(token.Name) > 30 {
		common.APIRespondWithError(c, http.StatusOK, errors.New("API key name is too long"))
		return
	}

	// 分组校验以影子账户(组织)的用户组为准
	if token.Group != "" {
		if err := validateTokenGroupForUser(token.Group, org.ShadowUserId); err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
	}
	if token.BackupGroup != "" {
		if err := validateTokenGroupForUser(token.BackupGroup, org.ShadowUserId); err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
	}

	setting := token.Setting.Data()
	if err := validateTokenSetting(&setting); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	// 非可信用户不能设置 BillingTag
	if userRole < config.RoleReliableUser {
		setting.BillingTag = nil
	}

	cleanToken := model.Token{
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
		LogIO:          token.LogIO,
	}
	cleanToken.Setting.Set(setting)
	if err := cleanToken.Insert(); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	model.RecordOrgAudit(org.Id, userId, "token.create", fmt.Sprintf("Created organization API key %s (ID: %d)", cleanToken.Name, cleanToken.Id))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
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
		common.APIRespondWithError(c, http.StatusOK, errors.New("API key name is too long"))
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
			common.APIRespondWithError(c, http.StatusOK, errors.New("API key has expired and cannot be enabled. Change its expiration time or set it to never expire first."))
			return
		}
		if cleanToken.Status == config.TokenStatusExhausted && cleanToken.RemainQuota <= 0 && !cleanToken.UnlimitedQuota {
			common.APIRespondWithError(c, http.StatusOK, errors.New("API key quota is exhausted and it cannot be enabled. Change its remaining quota or set it to unlimited first."))
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
		cleanToken.LogIO = token.LogIO

		// 处理 BillingTag: 非可信用户保持原值不变
		oldSetting := cleanToken.Setting.Data()
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
