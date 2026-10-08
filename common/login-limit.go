package common

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/redis"
)

// 账号级登录失败计数与锁定。
// 与 IP 级限流(CriticalRateLimit)互补：这里按「账号」聚合，跨来源 IP 统计，
// 用于遏制分布式撞库；达到阈值后在锁定时长内直接拒绝密码登录。
// 优先使用 Redis(多副本共享计数)，未启用 Redis 时回退到进程内内存计数
// (单副本准确，多副本下阈值按副本数放大，属可接受降级)。

const loginFailureKeyPrefix = "login_failure:"

type loginFailureEntry struct {
	count     int
	expiresAt time.Time
}

var loginFailureStore = struct {
	sync.Mutex
	entries map[string]*loginFailureEntry
}{entries: make(map[string]*loginFailureEntry)}

// IsEmailLoginIdentifier 判断登录标识符是否按邮箱解释：含 "@" 即视为邮箱。
// 登录路径据此显式决定查询列，不再做「用户名命不中就按邮箱猜」的跨列兜底。
func IsEmailLoginIdentifier(identifier string) bool {
	return strings.Contains(identifier, "@")
}

// LoginFailureKeyForUser 标识符已解析到用户时的计数 key：按用户 id 收敛，
// 使同一账号的用户名写法与邮箱写法共享同一份失败计数。
func LoginFailureKeyForUser(userId int) string {
	if userId <= 0 {
		return ""
	}
	return "uid:" + strconv.Itoa(userId)
}

// LoginFailureKeyForIdentifier 标识符解析不到用户时的计数 key：类型前缀 + 归一化标识符。
// 加类型前缀是为了与 LoginFailureKeyForUser 的 uid 空间隔离，
// 也避免「用户名恰好等于他人邮箱」的两个标识符共用计数。
func LoginFailureKeyForIdentifier(identifier string) string {
	if IsEmailLoginIdentifier(identifier) {
		email := NormalizeEmail(identifier)
		if email == "" {
			return ""
		}
		return "email:" + email
	}
	name := strings.ToLower(strings.TrimSpace(identifier))
	if name == "" {
		return ""
	}
	return "username:" + name
}

// loginFailureKey 归一化计数 key：去空格 + 转小写，避免大小写变体绕过计数。
func loginFailureKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

// IsLoginLocked 判断该计数 key 当前是否处于锁定状态。
func IsLoginLocked(key string) bool {
	key = loginFailureKey(key)
	if key == "" {
		return false
	}
	if config.LoginMaxFailures <= 0 {
		return false
	}
	return loginFailureCount(key) >= config.LoginMaxFailures
}

// RecordLoginFailure 记录一次失败登录；计数窗口即锁定时长(滚动刷新)。
func RecordLoginFailure(key string) {
	key = loginFailureKey(key)
	if key == "" || config.LoginMaxFailures <= 0 {
		return
	}

	if config.RedisEnabled {
		ctx := context.Background()
		rdb := redis.GetRedisClient()
		if err := rdb.Incr(ctx, loginFailureKeyPrefix+key).Err(); err != nil {
			logger.SysError("Failed to record failed login attempts: " + err.Error())
			return
		}
		// 每次失败都刷新过期时间，使锁定窗口从最近一次失败起算。
		if err := rdb.Expire(ctx, loginFailureKeyPrefix+key, config.LoginLockoutDuration).Err(); err != nil {
			logger.SysError("Failed to refresh failed-login counter expiry: " + err.Error())
		}
		return
	}

	loginFailureStore.Lock()
	defer loginFailureStore.Unlock()
	now := time.Now()
	entry, ok := loginFailureStore.entries[key]
	if !ok || now.After(entry.expiresAt) {
		loginFailureStore.entries[key] = &loginFailureEntry{count: 1, expiresAt: now.Add(config.LoginLockoutDuration)}
		cleanupLoginFailuresLocked(now)
		return
	}
	entry.count++
	entry.expiresAt = now.Add(config.LoginLockoutDuration)
}

// ClearLoginFailures 登录成功后清空该计数 key 的失败计数。
func ClearLoginFailures(key string) {
	key = loginFailureKey(key)
	if key == "" {
		return
	}

	if config.RedisEnabled {
		if err := redis.RedisDel(loginFailureKeyPrefix + key); err != nil {
			logger.SysError("Failed to clear failed-login counter: " + err.Error())
		}
		return
	}

	loginFailureStore.Lock()
	defer loginFailureStore.Unlock()
	delete(loginFailureStore.entries, key)
}

func loginFailureCount(key string) int {
	if config.RedisEnabled {
		val, err := redis.RedisGet(loginFailureKeyPrefix + key)
		if err != nil {
			// 键不存在或 Redis 异常时不阻断登录，交由密码校验与 IP 限流兜底。
			return 0
		}
		count, err := strconv.Atoi(val)
		if err != nil {
			return 0
		}
		return count
	}

	loginFailureStore.Lock()
	defer loginFailureStore.Unlock()
	entry, ok := loginFailureStore.entries[key]
	if !ok {
		return 0
	}
	if time.Now().After(entry.expiresAt) {
		delete(loginFailureStore.entries, key)
		return 0
	}
	return entry.count
}

// cleanupLoginFailuresLocked 惰性清理过期条目，避免内存无界增长。调用方需持锁。
func cleanupLoginFailuresLocked(now time.Time) {
	if len(loginFailureStore.entries) < 1024 {
		return
	}
	for k, v := range loginFailureStore.entries {
		if now.After(v.expiresAt) {
			delete(loginFailureStore.entries, k)
		}
	}
}
