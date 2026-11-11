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

// GetRoutingPolicies lists the platform policies, one organization's with ?organization_id=N,
// every organization's with ?organization_id=-1, or every user's personal ones with ?scope=personal,
// so system admins can review (and disable) what organizations and users configured.
func GetRoutingPolicies(c *gin.Context) {
	if c.Query("scope") == "personal" {
		respondAllPersonalRoutingPolicies(c)
		return
	}
	organizationId, _ := strconv.Atoi(c.DefaultQuery("organization_id", "0"))
	if organizationId < 0 {
		respondAllOrganizationRoutingPolicies(c)
		return
	}
	policies, err := model.GetRoutingPoliciesByScope(model.OrganizationRoutingScope(organizationId))
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

// ---- Own policies: organization and personal consoles ----
//
// Organization owners and admins manage the policies of their organization (members may read them);
// every user manages their personal policies. Handlers run behind UserRoutingPolicyEnabled; ids of
// other scopes' policies are 404.

func listScopedRoutingPolicies(c *gin.Context, scope model.RoutingScope) {
	policies, err := model.GetRoutingPoliciesByScope(scope)
	if err != nil {
		common.APIRespondWithError(c, http.StatusInternalServerError, err)
		return
	}
	if policies == nil {
		policies = []*model.RoutingPolicy{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": policies})
}

// scopedRoutingPolicyParam loads the :policy_id of the scope.
func scopedRoutingPolicyParam(c *gin.Context, scope model.RoutingScope) (*model.RoutingPolicy, bool) {
	id, err := strconv.Atoi(c.Param("policy_id"))
	if err != nil || id <= 0 {
		common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeInvalidParams, "invalid routing policy id", nil)
		return nil, false
	}
	policy, err := model.GetScopedRoutingPolicyById(scope, id)
	if err != nil {
		common.APIRespondWithError(c, routingPolicyStatus(err), err)
		return nil, false
	}
	return policy, true
}

func createScopedRoutingPolicy(c *gin.Context, scope model.RoutingScope) (*model.RoutingPolicy, bool) {
	policy, ok := bindRoutingPolicy(c)
	if !ok {
		return nil, false
	}
	policy.Id = 0
	policy.OrganizationId, policy.UserId = scope.OrganizationId, scope.UserId
	if err := policy.Insert(); err != nil {
		common.APIRespondWithError(c, routingPolicyStatus(err), err)
		return nil, false
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": policy})
	return policy, true
}

func updateScopedRoutingPolicy(c *gin.Context, scope model.RoutingScope) (*model.RoutingPolicy, bool) {
	current, ok := scopedRoutingPolicyParam(c, scope)
	if !ok {
		return nil, false
	}
	policy, ok := bindRoutingPolicy(c)
	if !ok {
		return nil, false
	}
	policy.Id = current.Id
	if err := policy.Update(); err != nil {
		common.APIRespondWithError(c, routingPolicyStatus(err), err)
		return nil, false
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": policy})
	return policy, true
}

func setScopedRoutingPolicyStatus(c *gin.Context, scope model.RoutingScope) (*model.RoutingPolicy, bool) {
	current, ok := scopedRoutingPolicyParam(c, scope)
	if !ok {
		return nil, false
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeInvalidParams, err.Error(), nil)
		return nil, false
	}
	policy, err := model.SetRoutingPolicyEnabled(current.Id, req.Enabled)
	if err != nil {
		common.APIRespondWithError(c, routingPolicyStatus(err), err)
		return nil, false
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": policy})
	return policy, true
}

func deleteScopedRoutingPolicy(c *gin.Context, scope model.RoutingScope) (*model.RoutingPolicy, bool) {
	current, ok := scopedRoutingPolicyParam(c, scope)
	if !ok {
		return nil, false
	}
	if err := model.DeleteRoutingPolicyById(current.Id); err != nil {
		common.APIRespondWithError(c, routingPolicyStatus(err), err)
		return nil, false
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
	return current, true
}

func respondScopedRoutingModels(c *gin.Context, scope model.RoutingScope) {
	models, err := model.ScopedRoutingModels(scope)
	if err != nil {
		common.APIRespondWithError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": models})
}

func orgRoutingScope(c *gin.Context) (*model.Organization, model.RoutingScope) {
	org, _ := getOrgFromContext(c)
	return org, model.OrganizationRoutingScope(org.Id)
}

// Organization console. Reads are open to every member; writes need Owner / Admin (router) and are
// written to the organization audit log.

func GetOrgRoutingPolicies(c *gin.Context) {
	_, scope := orgRoutingScope(c)
	listScopedRoutingPolicies(c, scope)
}

func GetOrgRoutingPolicy(c *gin.Context) {
	_, scope := orgRoutingScope(c)
	if policy, ok := scopedRoutingPolicyParam(c, scope); ok {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": policy})
	}
}

func CreateOrgRoutingPolicy(c *gin.Context) {
	org, scope := orgRoutingScope(c)
	if policy, ok := createScopedRoutingPolicy(c, scope); ok {
		model.RecordOrgAudit(org.Id, c.GetInt("id"), "routing_policy.create", fmt.Sprintf("Created routing policy %s (id: %d)", policy.Name, policy.Id))
	}
}

func UpdateOrgRoutingPolicy(c *gin.Context) {
	org, scope := orgRoutingScope(c)
	if policy, ok := updateScopedRoutingPolicy(c, scope); ok {
		model.RecordOrgAudit(org.Id, c.GetInt("id"), "routing_policy.update", fmt.Sprintf("Updated routing policy %s (id: %d)", policy.Name, policy.Id))
	}
}

func UpdateOrgRoutingPolicyStatus(c *gin.Context) {
	org, scope := orgRoutingScope(c)
	if policy, ok := setScopedRoutingPolicyStatus(c, scope); ok {
		model.RecordOrgAudit(org.Id, c.GetInt("id"), "routing_policy.status", fmt.Sprintf("Set routing policy %s (id: %d) enabled: %t", policy.Name, policy.Id, policy.Enabled))
	}
}

func DeleteOrgRoutingPolicy(c *gin.Context) {
	org, scope := orgRoutingScope(c)
	if policy, ok := deleteScopedRoutingPolicy(c, scope); ok {
		model.RecordOrgAudit(org.Id, c.GetInt("id"), "routing_policy.delete", fmt.Sprintf("Deleted routing policy %s (id: %d)", policy.Name, policy.Id))
	}
}

func GetOrgRoutingPolicyStats(c *gin.Context) {
	_, scope := orgRoutingScope(c)
	if policy, ok := scopedRoutingPolicyParam(c, scope); ok {
		respondRoutingPolicyStats(c, policy)
	}
}

// GetOrgRoutingModels lists the models the organization's policies may use (for the model pickers).
func GetOrgRoutingModels(c *gin.Context) {
	_, scope := orgRoutingScope(c)
	respondScopedRoutingModels(c, scope)
}

func respondManagedRoutingPolicies(c *gin.Context, scope model.RoutingScope) {
	policies, err := model.ManagedRoutingPoliciesFor(scope)
	if err != nil {
		common.APIRespondWithError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": policies})
}

// GetOrgManagedRoutingPolicies lists the platform policies the organization's keys can call.
func GetOrgManagedRoutingPolicies(c *gin.Context) {
	_, scope := orgRoutingScope(c)
	respondManagedRoutingPolicies(c, scope)
}

// GetUserManagedRoutingPolicies lists the platform policies the user's personal keys can call.
func GetUserManagedRoutingPolicies(c *gin.Context) {
	respondManagedRoutingPolicies(c, personalRoutingScope(c))
}

// Personal console: every signed-in user manages their own policies.

func personalRoutingScope(c *gin.Context) model.RoutingScope {
	return model.PersonalRoutingScope(c.GetInt("id"))
}

func GetUserRoutingPolicies(c *gin.Context) {
	listScopedRoutingPolicies(c, personalRoutingScope(c))
}

func GetUserRoutingPolicy(c *gin.Context) {
	if policy, ok := scopedRoutingPolicyParam(c, personalRoutingScope(c)); ok {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": policy})
	}
}

func CreateUserRoutingPolicy(c *gin.Context) {
	createScopedRoutingPolicy(c, personalRoutingScope(c))
}

func UpdateUserRoutingPolicy(c *gin.Context) {
	updateScopedRoutingPolicy(c, personalRoutingScope(c))
}

func UpdateUserRoutingPolicyStatus(c *gin.Context) {
	setScopedRoutingPolicyStatus(c, personalRoutingScope(c))
}

func DeleteUserRoutingPolicy(c *gin.Context) {
	deleteScopedRoutingPolicy(c, personalRoutingScope(c))
}

func GetUserRoutingPolicyStats(c *gin.Context) {
	if policy, ok := scopedRoutingPolicyParam(c, personalRoutingScope(c)); ok {
		respondRoutingPolicyStats(c, policy)
	}
}

// GetUserRoutingModels lists the models the user's personal policies may use.
func GetUserRoutingModels(c *gin.Context) {
	respondScopedRoutingModels(c, personalRoutingScope(c))
}

// adminPersonalRoutingPolicy is a personal policy as the platform console lists it.
type adminPersonalRoutingPolicy struct {
	*model.RoutingPolicy
	Username string `json:"username"`
	// OverridesPlatform the user's policy hides a platform policy of the same name for that user.
	OverridesPlatform bool `json:"overrides_platform"`
}

func respondAllPersonalRoutingPolicies(c *gin.Context) {
	policies, err := model.GetAllPersonalRoutingPolicies()
	if err != nil {
		common.APIRespondWithError(c, http.StatusInternalServerError, err)
		return
	}
	names := map[int]string{}
	items := make([]adminPersonalRoutingPolicy, 0, len(policies))
	for _, policy := range policies {
		name, ok := names[policy.UserId]
		if !ok {
			if user, err := model.GetUserById(policy.UserId, false); err == nil {
				name = user.Username
			}
			names[policy.UserId] = name
		}
		items = append(items, adminPersonalRoutingPolicy{
			RoutingPolicy:     policy,
			Username:          name,
			OverridesPlatform: model.LookupRoutingPolicy(policy.Name) != nil,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": items})
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
