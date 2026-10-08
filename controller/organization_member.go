package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// ---------- 代建成员账号(组织视角,Admin+;ORG-1) ----------

type CreateOrgMemberAccountRequest struct {
	Username    string                   `json:"username"`
	Password    string                   `json:"password"`
	DisplayName string                   `json:"display_name"`
	Email       string                   `json:"email"`
	Role        string                   `json:"role"`   // admin / member,默认 member,不得高于操作者
	Budget      *model.QuotaResetSetting `json:"budget"` // 可选:初始周期预算
	// GrantSignupBonus 可选:是否发放新用户注册奖励,默认 false(SEC-11:防止代建刷免费额度)
	GrantSignupBonus bool `json:"grant_signup_bonus"`
}

// CreateOrgMemberAccount 组织内代建成员账号(Owner/Admin):创建新用户并加入本组织,可选预置周期预算。
// "代建 key" 复用既有 AddOrgToken(账号建好后管理员可直接为其创建组织令牌)。
func CreateOrgMemberAccount(c *gin.Context) {
	var req CreateOrgMemberAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid parameters"))
		return
	}
	org, operatorRole := getOrgFromContext(c)
	operatorId := c.GetInt("id")
	role := req.Role
	if role == "" {
		role = model.OrgRoleMember
	}
	if role != model.OrgRoleAdmin && role != model.OrgRoleMember {
		common.APIRespondWithError(c, http.StatusOK, errors.New("role must be admin or member"))
		return
	}
	if model.OrgRoleWeight(role) > model.OrgRoleWeight(operatorRole) {
		common.APIRespondWithError(c, http.StatusOK, errors.New("you cannot create a member with a higher role than your own"))
		return
	}
	if strings.TrimSpace(req.Username) == "" {
		common.APIRespondWithError(c, http.StatusOK, errors.New("username must not be empty"))
		return
	}
	if strings.TrimSpace(req.Password) == "" {
		common.APIRespondWithError(c, http.StatusOK, errors.New("password must not be empty"))
		return
	}
	// 代建账号复用注册路径的用户名/密码/显示名/邮箱校验规则(SEC-16)
	req.Email = common.NormalizeEmail(req.Email)
	if err := common.Validate.Struct(&model.User{
		Username:    req.Username,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		Email:       model.NullableEmail(req.Email),
	}); err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New(getFriendlyValidationMessage(err)))
		return
	}
	user, _, err := model.CreateOrgMemberAccount(org, req.Username, req.Password, req.DisplayName, req.Email, role, req.GrantSignupBonus)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	// 可选:初始周期预算
	if req.Budget != nil {
		if _, err := model.UpdateOrgMemberLimits(org, user.Id, req.Budget, nil); err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
	}
	model.RecordOrgAudit(org.Id, operatorId, "org.member_account_create",
		fmt.Sprintf("Created member account %s (ID: %d, role: %s)", user.Username, user.Id, role))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"user_id":  user.Id,
			"username": user.Username,
			"role":     role,
		},
	})
}

// ---------- 邀请(组织视角,Admin+) ----------

type CreateOrgInvitationRequest struct {
	Email       string `json:"email"`        // 定向邀请:站内用户邮箱(与 username 二选一)
	Username    string `json:"username"`     // 定向邀请:站内用户名
	Role        string `json:"role"`         // admin / member,默认 member,不得高于操作者自身
	MaxUses     int    `json:"max_uses"`     // 链接邀请:最大使用次数,0=不限
	ExpiredTime int64  `json:"expired_time"` // 过期时间(unix 秒),0=永不过期
}

// CreateOrgInvitation 创建邀请(Admin+)。提供 email/username 为定向邀请,否则生成链接邀请
func CreateOrgInvitation(c *gin.Context) {
	var req CreateOrgInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid parameters"))
		return
	}
	org, operatorRole := getOrgFromContext(c)
	operatorId := c.GetInt("id")
	role := req.Role
	if role == "" {
		role = model.OrgRoleMember
	}
	if role != model.OrgRoleAdmin && role != model.OrgRoleMember {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invitation role must be admin or member"))
		return
	}
	if model.OrgRoleWeight(role) > model.OrgRoleWeight(operatorRole) {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invitation role cannot be higher than your own role"))
		return
	}
	if req.ExpiredTime != 0 && req.ExpiredTime <= utils.GetTimestamp() {
		common.APIRespondWithError(c, http.StatusOK, errors.New("expiration time must be later than the current time"))
		return
	}
	if req.MaxUses < 0 {
		common.APIRespondWithError(c, http.StatusOK, errors.New("max_uses must not be negative"))
		return
	}
	// 成员上限预校验(接受邀请时仍会在事务内复核)
	if maxMembers := org.GetMaxMembers(); maxMembers > 0 {
		count, err := model.CountOrganizationMembers(org.Id)
		if err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
		if count >= int64(maxMembers) {
			common.APIRespondWithError(c, http.StatusOK, errors.New("organization member limit reached"))
			return
		}
	}
	invitation := &model.OrganizationInvitation{
		OrganizationId: org.Id,
		InviterId:      operatorId,
		Role:           role,
		MaxUses:        req.MaxUses,
		ExpiredTime:    req.ExpiredTime,
	}
	email := common.NormalizeEmail(req.Email)
	username := strings.TrimSpace(req.Username)
	if email != "" || username != "" {
		// 定向邀请:解析站内被邀请人(查询已排除组织影子账户)
		invitee := &model.User{}
		var err error
		if email != "" {
			invitee.Email = model.NullableEmail(email)
			err = invitee.FillUserByEmail()
		} else {
			invitee.Username = username
			err = invitee.FillUserByUsername()
		}
		if err != nil {
			common.APIRespondWithError(c, http.StatusOK, errors.New("user not found"))
			return
		}
		if _, err := model.GetOrganizationMember(org.Id, invitee.Id); err == nil {
			common.APIRespondWithError(c, http.StatusOK, errors.New("this user is already a member of the organization"))
			return
		}
		if model.HasPendingDirectedInvitation(org.Id, invitee.Id) {
			common.APIRespondWithError(c, http.StatusOK, errors.New("a pending invitation for this user already exists"))
			return
		}
		invitation.InviteeId = invitee.Id
		invitation.Email = string(invitee.Email)
		invitation.MaxUses = 1
	}
	if err := invitation.Insert(); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if invitation.InviteeId > 0 {
		model.RecordOrgAudit(org.Id, operatorId, "org.invitation_create",
			fmt.Sprintf("Invited user %d (%s) to join the organization, role %s", invitation.InviteeId, invitation.Email, invitation.Role))
	} else {
		model.RecordOrgAudit(org.Id, operatorId, "org.invitation_create",
			fmt.Sprintf("Created invitation link (role %s, max uses %d)", invitation.Role, invitation.MaxUses))
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    invitation,
	})
}

// GetOrgInvitations 组织邀请列表(Admin+);query: page/size/order/status
func GetOrgInvitations(c *gin.Context) {
	var params model.PaginationParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	org, _ := getOrgFromContext(c)
	status := utils.String2Int(c.Query("status"))
	result, err := model.GetOrganizationInvitations(org.Id, status, &params)
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

// RevokeOrgInvitation 撤销待处理邀请(Admin+)
func RevokeOrgInvitation(c *gin.Context) {
	org, _ := getOrgFromContext(c)
	invitationId := utils.String2Int(c.Param("invitation_id"))
	if invitationId == 0 {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid invitation ID"))
		return
	}
	invitation, err := model.RevokeOrganizationInvitation(org.Id, invitationId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	operatorId := c.GetInt("id")
	model.RecordOrgAudit(org.Id, operatorId, "org.invitation_revoke", fmt.Sprintf("Revoked invitation %d", invitation.Id))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    invitation,
	})
}

// ---------- 邀请(被邀请人视角,仅需登录) ----------

// GetMyOrgInvitations 当前用户的待处理定向邀请
func GetMyOrgInvitations(c *gin.Context) {
	invitations, err := model.GetUserPendingOrgInvitations(c.GetInt("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    invitations,
	})
}

type OrgInvitationTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

// AcceptOrgInvitation 接受邀请加入组织(定向/链接邀请通用,token 鉴别)
func AcceptOrgInvitation(c *gin.Context) {
	var req OrgInvitationTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid parameters"))
		return
	}
	userId := c.GetInt("id")
	invitation, err := model.AcceptOrganizationInvitation(strings.TrimSpace(req.Token), userId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	model.RecordOrgAudit(invitation.OrganizationId, userId, "org.invitation_accept",
		fmt.Sprintf("User %d joined the organization via invitation %d, role %s", userId, invitation.Id, invitation.Role))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    invitation,
	})
}

// RejectOrgInvitation 拒绝定向邀请(仅被邀请人本人)
func RejectOrgInvitation(c *gin.Context) {
	var req OrgInvitationTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid parameters"))
		return
	}
	userId := c.GetInt("id")
	invitation, err := model.RejectOrganizationInvitation(strings.TrimSpace(req.Token), userId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	model.RecordOrgAudit(invitation.OrganizationId, userId, "org.invitation_reject",
		fmt.Sprintf("User %d declined invitation %d", userId, invitation.Id))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    invitation,
	})
}

// ---------- 成员管理 ----------

// GetOrgMembers 成员列表(Member+);query: page/size/order
func GetOrgMembers(c *gin.Context) {
	var params model.PaginationParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	org, role := getOrgFromContext(c)
	viewerIsAdmin := model.OrgRoleWeight(role) >= model.OrgRoleWeight(model.OrgRoleAdmin)
	result, err := model.GetOrganizationMembersList(org.Id, c.GetInt("id"), viewerIsAdmin, &params)
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

type UpdateOrgMemberRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// UpdateOrgMemberRole 修改成员角色(Admin+)。层级约束:
// 不能改自己;目标当前角色与新角色的权重均不得高于操作者
func UpdateOrgMemberRole(c *gin.Context) {
	var req UpdateOrgMemberRoleRequest
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
	if targetUserId == operatorId {
		common.APIRespondWithError(c, http.StatusOK, errors.New("you cannot change your own role"))
		return
	}
	target, err := model.GetOrganizationMember(org.Id, targetUserId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("this user is not a member of the organization"))
		return
	}
	if model.OrgRoleWeight(target.Role) > model.OrgRoleWeight(operatorRole) {
		common.APIRespondWithError(c, http.StatusOK, errors.New("you cannot modify members with a higher role than yours"))
		return
	}
	if model.OrgRoleWeight(req.Role) > model.OrgRoleWeight(operatorRole) {
		common.APIRespondWithError(c, http.StatusOK, errors.New("you cannot set a member's role higher than your own"))
		return
	}
	member, err := model.UpdateOrganizationMemberRole(org.Id, targetUserId, req.Role)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	model.RecordOrgAudit(org.Id, operatorId, "org.member_role_update",
		fmt.Sprintf("User %d role updated to %s", targetUserId, member.Role))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    member,
	})
}

// RemoveOrgMember 移除成员(Admin+)。不能移除自己(请用退出);
// 目标角色权重不得高于操作者;Owner 不可被移除。移除时自动禁用其组织令牌
func RemoveOrgMember(c *gin.Context) {
	org, operatorRole := getOrgFromContext(c)
	operatorId := c.GetInt("id")
	targetUserId := utils.String2Int(c.Param("user_id"))
	if targetUserId == 0 {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid user ID"))
		return
	}
	if targetUserId == operatorId {
		common.APIRespondWithError(c, http.StatusOK, errors.New("you cannot remove yourself; leave the organization instead"))
		return
	}
	target, err := model.GetOrganizationMember(org.Id, targetUserId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("this user is not a member of the organization"))
		return
	}
	if model.OrgRoleWeight(target.Role) > model.OrgRoleWeight(operatorRole) {
		common.APIRespondWithError(c, http.StatusOK, errors.New("you cannot remove members with a higher role than yours"))
		return
	}
	if err := model.RemoveOrganizationMember(org, targetUserId); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	model.RecordOrgAudit(org.Id, operatorId, "org.member_remove",
		fmt.Sprintf("Removed member %d (previous role %s); their organization API keys have been disabled", targetUserId, target.Role))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// LeaveOrganization 主动退出组织(Member+)。Owner 须先转移所有权;退出时自动禁用其组织令牌
func LeaveOrganization(c *gin.Context) {
	org, _ := getOrgFromContext(c)
	userId := c.GetInt("id")
	if err := model.RemoveOrganizationMember(org, userId); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	model.RecordOrgAudit(org.Id, userId, "org.member_leave",
		fmt.Sprintf("User %d left the organization; their organization API keys have been disabled", userId))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
