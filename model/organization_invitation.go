package model

import (
	"errors"
	"strings"

	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 组织邀请状态
const (
	OrgInvitationStatusPending  = 1 // don't use 0, 0 is the default value!
	OrgInvitationStatusAccepted = 2
	OrgInvitationStatusRejected = 3
	OrgInvitationStatusRevoked  = 4
	OrgInvitationStatusExpired  = 5
)

// OrganizationInvitation 组织邀请记录。
// 定向邀请: InviteeId > 0;链接邀请: InviteeId = 0,通过 Token 加入,可配 MaxUses/ExpiredTime
type OrganizationInvitation struct {
	Id             int    `json:"id"`
	OrganizationId int    `json:"organization_id" gorm:"type:int;not null;index"`
	InviterId      int    `json:"inviter_id" gorm:"type:int;not null"`
	InviteeId      int    `json:"invitee_id" gorm:"type:int;default:0;index"` // 0=链接邀请
	Email          string `json:"email" gorm:"type:varchar(100);default:''"`
	Token          string `json:"token" gorm:"type:varchar(64);not null;uniqueIndex"`
	Role           string `json:"role" gorm:"type:varchar(16);not null;default:'member'"`
	Status         int    `json:"status" gorm:"type:int;default:1;index"`
	MaxUses        int    `json:"max_uses" gorm:"type:int;default:1"`    // 0=不限次数(仅链接邀请)
	UsedCount      int    `json:"used_count" gorm:"type:int;default:0"`
	ExpiredTime    int64  `json:"expired_time" gorm:"bigint;default:0"` // 0=永不过期
	CreatedTime    int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime    int64  `json:"updated_time" gorm:"bigint"`
}

func (invitation *OrganizationInvitation) Insert() error {
	if invitation.OrganizationId == 0 || invitation.InviterId == 0 {
		return errors.New("organization id or inviter id must not be empty")
	}
	if !IsValidOrgRole(invitation.Role) {
		return errors.New("invalid organization role")
	}
	if invitation.Role == OrgRoleOwner {
		return errors.New("invitation role cannot be owner")
	}
	if strings.TrimSpace(invitation.Token) == "" {
		invitation.Token = utils.GetUUID()
	}
	if invitation.Status == 0 {
		invitation.Status = OrgInvitationStatusPending
	}
	invitation.CreatedTime = utils.GetTimestamp()
	invitation.UpdatedTime = invitation.CreatedTime
	return DB.Create(invitation).Error
}

func GetOrganizationInvitationByToken(token string) (*OrganizationInvitation, error) {
	if token == "" {
		return nil, errors.New("token is empty")
	}
	var invitation OrganizationInvitation
	err := DB.First(&invitation, "token = ?", token).Error
	return &invitation, err
}

// IsExpired 判断邀请是否已过期(ExpiredTime=0 表示永不过期)
func (invitation *OrganizationInvitation) IsExpired() bool {
	return invitation.ExpiredTime > 0 && utils.GetTimestamp() > invitation.ExpiredTime
}

// IsUsable 判断邀请当前是否可用(状态、过期时间、使用次数)
func (invitation *OrganizationInvitation) IsUsable() bool {
	if invitation.Status != OrgInvitationStatusPending {
		return false
	}
	if invitation.IsExpired() {
		return false
	}
	if invitation.MaxUses > 0 && invitation.UsedCount >= invitation.MaxUses {
		return false
	}
	return true
}

var allowedOrgInvitationOrderFields = map[string]bool{
	"id":           true,
	"status":       true,
	"created_time": true,
}

// GetOrganizationInvitations 分页查询组织邀请(Admin 视角);status>0 时按状态过滤
func GetOrganizationInvitations(organizationId int, status int, params *PaginationParams) (*DataResult[OrganizationInvitation], error) {
	db := DB.Where("organization_id = ?", organizationId)
	if status > 0 {
		db = db.Where("status = ?", status)
	}
	var invitations []*OrganizationInvitation
	return PaginateAndOrder[OrganizationInvitation](db, params, &invitations, allowedOrgInvitationOrderFields)
}

// HasPendingDirectedInvitation 是否已存在发给该用户的待处理定向邀请(防重复邀请)
func HasPendingDirectedInvitation(organizationId int, inviteeId int) bool {
	var count int64
	DB.Model(&OrganizationInvitation{}).
		Where("organization_id = ? AND invitee_id = ? AND status = ?", organizationId, inviteeId, OrgInvitationStatusPending).
		Count(&count)
	return count > 0
}

// UserOrgInvitation 我的邀请(被邀请人视角):邀请 + 组织名 + 邀请人用户名
type UserOrgInvitation struct {
	OrganizationInvitation
	OrganizationName string `json:"organization_name" gorm:"-"`
	InviterUsername  string `json:"inviter_username" gorm:"-"`
}

// GetUserPendingOrgInvitations 当前用户的待处理定向邀请(自动排除已过期与组织已解散/禁用的)
func GetUserPendingOrgInvitations(userId int) ([]*UserOrgInvitation, error) {
	if userId == 0 {
		return nil, errors.New("user id must not be empty")
	}
	var invitations []OrganizationInvitation
	now := utils.GetTimestamp()
	err := DB.Where("invitee_id = ? AND status = ? AND (expired_time = 0 OR expired_time > ?)",
		userId, OrgInvitationStatusPending, now).
		Order("id desc").Find(&invitations).Error
	if err != nil {
		return nil, err
	}
	result := make([]*UserOrgInvitation, 0, len(invitations))
	if len(invitations) == 0 {
		return result, nil
	}
	orgIds := make([]int, 0, len(invitations))
	inviterIds := make([]int, 0, len(invitations))
	for _, inv := range invitations {
		orgIds = append(orgIds, inv.OrganizationId)
		inviterIds = append(inviterIds, inv.InviterId)
	}
	var orgs []Organization
	if err := DB.Where("id IN ? AND status = ?", orgIds, OrganizationStatusEnabled).Find(&orgs).Error; err != nil {
		return nil, err
	}
	orgNameById := make(map[int]string, len(orgs))
	for _, org := range orgs {
		orgNameById[org.Id] = org.Name
	}
	var inviters []User
	if err := DB.Select("id", "username").Where("id IN ?", inviterIds).Find(&inviters).Error; err != nil {
		return nil, err
	}
	inviterNameById := make(map[int]string, len(inviters))
	for _, u := range inviters {
		inviterNameById[u.Id] = u.Username
	}
	for i := range invitations {
		orgName, ok := orgNameById[invitations[i].OrganizationId]
		if !ok {
			continue // 组织已解散或禁用,不再展示
		}
		result = append(result, &UserOrgInvitation{
			OrganizationInvitation: invitations[i],
			OrganizationName:       orgName,
			InviterUsername:        inviterNameById[invitations[i].InviterId],
		})
	}
	return result, nil
}

// invitationStatusError 按非 pending 状态返回明确错误(幂等语义:重复操作得到确定结果)
func invitationStatusError(status int) error {
	switch status {
	case OrgInvitationStatusAccepted:
		return errors.New("invitation already accepted")
	case OrgInvitationStatusRejected:
		return errors.New("invitation already declined")
	case OrgInvitationStatusRevoked:
		return errors.New("invitation has been revoked")
	case OrgInvitationStatusExpired:
		return errors.New("invitation has expired")
	default:
		return errors.New("invalid invitation status")
	}
}

// markInvitationExpired 惰性标记过期(pending 且已过 ExpiredTime 时调用)
func markInvitationExpired(invitationId int) {
	DB.Model(&OrganizationInvitation{}).
		Where("id = ? AND status = ?", invitationId, OrgInvitationStatusPending).
		Updates(map[string]interface{}{"status": OrgInvitationStatusExpired, "updated_time": utils.GetTimestamp()})
}

// AcceptOrganizationInvitation 接受邀请加入组织(定向与链接邀请均经本函数)。
// 事务内复核组织有效性、是否已是成员、成员上限,并以条件更新保证并发下邀请不被超额消耗:
// 定向邀请 pending → accepted;链接邀请消耗一次使用次数(用尽后不再可用,状态保持 pending 由 IsUsable 判定)
func AcceptOrganizationInvitation(token string, userId int) (*OrganizationInvitation, error) {
	if userId == 0 {
		return nil, errors.New("user id must not be empty")
	}
	invitation, err := GetOrganizationInvitationByToken(token)
	if err != nil {
		return nil, errors.New("invitation not found")
	}
	if invitation.InviteeId > 0 && invitation.InviteeId != userId {
		return nil, errors.New("this invitation is not addressed to the current user")
	}
	if invitation.Status != OrgInvitationStatusPending {
		return nil, invitationStatusError(invitation.Status)
	}
	if invitation.IsExpired() {
		markInvitationExpired(invitation.Id)
		return nil, errors.New("invitation has expired")
	}
	if invitation.MaxUses > 0 && invitation.UsedCount >= invitation.MaxUses {
		return nil, errors.New("invitation link has reached its maximum number of uses")
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		// 行锁组织记录,使"统计成员数→插入成员"在锁内串行,防并发绕过成员上限(ORG-5)
		var org Organization
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&org, "id = ?", invitation.OrganizationId).Error; err != nil {
			return errors.New("organization not found or deleted")
		}
		if org.Status != OrganizationStatusEnabled {
			return errors.New("organization is disabled")
		}
		var existing int64
		if err := tx.Model(&OrganizationMember{}).
			Where("organization_id = ? AND user_id = ?", org.Id, userId).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return errors.New("you are already a member of this organization")
		}
		if maxMembers := org.GetMaxMembers(); maxMembers > 0 {
			var count int64
			if err := tx.Model(&OrganizationMember{}).
				Where("organization_id = ?", org.Id).Count(&count).Error; err != nil {
				return err
			}
			if count >= int64(maxMembers) {
				return errors.New("organization member limit reached")
			}
		}
		now := utils.GetTimestamp()
		updates := map[string]interface{}{"used_count": gorm.Expr("used_count + 1"), "updated_time": now}
		cond := tx.Model(&OrganizationInvitation{}).Where("id = ? AND status = ?", invitation.Id, OrgInvitationStatusPending)
		if invitation.InviteeId > 0 {
			updates["status"] = OrgInvitationStatusAccepted
		} else {
			cond = cond.Where("max_uses = 0 OR used_count < max_uses")
		}
		result := cond.Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("invitation is no longer valid")
		}
		member := &OrganizationMember{
			OrganizationId: org.Id,
			UserId:         userId,
			Role:           invitation.Role,
			CreatedTime:    now,
			UpdatedTime:    now,
		}
		return tx.Create(member).Error
	})
	if err != nil {
		return nil, err
	}
	return GetOrganizationInvitationByToken(token)
}

// RejectOrganizationInvitation 拒绝定向邀请(仅被邀请人本人;链接邀请不支持拒绝)
func RejectOrganizationInvitation(token string, userId int) (*OrganizationInvitation, error) {
	invitation, err := GetOrganizationInvitationByToken(token)
	if err != nil {
		return nil, errors.New("invitation not found")
	}
	if invitation.InviteeId == 0 {
		return nil, errors.New("link invitations do not need to be declined")
	}
	if invitation.InviteeId != userId {
		return nil, errors.New("this invitation is not addressed to the current user")
	}
	if invitation.Status != OrgInvitationStatusPending {
		return nil, invitationStatusError(invitation.Status)
	}
	if invitation.IsExpired() {
		markInvitationExpired(invitation.Id)
		return nil, errors.New("invitation has expired")
	}
	result := DB.Model(&OrganizationInvitation{}).
		Where("id = ? AND status = ?", invitation.Id, OrgInvitationStatusPending).
		Updates(map[string]interface{}{"status": OrgInvitationStatusRejected, "updated_time": utils.GetTimestamp()})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, errors.New("invitation is no longer valid")
	}
	invitation.Status = OrgInvitationStatusRejected
	return invitation, nil
}

// RevokeOrganizationInvitation Admin 撤销待处理邀请(pending → revoked)
func RevokeOrganizationInvitation(organizationId int, invitationId int) (*OrganizationInvitation, error) {
	var invitation OrganizationInvitation
	if err := DB.First(&invitation, "id = ? AND organization_id = ?", invitationId, organizationId).Error; err != nil {
		return nil, errors.New("invitation not found")
	}
	if invitation.Status != OrgInvitationStatusPending {
		return nil, invitationStatusError(invitation.Status)
	}
	result := DB.Model(&OrganizationInvitation{}).
		Where("id = ? AND status = ?", invitation.Id, OrgInvitationStatusPending).
		Updates(map[string]interface{}{"status": OrgInvitationStatusRevoked, "updated_time": utils.GetTimestamp()})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, errors.New("invitation is no longer valid")
	}
	invitation.Status = OrgInvitationStatusRevoked
	return &invitation, nil
}
