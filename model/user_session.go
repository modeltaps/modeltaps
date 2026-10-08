package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/modeltaps/modeltaps/common/cache"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/redis"
	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/gorm"
)

// UserSession 一次登录会话。cookie 里只放随机 SessionKey，会话是否仍然有效以本表为准：
// 改密码踢出其它设备、「登出其它所有设备」、封禁 / 删除用户都直接删行；空闲与绝对有效期
// 按 LastSeenTime / CreatedTime 判定。经身份提供方登录的会话把 id_token 存在行上，
// 退出时作 RP-Initiated Logout 的 id_token_hint；IdToken 永远不进任何响应体。
type UserSession struct {
	Id          int    `json:"id"`
	UserId      int    `json:"user_id" gorm:"not null;index"`
	SessionKey  string `json:"-" gorm:"type:varchar(64);not null;uniqueIndex:idx_user_sessions_key"`
	LoginMethod string `json:"login_method" gorm:"type:varchar(32);default:''"`
	UserAgent   string `json:"user_agent" gorm:"type:varchar(500);default:''"`
	Ip          string `json:"ip" gorm:"type:varchar(64);default:''"`
	ProviderId  int    `json:"-" gorm:"default:0"`
	IdToken     string `json:"-" gorm:"type:text"`
	// CreatedTime 登录时刻；LastSeenTime 最近一次经鉴权的请求时刻（最多每 userSessionTouchInterval 写一次）
	CreatedTime  int64 `json:"created_time" gorm:"bigint"`
	LastSeenTime int64 `json:"last_seen_time" gorm:"bigint"`
}

// 登录方式常量，写在会话行上供账号安全页展示。
const (
	SessionMethodPassword  = "password"
	SessionMethodEmailCode = "email_code"
	SessionMethodPasskey   = "passkey"
	SessionMethodOidc      = "oidc"
)

// userSessionTouchInterval LastSeenTime 的最小回写间隔：每个请求都写库没有意义。
const userSessionTouchInterval = 5 * time.Minute

// userSessionMaxUserAgent 与列宽一致，超长 UA 截断而不是让写入报错。
const userSessionMaxUserAgent = 500

// UserSessionCacheKey 会话行缓存键（仅启用 Redis 时使用；TokenCacheSeconds=0 永不过期，靠删行时主动失效）。
const UserSessionCacheKey = "user_session:%s"

// ErrUserSessionInvalid 会话不存在或已过期。
var ErrUserSessionInvalid = errors.New("session expired, please sign in again")

// CreateUserSession 登录时建一行会话并返回；顺带清掉过期行，这些行没有别的回收时机。
func CreateUserSession(userId int, method string, userAgent string, ip string, providerId int, idToken string) (*UserSession, error) {
	if userId == 0 {
		return nil, errors.New("user id is empty")
	}
	now := utils.GetTimestamp()
	if err := PurgeExpiredUserSessions(now); err != nil {
		logger.SysError("failed to clean up expired sessions: " + err.Error())
	}
	if len(userAgent) > userSessionMaxUserAgent {
		userAgent = userAgent[:userSessionMaxUserAgent]
	}
	session := &UserSession{
		UserId:       userId,
		SessionKey:   utils.GetSecureRandomString(32),
		LoginMethod:  method,
		UserAgent:    userAgent,
		Ip:           ip,
		ProviderId:   providerId,
		IdToken:      idToken,
		CreatedTime:  now,
		LastSeenTime: now,
	}
	if err := DB.Create(session).Error; err != nil {
		return nil, err
	}
	return session, nil
}

// GetUserSessionByKey 按 cookie 里的键取会话行，未命中返回 gorm.ErrRecordNotFound。
func GetUserSessionByKey(key string) (*UserSession, error) {
	if key == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var session UserSession
	if err := DB.Where("session_key = ?", key).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

// userSessionExpired 按空闲与绝对有效期判定会话是否已过期。
func userSessionExpired(session *UserSession, now int64) bool {
	if now-session.LastSeenTime > int64(config.SessionIdleDuration.Seconds()) {
		return true
	}
	return now-session.CreatedTime > int64(config.SessionMaxDuration.Seconds())
}

// cachedUserSession 读会话行：启用 Redis 时走缓存，否则直接查库（与角色 / 状态缓存同一策略）。
func cachedUserSession(key string) (UserSession, error) {
	if !config.RedisEnabled {
		session, err := GetUserSessionByKey(key)
		if err != nil {
			return UserSession{}, err
		}
		return *session, nil
	}
	return cache.GetOrSetCache(
		fmt.Sprintf(UserSessionCacheKey, key),
		time.Duration(TokenCacheSeconds)*time.Second,
		func() (UserSession, error) {
			session, err := GetUserSessionByKey(key)
			if err != nil {
				return UserSession{}, err
			}
			return *session, nil
		},
		cache.CacheTimeout)
}

// invalidateUserSessionCache 删行 / 回写后主动失效缓存，保证吊销下一次请求即生效。
func invalidateUserSessionCache(keys ...string) {
	if !config.RedisEnabled {
		return
	}
	for _, key := range keys {
		cacheKey := fmt.Sprintf(UserSessionCacheKey, key)
		if err := redis.RedisDel(cacheKey); err != nil {
			logger.SysError("failed to clear session Redis cache: " + err.Error())
		}
		if err := cache.DeleteCache(cacheKey); err != nil {
			logger.SysError("failed to clear session cache: " + err.Error())
		}
	}
}

// ValidateUserSession 鉴权时校验 cookie 里的会话键：行存在、属于该用户、未过期即有效，
// 并按节流回写 LastSeenTime。过期行顺手删除。
func ValidateUserSession(key string, userId int) (*UserSession, error) {
	if key == "" || userId == 0 {
		return nil, ErrUserSessionInvalid
	}
	session, err := cachedUserSession(key)
	if err != nil {
		return nil, ErrUserSessionInvalid
	}
	if session.UserId != userId {
		return nil, ErrUserSessionInvalid
	}
	now := utils.GetTimestamp()
	if userSessionExpired(&session, now) {
		_ = DeleteUserSessionByKey(key)
		return nil, ErrUserSessionInvalid
	}
	if now-session.LastSeenTime >= int64(userSessionTouchInterval.Seconds()) {
		if err := DB.Model(&UserSession{}).Where("id = ?", session.Id).Update("last_seen_time", now).Error; err != nil {
			logger.SysError("failed to write back session last-active time: " + err.Error())
		} else {
			session.LastSeenTime = now
			invalidateUserSessionCache(key)
		}
	}
	return &session, nil
}

// ListUserSessions 列出该用户当前有效的会话，最近活跃的在前。过期行不列出。
func ListUserSessions(userId int) ([]*UserSession, error) {
	if userId == 0 {
		return nil, errors.New("user id is empty")
	}
	var sessions []*UserSession
	if err := DB.Where("user_id = ?", userId).Order("last_seen_time desc, id desc").Find(&sessions).Error; err != nil {
		return nil, err
	}
	now := utils.GetTimestamp()
	alive := sessions[:0]
	for _, session := range sessions {
		if !userSessionExpired(session, now) {
			alive = append(alive, session)
		}
	}
	return alive, nil
}

// DeleteUserSessionByKey 退出登录：按键删行并失效缓存。
func DeleteUserSessionByKey(key string) error {
	if key == "" {
		return nil
	}
	err := DB.Where("session_key = ?", key).Delete(&UserSession{}).Error
	invalidateUserSessionCache(key)
	return err
}

// DeleteUserSessionById 吊销该用户的某一个会话（只能删自己的）。返回是否真的删掉了一行。
func DeleteUserSessionById(userId int, id int) (bool, error) {
	if userId == 0 || id == 0 {
		return false, errors.New("user id or session id is empty")
	}
	var session UserSession
	if err := DB.Where("id = ? AND user_id = ?", id, userId).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	if err := DB.Delete(&session).Error; err != nil {
		return false, err
	}
	invalidateUserSessionCache(session.SessionKey)
	return true, nil
}

// DeleteUserSessionsExcept 吊销该用户除 keepKey 外的全部会话（改密码、「登出其它所有设备」）。
// keepKey 为空即吊销全部。tx 为空时走全局 DB。
func DeleteUserSessionsExcept(tx *gorm.DB, userId int, keepKey string) error {
	if userId == 0 {
		return errors.New("user id is empty")
	}
	if tx == nil {
		tx = DB
	}
	query := tx.Where("user_id = ?", userId)
	if keepKey != "" {
		query = query.Where("session_key <> ?", keepKey)
	}
	var keys []string
	if err := query.Model(&UserSession{}).Pluck("session_key", &keys).Error; err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	if err := tx.Where("session_key IN ?", keys).Delete(&UserSession{}).Error; err != nil {
		return err
	}
	invalidateUserSessionCache(keys...)
	return nil
}

// DeleteUserSessionsByProvider 删除提供方时，经它登录的会话一并结束。
func DeleteUserSessionsByProvider(tx *gorm.DB, providerId int) error {
	if providerId == 0 {
		return nil
	}
	if tx == nil {
		tx = DB
	}
	var keys []string
	if err := tx.Model(&UserSession{}).Where("provider_id = ?", providerId).Pluck("session_key", &keys).Error; err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	if err := tx.Where("session_key IN ?", keys).Delete(&UserSession{}).Error; err != nil {
		return err
	}
	invalidateUserSessionCache(keys...)
	return nil
}

// PurgeExpiredUserSessions 删除按空闲 / 绝对有效期已过期的会话行。缓存里若还有，
// 下次 ValidateUserSession 会按时间判定过期并删掉，不必逐键失效。
func PurgeExpiredUserSessions(now int64) error {
	idleBefore := now - int64(config.SessionIdleDuration.Seconds())
	createdBefore := now - int64(config.SessionMaxDuration.Seconds())
	return DB.Where("last_seen_time < ? OR created_time < ?", idleBefore, createdBefore).Delete(&UserSession{}).Error
}
