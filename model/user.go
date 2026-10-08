package model

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/cache"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/database"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/redis"
	"github.com/modeltaps/modeltaps/common/utils"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"
)

// NullableEmail 是 users.email 的列类型：Go 侧仍是字符串（空串表示"无邮箱"，
// 现有 `Email != ""` / `== ""` 判断语义不变），落库时空串写为 NULL。
// 这样 MySQL / PostgreSQL / SQLite 的普通唯一索引都能同时满足
// "一个邮箱只对应一个账号" 与 "任意多个无邮箱用户共存"（三库的唯一索引均不约束 NULL）。
type NullableEmail string

// Value 空串落库为 NULL，其余原样写入。
func (e NullableEmail) Value() (driver.Value, error) {
	if e == "" {
		return nil, nil
	}
	return string(e), nil
}

// Scan NULL 读回为空串，与历史上的 '' 语义一致。
func (e *NullableEmail) Scan(value any) error {
	switch v := value.(type) {
	case nil:
		*e = ""
	case string:
		*e = NullableEmail(v)
	case []byte:
		*e = NullableEmail(v)
	default:
		return fmt.Errorf("cannot parse %T as an email", value)
	}
	return nil
}

// GormDataType 保持与原 string 字段一致的列类型推导（各方言的默认字符串列）。
func (NullableEmail) GormDataType() string {
	return "string"
}

// NullablePhone 是 users.phone_number 的列类型，语义与 NullableEmail 完全一致：
// Go 侧仍是字符串（空串表示"无手机号"），落库时空串写为 NULL，
// 从而让普通唯一索引同时满足"一个手机号只对应一个账号"与"任意多个无手机号用户共存"。
type NullablePhone string

// Value 空串落库为 NULL，其余原样写入。
func (p NullablePhone) Value() (driver.Value, error) {
	if p == "" {
		return nil, nil
	}
	return string(p), nil
}

// Scan NULL 读回为空串，与历史上的 '' 语义一致。
func (p *NullablePhone) Scan(value any) error {
	switch v := value.(type) {
	case nil:
		*p = ""
	case string:
		*p = NullablePhone(v)
	case []byte:
		*p = NullablePhone(v)
	default:
		return fmt.Errorf("cannot parse %T as a phone number", value)
	}
	return nil
}

// GormDataType 保持与原 string 字段一致的列类型推导（各方言的默认字符串列）。
func (NullablePhone) GormDataType() string {
	return "string"
}

// User if you add sensitive fields, don't forget to clean them in setupLogin function.
// Otherwise, the sensitive information will be saved on local storage in plain text!
type User struct {
	Id          int    `json:"id"`
	Username    string `json:"username" gorm:"unique;index" validate:"required,max=12,excludes=@"`
	Password    string `json:"password" gorm:"not null;" validate:"min=8,max=64"`
	DisplayName string `json:"display_name" gorm:"index" validate:"max=20"`
	Role        int    `json:"role" gorm:"type:int;default:1"`   // admin, common
	Status      int    `json:"status" gorm:"type:int;default:1"` // enabled, disabled
	// uniqueIndex 标签必须保留：MySQL driver 的 MigrateColumnUnique 会在 AutoMigrate 时
	// 把 email 列上「schema 不认识」的单列唯一索引当作冗余 DROP 掉。声明出来之后
	// GORM 既不会删它，索引缺失时还会自动重建（详见 model/migrate.go 的 ensureUserEmailUniqueReady）。
	Email NullableEmail `json:"email" gorm:"index;uniqueIndex:idx_users_email_unique" validate:"max=50"`
	// EmailVerified 记录 users.email 的来源是否经过验证：邮箱验证码注册 / 绑定、IdP 已验证
	// email claim 写入时为 true；管理员后台直接改写等不经验证的写入一律为 false。
	// 账号安全页的「已验证」角标与按已验证邮箱自动关联（link_by_verified_email）都以它为准，
	// 故它只能由服务端授予：任何来自请求体的 email_verified 都不得采信。
	EmailVerified bool   `json:"email_verified" gorm:"type:boolean;default:false;column:email_verified"`
	AvatarUrl     string `json:"avatar_url" gorm:"type:varchar(500);column:avatar_url;default:''"`
	// PhoneNumber 来源是 IdP 下发的已验证 phone_number claim，用户不可自改。
	// 它同时是可关联标识：同一已验证手机号只能归属一个账号（提供方开启
	// link_by_verified_phone 时据此关联），故加唯一索引；空值以 NULL 落库，
	// 任意多个无手机号用户可共存。uniqueIndex 标签必须保留，理由同 Email。
	PhoneNumber       NullablePhone  `json:"phone_number" gorm:"type:varchar(32);column:phone_number;uniqueIndex:idx_users_phone_unique"`
	OidcId            string         `json:"oidc_id" gorm:"column:oidc_id;index"`
	GitHubId          string         `json:"github_id" gorm:"column:github_id;index"`
	GitHubIdNew       int            `json:"github_id_new" gorm:"column:github_id_new;index"`
	WeChatId          string         `json:"wechat_id" gorm:"column:wechat_id;index"`
	TelegramId        int64          `json:"telegram_id" gorm:"type:bigint;column:telegram_id;default:0;"`
	LarkId            string         `json:"lark_id" gorm:"column:lark_id;index"`
	LinuxDoId         int            `json:"linuxdo_id" gorm:"type:bigint;column:linuxdo_id;index;default:0;"`
	LinuxDoUsername   string         `json:"linuxdo_username" gorm:"column:linuxdo_username;index;default:'';"`
	LinuxDoTrustLevel int            `json:"linuxdo_trust_level" gorm:"type:int;column:linuxdo_trust_level;default:0;"`
	VerificationCode  string         `json:"verification_code" gorm:"-:all"`                                    // this field is only for Email verification, don't save it to database!
	InviteCode        string         `json:"invite_code" gorm:"-:all"`                                          // this field is only for registration, don't save it to database!
	UsedInviteCode    string         `json:"used_invite_code" gorm:"type:varchar(32);index;default:''"`         // the invite code used during registration, for statistics
	AccessToken       string         `json:"access_token" gorm:"type:char(32);column:access_token;uniqueIndex"` // this token is for system management
	Quota             int            `json:"quota" gorm:"type:bigint;default:0"`
	UsedQuota         int            `json:"used_quota" gorm:"type:bigint;default:0;column:used_quota"` // used quota
	RequestCount      int            `json:"request_count" gorm:"type:int;default:0;"`                  // request number
	Group             string         `json:"group" gorm:"type:varchar(32);default:'default'"`
	AffCode           string         `json:"aff_code" gorm:"type:varchar(32);column:aff_code;uniqueIndex"`
	AffCount          int            `json:"aff_count" gorm:"type:int;default:0;column:aff_count"`
	AffQuota          int            `json:"aff_quota" gorm:"type:bigint;default:0;column:aff_quota"`
	AffHistoryQuota   int            `json:"aff_history_quota" gorm:"type:bigint;default:0;column:aff_history"`
	InviterId         int            `json:"inviter_id" gorm:"type:int;column:inviter_id;index"`
	LastLoginTime     int64          `json:"last_login_time" gorm:"bigint;default:0"`
	LastLoginIp       string         `json:"last_login_ip" gorm:"type:varchar(128);default:''"`
	Type              int            `json:"type" gorm:"type:int;default:0;index"` // 0=普通用户 1=组织影子记账账户(见 config.UserTypeOrgShadow)
	CreatedTime       int64          `json:"created_time" gorm:"bigint"`
	DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`

	Setting database.JSONType[UserSetting] `json:"setting" gorm:"type:json"`
}

type UserUpdates func(*User)

func GetMaxUserId() int {
	var user User
	DB.Last(&user)
	return user.Id
}

var allowedUserOrderFields = map[string]bool{
	"id":              true,
	"username":        true,
	"role":            true,
	"status":          true,
	"created_time":    true,
	"last_login_time": true,
	"last_login_ip":   true,
	"quota":           true,
	"used_quota":      true,
	"request_count":   true,
}

// SearchUserParams 用户列表查询参数：在通用分页 / 关键字之上追加登录方式与组织筛选。
type SearchUserParams struct {
	GenericParams
	// LoginMethod 取 LoginMethodFilterNoOidc 时只返回未绑任何启用提供方身份的用户；其余值不筛选。
	LoginMethod string `form:"login_method"`
	// OrgId 大于 0 时只返回该组织的成员与该组织的影子账户。
	OrgId int `form:"org_id"`
	// Status 大于 0 时只返回该状态的用户（页头统计 chip：已启用 / 已禁用）。
	Status int `form:"status"`
	// MinRole 大于 0 时只返回角色不低于该值的用户（页头统计 chip：管理员 = 管理员 + 超级管理员）。
	MinRole int `form:"min_role"`
}

// UserListItem 用户列表行：用户本身加所属组织与影子账户标记（计算字段，不落库）。
// 成员属多个组织时取按 org_id 筛选的组织，否则取最早加入的组织；影子账户取其所记账的组织。
type UserListItem struct {
	User
	OrgId    int    `json:"org_id"`
	OrgName  string `json:"org_name"`
	IsShadow bool   `json:"is_shadow"`
}

func GetUsersList(params *SearchUserParams) (*DataResult[UserListItem], error) {
	var users []*User
	var db *gorm.DB
	if params.OrgId > 0 {
		db = DB.Omit("password").Where(
			"((type IS NULL OR type = ?) AND id IN (?)) OR id IN (?)",
			config.UserTypeNormal,
			DB.Model(&OrganizationMember{}).Select("user_id").Where("organization_id = ?", params.OrgId),
			DB.Model(&Organization{}).Select("shadow_user_id").Where("id = ?", params.OrgId),
		)
	} else {
		db = ExcludeShadowUsers(DB.Omit("password"))
	}
	if params.Status > 0 {
		db = db.Where("status = ?", params.Status)
	}
	if params.MinRole > 0 {
		db = db.Where("role >= ?", params.MinRole)
	}
	if params.LoginMethod == LoginMethodFilterNoOidc {
		db = db.Where("NOT EXISTS (?)", enabledOidcIdentityExists())
	}
	if params.Keyword != "" {
		groupCol := "`group`"
		if common.UsingPostgreSQL {
			groupCol = `"group"`
		}
		db = db.Where("id = ? or username LIKE ? or email LIKE ? or display_name LIKE ? or "+groupCol+" LIKE ? or linuxdo_username LIKE ?", utils.String2Int(params.Keyword), params.Keyword+"%", params.Keyword+"%", params.Keyword+"%", params.Keyword+"%", params.Keyword+"%")
	}

	result, err := PaginateAndOrder[User](db, &params.PaginationParams, &users, allowedUserOrderFields)
	if err != nil {
		return nil, err
	}
	items, err := attachUserOrgs(users, params.OrgId)
	if err != nil {
		return nil, err
	}
	return &DataResult[UserListItem]{
		Data:       &items,
		Page:       result.Page,
		Size:       result.Size,
		TotalCount: result.TotalCount,
	}, nil
}

// attachUserOrgs 以两次批量查询为本页用户补组织字段，避免逐行查库。
func attachUserOrgs(users []*User, preferOrgId int) ([]*UserListItem, error) {
	items := make([]*UserListItem, 0, len(users))
	if len(users) == 0 {
		return items, nil
	}
	userIds := make([]int, 0, len(users))
	for _, user := range users {
		userIds = append(userIds, user.Id)
	}

	var shadowOrgs []Organization
	if err := DB.Select("id", "name", "shadow_user_id").Where("shadow_user_id IN ?", userIds).Find(&shadowOrgs).Error; err != nil {
		return nil, err
	}
	shadowOrgByUser := make(map[int]Organization, len(shadowOrgs))
	for _, org := range shadowOrgs {
		shadowOrgByUser[org.ShadowUserId] = org
	}

	var memberships []struct {
		UserId         int
		OrganizationId int
		OrgName        string
	}
	err := DB.Table("organization_members").
		Select("organization_members.user_id, organization_members.organization_id, organizations.name AS org_name").
		Joins("JOIN organizations ON organizations.id = organization_members.organization_id AND organizations.deleted_at IS NULL").
		Where("organization_members.user_id IN ?", userIds).
		Order("organization_members.id ASC").
		Scan(&memberships).Error
	if err != nil {
		return nil, err
	}
	type orgRef struct {
		id   int
		name string
	}
	orgByUser := make(map[int]orgRef, len(memberships))
	for _, m := range memberships {
		if _, ok := orgByUser[m.UserId]; !ok || m.OrganizationId == preferOrgId {
			orgByUser[m.UserId] = orgRef{id: m.OrganizationId, name: m.OrgName}
		}
	}

	for _, user := range users {
		item := &UserListItem{User: *user}
		if org, ok := shadowOrgByUser[user.Id]; ok && user.Type == config.UserTypeOrgShadow {
			item.IsShadow = true
			item.OrgId = org.Id
			item.OrgName = org.Name
		} else if org, ok := orgByUser[user.Id]; ok {
			item.OrgId = org.id
			item.OrgName = org.name
		} else if user.Type == config.UserTypeOrgShadow {
			item.IsShadow = true
		}
		items = append(items, item)
	}
	return items, nil
}

// UserStats 用户列表页头统计：计数口径与列表一致（排除影子账户与已删除用户）。
type UserStats struct {
	Total    int64 `json:"total"`
	Enabled  int64 `json:"enabled"`
	Disabled int64 `json:"disabled"`
	Root     int64 `json:"root"`
	Admin    int64 `json:"admin"`
	Common   int64 `json:"common"`
}

func GetUserStats() (*UserStats, error) {
	var row UserStats
	err := ExcludeShadowUsers(DB.Model(&User{})).Select(
		"COUNT(*) AS total, "+
			"COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS enabled, "+
			"COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS disabled, "+
			"COALESCE(SUM(CASE WHEN role >= ? THEN 1 ELSE 0 END), 0) AS root, "+
			"COALESCE(SUM(CASE WHEN role >= ? AND role < ? THEN 1 ELSE 0 END), 0) AS admin, "+
			"COALESCE(SUM(CASE WHEN role < ? THEN 1 ELSE 0 END), 0) AS common",
		config.UserStatusEnabled, config.UserStatusDisabled,
		config.RoleRootUser, config.RoleAdminUser, config.RoleRootUser, config.RoleAdminUser,
	).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func GetUserById(id int, selectAll bool) (*User, error) {
	if id == 0 {
		return nil, errors.New("id is empty")
	}
	user := User{Id: id}
	var err error = nil
	if selectAll {
		err = DB.First(&user, "id = ?", id).Error
	} else {
		err = DB.Omit("password").First(&user, "id = ?", id).Error
	}
	return &user, err
}

// HasPasswordById 该账号在本站是否存了密码。GetUserById(id,false) Omit 了 password 列，
// 个人信息接口需要单独问一次；这里只回布尔，哈希不出 model 层。
func HasPasswordById(id int) (bool, error) {
	if id == 0 {
		return false, errors.New("id is empty")
	}
	var passwords []string
	err := DB.Model(&User{}).Where("id = ?", id).Limit(1).Pluck("password", &passwords).Error
	if err != nil {
		return false, err
	}
	return len(passwords) > 0 && passwords[0] != "", nil
}

func GetUserByTelegramId(telegramId int64) (*User, error) {
	if telegramId == 0 {
		return nil, errors.New("telegramId is empty")
	}

	var user User
	err := DB.First(&user, "telegram_id = ?", telegramId).Error

	return &user, err
}

func GetUserIdByAffCode(affCode string) (int, error) {
	if affCode == "" {
		return 0, errors.New("affCode is empty")
	}
	var user User
	err := ExcludeShadowUsers(DB.Select("id")).First(&user, "aff_code = ?", affCode).Error
	return user.Id, err
}

func DeleteUserById(id int) (err error) {
	if id == 0 {
		return errors.New("id is empty")
	}
	user := User{Id: id}
	return user.Delete()
}

func (user *User) Insert(inviterId int) error {
	if strings.TrimSpace(user.Username) == "" {
		return errors.New("username must not be empty")
	}
	if RecordExists(&User{}, "username", user.Username, nil) {
		return errors.New("username already exists")
	}

	// 如果提供了邮箱，先归一化再进行严格验证与查重
	user.Email = NullableEmail(common.NormalizeEmail(string(user.Email)))
	if user.Email != "" {
		if err := common.ValidateEmailStrict(string(user.Email)); err != nil {
			return errors.New("invalid email format")
		}
		if RecordExists(&User{}, "email", string(user.Email), nil) {
			return errors.New("this email is already in use")
		}
	}
	var err error
	if user.Password != "" {
		user.Password, err = common.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	}
	user.Quota = config.QuotaForNewUser
	user.AccessToken = utils.GetUUID()
	user.AffCode = utils.GetRandomString(4)
	user.CreatedTime = utils.GetTimestamp()
	result := DB.Create(user)
	if result.Error != nil {
		return result.Error
	}
	if config.QuotaForNewUser > 0 {
		RecordLog(user.Id, LogTypeSystem, fmt.Sprintf("Sign-up bonus of %s for new user", common.LogQuota(config.QuotaForNewUser)))
	}
	if inviterId != 0 {
		if config.QuotaForInvitee > 0 {
			_ = IncreaseUserQuota(user.Id, config.QuotaForInvitee)
			RecordLog(user.Id, LogTypeSystem, fmt.Sprintf("Invite code bonus of %s", common.LogQuota(config.QuotaForInvitee)))
		}
		// 注册时的邀请奖励保持原有逻辑，充值时的返利使用新的配置
		if config.QuotaForInviter > 0 {
			_ = IncreaseUserQuota(inviterId, config.QuotaForInviter)
			RecordLog(inviterId, LogTypeSystem, fmt.Sprintf("Referral bonus of %s for inviting a user", common.LogQuota(config.QuotaForInviter)))
		}
	}
	return nil
}

// InsertWithTx 在指定事务中创建用户（自助注册路径,发放新用户注册奖励）
func (user *User) InsertWithTx(tx *gorm.DB, inviterId int) error {
	return user.insertWithTx(tx, inviterId, true)
}

// insertWithTx 在指定事务中创建用户。grantSignupBonus 控制是否发放新用户注册奖励
// (QuotaForNewUser 与赠送日志);组织代建等受控路径应传 false,避免刷免费额度(SEC-11)
func (user *User) insertWithTx(tx *gorm.DB, inviterId int, grantSignupBonus bool) error {
	if strings.TrimSpace(user.Username) == "" {
		return errors.New("username must not be empty")
	}
	if RecordExistsWithTx(tx, &User{}, "username", user.Username, nil) {
		return errors.New("username already exists")
	}

	// 如果提供了邮箱，先归一化再进行严格验证与查重（与 username 查重同等位置，
	// 事务内先行给出友好错误，避免用户看到裸的数据库唯一约束冲突）
	user.Email = NullableEmail(common.NormalizeEmail(string(user.Email)))
	if user.Email != "" {
		if err := common.ValidateEmailStrict(string(user.Email)); err != nil {
			return errors.New("invalid email format")
		}
		if RecordExistsWithTx(tx, &User{}, "email", string(user.Email), nil) {
			return errors.New("this email is already in use")
		}
	}
	var err error
	if user.Password != "" {
		user.Password, err = common.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	}
	if grantSignupBonus {
		user.Quota = config.QuotaForNewUser
	}
	user.AccessToken = utils.GetUUID()
	user.AffCode = utils.GetRandomString(4)
	user.CreatedTime = utils.GetTimestamp()
	result := tx.Create(user)
	if result.Error != nil {
		return result.Error
	}
	if grantSignupBonus && config.QuotaForNewUser > 0 {
		RecordLogWithTx(tx, user.Id, LogTypeSystem, fmt.Sprintf("Sign-up bonus of %s for new user", common.LogQuota(config.QuotaForNewUser)))
	}
	if inviterId != 0 {
		if config.QuotaForInvitee > 0 {
			_ = IncreaseUserQuotaWithTx(tx, user.Id, config.QuotaForInvitee)
			RecordLogWithTx(tx, user.Id, LogTypeSystem, fmt.Sprintf("Invite code bonus of %s", common.LogQuota(config.QuotaForInvitee)))
		}
		// 注册时的邀请奖励保持原有逻辑，充值时的返利使用新的配置
		if config.QuotaForInviter > 0 {
			_ = IncreaseUserQuotaWithTx(tx, inviterId, config.QuotaForInviter)
			RecordLogWithTx(tx, inviterId, LogTypeSystem, fmt.Sprintf("Referral bonus of %s for inviting a user", common.LogQuota(config.QuotaForInviter)))
		}
	}
	return nil
}

func (user *User) Update(updatePassword bool) error {
	var err error
	omitFields := []string{"quota", "used_quota", "request_count", "aff_count", "aff_quota", "aff_history"}

	if updatePassword {
		user.Password, err = common.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	} else {
		omitFields = append(omitFields, "password")
	}

	user.Email = NullableEmail(common.NormalizeEmail(string(user.Email)))
	if user.Email != "" && RecordExists(&User{}, "email", string(user.Email), user.Id) {
		return errors.New("this email is already in use")
	}

	err = DB.Model(user).Omit(omitFields...).Updates(user).Error

	if err == nil && user.Role == config.RoleRootUser {
		config.RootUserEmail = string(user.Email)
	}

	// 删除缓存（支持两套缓存机制）
	if err == nil {
		ClearUserGroupAndTokensCache(user.Id)
	}

	return err
}

func UpdateUser(id int, fields map[string]interface{}) error {
	err := DB.Model(&User{}).Where("id = ?", id).Updates(fields).Error
	if err != nil {
		return err
	}

	// 如果更新了分组、角色或状态字段，清理鉴权相关缓存
	_, hasGroup := fields["group"]
	_, hasRole := fields["role"]
	_, hasStatus := fields["status"]
	if hasGroup || hasRole || hasStatus {
		ClearUserGroupAndTokensCache(id)
	}

	return nil
}

// ClearUserGroupAndTokensCache 清理用户鉴权相关缓存（分组 / 角色状态 / 启用状态 / 所有Token）
func ClearUserGroupAndTokensCache(userId int) {
	if !config.RedisEnabled {
		return
	}

	// 清理用户分组缓存
	userGroupKey := fmt.Sprintf(UserGroupCacheKey, userId)
	if err := redis.RedisDel(userGroupKey); err != nil {
		logger.SysError(fmt.Sprintf("failed to clear user group Redis cache userId=%d: %v", userId, err))
	}
	if err := cache.DeleteCache(userGroupKey); err != nil {
		logger.SysError(fmt.Sprintf("failed to clear user group cache userId=%d: %v", userId, err))
	}

	// 清理用户角色/状态缓存，保证降级、封禁等变更下一次鉴权即生效
	userRoleStatusKey := fmt.Sprintf(UserRoleStatusCacheKey, userId)
	if err := redis.RedisDel(userRoleStatusKey); err != nil {
		logger.SysError(fmt.Sprintf("failed to clear user role/status Redis cache userId=%d: %v", userId, err))
	}
	if err := cache.DeleteCache(userRoleStatusKey); err != nil {
		logger.SysError(fmt.Sprintf("failed to clear user role/status cache userId=%d: %v", userId, err))
	}

	// 清理用户启用状态缓存，保证封禁/删除后 relay(令牌)路径下一次请求即被拦截
	userEnabledKey := fmt.Sprintf(UserEnabledCacheKey, userId)
	if err := redis.RedisDel(userEnabledKey); err != nil {
		logger.SysError(fmt.Sprintf("failed to clear user enabled Redis cache userId=%d: %v", userId, err))
	}
	if err := cache.DeleteCache(userEnabledKey); err != nil {
		logger.SysError(fmt.Sprintf("failed to clear user enabled cache userId=%d: %v", userId, err))
	}

	// 获取用户所有Token的Key
	var tokenKeys []string
	err := DB.Model(&Token{}).Where("user_id = ?", userId).Pluck("key", &tokenKeys).Error
	if err != nil {
		logger.SysError(fmt.Sprintf("failed to get user API key list userId=%d: %v", userId, err))
		return
	}

	// 清理每个Token的缓存
	for _, tokenKey := range tokenKeys {
		if tokenKey != "" {
			cacheKey := fmt.Sprintf(UserTokensKey, tokenKey)
			if err := redis.RedisDel(cacheKey); err != nil {
				logger.SysError(fmt.Sprintf("failed to clear API key Redis cache key=%s: %v", tokenKey, err))
			}
			if err := cache.DeleteCache(cacheKey); err != nil {
				logger.SysError(fmt.Sprintf("failed to clear API key cache key=%s: %v", tokenKey, err))
			}
		}
	}
}

// Delete 软删用户：改名、清空邮箱与手机号以释放唯一索引，并连带删除其全部 OIDC 身份行。
func (user *User) Delete() error {
	if user.Id == 0 {
		return errors.New("id is empty")
	}

	// 不改变当前数据库索引，通过更改用户名来删除用户
	user.Username = user.Username + "_del_" + utils.GetRandomString(6)
	err := user.Update(false)
	if err != nil {
		return err
	}

	// 邮箱与手机号的唯一索引同样覆盖软删除行，一并清空以释放这两个标识供重新注册 / 关联。
	// 邮箱没了，「已验证」也就无从谈起，一并复位。
	// 需显式写列：Update 走 Updates(struct)，空串与 false 都是零值会被 GORM 跳过。
	if err := DB.Model(&User{}).Where("id = ?", user.Id).
		Updates(map[string]interface{}{"email": nil, "phone_number": nil, "email_verified": false}).Error; err != nil {
		return err
	}
	user.Email = ""
	user.EmailVerified = false
	user.PhoneNumber = ""

	// 身份行没有外键，须显式清理：(provider_id, subject) 唯一索引不区分用户是否软删，
	// 残留身份行会让原 IdP 主体无法再走 oidcRegister 建号。与软删同事务，保证同成同败。
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", user.Id).Delete(&UserOidcIdentity{}).Error; err != nil {
			return err
		}
		// 登录会话同样没有外键，随用户一起结束
		if err := DeleteUserSessionsExcept(tx, user.Id, ""); err != nil {
			return err
		}
		return tx.Delete(user).Error
	})
	if err != nil {
		return err
	}

	// 软删除提交后再清一次缓存：防止并发请求在 Update 清理与 DB.Delete 之间用旧数据回填，
	// 留下永不过期的脏缓存导致已删除用户仍能通过鉴权。
	ClearUserGroupAndTokensCache(user.Id)
	return nil
}

// dummyPasswordHash 是一个预计算的 bcrypt hash（cost 10，明文不对外使用），
// 仅用于用户不存在时执行一次等价开销的密码比较，拉平登录响应时序，防止用户名枚举。
const dummyPasswordHash = "$2a$10$G3XFuXDg4ofRs/KXk6L1L.TwAiaHBi20fbJH/U8uoJ6EiyEfYL13C"

// ErrLoginFailed 密码登录的统一失败文案。用户不存在、密码错误、账号被封禁共用同一文本，
// 避免通过响应差异区分账号是否存在。
var ErrLoginFailed = errors.New("incorrect username or password, or the user is banned")

// ResolveLoginUser 按标识符类型显式解析登录用户：标识符含 "@" 只按归一化邮箱查，
// 否则只按用户名查；两者均排除组织影子账户。不做跨列兜底，
// 因此「用户名恰好等于他人邮箱」的账号不会被另一种写法误命中。
// 未命中返回 ErrUserNotFound。
func ResolveLoginUser(identifier string) (*User, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, ErrUserNotFound
	}

	query := ExcludeShadowUsers(DB.Where("username = ?", identifier))
	if common.IsEmailLoginIdentifier(identifier) {
		email := common.NormalizeEmail(identifier)
		if email == "" {
			return nil, ErrUserNotFound
		}
		query = ExcludeShadowUsers(DB.Where("email = ?", email))
	}

	user := &User{}
	if err := query.First(user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

// ValidateLoginPassword 校验已解析用户的密码与账号状态。
// user 为 nil 表示标识符未解析到用户，此时仍执行一次等价开销的 bcrypt 比较拉平响应时序。
// 任何失败路径都返回 ErrLoginFailed，文案不泄露失败原因。
func ValidateLoginPassword(user *User, password string) error {
	hash := dummyPasswordHash
	if user != nil {
		hash = user.Password
	}
	okay := common.ValidatePasswordAndHash(password, hash)
	if user == nil || !okay || user.Status != config.UserStatusEnabled {
		return ErrLoginFailed
	}
	return nil
}

// ValidateAndFill check password & user status
func (user *User) ValidateAndFill() error {
	password := user.Password
	if strings.TrimSpace(user.Username) == "" || strings.TrimSpace(password) == "" {
		return errors.New("username or password is empty")
	}
	resolved, err := ResolveLoginUser(user.Username)
	if err != nil {
		return ValidateLoginPassword(nil, password)
	}
	if err := ValidateLoginPassword(resolved, password); err != nil {
		return err
	}
	*user = *resolved
	return nil
}

func (user *User) FillUserById() error {
	if user.Id == 0 {
		return errors.New("id is empty")
	}

	result := DB.Where(User{Id: user.Id}).First(user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return result.Error
	}
	return nil
}

func (user *User) FillUserByEmail() error {
	user.Email = NullableEmail(common.NormalizeEmail(string(user.Email)))
	if user.Email == "" {
		return errors.New("email is empty")
	}

	result := ExcludeShadowUsers(DB.Where("email = ?", string(user.Email))).First(user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return result.Error
	}
	return nil
}

// FillUserByPhoneNumber 按已验证手机号查账号（与 FillUserByEmail 同样排除影子账户）。
// phone_number 上有唯一索引，命中至多一行。
func (user *User) FillUserByPhoneNumber() error {
	user.PhoneNumber = NullablePhone(strings.TrimSpace(string(user.PhoneNumber)))
	if user.PhoneNumber == "" {
		return errors.New("phone number is empty")
	}

	result := ExcludeShadowUsers(DB.Where("phone_number = ?", string(user.PhoneNumber))).First(user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return result.Error
	}
	return nil
}

func (user *User) FillUserByGitHubId() error {
	if user.GitHubId == "" {
		return errors.New("GitHub id is empty")
	}
	DB.Where(User{GitHubId: user.GitHubId}).First(user)
	return nil
}

func (user *User) FillUserByGitHubIdNew() error {
	if user.GitHubIdNew == 0 {
		return errors.New("GitHub id new is empty")
	}
	DB.Where(User{GitHubIdNew: user.GitHubIdNew}).First(user)
	return nil
}

func (user *User) FillUserByWeChatId() error {
	if user.WeChatId == "" {
		return errors.New("WeChat id is empty")
	}
	DB.Where(User{WeChatId: user.WeChatId}).First(user)
	return nil
}

func (user *User) FillUserByLarkId() error {
	if user.LarkId == "" {
		return errors.New("lark id is empty")
	}
	DB.Where(User{LarkId: user.LarkId}).First(user)
	return nil
}

// ErrUserNotFound 用户查找未命中的哨兵错误。FillUserByXxx 用它替代裸 errors.New，
// 使 OAuth/OIDC 处理器可用 errors.Is 区分「未找到(继续/注册)」与「真实 DB 错误(中止)」。
// 消息文本不变,故对展示该错误的调用方零影响。
var ErrUserNotFound = errors.New("user not found")

func (user *User) FillUserByOidcId() error {
	if user.OidcId == "" {
		return errors.New("OIDC ID is empty")
	}
	result := DB.Where(User{OidcId: user.OidcId}).First(user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return result.Error
	}
	return nil
}

func (user *User) FillUserByLinuxDOId() error {
	if user.LinuxDoId == 0 {
		return errors.New("LINUX DO ID is empty")
	}
	result := DB.Where(User{LinuxDoId: user.LinuxDoId}).First(user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return result.Error
	}
	return nil
}

func (user *User) FillUserByUsername() error {
	if user.Username == "" {
		return errors.New("username is empty")
	}
	result := ExcludeShadowUsers(DB.Where(User{Username: user.Username})).First(user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return result.Error
	}
	return nil
}

func FindUserByField(field string, value any) (*User, error) {
	user := &User{}
	err := DB.Where(fmt.Sprintf("%s = ?", field), value).First(user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return user, err
}

func IsFieldAlreadyTaken(field string, value any) bool {
	var count int64
	DB.Model(&User{}).Where(fmt.Sprintf("%s = ?", field), value).Limit(1).Count(&count)
	return count > 0
}

func IsUsernameAlreadyTaken(username string) bool {
	return IsFieldAlreadyTaken("username", username)
}

func IsEmailAlreadyTaken(email string) bool {
	email = common.NormalizeEmail(email)
	if email == "" {
		return false
	}
	return IsFieldAlreadyTaken("email", email)
}

// IsPhoneAlreadyTaken 手机号是否已归属某个账号。空手机号永不算被占用（多个账号可无手机号）。
func IsPhoneAlreadyTaken(phone string) bool {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return false
	}
	return IsFieldAlreadyTaken("phone_number", phone)
}

func IsWeChatIdAlreadyTaken(wechatId string) bool {
	return IsFieldAlreadyTaken("wechat_id", wechatId)
}

func IsGitHubIdAlreadyTaken(githubId string) bool {
	return IsFieldAlreadyTaken("github_id", githubId)
}

func IsGitHubIdNewAlreadyTaken(githubIdNew int) bool {
	return IsFieldAlreadyTaken("github_id_new", githubIdNew)
}

func IsLarkIdAlreadyTaken(larkId string) bool {
	return IsFieldAlreadyTaken("lark_id", larkId)
}

func IsTelegramIdAlreadyTaken(telegramId int64) bool {
	return IsFieldAlreadyTaken("telegram_id", telegramId)
}

func IsLinuxDOIdAlreadyTaken(linuxdoId int) bool {
	return IsFieldAlreadyTaken("linuxdo_id", linuxdoId)
}

func ResetUserPasswordByEmail(email string, password string) error {
	email = common.NormalizeEmail(email)
	if email == "" || password == "" {
		return errors.New("email or password is empty")
	}
	// 邮箱无唯一约束，若同一邮箱对应多个账号则拒绝，避免一次重置改掉多个账号的密码
	var count int64
	if err := ExcludeShadowUsers(DB.Model(&User{}).Where("email = ?", email)).Count(&count).Error; err != nil {
		return err
	}
	if count > 1 {
		logger.SysError(fmt.Sprintf("password reset failed: email %s is linked to %d accounts", email, count))
		return errors.New("this email is linked to multiple accounts, so the password cannot be reset by yourself. Please contact the admin.")
	}
	if count == 0 {
		return errors.New("this email is not registered")
	}
	hashedPassword, err := common.Password2Hash(password)
	if err != nil {
		return err
	}
	err = ExcludeShadowUsers(DB.Model(&User{}).Where("email = ?", email)).Update("password", hashedPassword).Error
	if err != nil {
		return err
	}
	// 重置密码等同于「我怀疑密码泄露」，结束该账号的全部会话
	var ids []int
	if err := ExcludeShadowUsers(DB.Model(&User{}).Where("email = ?", email)).Pluck("id", &ids).Error; err == nil {
		for _, id := range ids {
			if err := DeleteUserSessionsExcept(nil, id, ""); err != nil {
				logger.SysError("failed to clear sessions after password reset: " + err.Error())
			}
		}
	}
	return nil
}

func IsAdmin(userId int) bool {
	if userId == 0 {
		return false
	}
	var user User
	err := DB.Where("id = ?", userId).Select("role").Find(&user).Error
	if err != nil {
		logger.SysError("no such user " + err.Error())
		return false
	}
	return user.Role >= config.RoleAdminUser
}

// GetUserRoleAndStatus 实时读取用户当前的角色与状态（仅查询 role、status 两列）。
// 用户不存在（含已删除）时返回 error，供鉴权侧拒绝。
func GetUserRoleAndStatus(userId int) (role int, status int, err error) {
	if userId == 0 {
		return 0, 0, errors.New("id is empty")
	}
	var user User
	err = DB.Where("id = ?", userId).Select("role", "status").First(&user).Error
	if err != nil {
		return 0, 0, err
	}
	return user.Role, user.Status, nil
}

func IsReliable(userId int) bool {
	if userId == 0 {
		return false
	}
	var user User
	err := DB.Where("id = ?", userId).Select("role").Find(&user).Error
	if err != nil {
		logger.SysError("no such user " + err.Error())
		return false
	}
	return user.Role >= config.RoleReliableUser
}

func IsUserEnabled(userId int) (bool, error) {
	if userId == 0 {
		return false, errors.New("user id is empty")
	}
	var user User
	err := DB.Where("id = ?", userId).Select("status").Find(&user).Error
	if err != nil {
		return false, err
	}
	return user.Status == config.UserStatusEnabled, nil
}

func ValidateAccessToken(token string) (user *User) {
	if token == "" {
		return nil
	}
	token = strings.Replace(token, "Bearer ", "", 1)
	user = &User{}
	if ExcludeShadowUsers(DB.Where("access_token = ?", token)).First(user).RowsAffected == 1 {
		return user
	}
	return nil
}

func GetUserFields(id int, fields []string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	err := GetFieldsByID(&User{}, fields, id, &result)
	return result, err
}

func GetUserQuota(id int) (quota int, err error) {
	err = DB.Model(&User{}).Where("id = ?", id).Select("quota").Find(&quota).Error
	return quota, err
}

func GetUserUsedQuota(id int) (quota int, err error) {
	err = DB.Model(&User{}).Where("id = ?", id).Select("used_quota").Find(&quota).Error
	return quota, err
}

func GetUserGroup(id int) (group string, err error) {
	groupCol := "`group`"
	if common.UsingPostgreSQL {
		groupCol = `"group"`
	}

	err = DB.Model(&User{}).Where("id = ?", id).Select(groupCol).Find(&group).Error
	return group, err
}

func IncreaseUserQuota(id int, quota int) (err error) {
	if quota < 0 {
		return errors.New("quota must not be negative")
	}
	if config.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeUserQuota, id, quota)
		return nil
	}
	return increaseUserQuota(id, quota)
}

func increaseUserQuota(id int, quota int) (err error) {
	err = DB.Model(&User{}).Where("id = ?", id).Update("quota", gorm.Expr("quota + ?", quota)).Error
	if err != nil {
		return err
	}
	// 刷新缓存，避免额度变动后读取到旧值
	if config.RedisEnabled {
		// 直接删除缓存键，下次读取时会重新写入最新值
		redis.RedisDel(fmt.Sprintf(UserQuotaCacheKey, id))
	}
	return nil
}

// IncreaseUserQuotaWithTx 在指定事务中增加用户配额
func IncreaseUserQuotaWithTx(tx *gorm.DB, id int, quota int) (err error) {
	if quota < 0 {
		return errors.New("quota must not be negative")
	}
	// 注意：在事务中不支持批量更新，直接执行数据库操作
	err = tx.Model(&User{}).Where("id = ?", id).Update("quota", gorm.Expr("quota + ?", quota)).Error
	if err != nil {
		return err
	}
	// 注意：在事务中不刷新缓存，等事务提交后再刷新
	return nil
}

func DecreaseUserQuota(id int, quota int) (err error) {
	if quota < 0 {
		return errors.New("quota must not be negative")
	}
	if config.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeUserQuota, id, -quota)
		return nil
	}
	return decreaseUserQuota(id, quota)
}

func decreaseUserQuota(id int, quota int) (err error) {
	err = DB.Model(&User{}).Where("id = ?", id).Update("quota", gorm.Expr("quota - ?", quota)).Error
	if err != nil {
		return err
	}
	// 刷新缓存，保持数据一致性
	if config.RedisEnabled {
		redis.RedisDel(fmt.Sprintf(UserQuotaCacheKey, id))
	}
	return nil
}

// ErrUserQuotaNotEnough 预扣阶段原子守卫命中余额不足(WHERE quota >= ? 影响 0 行)。
var ErrUserQuotaNotEnough = errors.New("insufficient user quota")

// PreDecreaseUserQuota 预扣阶段的原子条件扣减:UPDATE users SET quota = quota - ?
// WHERE id = ? AND quota >= ?，以 RowsAffected 判定结果。与 DecreaseUserQuota(结算/批量
// 路径,允许透支以保证记账完整)不同,本函数面向高并发预扣:必须同步执行且带余额守卫,
// RowsAffected==0 即返回 ErrUserQuotaNotEnough 拒绝请求,杜绝 check-then-act 竞态下的负额度透支。
// 条件 UPDATE + RowsAffected 跨库(MySQL/Postgres/SQLite)通用,不依赖行锁语法差异。
func PreDecreaseUserQuota(id int, quota int) (err error) {
	if quota < 0 {
		return errors.New("quota must not be negative")
	}
	if quota == 0 {
		return nil
	}
	result := DB.Model(&User{}).Where("id = ? AND quota >= ?", id, quota).Update("quota", gorm.Expr("quota - ?", quota))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserQuotaNotEnough
	}
	// 刷新缓存，保持数据一致性
	if config.RedisEnabled {
		redis.RedisDel(fmt.Sprintf(UserQuotaCacheKey, id))
	}
	return nil
}

func GetRootUserEmail() (email string) {
	email, _ = getRootUserEmail()
	return email
}

// getRootUserEmail email 列可为 NULL（无邮箱），用 sql.NullString 承接避免 Scan 到 NULL 报错
func getRootUserEmail() (string, error) {
	var values []sql.NullString
	err := DB.Model(&User{}).Where("role = ?", config.RoleRootUser).Limit(1).Pluck("email", &values).Error
	if err != nil || len(values) == 0 || !values[0].Valid {
		return "", err
	}
	return common.NormalizeEmail(values[0].String), nil
}

func UpdateUserUsedQuotaAndRequestCount(id int, quota int) {
	if config.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeUsedQuota, id, quota)
		addNewRecord(BatchUpdateTypeRequestCount, id, 1)
		return
	}
	updateUserUsedQuotaAndRequestCount(id, quota, 1)
}

func updateUserUsedQuotaAndRequestCount(id int, quota int, count int) {
	err := DB.Model(&User{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"used_quota":    gorm.Expr("used_quota + ?", quota),
			"request_count": gorm.Expr("request_count + ?", count),
		},
	).Error
	if err != nil {
		logger.SysError("failed to update user used quota and request count: " + err.Error())
	}
}

func updateUserUsedQuota(id int, quota int) {
	err := DB.Model(&User{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"used_quota": gorm.Expr("used_quota + ?", quota),
		},
	).Error
	if err != nil {
		logger.SysError("failed to update user used quota: " + err.Error())
	}
}

func updateUserRequestCount(id int, count int) {
	err := DB.Model(&User{}).Where("id = ?", id).Update("request_count", gorm.Expr("request_count + ?", count)).Error
	if err != nil {
		logger.SysError("failed to update user request count: " + err.Error())
	}
}

func GetUsernameById(id int) (username string) {
	DB.Model(&User{}).Where("id = ?", id).Select("username").Find(&username)
	return username
}

// GetUserInviteCount 获取用户的邀请人数
func GetUserInviteCount(userId int) (int64, error) {
	var count int64
	err := DB.Model(&User{}).Where("inviter_id = ?", userId).Count(&count).Error
	return count, err
}

type StatisticsUser struct {
	TotalQuota        int64 `json:"total_quota"`
	TotalUsedQuota    int64 `json:"total_used_quota"`
	TotalUser         int64 `json:"total_user"`
	TotalInviterUser  int64 `json:"total_inviter_user"`
	TotalRequestCount int64 `json:"total_request_count"`
	TotalTokens       int64 `json:"total_tokens"`
}

func GetStatisticsUser() (statisticsUser *StatisticsUser, err error) {
	// 排除组织影子账户:影子账户的 quota/used_quota 属于组织积分池,不计入用户统计口径
	err = ExcludeShadowUsers(DB.Model(&User{})).Select("sum(quota) as total_quota, sum(used_quota) as total_used_quota, count(*) as total_user, count(CASE WHEN inviter_id != 0 THEN 1 END) as total_inviter_user").Scan(&statisticsUser).Error
	if err != nil {
		return statisticsUser, err
	}

	// 全站请求总次数与总 token 数，从 statistics 表聚合（比扫 logs 高效）
	var logStat struct {
		TotalRequestCount int64 `gorm:"column:total_request_count"`
		TotalTokens       int64 `gorm:"column:total_tokens"`
	}
	if e := DB.Model(&Statistics{}).Select("COALESCE(sum(request_count),0) as total_request_count, COALESCE(sum(prompt_tokens + completion_tokens),0) as total_tokens").Scan(&logStat).Error; e == nil {
		statisticsUser.TotalRequestCount = logStat.TotalRequestCount
		statisticsUser.TotalTokens = logStat.TotalTokens
	}

	return statisticsUser, err
}

type UserStatisticsByPeriod struct {
	Date             string `json:"date"`
	UserCount        int64  `json:"user_count"`
	InviterUserCount int64  `json:"inviter_user_count"`
}

func GetUserStatisticsByPeriod(startTimestamp, endTimestamp int64) (statistics []*UserStatisticsByPeriod, err error) {
	groupSelect := getTimestampGroupsSelect("created_time", "day", "date")

	// 排除组织影子账户:影子账户不是真实注册用户,不计入注册统计
	err = DB.Raw(`
		SELECT `+groupSelect+`,
		count(*) as user_count,
		count(CASE WHEN inviter_id != 0 THEN 1 END) as inviter_user_count
		FROM users
		WHERE created_time BETWEEN ? AND ?
		AND (type IS NULL OR type = ?)
		GROUP BY date
		ORDER BY date
	`, startTimestamp, endTimestamp, config.UserTypeNormal).Scan(&statistics).Error

	return statistics, err
}

func ChangeUserQuota(id int, quota int, isRecharge bool) (err error) {
	updateMap := map[string]interface{}{
		"quota": gorm.Expr("quota + ?", quota),
	}

	if isRecharge {
		updateMap["recharge_count"] = gorm.Expr("recharge_count + 1")
	}

	err = DB.Model(&User{}).Where("id = ?", id).Updates(updateMap).Error

	if err != nil {
		return err
	}

	if config.RedisEnabled {
		redis.RedisDel(fmt.Sprintf(UserQuotaCacheKey, id))
	}

	return nil
}

// ProcessInviterReward 处理邀请人的充值返利
func ProcessInviterReward(userId int, rechargeQuota int, ip string) error {
	// 获取用户信息，查看是否有邀请人
	user := &User{}
	err := DB.Where("id = ?", userId).First(user).Error
	if err != nil {
		return err
	}

	// 如果没有邀请人，直接返回
	if user.InviterId == 0 {
		return nil
	}

	// 如果奖励值为0或奖励类型为空，直接返回
	if config.InviterRewardValue == 0 || config.InviterRewardType == "" {
		return nil
	}

	var rewardQuota int
	var logMessage string

	if config.InviterRewardType == "percentage" {
		// 百分比奖励
		rewardQuota = common.QuotaFromFloat(float64(rechargeQuota) * float64(config.InviterRewardValue) / 100.0)
		logMessage = fmt.Sprintf("Referral top-up reward of %s \n\n (top-up amount: %s, reward rate: %d%%)",
			common.LogQuota(rewardQuota),
			common.LogQuota(rechargeQuota),
			config.InviterRewardValue)
	} else {
		// 固定奖励
		rewardQuota = config.InviterRewardValue
		logMessage = fmt.Sprintf("Referral top-up reward of %s (fixed reward)",
			common.LogQuota(rewardQuota))
	}

	if rewardQuota <= 0 {
		return nil
	}

	// 给邀请人增加额度
	err = IncreaseUserQuota(user.InviterId, rewardQuota)
	if err != nil {
		return err
	}

	// 更新邀请人的aff_quota
	err = DB.Model(&User{}).Where("id = ?", user.InviterId).Update("aff_quota", gorm.Expr("aff_quota + ?", rewardQuota)).Error
	if err != nil {
		logger.SysError("failed to update inviter aff_quota: " + err.Error())
	}

	// 记录日志
	RecordLog(user.InviterId, LogTypeSystem, logMessage)

	return nil
}

// WebAuthn 相关方法，实现 webauthn.User 接口
func (user *User) WebAuthnID() []byte {
	return []byte(fmt.Sprintf("%d", user.Id))
}

func (user *User) WebAuthnName() string {
	return user.Username
}

func (user *User) WebAuthnDisplayName() string {
	if user.DisplayName != "" {
		return user.DisplayName
	}
	return user.Username
}

func (user *User) WebAuthnIcon() string {
	return user.AvatarUrl
}

func (user *User) WebAuthnCredentials() []webauthn.Credential {
	credentials := GetUserWebAuthnCredentials(user.Id)
	return credentials
}

// WebAuthnCredential 表示WebAuthn凭据
type WebAuthnCredential struct {
	Id              int    `json:"id" gorm:"primaryKey"`
	UserId          int    `json:"user_id" gorm:"index"`
	CredentialId    []byte `json:"credential_id" gorm:"unique;size:255"`
	PublicKey       []byte `json:"public_key"`
	AttestationType string `json:"attestation_type"`
	Alias           string `json:"alias" gorm:"type:varchar(255);default:''"`
	// Persist essential authenticator state and flags used during login validation
	BackupEligible bool                   `json:"backup_eligible" gorm:"column:backup_eligible;default:false"`
	BackupState    bool                   `json:"backup_state" gorm:"column:backup_state;default:false"`
	Authenticator  webauthn.Authenticator `json:"authenticator" gorm:"embedded"`
	CreatedTime    int64                  `json:"created_time"`
}

func (WebAuthnCredential) TableName() string {
	return "webauthn_credentials"
}

// 获取用户的WebAuthn凭据
func GetUserWebAuthnCredentials(userId int) []webauthn.Credential {
	var credentials []WebAuthnCredential
	DB.Where("user_id = ?", userId).Find(&credentials)

	var webauthnCredentials []webauthn.Credential
	for _, cred := range credentials {
		webauthnCredentials = append(webauthnCredentials, webauthn.Credential{
			ID:              cred.CredentialId,
			PublicKey:       cred.PublicKey,
			AttestationType: cred.AttestationType,
			Authenticator:   cred.Authenticator,
			Flags: webauthn.CredentialFlags{
				UserPresent:    false, // will be updated by library during validation
				UserVerified:   false, // will be updated by library during validation
				BackupEligible: cred.BackupEligible,
				BackupState:    cred.BackupState,
			},
		})
	}
	return webauthnCredentials
}

// 保存WebAuthn凭据
func SaveWebAuthnCredential(userId int, credential *webauthn.Credential, alias string) error {
	if alias == "" {
		alias = time.Now().Format("20060102150405")
	}
	webauthnCred := WebAuthnCredential{
		UserId:          userId,
		CredentialId:    credential.ID,
		PublicKey:       credential.PublicKey,
		AttestationType: credential.AttestationType,
		Alias:           alias,
		BackupEligible:  credential.Flags.BackupEligible,
		BackupState:     credential.Flags.BackupState,
		Authenticator:   credential.Authenticator,
		CreatedTime:     time.Now().Unix(),
	}
	return DB.Create(&webauthnCred).Error
}

// GetUserByWebAuthnCredentialId 通过WebAuthn凭据ID获取用户
func GetUserByWebAuthnCredentialId(credentialId []byte) (*User, error) {
	var cred WebAuthnCredential
	err := DB.Where("credential_id = ?", credentialId).First(&cred).Error
	if err != nil {
		return nil, err
	}
	return GetUserById(cred.UserId, false)
}
