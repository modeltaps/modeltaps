package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/middleware"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// getOrgFromContext 读取 OrgAuth 中间件注入的组织与角色
func getOrgFromContext(c *gin.Context) (*model.Organization, string) {
	org := c.MustGet(middleware.OrgContextKey).(*model.Organization)
	role := c.GetString(middleware.OrgRoleContextKey)
	return org, role
}

type CreateOrganizationRequest struct {
	Name      string `json:"name" binding:"required"`
	AvatarUrl string `json:"avatar_url"`
}

// CreateOrganization 创建组织(任何登录用户;受 OrganizationMaxPerUser 限制)
func CreateOrganization(c *gin.Context) {
	var req CreateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid parameters"))
		return
	}
	userId := c.GetInt("id")
	if config.OrganizationMaxPerUser > 0 {
		count, err := model.CountUserCreatedOrganizations(userId)
		if err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
		if count >= int64(config.OrganizationMaxPerUser) {
			common.APIRespondWithError(c, http.StatusOK, fmt.Errorf("each user can create at most %d organizations", config.OrganizationMaxPerUser))
			return
		}
	}
	org, err := model.CreateOrganizationWithOwner(req.Name, req.AvatarUrl, userId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	model.RecordOrgAudit(org.Id, userId, "org.create", fmt.Sprintf("Created organization %s (slug: %s)", org.Name, org.Slug))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    org,
	})
}

// GetMyOrganizations 当前用户所属组织列表(含自身角色)
func GetMyOrganizations(c *gin.Context) {
	orgs, err := model.GetUserOrganizationsWithRole(c.GetInt("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    orgs,
	})
}

// GetOrganization 组织详情(成员可见);Admin/Owner 额外返回积分池余额(影子账户 quota);
// 任意角色返回本人周期预算 my_budget / my_budget_used / my_budget_start(未配置时不返回)
func GetOrganization(c *gin.Context) {
	org, role := getOrgFromContext(c)
	memberCount, err := model.CountOrganizationMembers(org.Id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	data := gin.H{
		"organization": org,
		"role":         role,
		"member_count": memberCount,
	}
	if role == model.OrgRoleOwner || role == model.OrgRoleAdmin {
		if quota, err := model.GetUserQuota(org.ShadowUserId); err == nil {
			data["quota"] = quota
		}
		// 团队级周期预算(T1):已用额度按当前周期懒重置语义折算(跨期视为 0)
		budget := org.Setting.Data().Budget
		data["budget"] = budget
		data["budget_used"] = model.OrganizationBudgetUsedInPeriod(org, time.Now())
		data["budget_start"] = org.BudgetStart
	}
	if budget, err := model.GetOrgMemberBudget(org.Id, c.GetInt("id")); err == nil && budget != nil {
		data["my_budget"] = budget.Budget
		data["my_budget_used"] = budget.Used
		data["my_budget_start"] = budget.Start
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

type UpdateOrganizationRequest struct {
	Name      string `json:"name" binding:"required"`
	AvatarUrl string `json:"avatar_url"`
	// UsageVisibleToMembers 成员用量可见性开关(规格 a4);nil 表示不修改
	UsageVisibleToMembers *bool `json:"usage_visible_to_members"`
	// LogIODefault 组织令牌「完整请求/响应留存」继承默认;三态哨兵 "inherit"|"on"|"off",nil 表示不修改
	LogIODefault *string `json:"log_io_default"`
	// Budget 团队级周期预算(T1);nil 表示不修改,limit<=0 表示清除限制
	Budget *model.QuotaResetSetting `json:"budget"`
}

// UpdateOrganization 更新组织资料(名称/头像/用量可见性,Admin+)
func UpdateOrganization(c *gin.Context) {
	var req UpdateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid parameters"))
		return
	}
	org, _ := getOrgFromContext(c)
	org.Name = strings.TrimSpace(req.Name)
	org.AvatarUrl = req.AvatarUrl
	if err := org.Update(); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if req.UsageVisibleToMembers != nil {
		setting := org.Setting.Data()
		setting.UsageVisibleToMembers = req.UsageVisibleToMembers
		org.Setting.Set(setting)
		if err := org.UpdateSetting(); err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
		model.RecordOrgAudit(org.Id, c.GetInt("id"), "org.update_setting", fmt.Sprintf("Updated member usage visibility: %t", *req.UsageVisibleToMembers))
	}
	if req.LogIODefault != nil {
		v, ok := model.ParseLogIOTriState(*req.LogIODefault)
		if !ok {
			common.APIRespondWithError(c, http.StatusOK, errors.New("invalid log_io_default value"))
			return
		}
		setting := org.Setting.Data()
		setting.LogIODefault = v
		org.Setting.Set(setting)
		if err := org.UpdateSetting(); err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
		model.RecordOrgAudit(org.Id, c.GetInt("id"), "org.update_setting", fmt.Sprintf("Updated organization API key LogIO default: %s", *req.LogIODefault))
	}
	if req.Budget != nil {
		budget := req.Budget
		if budget.Limit <= 0 {
			budget = nil // limit<=0 视为清除限制
		}
		if err := model.UpdateOrganizationBudget(org, budget); err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
		budgetDesc := "unlimited"
		if budget != nil {
			budgetDesc = fmt.Sprintf("%s/%d", budget.Period, budget.Limit)
		}
		model.RecordOrgAudit(org.Id, c.GetInt("id"), "org.update_budget", fmt.Sprintf("Updated organization period budget: %s", budgetDesc))
	}
	model.RecordOrgAudit(org.Id, c.GetInt("id"), "org.update", fmt.Sprintf("Updated organization profile (name: %s)", org.Name))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    org,
	})
}

type TransferOrgOwnershipRequest struct {
	UserId int `json:"user_id" binding:"required"`
}

// TransferOrgOwnership 转移所有权(仅 Owner;目标须为该组织 Admin,自身降为 Admin)
func TransferOrgOwnership(c *gin.Context) {
	var req TransferOrgOwnershipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid parameters"))
		return
	}
	org, _ := getOrgFromContext(c)
	userId := c.GetInt("id")
	if err := model.TransferOrganizationOwnership(org, userId, req.UserId); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	model.RecordOrgAudit(org.Id, userId, "org.transfer_ownership", fmt.Sprintf("Ownership transferred from user %d to user %d", userId, req.UserId))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

type DissolveOrganizationRequest struct {
	Name string `json:"name" binding:"required"` // 须与组织名称一致,防误操作
}

// DissolveOrganization 解散组织(仅 Owner;需输入组织名称确认)
func DissolveOrganization(c *gin.Context) {
	var req DissolveOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid parameters"))
		return
	}
	org, _ := getOrgFromContext(c)
	if strings.TrimSpace(req.Name) != org.Name {
		common.APIRespondWithError(c, http.StatusOK, errors.New("organization name confirmation does not match"))
		return
	}
	userId := c.GetInt("id")
	if err := model.DissolveOrganization(org, userId); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	model.RecordOrgAudit(org.Id, userId, "org.dissolve", fmt.Sprintf("Dissolved organization %s (slug: %s)", org.Name, org.Slug))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// GetOrgAuditLogs 组织审计日志查询(仅 Owner/Admin;支持按操作者/动作/时间过滤+分页)
func GetOrgAuditLogs(c *gin.Context) {
	var params model.OrgAuditLogsParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	org, _ := getOrgFromContext(c)
	logs, err := model.GetOrganizationAuditLogs(org.Id, &params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    logs,
	})
}
