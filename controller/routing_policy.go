package controller

import (
	"errors"
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

func GetRoutingPolicies(c *gin.Context) {
	policies, err := model.GetRoutingPolicies()
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
