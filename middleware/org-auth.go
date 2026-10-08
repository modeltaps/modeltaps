package middleware

import (
	"errors"
	"net/http"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// 组织上下文键(OrgAuth 注入,供下游 handler 读取)
const (
	OrgContextKey     = "org"      // *model.Organization
	OrgRoleContextKey = "org_role" // string: owner/admin/member
)

// 角色权重(见规格 §2.2 权限矩阵):owner > admin > member
var orgRoleWeight = map[string]int{
	model.OrgRoleMember: 1,
	model.OrgRoleAdmin:  2,
	model.OrgRoleOwner:  3,
}

func abortOrgRequest(c *gin.Context, status int, message string) {
	common.APIRespondWithError(c, status, errors.New(message))
	c.Abort()
}

// OrganizationFeatureEnabled 组织功能全局开关:organization_enabled 关闭时 /api/org/* 全部返回功能未启用
func OrganizationFeatureEnabled() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.OrganizationEnabled {
			abortOrgRequest(c, http.StatusForbidden, "Organizations are not enabled")
			return
		}
		c.Next()
	}
}

// OrgAuth 组织权限中间件:解析路由参数 :id 为组织ID,校验组织存在且启用、
// 当前用户(UserAuth 注入的 id)为组织成员且角色不低于 minRole,
// 然后注入 org / org_role 到上下文。须置于 UserAuth 之后。
func OrgAuth(minRole string) gin.HandlerFunc {
	minWeight, ok := orgRoleWeight[minRole]
	if !ok {
		panic("OrgAuth: invalid minimum role " + minRole)
	}
	return func(c *gin.Context) {
		orgId := utils.String2Int(c.Param("id"))
		if orgId <= 0 {
			abortOrgRequest(c, http.StatusBadRequest, "Invalid organization ID")
			return
		}
		org, err := model.GetOrganizationById(orgId)
		if err != nil {
			abortOrgRequest(c, http.StatusNotFound, "Organization not found")
			return
		}
		if org.Status != model.OrganizationStatusEnabled {
			abortOrgRequest(c, http.StatusForbidden, "Organization is disabled")
			return
		}
		userId := c.GetInt("id")
		member, err := model.GetOrganizationMember(orgId, userId)
		if err != nil {
			abortOrgRequest(c, http.StatusForbidden, "You are not a member of this organization")
			return
		}
		if orgRoleWeight[member.Role] < minWeight {
			abortOrgRequest(c, http.StatusForbidden, "Permission denied for this organization")
			return
		}
		c.Set(OrgContextKey, org)
		c.Set(OrgRoleContextKey, member.Role)
		c.Next()
	}
}
