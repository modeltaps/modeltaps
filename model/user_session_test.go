package model

import (
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupUserSessionTestDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&UserSession{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
}

func mustCreateSession(t *testing.T, userId int, method string) *UserSession {
	t.Helper()
	session, err := CreateUserSession(userId, method, "Mozilla/5.0 test", "127.0.0.1", 0, "")
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	return session
}

func setSessionTime(t *testing.T, id int, created, lastSeen int64) {
	t.Helper()
	if err := DB.Model(&UserSession{}).Where("id = ?", id).
		Updates(map[string]interface{}{"created_time": created, "last_seen_time": lastSeen}).Error; err != nil {
		t.Fatalf("改写会话时间失败: %v", err)
	}
}

// 会话行的生命周期：键 + 用户匹配才有效；空闲或绝对期限过期即失效并删行；活跃时间按节流回写。
func TestUserSessionLifecycle(t *testing.T) {
	setupUserSessionTestDB(t)
	now := utils.GetTimestamp()
	session := mustCreateSession(t, 1, SessionMethodPassword)

	if _, err := ValidateUserSession(session.SessionKey, 1); err != nil {
		t.Fatalf("刚建立的会话应有效: %v", err)
	}
	if _, err := ValidateUserSession(session.SessionKey, 2); err == nil {
		t.Fatal("会话键属于用户 1，用户 2 不应通过")
	}
	if _, err := ValidateUserSession("", 1); err == nil {
		t.Fatal("没有会话键不应通过")
	}

	// 活跃时间超过节流间隔才回写
	setSessionTime(t, session.Id, now, now-int64(userSessionTouchInterval.Seconds())-1)
	validated, err := ValidateUserSession(session.SessionKey, 1)
	if err != nil {
		t.Fatalf("会话应仍有效: %v", err)
	}
	if validated.LastSeenTime < now {
		t.Fatalf("活跃时间应已回写，实际 %d", validated.LastSeenTime)
	}

	// 空闲超时
	setSessionTime(t, session.Id, now, now-int64(config.SessionIdleDuration.Seconds())-1)
	if _, err := ValidateUserSession(session.SessionKey, 1); err == nil {
		t.Fatal("空闲超过上限的会话应失效")
	}
	if _, err := GetUserSessionByKey(session.SessionKey); err == nil {
		t.Fatal("过期会话应被删除")
	}

	// 绝对期限
	old := mustCreateSession(t, 1, SessionMethodPassword)
	setSessionTime(t, old.Id, now-int64(config.SessionMaxDuration.Seconds())-1, now)
	if _, err := ValidateUserSession(old.SessionKey, 1); err == nil {
		t.Fatal("超过绝对期限的会话应失效")
	}
}

// 吊销：只保留指定会话 / 全部吊销 / 只能删自己的 / 按提供方结束；列表不含过期行且最近活跃在前。
func TestUserSessionRevocation(t *testing.T) {
	setupUserSessionTestDB(t)
	now := utils.GetTimestamp()
	keep := mustCreateSession(t, 1, SessionMethodPassword)
	other := mustCreateSession(t, 1, SessionMethodPasskey)
	stale := mustCreateSession(t, 1, SessionMethodPassword)
	setSessionTime(t, stale.Id, now, now-int64(config.SessionIdleDuration.Seconds())-1)
	foreign := mustCreateSession(t, 2, SessionMethodPassword)

	sessions, err := ListUserSessions(1)
	if err != nil {
		t.Fatalf("列会话失败: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("过期行不应列出，期望 2 行实际 %d", len(sessions))
	}

	if deleted, err := DeleteUserSessionById(1, foreign.Id); err != nil || deleted {
		t.Fatalf("不能删别人的会话: deleted=%v err=%v", deleted, err)
	}
	if deleted, err := DeleteUserSessionById(1, other.Id); err != nil || !deleted {
		t.Fatalf("应能删自己的会话: deleted=%v err=%v", deleted, err)
	}

	mustCreateSession(t, 1, SessionMethodPassword)
	if err := DeleteUserSessionsExcept(nil, 1, keep.SessionKey); err != nil {
		t.Fatalf("登出其它设备失败: %v", err)
	}
	sessions, _ = ListUserSessions(1)
	if len(sessions) != 1 || sessions[0].SessionKey != keep.SessionKey {
		t.Fatalf("应只剩保留的会话，实际 %+v", sessions)
	}
	if _, err := ValidateUserSession(foreign.SessionKey, 2); err != nil {
		t.Fatalf("其他用户的会话不应受影响: %v", err)
	}

	if err := DeleteUserSessionsExcept(nil, 1, ""); err != nil {
		t.Fatalf("吊销全部失败: %v", err)
	}
	if sessions, _ := ListUserSessions(1); len(sessions) != 0 {
		t.Fatalf("吊销全部后不应有会话，实际 %+v", sessions)
	}

	viaProvider, err := CreateUserSession(3, SessionMethodOidc, "ua", "::1", 7, "id-token")
	if err != nil {
		t.Fatalf("创建 OIDC 会话失败: %v", err)
	}
	if err := DeleteUserSessionsByProvider(nil, 7); err != nil {
		t.Fatalf("按提供方结束会话失败: %v", err)
	}
	if _, err := GetUserSessionByKey(viaProvider.SessionKey); err == nil {
		t.Fatal("删除提供方后经它登录的会话应结束")
	}
	_ = time.Now()
}
