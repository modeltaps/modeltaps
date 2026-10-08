package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/gorm"
)

// seedLegacyOidcOptions 预置升级前的单提供方配置。OIDCAuthEnabled 在 options 表里
// 存的是字符串 "true"，迁移必须按字符串判定而不是当成布尔列。
func seedLegacyOidcOptions(t *testing.T, db *gorm.DB) {
	t.Helper()
	options := [][2]string{
		{"OIDCAuthEnabled", "true"},
		{"OIDCClientId", "cid-123"},
		{"OIDCClientSecret", "secret-xyz"},
		{"OIDCIssuer", "https://issuer.example.com"},
		{"OIDCScopes", "openid profile email"},
		{"OIDCUsernameClaims", "preferred_username"},
	}
	for _, option := range options {
		if err := db.Create(&Option{Key: option[0], Value: option[1]}).Error; err != nil {
			t.Fatalf("预置选项 %s 失败: %v", option[0], err)
		}
	}
}

func seedLegacyOidcUser(t *testing.T, db *gorm.DB, username, oidcId string) {
	t.Helper()
	if err := db.Exec("INSERT INTO users (username, password, oidc_id, aff_code, access_token) VALUES (?, ?, ?, ?, ?)",
		username, "test-password", oidcId, utils.GetRandomString(8), utils.GetUUID()).Error; err != nil {
		t.Fatalf("预置用户 %s 失败: %v", username, err)
	}
}

// TestMigrateLegacyOidcProviderIdempotent 存量配置迁移一次建出提供方与身份行，
// 重复执行不得新增行（gormigrate 的 ID 记录之外，插入前都按唯一键查过一次）。
func TestMigrateLegacyOidcProviderIdempotent(t *testing.T) {
	db := setupOidcTestDB(t)

	seedLegacyOidcOptions(t, db)
	seedLegacyOidcUser(t, db, "u1", "sub-1")
	seedLegacyOidcUser(t, db, "u2", "sub-2")
	seedLegacyOidcUser(t, db, "u3", "")

	migration := migrateLegacyOidcProvider()
	for round := 1; round <= 2; round++ {
		if err := migration.Migrate(db); err != nil {
			t.Fatalf("第 %d 次迁移失败: %v", round, err)
		}

		var providerCount, identityCount int64
		if err := db.Model(&OidcProvider{}).Count(&providerCount).Error; err != nil {
			t.Fatalf("统计提供方失败: %v", err)
		}
		if err := db.Model(&UserOidcIdentity{}).Count(&identityCount).Error; err != nil {
			t.Fatalf("统计身份失败: %v", err)
		}
		if providerCount != 1 {
			t.Fatalf("第 %d 次迁移后应恰有 1 个提供方，实际 %d", round, providerCount)
		}
		// oidc_id 为空的 u3 不该被迁进来
		if identityCount != 2 {
			t.Fatalf("第 %d 次迁移后应恰有 2 条身份，实际 %d", round, identityCount)
		}
	}

	provider, err := GetOidcProviderBySlug(LegacyOidcProviderSlug)
	if err != nil {
		t.Fatalf("查询迁移出的提供方失败: %v", err)
	}
	checks := []struct {
		name string
		got  string
		want string
	}{
		{"issuer", provider.Issuer, "https://issuer.example.com"},
		{"client_id", provider.ClientId, "cid-123"},
		{"client_secret", provider.ClientSecret, "secret-xyz"},
		{"scopes", provider.Scopes, "openid profile email"},
		{"username_claim", provider.UsernameClaim, "preferred_username"},
		// 旧实现里这两个 claim 名是硬编码的（controller/oidc.go），迁移须原样沿用
		{"display_name_claim", provider.DisplayNameClaim, "displayName"},
		{"avatar_claim", provider.AvatarClaim, "avatar"},
		// OIDCDisplayName 未配置时兜底
		{"display_name", provider.DisplayName, "OIDC"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Fatalf("%s 应为 %q，实际 %q", c.name, c.want, c.got)
		}
	}
	// 字符串 "true" 必须被识别为启用；按已验证邮箱关联须保持开启，否则老用户会被重复注册
	if !provider.Enabled {
		t.Fatal("OIDCAuthEnabled=\"true\" 应迁移为 enabled=true")
	}
	if !provider.LinkByVerifiedEmail {
		t.Fatal("link_by_verified_email 应为 true")
	}

	for _, subject := range []string{"sub-1", "sub-2"} {
		identity, err := FindUserOidcIdentity(provider.Id, subject)
		if err != nil {
			t.Fatalf("subject %s 应已迁移: %v", subject, err)
		}
		if identity.UserId == 0 {
			t.Fatalf("subject %s 的身份缺少 user id", subject)
		}
	}
}

// TestMigrateLegacyOidcProviderNoLegacyConfig 没配过 OIDC 的库不该凭空建出提供方
func TestMigrateLegacyOidcProviderNoLegacyConfig(t *testing.T) {
	db := setupOidcTestDB(t)

	seedLegacyOidcUser(t, db, "u1", "")
	if err := migrateLegacyOidcProvider().Migrate(db); err != nil {
		t.Fatalf("空配置迁移应成功: %v", err)
	}

	var providerCount, identityCount int64
	if err := db.Model(&OidcProvider{}).Count(&providerCount).Error; err != nil {
		t.Fatalf("统计提供方失败: %v", err)
	}
	if err := db.Model(&UserOidcIdentity{}).Count(&identityCount).Error; err != nil {
		t.Fatalf("统计身份失败: %v", err)
	}
	if providerCount != 0 || identityCount != 0 {
		t.Fatalf("无旧配置时两表应为空，实际 %d / %d", providerCount, identityCount)
	}
}

// TestMigrateLegacyOidcProviderKeepsDisplayName 配置过 OIDCDisplayName 时不能被兜底值覆盖
func TestMigrateLegacyOidcProviderKeepsDisplayName(t *testing.T) {
	db := setupOidcTestDB(t)

	seedLegacyOidcOptions(t, db)
	if err := db.Create(&Option{Key: "OIDCDisplayName", Value: "公司登录"}).Error; err != nil {
		t.Fatalf("预置 OIDCDisplayName 失败: %v", err)
	}
	if err := migrateLegacyOidcProvider().Migrate(db); err != nil {
		t.Fatalf("迁移应成功: %v", err)
	}

	provider, err := GetOidcProviderBySlug(LegacyOidcProviderSlug)
	if err != nil {
		t.Fatalf("查询提供方失败: %v", err)
	}
	if provider.DisplayName != "公司登录" {
		t.Fatalf("display_name 应沿用 OIDCDisplayName，实际 %q", provider.DisplayName)
	}
}
