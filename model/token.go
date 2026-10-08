package model

import (
	"errors"
	"fmt"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/database"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/redis"
	"github.com/modeltaps/modeltaps/common/stmp"
	"github.com/modeltaps/modeltaps/common/utils"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	ErrTokenNotFound             = errors.New("API key not found")
	ErrTokenExpired              = errors.New("API key has expired")
	ErrTokenQuotaExhausted       = errors.New("API key quota exhausted")
	ErrTokenPeriodQuotaExhausted = errors.New("API key period quota exhausted")
	ErrTokenStatusUnavailable    = errors.New("API key is unavailable")
	ErrTokenInvalid              = errors.New("invalid API key")
	ErrTokenQuotaGet             = errors.New("failed to get API key quota")
)

type Token struct {
	Id             int            `json:"id"`
	UserId         int            `json:"user_id"`
	Key            string         `json:"key" gorm:"type:varchar(59);uniqueIndex"`
	Status         int            `json:"status" gorm:"default:1"`
	Name           string         `json:"name" gorm:"index" `
	CreatedTime    int64          `json:"created_time" gorm:"bigint"`
	AccessedTime   int64          `json:"accessed_time" gorm:"bigint"`
	ExpiredTime    int64          `json:"expired_time" gorm:"bigint;default:-1"` // -1 means never expired
	RemainQuota    int            `json:"remain_quota" gorm:"default:0"`
	UnlimitedQuota bool           `json:"unlimited_quota" gorm:"default:false"`
	UsedQuota      int            `json:"used_quota" gorm:"default:0"` // used quota
	// 周期模式计数(SEC-5):从 setting JSON 移出为专用列,使 accrue 走单条原子自增,杜绝并发丢更新
	PeriodUsed  int   `json:"period_used" gorm:"type:int;not null;default:0"`
	PeriodStart int64 `json:"period_start" gorm:"bigint;not null;default:0"`
	Group       string `json:"group" gorm:"default:''"`
	BackupGroup    string         `json:"backup_group" gorm:"default:''"`
	CreatedBy      int            `json:"created_by" gorm:"index;default:0"` // 实际创建者用户ID(组织令牌场景:user_id 为影子账户,created_by 为创建成员;0=历史数据,等同 user_id)
	LogIO          *bool          `json:"log_io"`                            // 令牌级「完整请求/响应留存」三态:nil=继承上级 / true=强制开 / false=强制关(见 LogIO 分层继承)
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`

	Setting database.JSONType[TokenSetting] `json:"setting" form:"setting" gorm:"type:json"`
}

var allowedTokenOrderFields = map[string]bool{
	"id":           true,
	"name":         true,
	"status":       true,
	"expired_time": true,
	"created_time": true,
	"remain_quota": true,
	"used_quota":   true,
}

// 添加 AfterCreate 钩子方法
func (token *Token) AfterCreate(tx *gorm.DB) (err error) {
	tokenKey, err := common.GenerateToken(token.Id, token.UserId)
	if err != nil {
		return err
	}

	// 同步到内存中的 token 实例，使 caller 可以在 Insert 返回后直接读取 Key
	token.Key = tokenKey

	// 更新 key 字段
	return tx.Model(token).Update("key", tokenKey).Error
}

type TokenSetting struct {
	Heartbeat  HeartbeatSetting `json:"heartbeat,omitempty"`
	Limits     LimitsConfig     `json:"limits,omitempty"`
	BillingTag *string          `json:"billing_tag,omitempty"` // 费用标签，用于按分组统计费用，仅可信内部员工和管理员可见

	// 周期性配额重置（OpenRouter 风格）。QuotaReset 为 nil 时不生效，零行为变化。
	// 周期计数 PeriodUsed/PeriodStart 已移出为 Token 专用列(SEC-5)，不再存于此 JSON。
	QuotaReset *QuotaResetSetting `json:"quota_reset,omitempty"`
}

type HeartbeatSetting struct {
	Enabled        bool `json:"enabled"`
	TimeoutSeconds int  `json:"timeout_seconds"`
}

type LimitsConfig struct {
	LimitModelSetting LimitModelSetting `json:"limit_model_setting,omitempty"`
	LimitsIPSetting   LimitsIPSetting   `json:"limits_ip_setting,omitempty"`
}

type LimitModelSetting struct {
	Enabled bool     `json:"enabled"`
	Models  []string `json:"models"`
}

type LimitsIPSetting struct {
	Enabled   bool     `json:"enabled"`
	Whitelist []string `json:"whitelist"`
}

func GetUserTokensList(userId int, params *GenericParams) (*DataResult[Token], error) {
	var tokens []*Token
	db := DB.Where("user_id = ?", userId)

	if params.Keyword != "" {
		db = db.Where("name LIKE ?", params.Keyword+"%")
	}

	return PaginateAndOrder(db, &params.PaginationParams, &tokens, allowedTokenOrderFields)
}

// todayStartTimestamp 返回当前时区下今日零点的 Unix 时间戳。
// 时区选择复用 controller.GetUserDashboard / UpdateStatistics 的 TZ 模式
// (time.Local + TZ 环境变量,非每用户),保证「当日」边界与既有统计一致。
func todayStartTimestamp() int64 {
	location := time.Local
	if tzEnv := os.Getenv("TZ"); tzEnv != "" {
		if loc, err := time.LoadLocation(tzEnv); err == nil {
			location = loc
		}
	}
	now := time.Now().In(location)
	toDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	return toDay.Unix()
}

// GetUserTodayUsageByToken 聚合用户当日(TZ 零点起)按 token_name 的消费配额,返回 token_name -> quota 映射。
// 数据源为 logs,仅有 token_name 无 token_id:同名令牌会合并、改名后历史不计入当日、空名令牌归入空名桶。
func GetUserTodayUsageByToken(userId int) (map[string]int64, error) {
	return sumUserUsageByTokenSince(userId, todayStartTimestamp())
}

// sumUserUsageByTokenSince 聚合用户自 startTimestamp(含)起的消费配额,按 token_name 分组。
// 抽出零点边界以便单测注入固定时间戳验证跨零点翻转。
func sumUserUsageByTokenSince(userId int, startTimestamp int64) (map[string]int64, error) {
	var rows []struct {
		TokenName string `gorm:"column:token_name"`
		Quota     int64  `gorm:"column:quota"`
	}
	err := DB.Table("logs").
		Select("token_name, "+assembleSumSelectStr("quota")+" as quota").
		Where("user_id = ? AND type = ? AND created_at >= ?", userId, LogTypeConsume, startTimestamp).
		Group("token_name").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, r := range rows {
		result[r.TokenName] = r.Quota
	}
	return result, nil
}

func GetUserTokenGroupSymbols(userId int) ([]string, error) {
	var tokens []Token
	err := DB.Where("user_id = ?", userId).Select("group", "backup_group").Find(&tokens).Error
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	for _, t := range tokens {
		if t.Group != "" {
			seen[t.Group] = struct{}{}
		}
		if t.BackupGroup != "" {
			seen[t.BackupGroup] = struct{}{}
		}
	}

	symbols := make([]string, 0, len(seen))
	for s := range seen {
		symbols = append(symbols, s)
	}
	return symbols, nil
}

// AdminSearchTokensParams 管理员搜索令牌的参数
type AdminSearchTokensParams struct {
	GenericParams
	UserId  int    `form:"user_id"`
	TokenId int    `form:"token_id"`
	Key     string `form:"key"`
}

// TokenWithOwner 包含令牌信息和所属用户信息
type TokenWithOwner struct {
	Token
	OwnerName string `json:"owner_name"` // 用户名称（优先显示 display_name，其次 username）
}

// GetTokensListByAdmin 管理员查询令牌列表（可按用户ID或令牌ID查询）
func GetTokensListByAdmin(params *AdminSearchTokensParams) (*DataResult[TokenWithOwner], error) {
	var tokens []*Token
	db := DB.Model(&Token{})

	if params.UserId > 0 {
		db = db.Where("user_id = ?", params.UserId)
	}

	if params.TokenId > 0 {
		db = db.Where("id = ?", params.TokenId)
	}

	if params.Key != "" {
		keyCol := "`key`"
		if common.UsingPostgreSQL {
			keyCol = `"key"`
		}
		key := strings.TrimPrefix(params.Key, "sk-")
		db = db.Where(keyCol+" LIKE ?", key+"%")
	}

	if params.Keyword != "" {
		db = db.Where("name LIKE ?", params.Keyword+"%")
	}

	result, err := PaginateAndOrder(db, &params.PaginationParams, &tokens, allowedTokenOrderFields)
	if err != nil {
		return nil, err
	}

	userIds := make([]int, 0)
	userIdMap := make(map[int]bool)
	for _, token := range *result.Data {
		if !userIdMap[token.UserId] {
			userIds = append(userIds, token.UserId)
			userIdMap[token.UserId] = true
		}
	}

	userNameMap := make(map[int]string)
	if len(userIds) > 0 {
		var users []User
		DB.Select("id, username, display_name").Where("id IN ?", userIds).Find(&users)
		for _, user := range users {
			name := user.DisplayName
			if name == "" {
				name = user.Username
			}
			userNameMap[user.Id] = name
		}
	}

	tokensWithOwner := make([]*TokenWithOwner, len(*result.Data))
	for i, token := range *result.Data {
		tokensWithOwner[i] = &TokenWithOwner{
			Token:     *token,
			OwnerName: userNameMap[token.UserId],
		}
	}

	return &DataResult[TokenWithOwner]{
		Data:       &tokensWithOwner,
		TotalCount: result.TotalCount,
	}, nil
}

func GetTokenModel(key string) (token *Token, err error) {
	if key == "" {
		return nil, ErrTokenInvalid
	}

	var userId int
	var tokenId int
	validUser := false

	switch len(key) {
	case 48:
		validUser = true
		if config.RedisEnabled {
			exists, _ := redis.RedisSIsMember(OldUserTokensCacheKey, key)
			if !exists {
				return nil, ErrTokenInvalid
			}
		}
	case 59:
		tokenId, userId, err = common.ValidateToken(key)
		if err != nil || userId == 0 || tokenId == 0 {
			return nil, ErrTokenInvalid
		}
		if userEnabled, err := CacheIsUserEnabled(userId); err != nil || !userEnabled {
			return nil, ErrTokenInvalid
		}
	default:
		return nil, ErrTokenInvalid
	}

	token, err = CacheGetTokenByKey(key)
	if err != nil {
		maskedKey := key[:3] + "*********" + key[len(key)-3:]
		logger.SysError(fmt.Sprintf("DB Not Found: userId=%d, tokenId=%d, key=%s, err=%s", userId, tokenId, maskedKey, err.Error()))
		return nil, ErrTokenInvalid
	}

	if validUser {
		if userEnabled, err := CacheIsUserEnabled(token.UserId); err != nil || !userEnabled {
			return nil, ErrTokenInvalid
		}
	}

	return token, nil
}

func ValidateUserToken(key string) (token *Token, err error) {
	token, err = GetTokenModel(key)
	if err != nil {
		return nil, err
	}

	if token.Status != config.TokenStatusEnabled {
		switch token.Status {
		case config.TokenStatusExhausted:
			return nil, ErrTokenQuotaExhausted
		case config.TokenStatusExpired:
			return nil, ErrTokenExpired
		default:
			return nil, ErrTokenStatusUnavailable
		}
	}

	if token.ExpiredTime != -1 && token.ExpiredTime < utils.GetTimestamp() {
		return nil, ErrTokenExpired
	}

	setting := token.Setting.Data()
	// period 模式（quota_reset + limit>0）：额度按周期重置，remain_quota 耗尽不再使 Key 失效，
	// 仅受周期上限约束，故跳过耗尽判定（含 !Redis 分支置 exhausted 的写入）。
	if !token.UnlimitedQuota && !tokenInPeriodMode(&setting) && token.RemainQuota <= 0 {
		if !config.RedisEnabled {
			// in this case, we can make sure the token is exhausted
			token.Status = config.TokenStatusExhausted
			err := token.SelectUpdate()
			if err != nil {
				logger.SysError("failed to update token status" + err.Error())
			}
		}
		return nil, ErrTokenQuotaExhausted
	}

	if tokenPeriodQuotaExceeded(&setting, token.PeriodUsed, token.PeriodStart, time.Now()) {
		return nil, ErrTokenPeriodQuotaExhausted
	}

	return token, nil
}

func GetTokenByIds(id int, userId int) (*Token, error) {
	if id == 0 || userId == 0 {
		return nil, errors.New("id or userId is empty")
	}
	token := Token{Id: id, UserId: userId}
	var err error = nil
	err = DB.First(&token, "id = ? and user_id = ?", id, userId).Error
	return &token, err
}

func GetTokenById(id int) (*Token, error) {
	if id == 0 {
		return nil, errors.New("id is empty")
	}
	var token Token
	err := DB.First(&token, "id = ?", id).Error
	return &token, err
}

func GetTokenByName(name string, userId int) (*Token, error) {
	if name == "" {
		return nil, errors.New("name is empty")
	}
	token := Token{Name: name}
	var err error = nil
	err = DB.First(&token, "user_id = ? and name = ?", userId, name).Error
	return &token, err
}

func GetTokenByKey(key string) (*Token, error) {
	keyCol := "`key`"
	if common.UsingPostgreSQL {
		keyCol = `"key"`
	}

	var token Token

	err := DB.Where(keyCol+" = ?", key).First(&token).Error
	return &token, err
}

func (token *Token) Insert() error {
	err := DB.Create(token).Error
	return err
}

// Update Make sure your token's fields is completed, because this will update non-zero values
func (token *Token) Update() error {
	err := DB.Model(token).Select("name", "status", "expired_time", "remain_quota", "unlimited_quota", "group", "backup_group", "setting", "log_io").Updates(token).Error
	// 防止Redis缓存不生效，直接删除
	if err == nil && config.RedisEnabled {
		redis.RedisDel(fmt.Sprintf(UserTokensKey, token.Key))
	}

	return err
}

// UpdateByAdmin 管理员更新token，支持更新user_id字段
func (token *Token) UpdateByAdmin() error {
	err := DB.Model(token).Select("user_id", "name", "status", "expired_time", "remain_quota", "unlimited_quota", "group", "backup_group", "setting", "log_io").Updates(token).Error
	if err == nil && config.RedisEnabled {
		redis.RedisDel(fmt.Sprintf(UserTokensKey, token.Key))
	}

	return err
}

// ResolveTokenLogIO 解析令牌在两层继承下的 LogIO 生效布尔值(站点开关 && 归属默认),供 middleware 注入。
// 判定链(权威语义):
//   - 站点总闸门 config.LogIOEnabled 关闭 → 恒 false,且不做任何上级查询(零开销)。
//   - 归属默认(按归属):组织令牌取组织默认→站点 OrganizationLogIODefault;个人令牌取用户默认→站点 LogIODefaultUser。
//
// 归属默认走缓存避免每请求打库。令牌级 log_io 与覆盖锁已废弃(两层简化),不再读取。
func ResolveTokenLogIO(token *Token) bool {
	if !config.LogIOEnabled || token == nil {
		return false
	}
	owner, err := CacheGetOwnerLogIODefault(token.UserId)
	if err != nil || owner == nil {
		// 归属解析失败时回退到个人站点默认,保持 ZDR 安全语义
		return config.LogIODefaultUser
	}
	return resolveOwnerDefault(owner)
}

// resolveOwnerDefault 把归属主体的上级默认三态原值套用站点默认,得到生效的上级默认布尔。
func resolveOwnerDefault(owner *ownerLogIODefault) bool {
	switch owner.State {
	case logIODefaultStateOn:
		return true
	case logIODefaultStateOff:
		return false
	}
	if owner.IsOrg {
		return config.OrganizationLogIODefault
	}
	return config.LogIODefaultUser
}

// LogIOWriteEnabled 报告是否应为该令牌写入完整请求/响应明细(T50e 写入闸门,供 T50d 调用)。
// 等价于解析后的生效值:站点总开关 && 令牌分层继承解析为开;关闭任一层级即不写明细。
func LogIOWriteEnabled(token *Token) bool {
	return ResolveTokenLogIO(token)
}

// CanViewTokenLogIO 判定 viewer 是否可查看 token 的完整请求/响应明细(T50e 权限语义,后端权威源,供 T50d/T50f 调用)。
//
// 站点管理员(全局 role>=RoleAdminUser)在任意场景均可见全部。
// 个人场景(orgRole==""):个人(非组织)令牌所有者可见自己的令牌 body。
// 组织场景(orgRole 非空):仅 Owner/Admin 可见组织内令牌 body;组织成员(Member)一律拒绝,
// 不得借「self」查看自建组织令牌 body(D2=b 后端权威拒绝,见 spec)。self 放行仅适用于个人/非组织令牌所有者。
func CanViewTokenLogIO(viewerId, viewerRole int, orgRole string, token *Token) bool {
	if token == nil {
		return false
	}
	if viewerRole >= config.RoleAdminUser {
		return true
	}
	switch orgRole {
	case OrgRoleOwner, OrgRoleAdmin:
		return true
	case OrgRoleMember:
		return false
	default:
		return token.UserId == viewerId
	}
}

func (token *Token) SelectUpdate() error {
	// This can update zero values
	err := DB.Model(token).Select("accessed_time", "status").Updates(token).Error

	// 清除缓存
	if err == nil && config.RedisEnabled {
		redis.RedisDel(fmt.Sprintf(UserTokensKey, token.Key))
	}

	return err
}

func (token *Token) Delete() error {
	err := DB.Delete(token).Error
	return err
}

func DeleteTokenById(id int, userId int) (err error) {
	// Why we need userId here? In case user want to delete other's token.
	if id == 0 || userId == 0 {
		return errors.New("id or userId is empty")
	}
	token := Token{Id: id, UserId: userId}
	err = DB.Where(token).First(&token).Error
	if err != nil {
		return err
	}
	err = token.Delete()

	if err == nil && config.RedisEnabled {
		redis.RedisDel(fmt.Sprintf(UserTokensKey, token.Key))
	}

	return err

}

// DeleteTokenByIdAdmin 管理员删除任意token（不限制所属用户）
func DeleteTokenByIdAdmin(id int) (err error) {
	if id == 0 {
		return errors.New("id is empty")
	}
	token := Token{Id: id}
	err = DB.Where("id = ?", id).First(&token).Error
	if err != nil {
		return err
	}
	err = token.Delete()

	if err == nil && config.RedisEnabled {
		redis.RedisDel(fmt.Sprintf(UserTokensKey, token.Key))
	}

	return err
}

func IncreaseTokenQuota(id int, quota int) (err error) {
	if quota < 0 {
		return errors.New("quota must not be negative")
	}
	if config.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeTokenQuota, id, quota)
		return nil
	}
	return increaseTokenQuota(id, quota)
}

func increaseTokenQuota(id int, quota int) (err error) {
	err = DB.Model(&Token{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"remain_quota":  gorm.Expr("remain_quota + ?", quota),
			"used_quota":    gorm.Expr("used_quota - ?", quota),
			"accessed_time": utils.GetTimestamp(),
		},
	).Error

	// 清除缓存
	if err == nil && config.RedisEnabled {
		var key string
		keyCol := "`key`"
		if common.UsingPostgreSQL {
			keyCol = `"key"`
		}
		if getErr := DB.Model(&Token{}).Where("id = ?", id).Select(keyCol).Scan(&key).Error; getErr == nil && key != "" {
			redis.RedisDel(fmt.Sprintf(UserTokensKey, key))
		}
	}

	return err
}

func DecreaseTokenQuota(id int, quota int) (err error) {
	if quota < 0 {
		return errors.New("quota must not be negative")
	}
	if config.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeTokenQuota, id, -quota)
		return nil
	}
	return decreaseTokenQuota(id, quota)
}

func decreaseTokenQuota(id int, quota int) (err error) {
	err = DB.Model(&Token{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"remain_quota":  gorm.Expr("remain_quota - ?", quota),
			"used_quota":    gorm.Expr("used_quota + ?", quota),
			"accessed_time": utils.GetTimestamp(),
		},
	).Error

	// 清除缓存
	if err == nil && config.RedisEnabled {
		var key string
		keyCol := "`key`"
		if common.UsingPostgreSQL {
			keyCol = `"key"`
		}
		if getErr := DB.Model(&Token{}).Where("id = ?", id).Select(keyCol).Scan(&key).Error; getErr == nil && key != "" {
			redis.RedisDel(fmt.Sprintf(UserTokensKey, key))
		}
	}

	return err
}

// ErrTokenQuotaNotEnough 预扣阶段令牌额度原子守卫命中余额不足(remain_quota >= ? 影响 0 行)。
var ErrTokenQuotaNotEnough = errors.New("insufficient API key quota")

// PreDecreaseTokenQuota 预扣阶段令牌额度原子条件扣减,语义同 PreDecreaseUserQuota:
// UPDATE tokens SET remain_quota = remain_quota - ?, used_quota = used_quota + ?
// WHERE id = ? AND remain_quota >= ?，RowsAffected==0 即返回 ErrTokenQuotaNotEnough。
// 必须同步执行(不走批量),以保证高并发预扣下令牌额度不透支为负。
func PreDecreaseTokenQuota(id int, quota int) (err error) {
	if quota < 0 {
		return errors.New("quota must not be negative")
	}
	if quota == 0 {
		return nil
	}
	result := DB.Model(&Token{}).Where("id = ? AND remain_quota >= ?", id, quota).Updates(
		map[string]interface{}{
			"remain_quota":  gorm.Expr("remain_quota - ?", quota),
			"used_quota":    gorm.Expr("used_quota + ?", quota),
			"accessed_time": utils.GetTimestamp(),
		},
	)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTokenQuotaNotEnough
	}
	// 清除缓存
	if config.RedisEnabled {
		var key string
		keyCol := "`key`"
		if common.UsingPostgreSQL {
			keyCol = `"key"`
		}
		if getErr := DB.Model(&Token{}).Where("id = ?", id).Select(keyCol).Scan(&key).Error; getErr == nil && key != "" {
			redis.RedisDel(fmt.Sprintf(UserTokensKey, key))
		}
	}
	return nil
}

// IncreaseTokenUsedQuota 仅累计 used_quota（quota 可为负，用于回退预扣），不动 remain_quota。
// 供 unlimited token 的消费路径使用：不限额仍需累计用量，否则统计/限额/重置缺乏可靠基数。
// 不清 Redis token 缓存：used_quota 不参与令牌校验，缓存陈旧无碍。
func IncreaseTokenUsedQuota(id int, quota int) (err error) {
	if quota == 0 {
		return nil
	}
	if config.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeTokenUsedQuota, id, quota)
		return nil
	}
	return increaseTokenUsedQuota(id, quota)
}

func increaseTokenUsedQuota(id int, quota int) error {
	return DB.Model(&Token{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"used_quota":    gorm.Expr("used_quota + ?", quota),
			"accessed_time": utils.GetTimestamp(),
		},
	).Error
}

func PreConsumeTokenQuota(tokenId int, quota int) (err error) {
	if quota < 0 {
		return errors.New("quota must not be negative")
	}
	token, err := GetTokenById(tokenId)
	if err != nil {
		return err
	}
	setting := token.Setting.Data()
	periodMode := tokenInPeriodMode(&setting)
	// period 模式：额度按周期重置，跳过 remain_quota 不足校验，仅依赖周期校验。
	if !token.UnlimitedQuota && !periodMode && token.RemainQuota < quota {
		return errors.New("insufficient API key quota")
	}
	if tokenPeriodQuotaExceeded(&setting, token.PeriodUsed, token.PeriodStart, time.Now()) {
		return ErrTokenPeriodQuotaExhausted
	}
	userQuota, err := GetUserQuota(token.UserId)
	if err != nil {
		return err
	}
	if userQuota < quota {
		return errors.New("insufficient user quota")
	}
	quotaTooLow := userQuota >= config.QuotaRemindThreshold && userQuota-quota < config.QuotaRemindThreshold
	noMoreQuota := userQuota-quota <= 0
	if quotaTooLow || noMoreQuota {
		go sendQuotaWarningEmail(token.UserId, userQuota, noMoreQuota)
	}
	// 先原子扣减用户额度(共享额度池,check-then-act 竞态的主战场):RowsAffected==0 直接拒绝,
	// 此时未触碰令牌额度与周期用量,零副作用,无需回滚。
	if err = PreDecreaseUserQuota(token.UserId, quota); err != nil {
		return err
	}
	if !token.UnlimitedQuota && !periodMode {
		err = PreDecreaseTokenQuota(tokenId, quota)
	} else {
		// unlimited 或 period 模式：不扣 remain_quota（period 仅受周期上限约束），但 used_quota 仍要累计
		err = IncreaseTokenUsedQuota(tokenId, quota)
	}
	if err != nil {
		// 令牌额度扣减失败(余额不足或 DB 错误):回滚已扣的用户额度,
		// 避免出现「用户已扣、令牌未扣」的记账不一致。
		if rbErr := increaseUserQuota(token.UserId, quota); rbErr != nil {
			logger.SysError("failed to rollback user quota after token quota deduction failed: " + rbErr.Error())
		}
		return err
	}
	// 预扣部分计入周期用量（结算差额/回退由 relay_util.Quota 负责累计）
	if setting.QuotaReset != nil {
		if perr := AccrueTokenPeriodUsed(tokenId, quota); perr != nil {
			logger.SysError("failed to accrue token period used: " + perr.Error())
		}
	}
	return nil
}

func sendQuotaWarningEmail(userId int, userQuota int, noMoreQuota bool) {
	user := User{Id: userId}

	if err := user.FillUserById(); err != nil {
		logger.SysError("failed to fetch user email: " + err.Error())
		return
	}

	// 组织影子账户无真实邮箱,不发送通知邮件
	if IsShadowUser(&user) {
		return
	}

	if user.Email == "" {
		logger.SysError("user email is empty")
		return
	}

	userName := user.DisplayName
	if userName == "" {
		userName = user.Username
	}

	err := stmp.SendQuotaWarningCodeEmail(userName, string(user.Email), userQuota, noMoreQuota)

	if err != nil {
		logger.SysError("failed to send email" + err.Error())
	}
}

// PostConsumeTokenQuotaWithInfo 消费 token 配额，直接使用传入的 userId 和 unlimitedQuota，避免数据库查询
func PostConsumeTokenQuotaWithInfo(tokenId int, userId int, unlimitedQuota bool, quota int) (err error) {
	if quota == 0 {
		return nil
	}
	if quota > 0 {
		err = DecreaseUserQuota(userId, quota)
	} else {
		err = IncreaseUserQuota(userId, -quota)
	}
	if err != nil {
		return err
	}
	if !unlimitedQuota {
		if quota > 0 {
			err = DecreaseTokenQuota(tokenId, quota)
		} else {
			err = IncreaseTokenQuota(tokenId, -quota)
		}
	} else {
		// unlimited 不扣 remain_quota，但 used_quota 仍要累计（quota 为负时回冲）
		err = IncreaseTokenUsedQuota(tokenId, quota)
	}
	if err != nil {
		return err
	}
	return nil
}
