package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/database"
	"github.com/modeltaps/modeltaps/common/redis"
	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 组织成员角色(三级,见规格 §2.2 角色与权限矩阵)
const (
	OrgRoleOwner  = "owner"
	OrgRoleAdmin  = "admin"
	OrgRoleMember = "member"
)

// OrganizationMember 用户与组织的多对多关系(带角色)。物理删除表示退出/移除,不做软删除
type OrganizationMember struct {
	Id             int    `json:"id"`
	OrganizationId int    `json:"organization_id" gorm:"type:int;not null;uniqueIndex:idx_org_member_org_user,priority:1"`
	UserId         int    `json:"user_id" gorm:"type:int;not null;uniqueIndex:idx_org_member_org_user,priority:2;index"`
	Role           string `json:"role" gorm:"type:varchar(16);not null;default:'member'"`
	CreatedTime    int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime    int64  `json:"updated_time" gorm:"bigint"`

	// Setting 成员级限制(T5):周期预算 + 模型白名单,见 OrgMemberSetting
	Setting database.JSONType[OrgMemberSetting] `json:"setting" gorm:"type:json"`

	// 周期预算计数(SEC-4):从 setting JSON 移出为专用列,使 relay 计费 accrue 走单条原子自增,杜绝并发丢更新
	BudgetUsed  int   `json:"budget_used" gorm:"type:bigint;not null;default:0"`
	BudgetStart int64 `json:"budget_start" gorm:"bigint;not null;default:0"`
}

func IsValidOrgRole(role string) bool {
	return role == OrgRoleOwner || role == OrgRoleAdmin || role == OrgRoleMember
}

// OrgRoleWeight 角色权重(见规格 §2.2):owner(3) > admin(2) > member(1);无效角色返回 0
func OrgRoleWeight(role string) int {
	switch role {
	case OrgRoleOwner:
		return 3
	case OrgRoleAdmin:
		return 2
	case OrgRoleMember:
		return 1
	default:
		return 0
	}
}

func (member *OrganizationMember) Insert() error {
	if member.OrganizationId == 0 || member.UserId == 0 {
		return errors.New("organization id or user id must not be empty")
	}
	if !IsValidOrgRole(member.Role) {
		return errors.New("invalid organization role")
	}
	member.CreatedTime = utils.GetTimestamp()
	member.UpdatedTime = member.CreatedTime
	return DB.Create(member).Error
}

func GetOrganizationMember(organizationId int, userId int) (*OrganizationMember, error) {
	if organizationId == 0 || userId == 0 {
		return nil, errors.New("organization id or user id must not be empty")
	}
	var member OrganizationMember
	err := DB.First(&member, "organization_id = ? AND user_id = ?", organizationId, userId).Error
	return &member, err
}

func CountOrganizationMembers(organizationId int) (int64, error) {
	var count int64
	err := DB.Model(&OrganizationMember{}).Where("organization_id = ?", organizationId).Count(&count).Error
	return count, err
}

// GetUserOrganizationIds 返回用户所属的全部组织ID
func GetUserOrganizationIds(userId int) ([]int, error) {
	var ids []int
	err := DB.Model(&OrganizationMember{}).Where("user_id = ?", userId).Pluck("organization_id", &ids).Error
	return ids, err
}

// UserOrganization 组织信息 + 当前用户在该组织中的角色
type UserOrganization struct {
	Organization
	Role string `json:"role"`
}

// GetUserOrganizationsWithRole 返回用户所属的全部组织(含自身角色,组织软删除自动排除)
func GetUserOrganizationsWithRole(userId int) ([]*UserOrganization, error) {
	var members []OrganizationMember
	if err := DB.Where("user_id = ?", userId).Find(&members).Error; err != nil {
		return nil, err
	}
	result := make([]*UserOrganization, 0, len(members))
	if len(members) == 0 {
		return result, nil
	}
	roleByOrgId := make(map[int]string, len(members))
	ids := make([]int, 0, len(members))
	for _, m := range members {
		roleByOrgId[m.OrganizationId] = m.Role
		ids = append(ids, m.OrganizationId)
	}
	var orgs []Organization
	if err := DB.Where("id IN ?", ids).Order("id desc").Find(&orgs).Error; err != nil {
		return nil, err
	}
	for _, org := range orgs {
		result = append(result, &UserOrganization{Organization: org, Role: roleByOrgId[org.Id]})
	}
	return result, nil
}

var allowedOrgMemberOrderFields = map[string]bool{
	"id":           true,
	"role":         true,
	"created_time": true,
}

// OrganizationMemberInfo 成员 + 用户基础信息(成员列表展示用)。
// 不透出原始 setting JSON;预算字段按查看者权限选择性填充(见 GetOrganizationMembersList)
type OrganizationMemberInfo struct {
	Id             int                `json:"id"`
	OrganizationId int                `json:"organization_id"`
	UserId         int                `json:"user_id"`
	Role           string             `json:"role"`
	CreatedTime    int64              `json:"created_time"`
	UpdatedTime    int64              `json:"updated_time"`
	Username       string             `json:"username"`
	DisplayName    string             `json:"display_name"`
	Budget         *QuotaResetSetting `json:"budget,omitempty"`
	BudgetUsed     int                `json:"budget_used,omitempty"`
}

// GetOrganizationMembersList 分页查询组织成员(附用户名/显示名)。
// 预算字段(T21c):viewerIsAdmin 返回全员,否则仅返回 viewer 本人;
// 已用额度按当前周期懒重置语义折算(跨期视为 0,同 BudgetExceeded)
func GetOrganizationMembersList(organizationId int, viewerUserId int, viewerIsAdmin bool, params *PaginationParams) (*DataResult[OrganizationMemberInfo], error) {
	var members []*OrganizationMember
	pageResult, err := PaginateAndOrder[OrganizationMember](DB.Where("organization_id = ?", organizationId), params, &members, allowedOrgMemberOrderFields)
	if err != nil {
		return nil, err
	}
	userById := make(map[int]User, len(members))
	if len(members) > 0 {
		userIds := make([]int, 0, len(members))
		for _, m := range members {
			userIds = append(userIds, m.UserId)
		}
		var users []User
		if err := DB.Select("id", "username", "display_name").Where("id IN ?", userIds).Find(&users).Error; err != nil {
			return nil, err
		}
		for _, u := range users {
			userById[u.Id] = u
		}
	}
	now := time.Now()
	infos := make([]*OrganizationMemberInfo, 0, len(members))
	for _, m := range members {
		u := userById[m.UserId]
		info := &OrganizationMemberInfo{
			Id:             m.Id,
			OrganizationId: m.OrganizationId,
			UserId:         m.UserId,
			Role:           m.Role,
			CreatedTime:    m.CreatedTime,
			UpdatedTime:    m.UpdatedTime,
			Username:       u.Username,
			DisplayName:    u.DisplayName,
		}
		if viewerIsAdmin || m.UserId == viewerUserId {
			setting := m.Setting.Data()
			if setting.Budget != nil && setting.Budget.Limit > 0 {
				info.Budget = setting.Budget
				if ps := currentPeriodStart(setting.Budget.Period, now); !ps.IsZero() && m.BudgetStart >= ps.Unix() {
					info.BudgetUsed = m.BudgetUsed
				}
			}
		}
		infos = append(infos, info)
	}
	return &DataResult[OrganizationMemberInfo]{Data: &infos, Page: pageResult.Page, Size: pageResult.Size, TotalCount: pageResult.TotalCount}, nil
}

// MemberBudgetView 成员本人的周期预算视图(组织详情用):Budget 为 nil 表示未配置(不限额)
type MemberBudgetView struct {
	Budget *QuotaResetSetting
	Used   int
	Start  int64
}

// GetOrgMemberBudget 返回成员本人的周期预算与本期已用(跨期按懒重置语义折算为 0,同 BudgetExceeded)。
// 未配置预算(nil 或 limit<=0)时返回 nil,不视为错误。
func GetOrgMemberBudget(organizationId int, userId int) (*MemberBudgetView, error) {
	member, err := GetOrganizationMember(organizationId, userId)
	if err != nil {
		return nil, err
	}
	setting := member.Setting.Data()
	if setting.Budget == nil || setting.Budget.Limit <= 0 {
		return nil, nil
	}
	view := &MemberBudgetView{Budget: setting.Budget, Start: member.BudgetStart}
	if ps := currentPeriodStart(setting.Budget.Period, time.Now()); !ps.IsZero() && member.BudgetStart >= ps.Unix() {
		view.Used = member.BudgetUsed
	}
	return view, nil
}

// CreateOrgMemberAccount 在组织下代为创建新用户并加入为成员(Owner/Admin;规格 §2.3 扩展)。
// 事务内:行锁组织 → 复核成员上限 → 创建 User → 建立成员关系。role 仅限 admin/member;失败整体回滚。
// grantSignupBonus 默认应传 false:代建账号不发新用户注册奖励,防止刷免费额度(SEC-11)。
func CreateOrgMemberAccount(org *Organization, username, password, displayName, email, role string, grantSignupBonus bool) (*User, *OrganizationMember, error) {
	if role != OrgRoleAdmin && role != OrgRoleMember {
		return nil, nil, errors.New("role must be admin or member")
	}
	if password == "" {
		return nil, nil, errors.New("password must not be empty")
	}
	if displayName == "" {
		displayName = username
	}
	// 邮箱归一化后落库；insertWithTx 内会再做一次归一化 + 查重，此处显式转换保持类型一致
	user := &User{Username: username, Password: password, DisplayName: displayName,
		Email: NullableEmail(common.NormalizeEmail(email))}
	var member *OrganizationMember
	err := DB.Transaction(func(tx *gorm.DB) error {
		// 行锁组织记录,使"统计成员数→插入成员"在锁内串行,防并发绕过成员上限(ORG-5)
		var locked Organization
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&locked, "id = ?", org.Id).Error; err != nil {
			return errors.New("organization not found or deleted")
		}
		if maxMembers := locked.GetMaxMembers(); maxMembers > 0 {
			var count int64
			if err := tx.Model(&OrganizationMember{}).Where("organization_id = ?", org.Id).Count(&count).Error; err != nil {
				return err
			}
			if count >= int64(maxMembers) {
				return errors.New("organization member limit reached")
			}
		}
		if err := user.insertWithTx(tx, 0, grantSignupBonus); err != nil {
			return err
		}
		now := utils.GetTimestamp()
		member = &OrganizationMember{
			OrganizationId: org.Id,
			UserId:         user.Id,
			Role:           role,
			CreatedTime:    now,
			UpdatedTime:    now,
		}
		return tx.Create(member).Error
	})
	if err != nil {
		return nil, nil, err
	}
	return user, member, nil
}

// UpdateOrganizationMemberRole 修改成员角色(admin/member 之间)。
// Owner 角色不可经本函数授予或剥夺(走所有权转移);角色层级校验(操作者 vs 目标)由调用方完成
func UpdateOrganizationMemberRole(organizationId int, targetUserId int, newRole string) (*OrganizationMember, error) {
	if newRole != OrgRoleAdmin && newRole != OrgRoleMember {
		return nil, errors.New("role must be admin or member")
	}
	member, err := GetOrganizationMember(organizationId, targetUserId)
	if err != nil {
		return nil, errors.New("this user is not a member of the organization")
	}
	if member.Role == OrgRoleOwner {
		return nil, errors.New("cannot change the role of the organization owner")
	}
	if member.Role == newRole {
		return member, nil
	}
	member.Role = newRole
	member.UpdatedTime = utils.GetTimestamp()
	if err := DB.Model(member).Select("role", "updated_time").Updates(member).Error; err != nil {
		return nil, err
	}
	return member, nil
}

// RemoveOrganizationMember 将成员移出组织(移除/主动退出共用,事务):
// 删除成员关系并禁用其创建的全部组织令牌(tokens.user_id=影子账户 AND created_by=该成员),
// 事务提交后清理令牌 Redis 缓存。Owner 不可经本函数移除(须先转移所有权)
func RemoveOrganizationMember(org *Organization, targetUserId int) error {
	if targetUserId == 0 {
		return errors.New("user id must not be empty")
	}
	var tokenKeys []string
	err := DB.Transaction(func(tx *gorm.DB) error {
		var member OrganizationMember
		if err := tx.First(&member, "organization_id = ? AND user_id = ?", org.Id, targetUserId).Error; err != nil {
			return errors.New("this user is not a member of the organization")
		}
		if member.Role == OrgRoleOwner {
			return errors.New("the organization owner cannot leave or be removed; transfer ownership first")
		}
		if err := tx.Delete(&member).Error; err != nil {
			return err
		}
		memberTokens := tx.Model(&Token{}).Where("user_id = ? AND created_by = ?", org.ShadowUserId, targetUserId)
		if err := memberTokens.Pluck("key", &tokenKeys).Error; err != nil {
			return err
		}
		return tx.Model(&Token{}).Where("user_id = ? AND created_by = ?", org.ShadowUserId, targetUserId).
			Updates(map[string]interface{}{
				"status":        config.TokenStatusDisabled,
				"accessed_time": utils.GetTimestamp(),
			}).Error
	})
	if err != nil {
		return err
	}
	if config.RedisEnabled {
		for _, key := range tokenKeys {
			if key != "" {
				redis.RedisDel(fmt.Sprintf(UserTokensKey, key))
			}
		}
	}
	return nil
}
