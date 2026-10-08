package model

import (
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupOidcTestDB 建一个只含 OIDC 相关表的内存库。唯一索引由 gorm 标签声明，
// AutoMigrate 直接建出来，测试据此断言约束真实生效。
func setupOidcTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&OidcProvider{}, &UserOidcIdentity{}, &UserSession{}, &Option{}, &User{}, &Organization{}, &OrganizationMember{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
	return testDB
}

// TestValidateOidcProviderSlug slug 会进 URL（/oauth/oidc/{slug}），只放行小写字母、数字与连字符
func TestValidateOidcProviderSlug(t *testing.T) {
	valid := []string{"oidc", "a", "my-idp", "idp2", "0", strings.Repeat("a", 32)}
	for _, slug := range valid {
		if err := ValidateOidcProviderSlug(slug); err != nil {
			t.Fatalf("slug %q 应合法，实际: %v", slug, err)
		}
	}
	invalid := []string{"", "OIDC", "my_idp", "my idp", "a@b", "idp.com", "中文", strings.Repeat("a", 33)}
	for _, slug := range invalid {
		if err := ValidateOidcProviderSlug(slug); err == nil {
			t.Fatalf("slug %q 应被拒绝", slug)
		}
	}
}

// TestOidcProviderInsertRejectsBadSlug 非法 slug 在 Insert 层就要被挡掉，不能落库
func TestOidcProviderInsertRejectsBadSlug(t *testing.T) {
	setupOidcTestDB(t)

	bad := OidcProvider{Slug: "Bad_Slug"}
	if err := bad.Insert(); err == nil {
		t.Fatal("非法 slug 插入应被拒绝")
	}
	var count int64
	if err := DB.Model(&OidcProvider{}).Count(&count).Error; err != nil {
		t.Fatalf("统计提供方失败: %v", err)
	}
	if count != 0 {
		t.Fatalf("非法 slug 不应落库，实际 %d 行", count)
	}
}

// TestOidcProviderSlugUnique slug 唯一索引由 gorm 标签声明，重复创建必须报冲突
func TestOidcProviderSlugUnique(t *testing.T) {
	setupOidcTestDB(t)

	first := OidcProvider{Slug: "my-idp", DisplayName: "第一个"}
	if err := first.Insert(); err != nil {
		t.Fatalf("首次插入应成功: %v", err)
	}
	second := OidcProvider{Slug: "my-idp", DisplayName: "第二个"}
	if err := second.Insert(); err == nil {
		t.Fatal("重复 slug 插入应被拒绝")
	}
}

// TestUserOidcIdentityUnique 同一 (provider_id, subject) 只能归属一个用户；
// 同一 subject 在不同提供方下互不影响。
func TestUserOidcIdentityUnique(t *testing.T) {
	setupOidcTestDB(t)

	a := OidcProvider{Slug: "idp-a"}
	b := OidcProvider{Slug: "idp-b"}
	for _, p := range []*OidcProvider{&a, &b} {
		if err := p.Insert(); err != nil {
			t.Fatalf("插入提供方 %s 失败: %v", p.Slug, err)
		}
	}

	first := UserOidcIdentity{UserId: 1, ProviderId: a.Id, Subject: "sub-1"}
	if err := first.Insert(); err != nil {
		t.Fatalf("首次绑定应成功: %v", err)
	}
	dup := UserOidcIdentity{UserId: 2, ProviderId: a.Id, Subject: "sub-1"}
	if err := dup.Insert(); err == nil {
		t.Fatal("同一 (provider_id, subject) 重复插入应被拒绝")
	}
	crossProvider := UserOidcIdentity{UserId: 2, ProviderId: b.Id, Subject: "sub-1"}
	if err := crossProvider.Insert(); err != nil {
		t.Fatalf("同 subject 不同提供方应可共存: %v", err)
	}

	found, err := FindUserOidcIdentity(a.Id, "sub-1")
	if err != nil {
		t.Fatalf("按 (provider_id, subject) 查询失败: %v", err)
	}
	if found.UserId != 1 {
		t.Fatalf("应查到 user id 1，实际 %d", found.UserId)
	}
	identities, err := ListUserOidcIdentities(2)
	if err != nil {
		t.Fatalf("列出用户身份失败: %v", err)
	}
	if len(identities) != 1 || identities[0].ProviderId != b.Id {
		t.Fatalf("user 2 应只绑定 idp-b，实际 %+v", identities)
	}
}

// TestUserDeleteRemovesOidcIdentities 软删用户须连带删掉其身份行，
// 否则 (provider_id, subject) 唯一索引一直被占，原 IdP 主体再也无法注册。
func TestUserDeleteRemovesOidcIdentities(t *testing.T) {
	setupOidcTestDB(t)

	provider := OidcProvider{Slug: "idp-a"}
	if err := provider.Insert(); err != nil {
		t.Fatalf("插入提供方失败: %v", err)
	}

	user := User{Username: "u1", Password: "test-password", AffCode: utils.GetRandomString(8)}
	if err := DB.Create(&user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	identity := UserOidcIdentity{UserId: user.Id, ProviderId: provider.Id, Subject: "sub-1"}
	if err := identity.Insert(); err != nil {
		t.Fatalf("绑定身份失败: %v", err)
	}

	if err := user.Delete(); err != nil {
		t.Fatalf("删除用户失败: %v", err)
	}

	var identityCount int64
	if err := DB.Model(&UserOidcIdentity{}).Where("user_id = ?", user.Id).Count(&identityCount).Error; err != nil {
		t.Fatalf("统计身份失败: %v", err)
	}
	if identityCount != 0 {
		t.Fatalf("删号后身份行应清空，实际 %d 行", identityCount)
	}

	var deleted User
	if err := DB.Unscoped().Where("id = ?", user.Id).First(&deleted).Error; err != nil {
		t.Fatalf("软删用户应仍可查到: %v", err)
	}
	if !deleted.DeletedAt.Valid {
		t.Fatal("用户应为软删状态（deleted_at 非空）")
	}
	if !strings.Contains(deleted.Username, "_del_") {
		t.Fatalf("用户名应带 _del_ 后缀，实际 %q", deleted.Username)
	}
	if deleted.Email != "" {
		t.Fatalf("邮箱应被清空，实际 %q", deleted.Email)
	}

	// 同 (provider_id, subject) 必须能重新绑定到新账号
	reuse := UserOidcIdentity{UserId: user.Id + 1, ProviderId: provider.Id, Subject: "sub-1"}
	if err := reuse.Insert(); err != nil {
		t.Fatalf("删号后同 (provider_id, subject) 应可重新插入: %v", err)
	}
}
