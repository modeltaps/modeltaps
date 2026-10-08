package model

import (
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common"

	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupRootTestDB(t *testing.T) {
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
}

func fetchRootUser(t *testing.T) User {
	t.Helper()
	var user User
	if err := DB.Where("username = ?", "root").First(&user).Error; err != nil {
		t.Fatalf("未找到 root 用户: %v", err)
	}
	return user
}

func TestCreateRootAccountFromEnv(t *testing.T) {
	setupRootTestDB(t)
	viper.Set("root_password", "envpass-12345")
	t.Cleanup(func() { viper.Set("root_password", "") })

	if err := createRootAccountIfNeed(); err != nil {
		t.Fatalf("createRootAccountIfNeed 应成功: %v", err)
	}
	user := fetchRootUser(t)
	if !common.ValidatePasswordAndHash("envpass-12345", user.Password) {
		t.Fatal("root 用户应可用 ROOT_PASSWORD 登录")
	}
	if common.ValidatePasswordAndHash("123456", user.Password) {
		t.Fatal("不得再使用旧默认密码 123456")
	}
}

func TestCreateRootAccountRandom(t *testing.T) {
	setupRootTestDB(t)
	viper.Set("root_password", "")

	if err := createRootAccountIfNeed(); err != nil {
		t.Fatalf("createRootAccountIfNeed 应成功: %v", err)
	}
	user := fetchRootUser(t)
	if common.ValidatePasswordAndHash("123456", user.Password) {
		t.Fatal("随机密码路径不得使用旧默认密码 123456")
	}

	pw, err := generateRootPassword(16)
	if err != nil {
		t.Fatalf("生成随机密码失败: %v", err)
	}
	if len(pw) != 16 {
		t.Fatalf("随机密码长度应为 16，实际 %d", len(pw))
	}
	if len(pw) < 8 || len(pw) > 64 {
		t.Fatalf("随机密码须落在 [8,64] 区间，实际 %d", len(pw))
	}
}

func TestCreateRootAccountEnvLengthInvalid(t *testing.T) {
	longPassword := ""
	for i := 0; i < 65; i++ {
		longPassword += "a"
	}
	cases := []struct {
		name     string
		password string
	}{
		{"tooShort", "short"},
		{"tooLong", longPassword},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setupRootTestDB(t)
			viper.Set("root_password", c.password)
			t.Cleanup(func() { viper.Set("root_password", "") })

			if err := createRootAccountIfNeed(); err == nil {
				t.Fatal("ROOT_PASSWORD 长度不合规应返回 error")
			}
			var count int64
			DB.Model(&User{}).Count(&count)
			if count != 0 {
				t.Fatalf("长度不合规时不应创建用户，实际创建 %d 个", count)
			}
		})
	}
}

func TestCreateRootAccountAllowInsecure(t *testing.T) {
	t.Run("offRejectsShortPassword", func(t *testing.T) {
		setupRootTestDB(t)
		viper.Set("root_password", "root")
		t.Cleanup(func() { viper.Set("root_password", "") })

		err := createRootAccountIfNeed()
		if err == nil {
			t.Fatal("开关关闭时短密码应返回 error")
		}
		if !strings.Contains(err.Error(), "got 4") {
			t.Fatalf("错误信息应包含长度提示，实际: %v", err)
		}
		var count int64
		DB.Model(&User{}).Count(&count)
		if count != 0 {
			t.Fatalf("报错时不应创建用户，实际创建 %d 个", count)
		}
	})

	t.Run("onAcceptsShortPassword", func(t *testing.T) {
		setupRootTestDB(t)
		viper.Set("root_password", "root")
		viper.Set("root_password_allow_insecure", true)
		t.Cleanup(func() {
			viper.Set("root_password", "")
			viper.Set("root_password_allow_insecure", false)
		})

		if err := createRootAccountIfNeed(); err != nil {
			t.Fatalf("开关开启时短密码应成功: %v", err)
		}
		user := fetchRootUser(t)
		if !common.ValidatePasswordAndHash("root", user.Password) {
			t.Fatal("root 用户应可用 root/root 登录")
		}
	})

	t.Run("onKeepsRandomPasswordBranch", func(t *testing.T) {
		setupRootTestDB(t)
		viper.Set("root_password", "")
		viper.Set("root_password_allow_insecure", true)
		t.Cleanup(func() { viper.Set("root_password_allow_insecure", false) })

		if err := createRootAccountIfNeed(); err != nil {
			t.Fatalf("开关开启且未设 ROOT_PASSWORD 时应走随机密码分支: %v", err)
		}
		user := fetchRootUser(t)
		if common.ValidatePasswordAndHash("root", user.Password) {
			t.Fatal("随机密码分支不应使用 root 作为密码")
		}
	})
}

func TestCreateRootAccountExistingUserUntouched(t *testing.T) {
	setupRootTestDB(t)
	existing := User{Username: "alice", Password: "already-hashed"}
	if err := DB.Create(&existing).Error; err != nil {
		t.Fatalf("预置用户失败: %v", err)
	}
	viper.Set("root_password", "envpass-12345")
	t.Cleanup(func() { viper.Set("root_password", "") })

	if err := createRootAccountIfNeed(); err != nil {
		t.Fatalf("已有用户时应直接返回 nil: %v", err)
	}
	var count int64
	DB.Model(&User{}).Count(&count)
	if count != 1 {
		t.Fatalf("已有用户库不应新增 root，用户数应为 1，实际 %d", count)
	}
	var rootCount int64
	DB.Model(&User{}).Where("username = ?", "root").Count(&rootCount)
	if rootCount != 0 {
		t.Fatal("已有用户时不应创建 root 用户")
	}
}
