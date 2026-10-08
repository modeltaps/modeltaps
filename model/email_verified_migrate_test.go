package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/utils"
)

// 存量库升级：非空邮箱一律回填为已验证（历史上分不出来源，宁可放过存量，
// 也不让全站用户重新验证），没有邮箱的账号保持未验证。重复执行结果不变。
func TestBackfillUserEmailVerified(t *testing.T) {
	db := setupOidcTestDB(t)

	withEmail := &User{
		Username: "hasmail", Email: "hasmail@example.com",
		AccessToken: utils.GetUUID(), AffCode: utils.GetRandomString(8),
	}
	withoutEmail := &User{
		Username:    "nomail",
		AccessToken: utils.GetUUID(), AffCode: utils.GetRandomString(8),
	}
	for _, user := range []*User{withEmail, withoutEmail} {
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("预置用户 %s 失败: %v", user.Username, err)
		}
	}

	migration := backfillUserEmailVerified()
	for round := 1; round <= 2; round++ {
		if err := migration.Migrate(db); err != nil {
			t.Fatalf("第 %d 次迁移失败: %v", round, err)
		}
		var mailed, bare User
		if err := db.First(&mailed, withEmail.Id).Error; err != nil {
			t.Fatalf("读取用户失败: %v", err)
		}
		if !mailed.EmailVerified {
			t.Fatal("存量非空邮箱应回填为已验证")
		}
		if err := db.First(&bare, withoutEmail.Id).Error; err != nil {
			t.Fatalf("读取用户失败: %v", err)
		}
		if bare.EmailVerified {
			t.Fatal("没有邮箱的账号不应被标为已验证")
		}
	}

	// 回滚删列：列没了说明回滚路径可用，重复回滚不报错。
	if err := migration.Rollback(db); err != nil {
		t.Fatalf("回滚失败: %v", err)
	}
	if db.Migrator().HasColumn(&User{}, "email_verified") {
		t.Fatal("回滚后 email_verified 列应已删除")
	}
	if err := migration.Rollback(db); err != nil {
		t.Fatalf("重复回滚不应报错: %v", err)
	}
}

// 新库：users 表为空时迁移是无操作，不得报错。
func TestBackfillUserEmailVerifiedOnFreshDB(t *testing.T) {
	db := setupOidcTestDB(t)
	if err := backfillUserEmailVerified().Migrate(db); err != nil {
		t.Fatalf("新库迁移失败: %v", err)
	}
}
