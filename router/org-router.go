package router

import (
	"github.com/modeltaps/modeltaps/controller"
	"github.com/modeltaps/modeltaps/middleware"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// setOrgRouter 注册 /api/org/* 路由。
// 全部路由受 OrganizationFeatureEnabled(全局开关)与 UserAuth(登录)保护;
// 组织级路由按最低角色分为 member/admin/owner 三个分组,T3(成员/邀请)、T4(令牌)在对应分组下追加。
func setOrgRouter(apiRouter *gin.RouterGroup) {
	orgRoute := apiRouter.Group("/org")
	orgRoute.Use(middleware.OrganizationFeatureEnabled(), middleware.UserAuth())
	{
		orgRoute.POST("/", controller.CreateOrganization)
		orgRoute.GET("/", controller.GetMyOrganizations)

		// 邀请(T3,被邀请人视角):仅需登录,不要求组织成员身份
		orgRoute.GET("/invitations", controller.GetMyOrgInvitations)
		orgRoute.POST("/invitations/accept", controller.AcceptOrgInvitation)
		orgRoute.POST("/invitations/reject", controller.RejectOrgInvitation)

		// 成员可访问(Member+)
		orgMemberRoute := orgRoute.Group("/:id")
		orgMemberRoute.Use(middleware.OrgAuth(model.OrgRoleMember))
		{
			orgMemberRoute.GET("/", controller.GetOrganization)

			// 成员(T3):列表与主动退出
			orgMemberRoute.GET("/members", controller.GetOrgMembers)
			orgMemberRoute.POST("/leave", controller.LeaveOrganization)

			// 组织令牌(T4):Member 仅自己创建的,Owner/Admin 全部
			orgMemberRoute.GET("/token", controller.GetOrgTokensList)
			orgMemberRoute.GET("/token/:token_id", controller.GetOrgToken)
			orgMemberRoute.POST("/token", controller.AddOrgToken)
			orgMemberRoute.PUT("/token", controller.UpdateOrgToken)
			orgMemberRoute.DELETE("/token/:token_id", controller.DeleteOrgToken)

			// 组织日志与用量(T6):Member 受可见性策略约束(开关关闭时仅可见自己)
			orgMemberRoute.GET("/logs", controller.GetOrgLogs)
			orgMemberRoute.GET("/logs/stat", controller.GetOrgLogsStat)
			orgMemberRoute.GET("/logs/histogram", controller.GetOrgLogsHistogram)
			orgMemberRoute.GET("/logs/export", controller.ExportOrgLogs)
			// 完整请求/响应明细(T50f):以 CanViewTokenLogIO 越权拒绝,且须归属本组织
			orgMemberRoute.GET("/logs/detail/:log_id", controller.GetOrgLogDetail)
			orgMemberRoute.GET("/analytics", controller.GetOrgUsageAnalytics)
			orgMemberRoute.GET("/analytics/export", controller.ExportOrgUsageAnalytics)
		}

		// 管理可访问(Admin+)
		orgAdminRoute := orgRoute.Group("/:id")
		orgAdminRoute.Use(middleware.OrgAuth(model.OrgRoleAdmin))
		{
			orgAdminRoute.PUT("/", controller.UpdateOrganization)

			// 成员管理与邀请(T3)
			orgAdminRoute.POST("/members/account", controller.CreateOrgMemberAccount) // ORG-1:代建成员账号
			orgAdminRoute.PUT("/members/:user_id", controller.UpdateOrgMemberRole)
			orgAdminRoute.DELETE("/members/:user_id", controller.RemoveOrgMember)

			// 成员预算与模型白名单(T5)
			orgAdminRoute.GET("/members/:user_id/limits", controller.GetOrgMemberLimits)
			orgAdminRoute.PUT("/members/:user_id/limits", controller.UpdateOrgMemberLimits)
			orgAdminRoute.POST("/invitations", controller.CreateOrgInvitation)
			orgAdminRoute.GET("/invitations", controller.GetOrgInvitations)
			orgAdminRoute.DELETE("/invitations/:invitation_id", controller.RevokeOrgInvitation)

			// 组织审计日志(T7):仅 Owner/Admin 可查
			orgAdminRoute.GET("/audit_logs", controller.GetOrgAuditLogs)

			// 组织计费(T4):积分转移、充值、订单(仅 Owner/Admin)
			orgAdminRoute.POST("/quota/transfer", controller.TransferQuotaToOrg)
			orgAdminRoute.POST("/topup", controller.OrgTopUp)
			orgAdminRoute.POST("/order", controller.CreateOrgOrder)
			orgAdminRoute.GET("/order", controller.GetOrgOrdersList)
			orgAdminRoute.GET("/order/status", controller.CheckOrgOrderStatus)
		}

		// 仅 Owner
		orgOwnerRoute := orgRoute.Group("/:id")
		orgOwnerRoute.Use(middleware.OrgAuth(model.OrgRoleOwner))
		{
			orgOwnerRoute.POST("/transfer", controller.TransferOrgOwnership)
			orgOwnerRoute.DELETE("/", controller.DissolveOrganization)
		}
	}
}
