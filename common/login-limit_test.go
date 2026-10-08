package common

import (
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
)

// resetLoginFailures 清空内存计数，避免用例间互相污染
func resetLoginFailures(t *testing.T) {
	t.Helper()
	loginFailureStore.Lock()
	loginFailureStore.entries = make(map[string]*loginFailureEntry)
	loginFailureStore.Unlock()
}

func TestLoginLockoutAfterMaxFailures(t *testing.T) {
	resetLoginFailures(t)

	for i := 0; i < config.LoginMaxFailures-1; i++ {
		RecordLoginFailure("Alice")
		if IsLoginLocked("alice") {
			t.Fatalf("第 %d 次失败后不应锁定", i+1)
		}
	}
	RecordLoginFailure("alice")
	if !IsLoginLocked("alice") {
		t.Fatalf("达到 %d 次失败后应锁定", config.LoginMaxFailures)
	}
	// 大小写与空格归一：同一账号的不同书写形式共享计数
	if !IsLoginLocked("  ALICE  ") {
		t.Error("账号标识应按小写去空格归一")
	}
	if IsLoginLocked("bob") {
		t.Error("其它账号不应被牵连")
	}
}

func TestLoginFailuresClearedOnSuccess(t *testing.T) {
	resetLoginFailures(t)

	for i := 0; i < config.LoginMaxFailures; i++ {
		RecordLoginFailure("carol")
	}
	if !IsLoginLocked("carol") {
		t.Fatal("应先进入锁定状态")
	}
	ClearLoginFailures("CAROL")
	if IsLoginLocked("carol") {
		t.Error("登录成功后计数应清零")
	}
}

func TestLoginLockoutExpires(t *testing.T) {
	resetLoginFailures(t)

	old := config.LoginLockoutDuration
	config.LoginLockoutDuration = 20 * time.Millisecond
	t.Cleanup(func() { config.LoginLockoutDuration = old })

	for i := 0; i < config.LoginMaxFailures; i++ {
		RecordLoginFailure("dave")
	}
	if !IsLoginLocked("dave") {
		t.Fatal("应先进入锁定状态")
	}
	time.Sleep(40 * time.Millisecond)
	if IsLoginLocked("dave") {
		t.Error("锁定窗口过期后应自动解锁")
	}
}

// 计数 key 的收敛规则：解析到用户按 uid 收敛，未解析到按「类型前缀 + 归一化标识符」，
// 两个空间互不串号。
func TestLoginFailureKeyDerivation(t *testing.T) {
	if got := LoginFailureKeyForUser(7); got != "uid:7" {
		t.Errorf("已解析用户的 key = %q，期望 uid:7", got)
	}
	if got := LoginFailureKeyForUser(0); got != "" {
		t.Errorf("非法用户 id 应返回空 key，实际 %q", got)
	}

	if got := LoginFailureKeyForIdentifier("  Alice  "); got != "username:alice" {
		t.Errorf("用户名标识符 key = %q，期望 username:alice", got)
	}
	if got := LoginFailureKeyForIdentifier(" Alice@Example.COM "); got != "email:"+NormalizeEmail("Alice@Example.COM") {
		t.Errorf("邮箱标识符 key = %q，期望按归一化邮箱", got)
	}
	if got := LoginFailureKeyForIdentifier("   "); got != "" {
		t.Errorf("空标识符应返回空 key，实际 %q", got)
	}

	// 「用户名恰好等于他人邮箱」时两个标识符不共用计数
	if LoginFailureKeyForIdentifier("bob@example.com") == LoginFailureKeyForIdentifier("bob") {
		t.Error("邮箱与用户名标识符不应共用计数 key")
	}
}

// 同一账号解析成功后，用户名写法与邮箱写法共享同一份失败计数（key 收敛到 uid）。
func TestLoginFailureKeyConvergesByUserId(t *testing.T) {
	resetLoginFailures(t)

	byUsername := LoginFailureKeyForUser(42)
	byEmail := LoginFailureKeyForUser(42)
	for i := 0; i < config.LoginMaxFailures-1; i++ {
		RecordLoginFailure(byUsername)
	}
	RecordLoginFailure(byEmail)
	if !IsLoginLocked(byUsername) {
		t.Fatal("用户名与邮箱两种写法的失败次数应累加到同一 key")
	}
	if IsLoginLocked(LoginFailureKeyForUser(43)) {
		t.Error("其它用户不应被牵连")
	}
}

func TestLoginLockoutDisabledWhenThresholdNonPositive(t *testing.T) {
	resetLoginFailures(t)

	old := config.LoginMaxFailures
	config.LoginMaxFailures = 0
	t.Cleanup(func() { config.LoginMaxFailures = old })

	for i := 0; i < 10; i++ {
		RecordLoginFailure("erin")
	}
	if IsLoginLocked("erin") {
		t.Error("阈值 <= 0 时应关闭账号级锁定")
	}
}
