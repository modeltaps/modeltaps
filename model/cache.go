package model

import (
	"context"
	"fmt"
	"github.com/modeltaps/modeltaps/common/cache"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/redis"
	"strconv"
	"time"
)

var (
	TokenCacheSeconds           = 0
	UserGroupCacheKey           = "user_group:%d"
	// UserTokensKey caches an API key by its hash (SEC-47); never by the key itself.
	UserTokensKey               = "token_hash:%s"
	UsernameCacheKey            = "user_name:%d"
	UserQuotaCacheKey           = "user_quota:%d"
	UserEnabledCacheKey         = "user_enabled:%d"
	UserRoleStatusCacheKey      = "user_role_status:%d"
	UserRealtimeQuotaKey        = "user_realtime_quota:%d"
	UserRealtimeQuotaExpiration = 24 * time.Hour
	// 成员预算实时 tally(SEC-12):键按 org_members.id 维度,跨并发请求共享,流式期间逐增量累计,
	// 结算时由 relay 回收;TTL 兜底防结算路径缺失时键泄漏。
	OrgMemberBudgetRealtimeKey = "org_member_budget_realtime:%d"
	LogIOOwnerDefaultCacheKey   = "log_io_owner_default:%d"

	// OldUserTokenHashesKey is the set of hashes of the 48-character legacy keys, a cheap filter
	// before the database is asked (see HandleOldTokenMaxId).
	OldUserTokenHashesKey = "old_user_token_hashes"

	// legacyPlaintextTokenCachePattern and legacyOldUserTokensCacheKey are where releases before
	// SEC-47 cached API keys under the plaintext key; PurgeLegacyTokenCache removes them.
	legacyPlaintextTokenCachePattern = "token:*"
	legacyOldUserTokensCacheKey      = "old_user_tokens_cache"
	legacyTokenCachePurgedKey        = "token_cache_plaintext_purged"
)

// 令牌归属主体(用户 / 组织)上的 LogIO 开关原值(三态),供 ResolveTokenLogIO 判定;未配置按关闭处理。
const (
	logIODefaultStateUnset int8 = 0 // 未配置:不留存
	logIODefaultStateOn    int8 = 1 // 强制开
	logIODefaultStateOff   int8 = 2 // 强制关
)

// ownerLogIODefault 缓存令牌归属主体(个人用户 / 组织影子账户)上的 LogIO 开关。
// 随 setting 写入失效(触发 InvalidateOwnerLogIODefaultCache)。
type ownerLogIODefault struct {
	State int8 `json:"state"` // 开关三态原值
}

func logIODefaultState(b *bool) int8 {
	if b == nil {
		return logIODefaultStateUnset
	}
	if *b {
		return logIODefaultStateOn
	}
	return logIODefaultStateOff
}

func getOwnerLogIODefaultFromDB(userId int) (*ownerLogIODefault, error) {
	var user User
	if err := DB.Select("type", "setting").Where("id = ?", userId).First(&user).Error; err != nil {
		return nil, err
	}
	if user.Type == config.UserTypeOrgShadow {
		var org Organization
		if err := DB.Select("setting").Where("shadow_user_id = ?", userId).First(&org).Error; err != nil {
			return nil, err
		}
		orgSetting := org.Setting.Data()
		return &ownerLogIODefault{State: logIODefaultState(orgSetting.LogIODefault)}, nil
	}
	userSetting := user.Setting.Data()
	return &ownerLogIODefault{State: logIODefaultState(userSetting.LogIODefault)}, nil
}

// CacheGetOwnerLogIODefault 读取令牌归属主体的 LogIO 默认(走缓存,避免解析链每请求打库)。
func CacheGetOwnerLogIODefault(userId int) (*ownerLogIODefault, error) {
	if !config.RedisEnabled {
		return getOwnerLogIODefaultFromDB(userId)
	}
	return cache.GetOrSetCache(
		fmt.Sprintf(LogIOOwnerDefaultCacheKey, userId),
		time.Duration(TokenCacheSeconds)*time.Second,
		func() (*ownerLogIODefault, error) {
			return getOwnerLogIODefaultFromDB(userId)
		},
		cache.CacheTimeout)
}

// InvalidateOwnerLogIODefaultCache 在用户/组织默认变更后失效解析缓存(userId 为个人ID或组织影子账户ID)。
func InvalidateOwnerLogIODefaultCache(userId int) {
	if !config.RedisEnabled || userId == 0 {
		return
	}
	_ = cache.DeleteCache(fmt.Sprintf(LogIOOwnerDefaultCacheKey, userId))
}

// UserRoleStatus 缓存中保存的用户实时角色与状态。
type UserRoleStatus struct {
	Role   int `json:"role"`
	Status int `json:"status"`
}

// CacheGetTokenByKeyHash reads an API key by the hash of its secret, through the cache.
func CacheGetTokenByKeyHash(keyHash string) (*Token, error) {
	if !config.RedisEnabled {
		return GetTokenByKeyHash(keyHash)
	}

	token, err := cache.GetOrSetCache(
		fmt.Sprintf(UserTokensKey, keyHash),
		time.Duration(TokenCacheSeconds)*time.Second,
		func() (*Token, error) {
			return GetTokenByKeyHash(keyHash)
		},
		cache.CacheTimeout)

	return token, err
}

func CacheGetUserGroup(id int) (group string, err error) {
	if !config.RedisEnabled {
		return GetUserGroup(id)
	}

	group, err = cache.GetOrSetCache(
		fmt.Sprintf(UserGroupCacheKey, id),
		time.Duration(TokenCacheSeconds)*time.Second,
		func() (string, error) {
			groupId, err := GetUserGroup(id)
			if err != nil {
				return "", err
			}
			return groupId, nil
		},
		cache.CacheTimeout)

	return group, err
}

func CacheGetUserQuota(id int) (quota int, err error) {
	if !config.RedisEnabled {
		return GetUserQuota(id)
	}
	quotaString, err := redis.RedisGet(fmt.Sprintf(UserQuotaCacheKey, id))
	if err != nil {
		quota, err = GetUserQuota(id)
		if err != nil {
			return 0, err
		}
		err = redis.RedisSet(fmt.Sprintf(UserQuotaCacheKey, id), fmt.Sprintf("%d", quota), time.Duration(TokenCacheSeconds)*time.Second)
		if err != nil {
			logger.SysError("Redis set user quota error: " + err.Error())
		}
		return quota, err
	}
	quota, err = strconv.Atoi(quotaString)
	return quota, err
}

func CacheUpdateUserQuota(id int) error {
	if !config.RedisEnabled {
		return nil
	}
	quota, err := GetUserQuota(id)
	if err != nil {
		return err
	}
	err = redis.RedisSet(fmt.Sprintf(UserQuotaCacheKey, id), fmt.Sprintf("%d", quota), time.Duration(TokenCacheSeconds)*time.Second)
	return err
}

func CacheDecreaseUserQuota(id int, quota int) error {
	if !config.RedisEnabled {
		return nil
	}
	err := redis.RedisDecrease(fmt.Sprintf(UserQuotaCacheKey, id), int64(quota))
	return err
}

func CacheIsUserEnabled(userId int) (bool, error) {
	if !config.RedisEnabled {
		return IsUserEnabled(userId)
	}

	enabled, err := cache.GetOrSetCache(
		fmt.Sprintf(UserEnabledCacheKey, userId),
		time.Duration(TokenCacheSeconds)*time.Second,
		func() (bool, error) {
			enabled, err := IsUserEnabled(userId)
			if err != nil {
				return false, err
			}
			return enabled, nil
		},
		cache.CacheTimeout)

	return enabled, err
}

// CacheGetUserRoleStatus 读取用户实时的角色与状态，优先命中缓存（TokenCacheSeconds=0 永不过期，
// 靠 ClearUserGroupAndTokensCache 主动失效）。用户角色/状态变更后下一次鉴权即生效，
// 同时避免每个请求都回库。未启用 Redis 时退化为直接查库。
func CacheGetUserRoleStatus(userId int) (role int, status int, err error) {
	if !config.RedisEnabled {
		return GetUserRoleAndStatus(userId)
	}

	rs, err := cache.GetOrSetCache(
		fmt.Sprintf(UserRoleStatusCacheKey, userId),
		time.Duration(TokenCacheSeconds)*time.Second,
		func() (UserRoleStatus, error) {
			r, s, e := GetUserRoleAndStatus(userId)
			if e != nil {
				return UserRoleStatus{}, e
			}
			return UserRoleStatus{Role: r, Status: s}, nil
		},
		cache.CacheTimeout)
	if err != nil {
		return 0, 0, err
	}

	return rs.Role, rs.Status, nil
}

func CacheGetUsername(id int) (username string, err error) {
	if !config.RedisEnabled {
		return GetUsernameById(id), nil
	}

	username, err = cache.GetOrSetCache(
		fmt.Sprintf(UsernameCacheKey, id),
		time.Duration(TokenCacheSeconds)*time.Second,
		func() (string, error) {
			username := GetUsernameById(id)
			if username == "" {
				return "", fmt.Errorf("user %d not found", id)
			}

			return username, nil
		},
		cache.CacheTimeout)

	return username, err
}

func CacheDecreaseUserRealtimeQuota(id int, quota int) (int64, error) {
	if !config.RedisEnabled {
		return 0, nil
	}
	return CacheUpdateUserRealtimeQuota(id, -quota)
}

func CacheIncreaseUserRealtimeQuota(id int, quota int) (int64, error) {
	if !config.RedisEnabled {
		return 0, nil
	}
	return CacheUpdateUserRealtimeQuota(id, quota)
}

var (
	updateQuotaScript = redis.NewScript(`
		local key = KEYS[1]
		local increment = tonumber(ARGV[1])
		local expiration = tonumber(ARGV[2])

		local exists = redis.call("EXISTS", key)
		if exists == 0 then
			if increment < 0 then
				return 0
			end
			redis.call("SET", key, "0", "EX", expiration)
		end

		local newValue = redis.call("INCRBY", key, increment)
		redis.call("EXPIRE", key, expiration)

		return newValue
	`)
)

func CacheUpdateUserRealtimeQuota(id int, quota int) (int64, error) {
	if !config.RedisEnabled {
		return 0, nil
	}
	key := fmt.Sprintf(UserRealtimeQuotaKey, id)

	newValue, err := updateQuotaScript.Run(context.Background(), redis.GetRedisClient(), []string{key}, quota, int(UserRealtimeQuotaExpiration.Seconds())).Int64()
	if err != nil {
		return 0, fmt.Errorf("failed to update user quota: %w", err)
	}

	return newValue, nil
}

// CacheIncreaseOrgMemberRealtimeBudget 累计成员预算实时 tally(SEC-12,镜像 CacheIncreaseUserRealtimeQuota),
// 返回累计后的 tally 值。Redis 关闭时返回 0(退化为 SEC-9 入场预留硬闸)。
func CacheIncreaseOrgMemberRealtimeBudget(memberId int, quota int) (int64, error) {
	if !config.RedisEnabled {
		return 0, nil
	}
	return cacheUpdateOrgMemberRealtimeBudget(memberId, quota)
}

// CacheDecreaseOrgMemberRealtimeBudget 回收成员预算实时 tally(结算清理,镜像 CacheDecreaseUserRealtimeQuota)。
func CacheDecreaseOrgMemberRealtimeBudget(memberId int, quota int) (int64, error) {
	if !config.RedisEnabled {
		return 0, nil
	}
	return cacheUpdateOrgMemberRealtimeBudget(memberId, -quota)
}

func cacheUpdateOrgMemberRealtimeBudget(memberId int, quota int) (int64, error) {
	key := fmt.Sprintf(OrgMemberBudgetRealtimeKey, memberId)

	newValue, err := updateQuotaScript.Run(context.Background(), redis.GetRedisClient(), []string{key}, quota, int(UserRealtimeQuotaExpiration.Seconds())).Int64()
	if err != nil {
		return 0, fmt.Errorf("failed to update member budget live counter: %w", err)
	}

	return newValue, nil
}

// PurgeLegacyTokenCache deletes, once per Redis, the cache entries releases before SEC-47 kept
// under the plaintext API key ("token:<key>" and the set of legacy keys). Entries written with
// no expiry would otherwise keep every key readable in Redis after the upgrade.
func PurgeLegacyTokenCache() {
	if !config.RedisEnabled {
		return
	}
	if done, _ := redis.RedisExists(legacyTokenCachePurgedKey); done {
		return
	}
	ctx := context.Background()
	client := redis.GetRedisClient()
	deleted := 0
	batch := make([]string, 0, 500)
	flush := func() bool {
		if len(batch) == 0 {
			return true
		}
		if err := client.Del(ctx, batch...).Err(); err != nil {
			logger.SysError("failed to delete legacy API key cache entries: " + err.Error())
			return false
		}
		deleted += len(batch)
		batch = batch[:0]
		return true
	}
	iter := client.Scan(ctx, 0, legacyPlaintextTokenCachePattern, 1000).Iterator()
	for iter.Next(ctx) {
		batch = append(batch, iter.Val())
		if len(batch) == cap(batch) && !flush() {
			return
		}
	}
	if err := iter.Err(); err != nil {
		logger.SysError("failed to scan legacy API key cache entries: " + err.Error())
		return
	}
	batch = append(batch, legacyOldUserTokensCacheKey)
	if !flush() {
		return
	}
	if err := redis.RedisSet(legacyTokenCachePurgedKey, "1", 0); err != nil {
		logger.SysError("failed to record the legacy API key cache purge: " + err.Error())
	}
	logger.SysLog(fmt.Sprintf("removed %d legacy API key cache entries", deleted))
}

// HandleOldTokenMaxId fills the Redis set of hashes of the 48-character legacy keys (ids up to
// OldTokenMaxId), so a 48-character key that is not one of them is refused without a query.
func HandleOldTokenMaxId() {
	if config.OldTokenMaxId == 0 || !config.RedisEnabled {
		return
	}

	exists, _ := redis.RedisExists(OldUserTokenHashesKey)
	if exists {
		return
	}
	const batchSize = 1000
	var offset int

	for {
		var tokenKeys []interface{}
		result := DB.Model(&Token{}).
			Where("id <= ? AND key_hash IS NOT NULL AND key_hash <> ''", config.OldTokenMaxId).
			Order("id").
			Limit(batchSize).
			Offset(offset).
			Pluck("key_hash", &tokenKeys)

		if result.Error != nil {
			logger.SysError("failed to query legacy tokens: " + result.Error.Error())
			return
		}

		if len(tokenKeys) == 0 {
			if offset == 0 {
				logger.SysLog("no legacy tokens found")
			}
			break
		}

		if err := redis.RedisSAdd(OldUserTokenHashesKey, tokenKeys...); err != nil {
			logger.SysError("failed to add legacy token to Redis: " + err.Error())
		}

		logger.SysLog(fmt.Sprintf("processed %d legacy tokens", offset+len(tokenKeys)))
		offset += batchSize

		time.Sleep(100 * time.Millisecond)
	}
}
