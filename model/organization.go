package model

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/database"
	"github.com/modeltaps/modeltaps/common/redis"
	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	OrganizationStatusEnabled  = 1 // don't use 0, 0 is the default value!
	OrganizationStatusDisabled = 2
)

// OrganizationSetting 组织级设置(organizations.setting JSON)
type OrganizationSetting struct {
	// UsageVisibleToMembers Member 是否可见全员用量;nil 时取站点默认值 config.OrganizationUsageVisibleDefault
	UsageVisibleToMembers *bool `json:"usage_visible_to_members,omitempty"`
	// ModelWhitelist 组织级模型白名单缺省(空=不限制)
	ModelWhitelist []string `json:"model_whitelist,omitempty"`
	// LogIODefault 组织令牌「完整请求/响应留存」继承默认;nil 时取站点默认 config.OrganizationLogIODefault
	LogIODefault *bool `json:"log_io_default,omitempty"`
	// Budget 组织级周期预算(T1):跨全体成员合计上限;nil 或 limit<=0 表示不限
	Budget *QuotaResetSetting `json:"budget,omitempty"`
}

// Organization 组织元数据表。积分池不在本表:方案 B 中组织积分复用影子账户(users 表)的 quota/used_quota
type Organization struct {
	Id           int            `json:"id"`
	Name         string         `json:"name" gorm:"type:varchar(100);not null;index"`
	Slug         string         `json:"slug" gorm:"type:varchar(64);not null;uniqueIndex"`
	AvatarUrl    string         `json:"avatar_url" gorm:"type:varchar(500);default:''"`
	Status       int            `json:"status" gorm:"type:int;default:1"`
	ShadowUserId int            `json:"-" gorm:"type:int;not null;uniqueIndex"`    // 影子记账账户 users.id,仅服务端使用,不对外暴露
	CreatedBy    int            `json:"created_by" gorm:"type:int;not null;index"` // 创建者(首任 Owner)用户ID
	MaxMembers   int            `json:"max_members" gorm:"type:int;default:0"`     // 0=使用站点默认值 config.OrganizationDefaultMaxMembers
	CreatedTime  int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime  int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`

	Setting database.JSONType[OrganizationSetting] `json:"setting" gorm:"type:json"`

	// 组织级周期预算计数(T1,沿用 SEC-4 成员预算的「专用列 + 单条原子 UPDATE」模式,不放 JSON)。
	// 不随组织资料对外序列化:仅 GetOrganization 对 Owner/Admin 显式返回
	BudgetUsed  int   `json:"-" gorm:"type:bigint;not null;default:0"`
	BudgetStart int64 `json:"-" gorm:"bigint;not null;default:0"`
}

func (org *Organization) Insert() error {
	if strings.TrimSpace(org.Name) == "" {
		return errors.New("organization name must not be empty")
	}
	if strings.TrimSpace(org.Slug) == "" {
		org.Slug = GenerateOrgSlug(org.Name)
	}
	if org.Status == 0 {
		org.Status = OrganizationStatusEnabled
	}
	org.CreatedTime = utils.GetTimestamp()
	org.UpdatedTime = org.CreatedTime
	return DB.Create(org).Error
}

func GetOrganizationById(id int) (*Organization, error) {
	if id == 0 {
		return nil, errors.New("id is empty")
	}
	var org Organization
	err := DB.First(&org, "id = ?", id).Error
	return &org, err
}

func GetOrganizationBySlug(slug string) (*Organization, error) {
	if slug == "" {
		return nil, errors.New("slug is empty")
	}
	var org Organization
	err := DB.First(&org, "slug = ?", slug).Error
	return &org, err
}

// CountUserCreatedOrganizations 统计用户已创建的组织数,用于校验 config.OrganizationMaxPerUser
func CountUserCreatedOrganizations(userId int) (int64, error) {
	var count int64
	err := DB.Model(&Organization{}).Where("created_by = ?", userId).Count(&count).Error
	return count, err
}

// GetMaxMembers 返回组织生效的成员上限(0=不限制)
func (org *Organization) GetMaxMembers() int {
	if org.MaxMembers > 0 {
		return org.MaxMembers
	}
	return config.OrganizationDefaultMaxMembers
}

// IsUsageVisibleToMembers 返回 Member 是否可见全员用量(组织未配置时取站点默认值)
func (org *Organization) IsUsageVisibleToMembers() bool {
	setting := org.Setting.Data()
	if setting.UsageVisibleToMembers != nil {
		return *setting.UsageVisibleToMembers
	}
	return config.OrganizationUsageVisibleDefault
}

// ResolveLogIODefault 返回组织令牌「继承」时的 LogIO 默认(组织未配置时取站点默认 config.OrganizationLogIODefault)
func (org *Organization) ResolveLogIODefault() bool {
	setting := org.Setting.Data()
	if setting.LogIODefault != nil {
		return *setting.LogIODefault
	}
	return config.OrganizationLogIODefault
}

var orgSlugInvalidChars = regexp.MustCompile(`[^a-z0-9-]+`)
var orgSlugDashCollapse = regexp.MustCompile(`-{2,}`)

// GenerateOrgSlug 根据组织名生成全局唯一 slug:小写化清洗 + 随机后缀,冲突时重试
func GenerateOrgSlug(name string) string {
	base := strings.ToLower(strings.TrimSpace(name))
	base = orgSlugInvalidChars.ReplaceAllString(base, "-")
	base = orgSlugDashCollapse.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if len(base) > 32 {
		base = base[:32]
	}
	if base == "" {
		base = "org"
	}
	slug := base + "-" + strings.ToLower(utils.GetRandomString(6))
	for i := 0; i < 5 && RecordExists(&Organization{}, "slug", slug, nil); i++ {
		slug = base + "-" + strings.ToLower(utils.GetRandomString(6))
	}
	return slug
}

// CreateShadowUserForOrg 在事务中为组织创建影子记账账户(禁止登录的虚拟用户)。
// 不走 User.Insert:不发放新用户配额、不参与邀请奖励;密码为随机值且登录路径统一排除影子账户。
func CreateShadowUserForOrg(tx *gorm.DB, orgSlug string) (*User, error) {
	if strings.TrimSpace(orgSlug) == "" {
		return nil, errors.New("organization slug must not be empty")
	}
	hashedPassword, err := common.Password2Hash(utils.GetUUID() + utils.GetUUID())
	if err != nil {
		return nil, err
	}
	user := &User{
		Username:    "org-" + orgSlug,
		Password:    hashedPassword,
		DisplayName: "Organization " + orgSlug,
		Role:        config.RoleGuestUser,
		Status:      config.UserStatusEnabled,
		Type:        config.UserTypeOrgShadow,
		Quota:       0,
		AccessToken: utils.GetUUID(),
		AffCode:     strings.ToLower(utils.GetRandomString(8)),
		CreatedTime: utils.GetTimestamp(),
	}
	if err := tx.Create(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

// Update 更新组织资料(名称/头像),同时刷新 UpdatedTime
func (org *Organization) Update() error {
	if strings.TrimSpace(org.Name) == "" {
		return errors.New("organization name must not be empty")
	}
	org.UpdatedTime = utils.GetTimestamp()
	return DB.Model(org).Select("name", "avatar_url", "updated_time").Updates(org).Error
}

// UpdateSetting 更新组织设置(setting JSON),同时刷新 UpdatedTime
func (org *Organization) UpdateSetting() error {
	org.UpdatedTime = utils.GetTimestamp()
	err := DB.Model(org).Select("setting", "updated_time").Updates(org).Error
	if err == nil {
		// 组织默认变更后失效解析缓存(键为影子账户ID),确保组织令牌「继承」按新默认回退
		InvalidateOwnerLogIODefaultCache(org.ShadowUserId)
		// 组织级白名单缺省/预算配置进入成员守护视图,变更后须失效全体成员的守护缓存
		invalidateOrgGuardrailCacheAllMembers(org)
	}
	return err
}

// CreateOrganizationWithOwner 在单事务中创建组织:生成 slug → 创建影子记账账户 → 创建组织 → 创建者设为 Owner
func CreateOrganizationWithOwner(name string, avatarUrl string, creatorId int) (*Organization, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("organization name must not be empty")
	}
	if creatorId == 0 {
		return nil, errors.New("creator id must not be empty")
	}
	// 普通用户须有一条已验证的联系通道(邮箱或手机号任一非空即已验证:邮箱经验证码/IdP 已验证 claim
	// 写入,手机号只由 IdP 已验证 claim 写入);root 免检
	var creator User
	if err := DB.Select("id", "role", "email", "phone_number").Where("id = ?", creatorId).First(&creator).Error; err != nil {
		return nil, errors.New("creator not found")
	}
	if creator.Role != config.RoleRootUser &&
		strings.TrimSpace(string(creator.Email)) == "" &&
		strings.TrimSpace(string(creator.PhoneNumber)) == "" {
		return nil, errors.New("verify your email or phone number before creating an organization")
	}
	slug := GenerateOrgSlug(name)
	org := &Organization{
		Name:      name,
		Slug:      slug,
		AvatarUrl: avatarUrl,
		Status:    OrganizationStatusEnabled,
		CreatedBy: creatorId,
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		shadowUser, err := CreateShadowUserForOrg(tx, slug)
		if err != nil {
			return err
		}
		org.ShadowUserId = shadowUser.Id
		org.CreatedTime = utils.GetTimestamp()
		org.UpdatedTime = org.CreatedTime
		if err := tx.Create(org).Error; err != nil {
			return err
		}
		member := &OrganizationMember{
			OrganizationId: org.Id,
			UserId:         creatorId,
			Role:           OrgRoleOwner,
			CreatedTime:    org.CreatedTime,
			UpdatedTime:    org.CreatedTime,
		}
		return tx.Create(member).Error
	})
	if err != nil {
		return nil, err
	}
	return org, nil
}

// TransferOrganizationOwnership Owner 将所有权转移给指定 Admin,自身降为 Admin(事务,见规格 §3.1)
func TransferOrganizationOwnership(org *Organization, fromUserId int, toUserId int) error {
	if fromUserId == toUserId {
		return errors.New("you cannot transfer ownership to yourself")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var target OrganizationMember
		if err := tx.First(&target, "organization_id = ? AND user_id = ?", org.Id, toUserId).Error; err != nil {
			return errors.New("the target user is not a member of this organization")
		}
		if target.Role != OrgRoleAdmin {
			return errors.New("ownership can only be transferred to an organization admin")
		}
		now := utils.GetTimestamp()
		result := tx.Model(&OrganizationMember{}).
			Where("organization_id = ? AND user_id = ? AND role = ?", org.Id, fromUserId, OrgRoleOwner).
			Updates(map[string]interface{}{"role": OrgRoleAdmin, "updated_time": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("only the organization owner can transfer ownership")
		}
		return tx.Model(&OrganizationMember{}).
			Where("id = ?", target.Id).
			Updates(map[string]interface{}{"role": OrgRoleOwner, "updated_time": now}).Error
	})
}

// DissolveOrganization 解散组织(事务):组织令牌全部禁用、剩余积分按站点策略退回、
// 影子账户停用并清零、成员清空、待处理邀请撤销、组织软删除;事务提交后清理相关 Redis 缓存
func DissolveOrganization(org *Organization, ownerId int) error {
	shadowId := org.ShadowUserId
	var tokenKeys []string
	err := DB.Transaction(func(tx *gorm.DB) error {
		// 先收集组织令牌 key,事务提交后统一清缓存
		if err := tx.Model(&Token{}).Where("user_id = ?", shadowId).Pluck("key", &tokenKeys).Error; err != nil {
			return err
		}
		// 1. 组织令牌全部失效
		if err := tx.Model(&Token{}).Where("user_id = ?", shadowId).Updates(map[string]interface{}{
			"status":        config.TokenStatusDisabled,
			"accessed_time": utils.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		// 2. 影子账户停用并清零积分:行锁读取余额 + 条件更新双保险(跨库),
		// 消除"读余额→退款→清零"之间的窗口,避免与在途结算并发多退(ORG-4)
		var shadow User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "quota").Where("id = ?", shadowId).First(&shadow).Error; err != nil {
			return err
		}
		result := tx.Model(&User{}).Where("id = ? AND quota = ?", shadowId, shadow.Quota).Updates(map[string]interface{}{
			"status": config.UserStatusDisabled,
			"quota":  0,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("the organization balance is changing, please try deleting it again later")
		}
		// 3. 剩余积分按站点策略处理(owner=退回 Owner 个人账户)
		if config.OrganizationDissolveQuotaRefund == "owner" && shadow.Quota > 0 {
			if err := IncreaseUserQuotaWithTx(tx, ownerId, shadow.Quota); err != nil {
				return err
			}
			RecordLogWithTx(tx, ownerId, LogTypeSystem, fmt.Sprintf("Deleted organization %s, remaining balance %s returned to personal account", org.Name, common.LogQuota(shadow.Quota)))
		}
		// 4. 清空成员、撤销待处理邀请
		if err := tx.Where("organization_id = ?", org.Id).Delete(&OrganizationMember{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&OrganizationInvitation{}).
			Where("organization_id = ? AND status = ?", org.Id, OrgInvitationStatusPending).
			Updates(map[string]interface{}{"status": OrgInvitationStatusRevoked, "updated_time": utils.GetTimestamp()}).Error; err != nil {
			return err
		}
		// 5. 组织软删除
		return tx.Delete(org).Error
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
		redis.RedisDel(fmt.Sprintf(UserQuotaCacheKey, shadowId))
		redis.RedisDel(fmt.Sprintf(UserQuotaCacheKey, ownerId))
	}
	return nil
}

// IsShadowUser 判断用户是否为组织影子记账账户
func IsShadowUser(user *User) bool {
	return user != nil && user.Type == config.UserTypeOrgShadow
}

// IsShadowUserId 按用户ID判断是否为组织影子记账账户
func IsShadowUserId(userId int) bool {
	if userId == 0 {
		return false
	}
	var user User
	if err := DB.Select("type").Where("id = ?", userId).First(&user).Error; err != nil {
		return false
	}
	return IsShadowUser(&user)
}

// ExcludeShadowUsers 统一的"排除组织影子账户"查询封装。
// 所有面向真实用户的查询(登录、用户列表/搜索、邮件通知、密码重置等)都应套用本函数。
func ExcludeShadowUsers(db *gorm.DB) *gorm.DB {
	return db.Where("type IS NULL OR type = ?", config.UserTypeNormal)
}

// ---- 站点级组织管理(T7,规格 §3.8,仅 root) ----

// OrganizationAdminInfo 站点管理后台视角的组织信息:组织元数据 + 组织池余额(影子账户)+ 成员数 + Owner
type OrganizationAdminInfo struct {
	Organization
	OwnerId     int   `json:"owner_id"`
	Quota       int   `json:"quota"`
	UsedQuota   int   `json:"used_quota"`
	MemberCount int64 `json:"member_count"`
}

// fillOrganizationAdminInfos 批量补全组织池余额、成员数与 Owner
func fillOrganizationAdminInfos(orgs []*Organization) ([]*OrganizationAdminInfo, error) {
	infos := make([]*OrganizationAdminInfo, 0, len(orgs))
	if len(orgs) == 0 {
		return infos, nil
	}
	orgIds := make([]int, 0, len(orgs))
	shadowIds := make([]int, 0, len(orgs))
	for _, org := range orgs {
		orgIds = append(orgIds, org.Id)
		shadowIds = append(shadowIds, org.ShadowUserId)
	}
	var shadows []User
	if err := DB.Select("id", "quota", "used_quota").Where("id IN ?", shadowIds).Find(&shadows).Error; err != nil {
		return nil, err
	}
	shadowById := make(map[int]User, len(shadows))
	for _, u := range shadows {
		shadowById[u.Id] = u
	}
	type orgCount struct {
		OrganizationId int
		Count          int64
	}
	var counts []orgCount
	if err := DB.Model(&OrganizationMember{}).Select("organization_id, COUNT(*) as count").
		Where("organization_id IN ?", orgIds).Group("organization_id").Find(&counts).Error; err != nil {
		return nil, err
	}
	countByOrg := make(map[int]int64, len(counts))
	for _, c := range counts {
		countByOrg[c.OrganizationId] = c.Count
	}
	var owners []OrganizationMember
	if err := DB.Select("organization_id", "user_id").
		Where("organization_id IN ? AND role = ?", orgIds, OrgRoleOwner).Find(&owners).Error; err != nil {
		return nil, err
	}
	ownerByOrg := make(map[int]int, len(owners))
	for _, m := range owners {
		ownerByOrg[m.OrganizationId] = m.UserId
	}
	for _, org := range orgs {
		shadow := shadowById[org.ShadowUserId]
		infos = append(infos, &OrganizationAdminInfo{
			Organization: *org,
			OwnerId:      ownerByOrg[org.Id],
			Quota:        shadow.Quota,
			UsedQuota:    shadow.UsedQuota,
			MemberCount:  countByOrg[org.Id],
		})
	}
	return infos, nil
}

// GetOrganizationsAdminList 站点管理后台组织分页列表(keyword 按名称/slug 前缀搜索)
func GetOrganizationsAdminList(params *GenericParams) (*DataResult[OrganizationAdminInfo], error) {
	var orgs []*Organization
	db := DB.Model(&Organization{})
	if params.Keyword != "" {
		db = db.Where("name LIKE ? OR slug LIKE ?", params.Keyword+"%", params.Keyword+"%")
	}
	pageResult, err := PaginateAndOrder[Organization](db, &params.PaginationParams, &orgs, allowedOrgAdminOrderFields)
	if err != nil {
		return nil, err
	}
	infos, err := fillOrganizationAdminInfos(orgs)
	if err != nil {
		return nil, err
	}
	return &DataResult[OrganizationAdminInfo]{
		Data:       &infos,
		Page:       pageResult.Page,
		Size:       pageResult.Size,
		TotalCount: pageResult.TotalCount,
	}, nil
}

var allowedOrgAdminOrderFields = map[string]bool{
	"id":           true,
	"name":         true,
	"status":       true,
	"created_time": true,
}

// GetOrganizationAdminInfo 站点管理后台组织详情(含组织池余额、成员数与 Owner)
func GetOrganizationAdminInfo(id int) (*OrganizationAdminInfo, error) {
	org, err := GetOrganizationById(id)
	if err != nil {
		return nil, err
	}
	infos, err := fillOrganizationAdminInfos([]*Organization{org})
	if err != nil {
		return nil, err
	}
	return infos[0], nil
}

// SetOrganizationStatus 站点管理员启用/禁用组织:同步影子账户状态使组织令牌即刻不可用/恢复,
// 并清理影子账户启用态缓存(令牌校验链路依赖 CacheIsUserEnabled)
func SetOrganizationStatus(org *Organization, status int) error {
	if status != OrganizationStatusEnabled && status != OrganizationStatusDisabled {
		return errors.New("invalid organization status")
	}
	userStatus := config.UserStatusEnabled
	if status == OrganizationStatusDisabled {
		userStatus = config.UserStatusDisabled
	}
	now := utils.GetTimestamp()
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Organization{}).Where("id = ?", org.Id).
			Updates(map[string]interface{}{"status": status, "updated_time": now}).Error; err != nil {
			return err
		}
		return tx.Model(&User{}).Where("id = ?", org.ShadowUserId).Update("status", userStatus).Error
	})
	if err != nil {
		return err
	}
	org.Status = status
	org.UpdatedTime = now
	if config.RedisEnabled {
		redis.RedisDel(fmt.Sprintf(UserEnabledCacheKey, org.ShadowUserId))
	}
	return nil
}

// GetOrganizationOwnerId 查询组织 Owner 的用户ID(站点后台解散时用于剩余积分退回)
func GetOrganizationOwnerId(orgId int) (int, error) {
	var member OrganizationMember
	if err := DB.Select("user_id").First(&member, "organization_id = ? AND role = ?", orgId, OrgRoleOwner).Error; err != nil {
		return 0, errors.New("organization owner not found")
	}
	return member.UserId, nil
}
