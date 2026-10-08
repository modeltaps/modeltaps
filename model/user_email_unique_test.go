package model

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupEmailTestDB 建一个只含 users 表的内存库，并跑一次邮箱唯一约束迁移。
// runMigration=false 用来模拟「升级前的历史库」：唯一索引现在由 User 的 gorm 标签声明、
// AutoMigrate 会直接建出来，所以要显式删掉才能构造重复邮箱等脏数据。
func setupEmailTestDB(t *testing.T, runMigration bool) *gorm.DB {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	// user_oidc_identities：Delete() 会连带清理身份行，缺表会让删除用例直接报错
	if err := testDB.AutoMigrate(&User{}, &Log{}, &UserOidcIdentity{}, &UserSession{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
	if runMigration {
		if err := migrateUserEmailUnique().Migrate(testDB); err != nil {
			t.Fatalf("邮箱唯一约束迁移失败: %v", err)
		}
		return testDB
	}
	if err := testDB.Migrator().DropIndex(&User{}, "idx_users_email_unique"); err != nil {
		t.Fatalf("构造历史库（删唯一索引）失败: %v", err)
	}
	return testDB
}

func newEmailTestUser(username, email string) *User {
	return &User{
		Username:    username,
		Password:    "test-password",
		Email:       NullableEmail(email),
		Role:        config.RoleCommonUser,
		Status:      config.UserStatusEnabled,
		AccessToken: utils.GetUUID(),
		AffCode:     strings.ToLower(utils.GetRandomString(8)),
		CreatedTime: utils.GetTimestamp(),
	}
}

func TestNormalizeEmail(t *testing.T) {
	cases := map[string]string{
		"":                    "",
		"   ":                 "",
		"Foo@X.com":           "foo@x.com",
		"  FOO@X.COM  ":       "foo@x.com",
		"already@lower.email": "already@lower.email",
	}
	for in, want := range cases {
		if got := common.NormalizeEmail(in); got != want {
			t.Fatalf("NormalizeEmail(%q) = %q，期望 %q", in, got, want)
		}
	}
}

// TestEmailUniqueCaseInsensitive 仅大小写不同的邮箱第二次插入必须被友好错误拒绝
func TestEmailUniqueCaseInsensitive(t *testing.T) {
	setupEmailTestDB(t, true)

	first := newEmailTestUser("alice", "Foo@X.com")
	if err := first.Insert(0); err != nil {
		t.Fatalf("首次插入应成功: %v", err)
	}
	if first.Email != "foo@x.com" {
		t.Fatalf("落库邮箱应归一化为小写，实际 %q", first.Email)
	}

	second := newEmailTestUser("bob", "  FOO@X.COM ")
	err := second.Insert(0)
	if err == nil {
		t.Fatal("重复邮箱插入应被拒绝")
	}
	if !strings.Contains(err.Error(), "this email is already in use") {
		t.Fatalf("应返回友好错误，实际: %v", err)
	}
}

// TestEmptyEmailAllowsMultipleUsers 唯一约束下任意多个无邮箱用户可共存（空串落库为 NULL）
func TestEmptyEmailAllowsMultipleUsers(t *testing.T) {
	setupEmailTestDB(t, true)

	for _, name := range []string{"noemail1", "noemail2", "noemail3"} {
		u := newEmailTestUser(name, "")
		if err := u.Insert(0); err != nil {
			t.Fatalf("无邮箱用户 %s 插入应成功: %v", name, err)
		}
	}

	var nullCount int64
	if err := DB.Model(&User{}).Where("email IS NULL").Count(&nullCount).Error; err != nil {
		t.Fatalf("统计 NULL 邮箱失败: %v", err)
	}
	if nullCount != 3 {
		t.Fatalf("空邮箱应以 NULL 落库，NULL 行数 = %d，期望 3", nullCount)
	}
}

// TestGetRootUserEmail 站点邮件通知依赖它取默认收件人，须返回归一化后的非空邮箱
func TestGetRootUserEmail(t *testing.T) {
	setupEmailTestDB(t, true)

	root := newEmailTestUser("root", "Root@X.com")
	root.Role = config.RoleRootUser
	if err := root.Insert(0); err != nil {
		t.Fatalf("插入 root 用户应成功: %v", err)
	}
	// 无邮箱的普通用户不应干扰取值
	if err := newEmailTestUser("plain", "").Insert(0); err != nil {
		t.Fatalf("插入无邮箱用户应成功: %v", err)
	}

	if got := GetRootUserEmail(); got != "root@x.com" {
		t.Fatalf("GetRootUserEmail 应返回 root@x.com，实际: %q", got)
	}
}

// captureWriter 收集 gorm 日志输出，用于断言查询没有打印错误
type captureWriter struct {
	lines []string
}

func (w *captureWriter) Printf(format string, args ...interface{}) {
	w.lines = append(w.lines, fmt.Sprintf(format, args...))
}

// TestGetRootUserEmailWhenNull root 无邮箱时应安全返回空串，且查询本身不报错、不打印错误日志
func TestGetRootUserEmailWhenNull(t *testing.T) {
	testDB := setupEmailTestDB(t, true)

	root := newEmailTestUser("root", "")
	root.Role = config.RoleRootUser
	if err := root.Insert(0); err != nil {
		t.Fatalf("插入 root 用户应成功: %v", err)
	}

	// 不用 Silent 掩盖错误：打开 Error 级别日志并捕获输出
	writer := &captureWriter{}
	testDB.Logger = gormlogger.New(writer, gormlogger.Config{LogLevel: gormlogger.Error})

	got, err := getRootUserEmail()
	if err != nil {
		t.Fatalf("root 无邮箱时查询不应报错: %v", err)
	}
	if got != "" {
		t.Fatalf("root 无邮箱时应返回空串，实际: %q", got)
	}
	if len(writer.lines) != 0 {
		t.Fatalf("root 无邮箱时不应打印错误日志，实际: %v", writer.lines)
	}
	if got := GetRootUserEmail(); got != "" {
		t.Fatalf("root 无邮箱时应返回空串，实际: %q", got)
	}
}

// TestDeleteReleasesEmail 唯一索引同样覆盖软删除行，删除账号后邮箱须可被重新注册
func TestDeleteReleasesEmail(t *testing.T) {
	setupEmailTestDB(t, true)

	first := newEmailTestUser("frank", "reuse@x.com")
	if err := first.Insert(0); err != nil {
		t.Fatalf("首次插入应成功: %v", err)
	}
	if err := first.Delete(); err != nil {
		t.Fatalf("删除用户应成功: %v", err)
	}

	again := newEmailTestUser("grace", "reuse@x.com")
	if err := again.Insert(0); err != nil {
		t.Fatalf("删除后同邮箱应可重新注册: %v", err)
	}
}

// TestEmailMigrationNormalizesAndDetectsConflict 迁移应归一化历史数据，
// 遇到归一化后重复则失败并列出冲突邮箱与 user id
func TestEmailMigrationNormalizesAndDetectsConflict(t *testing.T) {
	db := setupEmailTestDB(t, false)

	mixed := newEmailTestUser("carol", "")
	mixed.Email = " Mixed@Case.COM "
	if err := db.Create(mixed).Error; err != nil {
		t.Fatalf("构造历史数据失败: %v", err)
	}
	if err := migrateUserEmailUnique().Migrate(db); err != nil {
		t.Fatalf("单条数据迁移应成功: %v", err)
	}
	var got NullableEmail
	if err := db.Model(&User{}).Where("id = ?", mixed.Id).Select("email").Scan(&got).Error; err != nil {
		t.Fatalf("读取归一化后邮箱失败: %v", err)
	}
	if got != "mixed@case.com" {
		t.Fatalf("迁移应归一化历史邮箱，实际 %q", got)
	}

	// 重新构造一个含重复邮箱的库，验证 fail-closed
	dup := setupEmailTestDB(t, false)
	a := newEmailTestUser("dave", "")
	a.Email = "Dup@X.com"
	b := newEmailTestUser("erin", "")
	b.Email = "dup@x.com"
	if err := dup.Create(a).Error; err != nil {
		t.Fatalf("构造重复数据失败: %v", err)
	}
	if err := dup.Create(b).Error; err != nil {
		t.Fatalf("构造重复数据失败: %v", err)
	}
	err := migrateUserEmailUnique().Migrate(dup)
	if err == nil {
		t.Fatal("存在重复邮箱时迁移必须失败")
	}
	msg := err.Error()
	if !strings.Contains(msg, "dup@x.com") {
		t.Fatalf("错误信息应列出冲突邮箱，实际: %s", msg)
	}
	for _, id := range []int{a.Id, b.Id} {
		if !strings.Contains(msg, strconv.Itoa(id)) {
			t.Fatalf("错误信息应列出冲突 user id %d，实际: %s", id, msg)
		}
	}
}

// TestEmailUniqueIndexSurvivesAutoMigrate 唯一索引必须由 schema 声明并跨重复 AutoMigrate 存活。
// 回归守护：MySQL driver 的 MigrateColumnUnique 曾把 schema 不认识的 idx_users_email_unique
// 当冗余索引 DROP 掉，导致约束在第二次启动后永久消失。
func TestEmailUniqueIndexSurvivesAutoMigrate(t *testing.T) {
	db := setupEmailTestDB(t, true)

	// 模拟历史库上索引已被删除：AutoMigrate 应按标签重新建出来
	if err := db.Migrator().DropIndex(&User{}, "idx_users_email_unique"); err != nil {
		t.Fatalf("删除唯一索引失败: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := db.AutoMigrate(&User{}); err != nil {
			t.Fatalf("第 %d 次 AutoMigrate 失败: %v", i+1, err)
		}
		if !db.Migrator().HasIndex(&User{}, "idx_users_email_unique") {
			t.Fatalf("第 %d 次 AutoMigrate 后唯一索引应存在", i+1)
		}
	}

	if err := newEmailTestUser("alice", "dup@x.com").Insert(0); err != nil {
		t.Fatalf("首次插入应成功: %v", err)
	}
	if err := newEmailTestUser("bob", "dup@x.com").Insert(0); err == nil {
		t.Fatal("重建索引后重复邮箱插入应被拒绝")
	}
}

// TestEnsureUserEmailUniqueReadyNormalizes 升级路径：历史库里空邮箱是 '' 而非 NULL，
// 直接让 AutoMigrate 建唯一索引会撞多行 '' 而失败。预处理须先归一化，随后建索引成功。
func TestEnsureUserEmailUniqueReadyNormalizes(t *testing.T) {
	db := setupEmailTestDB(t, false)

	for _, row := range [][2]string{{"empty1", ""}, {"empty2", ""}, {"mixed", " Mixed@Case.COM "}} {
		if err := db.Exec("INSERT INTO users (username, password, email, aff_code, access_token) VALUES (?, ?, ?, ?, ?)",
			row[0], "test-password", row[1], utils.GetRandomString(8), utils.GetUUID()).Error; err != nil {
			t.Fatalf("构造历史数据失败: %v", err)
		}
	}

	if err := ensureUserEmailUniqueReady(db); err != nil {
		t.Fatalf("预处理应成功: %v", err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("预处理后 AutoMigrate 应能建出唯一索引: %v", err)
	}
	if !db.Migrator().HasIndex(&User{}, "idx_users_email_unique") {
		t.Fatal("AutoMigrate 后唯一索引应存在")
	}

	var nullCount int64
	if err := db.Model(&User{}).Where("email IS NULL").Count(&nullCount).Error; err != nil {
		t.Fatalf("统计 NULL 邮箱失败: %v", err)
	}
	if nullCount != 2 {
		t.Fatalf("两个空邮箱应归一化为 NULL，实际 %d", nullCount)
	}
	var mixed NullableEmail
	if err := db.Model(&User{}).Where("username = ?", "mixed").Select("email").Scan(&mixed).Error; err != nil {
		t.Fatalf("读取归一化后邮箱失败: %v", err)
	}
	if mixed != "mixed@case.com" {
		t.Fatalf("历史邮箱应被归一化，实际 %q", mixed)
	}
}

// TestEnsureUserEmailUniqueReadyFailsClosed 存在重复邮箱时预处理必须 fail-closed，
// 给出可读的冲突清单，而不是把原始的建索引冲突错误抛给管理员。
func TestEnsureUserEmailUniqueReadyFailsClosed(t *testing.T) {
	db := setupEmailTestDB(t, false)

	a := newEmailTestUser("dave", "")
	a.Email = "Dup@X.com"
	b := newEmailTestUser("erin", "")
	b.Email = "dup@x.com"
	for _, u := range []*User{a, b} {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("构造重复数据失败: %v", err)
		}
	}

	err := ensureUserEmailUniqueReady(db)
	if err == nil {
		t.Fatal("存在重复邮箱时预处理必须失败")
	}
	if !strings.Contains(err.Error(), "dup@x.com") {
		t.Fatalf("错误信息应列出冲突邮箱，实际: %s", err.Error())
	}
	for _, id := range []int{a.Id, b.Id} {
		if !strings.Contains(err.Error(), strconv.Itoa(id)) {
			t.Fatalf("错误信息应列出冲突 user id %d，实际: %s", id, err.Error())
		}
	}
}

// TestEnsureUserEmailUniqueReadySkipsWhenIndexExists 索引已存在时预处理整体跳过，
// 不做全表 UPDATE（否则每次启动都要重写全表）。
func TestEnsureUserEmailUniqueReadySkipsWhenIndexExists(t *testing.T) {
	db := setupEmailTestDB(t, true)

	if err := db.Exec("INSERT INTO users (username, password, email, aff_code, access_token) VALUES (?, ?, ?, ?, ?)",
		"legacy", "test-password", " Keep@Case.COM ", utils.GetRandomString(8), utils.GetUUID()).Error; err != nil {
		t.Fatalf("构造数据失败: %v", err)
	}
	if err := ensureUserEmailUniqueReady(db); err != nil {
		t.Fatalf("预处理应成功: %v", err)
	}
	var got NullableEmail
	if err := db.Model(&User{}).Where("username = ?", "legacy").Select("email").Scan(&got).Error; err != nil {
		t.Fatalf("读取邮箱失败: %v", err)
	}
	if got != " Keep@Case.COM " {
		t.Fatalf("索引已存在时不应改写数据，实际 %q", got)
	}
}

// seedRawUser 绕过 gorm 的软删过滤直接造行，deletedAt 非空即为已软删账号。
func seedRawUser(t *testing.T, db *gorm.DB, username, email string, deletedAt *time.Time) {
	t.Helper()
	if err := db.Exec("INSERT INTO users (username, password, email, aff_code, access_token, deleted_at) VALUES (?, ?, ?, ?, ?, ?)",
		username, "test-password", email, utils.GetRandomString(8), utils.GetUUID(), deletedAt).Error; err != nil {
		t.Fatalf("构造用户 %s 失败: %v", username, err)
	}
}

// rawUserEmail 读取指定用户的 email 原值（含软删行），未取到时 ok 为 false。
func rawUserEmail(t *testing.T, db *gorm.DB, username string) (email sql.NullString, ok bool) {
	t.Helper()
	var rows []struct {
		Email sql.NullString `gorm:"column:email"`
	}
	if err := db.Raw("SELECT email FROM users WHERE username = ?", username).Scan(&rows).Error; err != nil {
		t.Fatalf("读取用户 %s 的邮箱失败: %v", username, err)
	}
	if len(rows) != 1 {
		return sql.NullString{}, false
	}
	return rows[0].Email, true
}

// TestNormalizeUserEmailsClearsSoftDeleted 历史软删账号占着的邮箱须在归一化时清空，
// 与 User.Delete() 对齐；「软删 + 活跃同邮箱」因此不再被判为冲突。
func TestNormalizeUserEmailsClearsSoftDeleted(t *testing.T) {
	db := setupEmailTestDB(t, false)
	deletedAt := time.Now()

	seedRawUser(t, db, "gone", "Foo@X.com", &deletedAt)
	seedRawUser(t, db, "alive", "foo@x.com", nil)

	if err := normalizeUserEmails(db); err != nil {
		t.Fatalf("归一化应成功: %v", err)
	}
	if got, ok := rawUserEmail(t, db, "gone"); !ok || got.Valid {
		t.Fatalf("软删用户邮箱应为 NULL，实际 ok=%v value=%+v", ok, got)
	}
	if got, ok := rawUserEmail(t, db, "alive"); !ok || got.String != "foo@x.com" {
		t.Fatalf("活跃用户邮箱应保留，实际 ok=%v value=%+v", ok, got)
	}
	if err := detectDuplicateUserEmails(db); err != nil {
		t.Fatalf("清空软删邮箱后不应再有冲突: %v", err)
	}
}

// TestNormalizeUserEmailsSoftDeletedOnly 只有软删账号持有邮箱时同样清空
func TestNormalizeUserEmailsSoftDeletedOnly(t *testing.T) {
	db := setupEmailTestDB(t, false)
	deletedAt := time.Now()

	seedRawUser(t, db, "gone", "Solo@X.com", &deletedAt)

	if err := normalizeUserEmails(db); err != nil {
		t.Fatalf("归一化应成功: %v", err)
	}
	if got, ok := rawUserEmail(t, db, "gone"); !ok || got.Valid {
		t.Fatalf("软删用户邮箱应为 NULL，实际 ok=%v value=%+v", ok, got)
	}
}

// TestNormalizeUserEmailsKeepsActive 活跃用户的归一化行为不受清空软删邮箱影响
func TestNormalizeUserEmailsKeepsActive(t *testing.T) {
	db := setupEmailTestDB(t, false)

	seedRawUser(t, db, "mixed", " Mixed@Case.COM ", nil)
	seedRawUser(t, db, "blank", "", nil)

	if err := normalizeUserEmails(db); err != nil {
		t.Fatalf("归一化应成功: %v", err)
	}
	if got, ok := rawUserEmail(t, db, "mixed"); !ok || got.String != "mixed@case.com" {
		t.Fatalf("活跃用户邮箱应被归一化，实际 ok=%v value=%+v", ok, got)
	}
	if got, ok := rawUserEmail(t, db, "blank"); !ok || got.Valid {
		t.Fatalf("空邮箱应写为 NULL，实际 ok=%v value=%+v", ok, got)
	}
}
