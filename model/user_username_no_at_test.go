package model

import (
	"strconv"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupUsernameTestDB 建一个只含 users 表的内存库，迁移由各用例自行触发。
func setupUsernameTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&User{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
	return testDB
}

// 无含 @ 用户名时迁移通过，且可重复执行（迁移只读、幂等）。
func TestMigrateUserNameWithoutAtPasses(t *testing.T) {
	testDB := setupUsernameTestDB(t)
	if err := testDB.Create(newEmailTestUser("alice", "alice@example.com")).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	for i := 0; i < 2; i++ {
		if err := migrateUserNameWithoutAt().Migrate(testDB); err != nil {
			t.Fatalf("第 %d 次迁移应通过，实际: %v", i+1, err)
		}
	}
}

// 存量含 @ 用户名时 fail-closed，错误里列出 id 与用户名；改名后重跑通过。
func TestMigrateUserNameWithoutAtFailsClosed(t *testing.T) {
	testDB := setupUsernameTestDB(t)
	bad := newEmailTestUser("a@b.co", "someone@example.com")
	if err := testDB.Create(bad).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	err := migrateUserNameWithoutAt().Migrate(testDB)
	if err == nil {
		t.Fatal("存量含 @ 用户名时迁移应失败")
	}
	if !strings.Contains(err.Error(), "a@b.co") {
		t.Fatalf("错误应列出用户名，实际: %v", err)
	}
	if !strings.Contains(err.Error(), "user id ["+strconv.Itoa(bad.Id)+"]") {
		t.Fatalf("错误应列出 user id %d，实际: %v", bad.Id, err)
	}

	if err := testDB.Model(&User{}).Where("id = ?", bad.Id).Update("username", "renamed").Error; err != nil {
		t.Fatalf("改名失败: %v", err)
	}
	if err := migrateUserNameWithoutAt().Migrate(testDB); err != nil {
		t.Fatalf("改名后迁移应通过，实际: %v", err)
	}
}

// 软删除账号无法登录、也无法在管理端改名，纳入只会造成无法解除的启动失败，故不检查。
func TestMigrateUserNameWithoutAtSkipsSoftDeleted(t *testing.T) {
	testDB := setupUsernameTestDB(t)
	gone := newEmailTestUser("c@d.co", "gone@example.com")
	if err := testDB.Create(gone).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	if err := testDB.Delete(&User{}, gone.Id).Error; err != nil {
		t.Fatalf("软删除失败: %v", err)
	}

	if err := migrateUserNameWithoutAt().Migrate(testDB); err != nil {
		t.Fatalf("软删除账号不应阻断迁移，实际: %v", err)
	}
}
