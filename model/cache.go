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
	UserTokensKey               = "token:%s"
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

	OldUserTokensCacheKey = "old_user_tokens_cache"
)

// LogIO 分层继承中,令牌归属主体的默认配置原值(三态),供 ResolveTokenLogIO 在令牌为「继承」时回退。
// 缓存原值而非已套用站点默认的结果,使站点默认变更无需失效本缓存即可生效。
const (
	logIODefaultStateUnset int8 = 0 // 未配置:跟随站点默认
	logIODefaultStateOn    int8 = 1 // 强制开
	logIODefaultStateOff   int8 = 2 // 强制关
)

// ownerLogIODefault 缓存令牌归属主体(个人用户 / 组织影子账户)的 LogIO 上级默认。
// 上级默认随 setting 写入失效(触发 InvalidateOwnerLogIODefaultCache)。
type ownerLogIODefault struct {
	IsOrg bool `json:"is_org"` // 归属是否为组织影子账户(决定 nil 时回退到组织站点默认)
	State int8 `json:"state"`  // 上级默认三态原值
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
		return &ownerLogIODefault{
			IsOrg: true,
			State: logIODefaultState(orgSetting.LogIODefault),
		}, nil
	}
	userSetting := user.Setting.Data()
	return &ownerLogIODefault{
		IsOrg: false,
		State: logIODefaultState(userSetting.LogIODefault),
	}, nil
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

func CacheGetTokenByKey(key string) (*Token, error) {
	if !config.RedisEnabled {
		return GetTokenByKey(key)
	}

	token, err := cache.GetOrSetCache(
		fmt.Sprintf(UserTokensKey, key),
		time.Duration(TokenCacheSeconds)*time.Second,
		func() (*Token, error) {
			return GetTokenByKey(key)
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

func HandleOldTokenMaxId() {
	if config.OldTokenMaxId == 0 || !config.RedisEnabled {
		return
	}

	// 检测OldUserTokensCacheKey是否存在
	exists, _ := redis.RedisExists(OldUserTokensCacheKey)
	if exists {
		return
	}
	const batchSize = 1000
	var offset int

	for {
		var tokenKeys []interface{}
		result := DB.Model(&Token{}).
			Where("id <= ?", config.OldTokenMaxId).
			Limit(batchSize).
			Offset(offset).
			Pluck("key", &tokenKeys)

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

		if err := redis.RedisSAdd(OldUserTokensCacheKey, tokenKeys...); err != nil {
			logger.SysError("failed to add legacy token to Redis: " + err.Error())
		}

		logger.SysLog(fmt.Sprintf("processed %d legacy tokens", offset+len(tokenKeys)))
		offset += batchSize

		time.Sleep(100 * time.Millisecond)
	}
}
