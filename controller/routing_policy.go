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

// 自动路由策略的后台接口。校验与目录行联动都在 model 层，这里只做参数绑定与响应。

func routingPolicyIdParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeInvalidParams, "invalid routing policy id", nil)
		return 0, false
	}
	return id, true
}

// routingPolicyStatus 把 model 层错误映射成状态码：带稳定 code 的是校验类（400 / 404），其余是库错误。
func routingPolicyStatus(err error) int {
	var apiErr *common.APIError
	if errors.As(err, &apiErr) {
		if apiErr.Code == common.APICodeRoutingPolicyNotFound {
			return http.StatusNotFound
		}
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

// adminOrgRoutingPolicy is an organization policy as the platform console lists it.
type adminOrgRoutingPolicy struct {
	*model.RoutingPolicy
	OrganizationName string `json:"organization_name"`
	// OverridesPlatform the organization's policy hides a platform policy of the same name.
	OverridesPlatform bool `json:"overrides_platform"`
}

// GetRoutingPolicies lists the platform policies, one organization's with ?organization_id=N, or
// every organization's with ?organization_id=-1, so system admins can review (and disable) what
// organizations configured.
func GetRoutingPolicies(c *gin.Context) {
	organizationId, _ := strconv.Atoi(c.DefaultQuery("organization_id", "0"))
	if organizationId < 0 {
		respondAllOrganizationRoutingPolicies(c)
		return
	}
	policies, err := model.GetRoutingPoliciesByOrganization(organizationId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusInternalServerError, err)
		return
	}
	if policies == nil {
		policies = []*model.RoutingPolicy{}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    policies,
	})
}

func GetRoutingPolicy(c *gin.Context) {
	id, ok := routingPolicyIdParam(c)
	if !ok {
		return
	}
	policy, err := model.GetRoutingPolicyById(id)
	if err != nil {
		common.APIRespondWithError(c, routingPolicyStatus(err), err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    policy,
	})
}

// routingPolicyRequest 在策略之上把 enabled 改成指针：请求里省略时默认启用。
type routingPolicyRequest struct {
	model.RoutingPolicy
	Enabled *bool `json:"enabled"`
}

func bindRoutingPolicy(c *gin.Context) (*model.RoutingPolicy, bool) {
	req := routingPolicyRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeInvalidParams, err.Error(), nil)
		return nil, false
	}
	policy := req.RoutingPolicy
	policy.Enabled = req.Enabled == nil || *req.Enabled
	return &policy, true
}

func CreateRoutingPolicy(c *gin.Context) {
	policy, ok := bindRoutingPolicy(c)
	if !ok {
		return
	}
	policy.Id = 0
	policy.OrganizationId = 0
	if err := policy.Insert(); err != nil {
		common.APIRespondWithError(c, routingPolicyStatus(err), err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    policy,
	})
}

func UpdateRoutingPolicy(c *gin.Context) {
	id, ok := routingPolicyIdParam(c)
	if !ok {
		return
	}
	policy, ok := bindRoutingPolicy(c)
	if !ok {
		return
	}
	policy.Id = id
	if err := policy.Update(); err != nil {
		common.APIRespondWithError(c, routingPolicyStatus(err), err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    policy,
	})
}

func UpdateRoutingPolicyStatus(c *gin.Context) {
	id, ok := routingPolicyIdParam(c)
	if !ok {
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeInvalidParams, err.Error(), nil)
		return
	}
	policy, err := model.SetRoutingPolicyEnabled(id, req.Enabled)
	if err != nil {
		common.APIRespondWithError(c, routingPolicyStatus(err), err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    policy,
	})
}

func DeleteRoutingPolicy(c *gin.Context) {
	id, ok := routingPolicyIdParam(c)
	if !ok {
		return
	}
	if err := model.DeleteRoutingPolicyById(id); err != nil {
		common.APIRespondWithError(c, routingPolicyStatus(err), err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// GetRoutingPolicyStats 策略近 N 天效果：原因分布、标签命中、决策耗时、决策成本与估算节省。days 取 7 或 30。
func GetRoutingPolicyStats(c *gin.Context) {
	id, ok := routingPolicyIdParam(c)
	if !ok {
		return
	}
	policy, err := model.GetRoutingPolicyById(id)
	if err != nil {
		common.APIRespondWithError(c, routingPolicyStatus(err), err)
		return
	}
	respondRoutingPolicyStats(c, policy)
}

func respondRoutingPolicyStats(c *gin.Context, policy *model.RoutingPolicy) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	if days != 30 {
		days = 7
	}
	stats, err := model.GetRoutingPolicyStats(policy, days)
	if err != nil {
		common.APIRespondWithError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    stats,
	})
}

// ---- Organization console ----
//
// Owners and admins manage the routing policies of their own organization. Every handler runs behind
// OrgAuth(OrgRoleAdmin) and OrgRoutingPolicyEnabled; ids of other organizations' policies are 404.

func GetOrgRoutingPolicies(c *gin.Context) {
	org, _ := getOrgFromContext(c)
	policies, err := model.GetRoutingPoliciesByOrganization(org.Id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusInternalServerError, err)
		return
	}
	if policies == nil {
		policies = []*model.RoutingPolicy{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": policies})
}

// orgRoutingPolicyParam loads the :policy_id of the current organization.
func orgRoutingPolicyParam(c *gin.Context) (*model.Organization, *model.RoutingPolicy, bool) {
	org, _ := getOrgFromContext(c)
	id, err := strconv.Atoi(c.Param("policy_id"))
	if err != nil || id <= 0 {
		common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeInvalidParams, "invalid routing policy id", nil)
		return nil, nil, false
	}
	policy, err := model.GetOrganizationRoutingPolicyById(org.Id, id)
	if err != nil {
		common.APIRespondWithError(c, routingPolicyStatus(err), err)
		return nil, nil, false
	}
	return org, policy, true
}

func GetOrgRoutingPolicy(c *gin.Context) {
	_, policy, ok := orgRoutingPolicyParam(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": policy})
}

func CreateOrgRoutingPolicy(c *gin.Context) {
	org, _ := getOrgFromContext(c)
	policy, ok := bindRoutingPolicy(c)
	if !ok {
		return
	}
	policy.Id = 0
	policy.OrganizationId = org.Id
	if err := policy.Insert(); err != nil {
		common.APIRespondWithError(c, routingPolicyStatus(err), err)
		return
	}
	model.RecordOrgAudit(org.Id, c.GetInt("id"), "routing_policy.create", fmt.Sprintf("Created routing policy %s (id: %d)", policy.Name, policy.Id))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": policy})
}

func UpdateOrgRoutingPolicy(c *gin.Context) {
	org, current, ok := orgRoutingPolicyParam(c)
	if !ok {
		return
	}
	policy, ok := bindRoutingPolicy(c)
	if !ok {
		return
	}
	policy.Id = current.Id
	if err := policy.Update(); err != nil {
		common.APIRespondWithError(c, routingPolicyStatus(err), err)
		return
	}
	model.RecordOrgAudit(org.Id, c.GetInt("id"), "routing_policy.update", fmt.Sprintf("Updated routing policy %s (id: %d)", policy.Name, policy.Id))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": policy})
}

func UpdateOrgRoutingPolicyStatus(c *gin.Context) {
	org, current, ok := orgRoutingPolicyParam(c)
	if !ok {
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeInvalidParams, err.Error(), nil)
		return
	}
	policy, err := model.SetRoutingPolicyEnabled(current.Id, req.Enabled)
	if err != nil {
		common.APIRespondWithError(c, routingPolicyStatus(err), err)
		return
	}
	model.RecordOrgAudit(org.Id, c.GetInt("id"), "routing_policy.status", fmt.Sprintf("Set routing policy %s (id: %d) enabled: %t", policy.Name, policy.Id, req.Enabled))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": policy})
}

func DeleteOrgRoutingPolicy(c *gin.Context) {
	org, current, ok := orgRoutingPolicyParam(c)
	if !ok {
		return
	}
	if err := model.DeleteRoutingPolicyById(current.Id); err != nil {
		common.APIRespondWithError(c, routingPolicyStatus(err), err)
		return
	}
	model.RecordOrgAudit(org.Id, c.GetInt("id"), "routing_policy.delete", fmt.Sprintf("Deleted routing policy %s (id: %d)", current.Name, current.Id))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func GetOrgRoutingPolicyStats(c *gin.Context) {
	_, policy, ok := orgRoutingPolicyParam(c)
	if !ok {
		return
	}
	respondRoutingPolicyStats(c, policy)
}

// GetOrgRoutingModels lists the models the organization's policies may use (for the model pickers).
func GetOrgRoutingModels(c *gin.Context) {
	org, _ := getOrgFromContext(c)
	models, err := model.OrganizationRoutingModels(org.Id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": models})
}

func respondAllOrganizationRoutingPolicies(c *gin.Context) {
	policies, err := model.GetAllOrganizationRoutingPolicies()
	if err != nil {
		common.APIRespondWithError(c, http.StatusInternalServerError, err)
		return
	}
	names := map[int]string{}
	items := make([]adminOrgRoutingPolicy, 0, len(policies))
	for _, policy := range policies {
		name, ok := names[policy.OrganizationId]
		if !ok {
			if org, err := model.GetOrganizationById(policy.OrganizationId); err == nil {
				name = org.Name
			}
			names[policy.OrganizationId] = name
		}
		items = append(items, adminOrgRoutingPolicy{
			RoutingPolicy:     policy,
			OrganizationName:  name,
			OverridesPlatform: model.LookupRoutingPolicy(policy.Name) != nil,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": items})
}
