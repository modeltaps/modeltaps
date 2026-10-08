package model

import (
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupPhoneTestDB 建一个只含 users 表的内存库。runMigration=false 用来模拟「升级前的历史库」：
// 唯一索引由 User 的 gorm 标签声明、AutoMigrate 会直接建出来，须显式删掉才能构造重复手机号。
func setupPhoneTestDB(t *testing.T, runMigration bool) *gorm.DB {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&User{}, &Log{}, &UserOidcIdentity{}, &UserSession{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
	if runMigration {
		if err := migrateUserPhoneUnique().Migrate(testDB); err != nil {
			t.Fatalf("手机号唯一约束迁移失败: %v", err)
		}
		return testDB
	}
	if err := testDB.Migrator().DropIndex(&User{}, "idx_users_phone_unique"); err != nil {
		t.Fatalf("构造历史库（删唯一索引）失败: %v", err)
	}
	return testDB
}

func newPhoneTestUser(username, phone string) *User {
	return &User{
		Username:    username,
		Password:    "test-password",
		PhoneNumber: NullablePhone(phone),
		Role:        config.RoleCommonUser,
		Status:      config.UserStatusEnabled,
		AccessToken: utils.GetUUID(),
		AffCode:     strings.ToLower(utils.GetRandomString(8)),
		CreatedTime: utils.GetTimestamp(),
	}
}

// 空手机号必须以 NULL 落库并原样读回空串，否则多个无手机号用户会撞唯一索引。
func TestNullablePhoneEmptyRoundTrip(t *testing.T) {
	db := setupPhoneTestDB(t, true)

	for _, name := range []string{"nophone1", "nophone2", "nophone3"} {
		if err := db.Create(newPhoneTestUser(name, "")).Error; err != nil {
			t.Fatalf("无手机号用户 %s 插入应成功: %v", name, err)
		}
	}

	var nullCount int64
	if err := db.Model(&User{}).Where("phone_number IS NULL").Count(&nullCount).Error; err != nil {
		t.Fatalf("统计 NULL 手机号失败: %v", err)
	}
	if nullCount != 3 {
		t.Fatalf("空手机号应以 NULL 落库，NULL 行数 = %d，期望 3", nullCount)
	}

	var got User
	if err := db.First(&got, "username = ?", "nophone1").Error; err != nil {
		t.Fatalf("读取用户失败: %v", err)
	}
	if got.PhoneNumber != "" {
		t.Fatalf("NULL 应读回空串，实际 %q", got.PhoneNumber)
	}
}

// 同一手机号只能归属一个账号：第二次插入被唯一索引拒绝。
func TestPhoneUniqueRejectsDuplicate(t *testing.T) {
	db := setupPhoneTestDB(t, true)

	if err := db.Create(newPhoneTestUser("owner", "+85251234567")).Error; err != nil {
		t.Fatalf("首次插入应成功: %v", err)
	}
	if err := db.Create(newPhoneTestUser("taker", "+85251234567")).Error; err == nil {
		t.Fatal("重复手机号插入应被唯一索引拒绝")
	}
	if !IsPhoneAlreadyTaken("+85251234567") {
		t.Fatal("IsPhoneAlreadyTaken 应判定该号码已被占用")
	}
	if IsPhoneAlreadyTaken("") {
		t.Fatal("空手机号不应算被占用")
	}
}

// 升级路径：历史库里空手机号是 ''、且可能存在重复号码（此前它只是展示字段）。
// 预处理须把 '' 归一为 NULL、每组重复保留 id 最小的账号，并且不阻断启动。
func TestEnsureUserPhoneUniqueReadyNormalizesAndDedupes(t *testing.T) {
	db := setupPhoneTestDB(t, false)

	for _, row := range [][2]string{{"empty1", ""}, {"empty2", ""}, {"dup1", "+85251234567"}, {"dup2", " +85251234567 "}} {
		if err := db.Exec("INSERT INTO users (username, password, phone_number, aff_code, access_token) VALUES (?, ?, ?, ?, ?)",
			row[0], "test-password", row[1], utils.GetRandomString(8), utils.GetUUID()).Error; err != nil {
			t.Fatalf("构造历史数据失败: %v", err)
		}
	}

	if err := ensureUserPhoneUniqueReady(db); err != nil {
		t.Fatalf("预处理不应阻断启动: %v", err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("预处理后 AutoMigrate 应能建出唯一索引: %v", err)
	}
	if !db.Migrator().HasIndex(&User{}, "idx_users_phone_unique") {
		t.Fatal("AutoMigrate 后唯一索引应存在")
	}

	var kept User
	if err := db.First(&kept, "username = ?", "dup1").Error; err != nil {
		t.Fatalf("读取用户失败: %v", err)
	}
	if kept.PhoneNumber != "+85251234567" {
		t.Fatalf("重复组中 id 最小的账号应保留手机号，实际 %q", kept.PhoneNumber)
	}
	var cleared User
	if err := db.First(&cleared, "username = ?", "dup2").Error; err != nil {
		t.Fatalf("读取用户失败: %v", err)
	}
	if cleared.PhoneNumber != "" {
		t.Fatalf("重复组中的其余账号手机号应被清空，实际 %q", cleared.PhoneNumber)
	}
}

// 软删账号占用的手机号必须被释放，否则该号码永远无法再关联到新账号。
func TestDeleteReleasesPhoneNumber(t *testing.T) {
	db := setupPhoneTestDB(t, true)

	user := newPhoneTestUser("leaver", "+85251234567")
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("插入应成功: %v", err)
	}
	if err := user.Delete(); err != nil {
		t.Fatalf("软删应成功: %v", err)
	}
	if IsPhoneAlreadyTaken("+85251234567") {
		t.Fatal("软删后手机号应被释放")
	}
	if err := db.Create(newPhoneTestUser("newcomer", "+85251234567")).Error; err != nil {
		t.Fatalf("释放后同号码应可再次使用: %v", err)
	}
}
