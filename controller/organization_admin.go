package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// getOrgByIdParam 站点管理后台按路径参数取组织
func getOrgByIdParam(c *gin.Context) (*model.Organization, error) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return nil, errors.New("Invalid parameters")
	}
	return model.GetOrganizationById(id)
}

// AdminGetOrganizationsList 站点管理后台组织分页列表(仅 root;keyword 按名称/slug 搜索)
func AdminGetOrganizationsList(c *gin.Context) {
	var params model.GenericParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	result, err := model.GetOrganizationsAdminList(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result,
	})
}

// AdminGetOrganization 站点管理后台组织详情(仅 root)
func AdminGetOrganization(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("Invalid parameters"))
		return
	}
	info, err := model.GetOrganizationAdminInfo(id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    info,
	})
}

type AdminSetOrgStatusRequest struct {
	Status int `json:"status"`
}

// AdminSetOrganizationStatus 站点管理后台启用/禁用组织(仅 root;同步影子账户使组织令牌即刻生效)
func AdminSetOrganizationStatus(c *gin.Context) {
	org, err := getOrgByIdParam(c)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	var req AdminSetOrgStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("Invalid parameters"))
		return
	}
	if err := model.SetOrganizationStatus(org, req.Status); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	action := "org.admin_enable"
	detail := "Site admin enabled the organization"
	if req.Status == model.OrganizationStatusDisabled {
		action = "org.admin_disable"
		detail = "Site admin disabled the organization"
	}
	model.RecordOrgAudit(org.Id, c.GetInt("id"), action, detail)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// AdminChangeOrganizationQuota 站点管理后台增减组织池余额(仅 root;落审计并记 root 操作者)
func AdminChangeOrganizationQuota(c *gin.Context) {
	org, err := getOrgByIdParam(c)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	var req ChangeUserQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("Invalid parameters"))
		return
	}
	if req.Quota == 0 {
		common.APIRespondWithError(c, http.StatusOK, errors.New("Quota cannot be 0"))
		return
	}
	if err := model.ChangeUserQuota(org.ShadowUserId, req.Quota, false); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	remark := fmt.Sprintf("Site admin adjusted the organization pool balance by %s", common.LogQuota(req.Quota))
	if req.Remark != "" {
		remark = fmt.Sprintf("%s, remark: %s", remark, req.Remark)
	}
	model.RecordQuotaLog(org.ShadowUserId, model.LogTypeManage, req.Quota, c.ClientIP(), remark)
	model.RecordOrgAudit(org.Id, c.GetInt("id"), "billing.admin_adjust", remark)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// AdminDissolveOrganization 站点管理后台解散组织(仅 root;复用解散流程,剩余积分按站点策略退回 Owner)
func AdminDissolveOrganization(c *gin.Context) {
	org, err := getOrgByIdParam(c)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	ownerId, err := model.GetOrganizationOwnerId(org.Id)
	if err != nil {
		ownerId = org.CreatedBy
	}
	if err := model.DissolveOrganization(org, ownerId); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	model.RecordOrgAudit(org.Id, c.GetInt("id"), "org.admin_dissolve",
		fmt.Sprintf("Site admin dissolved organization %s (slug: %s)", org.Name, org.Slug))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
