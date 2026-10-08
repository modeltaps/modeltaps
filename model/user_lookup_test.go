package model

import (
	"errors"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// TestFillUserNotFoundSentinel 守护 OIDC/OAuth 关键不变量:FillUserByXxx 未命中时返回的错误
// 必须满足 errors.Is(err, ErrUserNotFound)，否则 oidc.go 的「未找到→注册/绑定」分支会被
// errors.Is 判断挡掉,导致首次 OIDC 登录永远失败(本次 Logto 集成测试发现的真实 bug)。
func TestFillUserNotFoundSentinel(t *testing.T) {
	setupOrgTestDB(t)

	uo := &User{OidcId: "no-such-oidc-subject-xyz"}
	if err := uo.FillUserByOidcId(); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("FillUserByOidcId 未命中应 errors.Is(ErrUserNotFound)，实际: %v", err)
	}

	uu := &User{Username: "no-such-username-xyz"}
	if err := uu.FillUserByUsername(); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("FillUserByUsername 未命中应 errors.Is(ErrUserNotFound)，实际: %v", err)
	}
}

// ResolveLoginUser 按标识符类型显式解析：含 @ 只查邮箱，否则只查用户名，
// 且两条路径都排除组织影子账户。
func TestResolveLoginUserExplicitIdentifier(t *testing.T) {
	setupOrgTestDB(t)

	normal := &User{
		Username:    "grace",
		Email:       NullableEmail("grace@example.com"),
		Role:        config.RoleCommonUser,
		Status:      config.UserStatusEnabled,
		Type:        config.UserTypeNormal,
		AccessToken: "resolve-token-grace",
		AffCode:     "affg",
	}
	if err := DB.Create(normal).Error; err != nil {
		t.Fatalf("创建普通用户失败: %v", err)
	}
	shadow := &User{
		Username:    "org-shadow-resolve",
		Email:       NullableEmail("shadow-resolve@example.com"),
		Role:        config.RoleGuestUser,
		Status:      config.UserStatusEnabled,
		Type:        config.UserTypeOrgShadow,
		AccessToken: "resolve-token-shadow",
		AffCode:     "affsr",
	}
	if err := DB.Create(shadow).Error; err != nil {
		t.Fatalf("创建影子用户失败: %v", err)
	}

	t.Run("用户名命中", func(t *testing.T) {
		u, err := ResolveLoginUser("grace")
		if err != nil || u.Id != normal.Id {
			t.Fatalf("按用户名应命中 grace，实际: %v %v", u, err)
		}
	})

	t.Run("邮箱大小写归一命中", func(t *testing.T) {
		u, err := ResolveLoginUser("  Grace@Example.COM ")
		if err != nil || u.Id != normal.Id {
			t.Fatalf("按归一化邮箱应命中 grace，实际: %v %v", u, err)
		}
	})

	t.Run("用户名不按邮箱兜底", func(t *testing.T) {
		if _, err := ResolveLoginUser("grace@example.com.invalid"); !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("未命中应返回 ErrUserNotFound，实际: %v", err)
		}
	})

	t.Run("影子账户不可登录", func(t *testing.T) {
		if _, err := ResolveLoginUser("org-shadow-resolve"); !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("影子账户按用户名应不可解析，实际: %v", err)
		}
		if _, err := ResolveLoginUser("shadow-resolve@example.com"); !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("影子账户按邮箱应不可解析，实际: %v", err)
		}
	})

	t.Run("空标识符", func(t *testing.T) {
		if _, err := ResolveLoginUser("   "); !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("空标识符应返回 ErrUserNotFound，实际: %v", err)
		}
	})
}
