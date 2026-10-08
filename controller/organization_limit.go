package controller

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// ---------- 成员预算与模型白名单(T5,Admin+) ----------

type OrgMemberLimitsRequest struct {
	Budget         *model.QuotaResetSetting `json:"budget"`          // nil=不限预算;period: daily/weekly/monthly,limit>0
	ModelWhitelist []string                 `json:"model_whitelist"` // 空=回退组织级缺省
}

// GetOrgMemberLimits 查询成员预算与模型白名单(Admin+)
func GetOrgMemberLimits(c *gin.Context) {
	org, _ := getOrgFromContext(c)
	targetUserId := utils.String2Int(c.Param("user_id"))
	if targetUserId == 0 {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid user ID"))
		return
	}
	member, err := model.GetOrganizationMember(org.Id, targetUserId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("this user is not a member of the organization"))
		return
	}
	setting := member.Setting.Data()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"budget":          setting.Budget,
			"budget_used":     member.BudgetUsed,
			"budget_start":    member.BudgetStart,
			"model_whitelist": setting.ModelWhitelist,
		},
	})
}

// UpdateOrgMemberLimits 配置成员预算与模型白名单(Admin+)。
// 目标角色权重不得高于操作者(Admin 不能限制 Owner);可对自己设置
func UpdateOrgMemberLimits(c *gin.Context) {
	var req OrgMemberLimitsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid parameters"))
		return
	}
	org, operatorRole := getOrgFromContext(c)
	operatorId := c.GetInt("id")
	targetUserId := utils.String2Int(c.Param("user_id"))
	if targetUserId == 0 {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid user ID"))
		return
	}
	target, err := model.GetOrganizationMember(org.Id, targetUserId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("this user is not a member of the organization"))
		return
	}
	if model.OrgRoleWeight(target.Role) > model.OrgRoleWeight(operatorRole) {
		common.APIRespondWithError(c, http.StatusOK, errors.New("you cannot configure members with a higher role than yours"))
		return
	}
	member, err := model.UpdateOrgMemberLimits(org, targetUserId, req.Budget, req.ModelWhitelist)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	setting := member.Setting.Data()
	budgetDesc := "unlimited"
	if setting.Budget != nil {
		budgetDesc = fmt.Sprintf("%s/%d", setting.Budget.Period, setting.Budget.Limit)
	}
	model.RecordOrgAudit(org.Id, operatorId, "org.member_limits_update",
		fmt.Sprintf("Updated limits for user %d: budget=%s, model allowlist=%v", targetUserId, budgetDesc, setting.ModelWhitelist))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"budget":          setting.Budget,
			"budget_used":     member.BudgetUsed,
			"budget_start":    member.BudgetStart,
			"model_whitelist": setting.ModelWhitelist,
		},
	})
}
