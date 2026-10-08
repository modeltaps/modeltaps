package model

import (
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupSlugTestDB 建一个只含 model_owned_by 表的内存库。runMigration=false 用来模拟
// 「升级前的历史库」：唯一索引由 ModelOwnedBy 的 gorm 标签声明、AutoMigrate 会直接建出来，
// 须显式删掉才能构造重复 slug。
func setupSlugTestDB(t *testing.T, runMigration bool) *gorm.DB {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&ModelOwnedBy{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB, oldInstance := DB, ModelOwnedBysInstance
	DB = testDB
	ModelOwnedBysInstance = &ModelOwnedBys{}
	t.Cleanup(func() { DB, ModelOwnedBysInstance = oldDB, oldInstance })
	if runMigration {
		if err := migrateModelOwnedBySlugUnique().Migrate(testDB); err != nil {
			t.Fatalf("slug 唯一约束迁移失败: %v", err)
		}
		return testDB
	}
	if err := testDB.Migrator().DropIndex(&ModelOwnedBy{}, "idx_model_owned_by_slug"); err != nil {
		t.Fatalf("构造历史库（删唯一索引）失败: %v", err)
	}
	return testDB
}

// 空 slug 必须以 NULL 落库并原样读回空串，否则多个「无公开标识」的厂商会撞唯一索引。
func TestNullableSlugEmptyRoundTrip(t *testing.T) {
	db := setupSlugTestDB(t, true)

	for id, name := range map[int]string{1001: "A", 1002: "B", 1003: "C"} {
		if err := db.Create(&ModelOwnedBy{Id: id, Name: name}).Error; err != nil {
			t.Fatalf("无 slug 厂商 %s 插入应成功: %v", name, err)
		}
	}

	var nullCount int64
	if err := db.Model(&ModelOwnedBy{}).Where("slug IS NULL").Count(&nullCount).Error; err != nil {
		t.Fatalf("统计 NULL slug 失败: %v", err)
	}
	if nullCount != 3 {
		t.Fatalf("空 slug 应以 NULL 落库，NULL 行数 = %d，期望 3", nullCount)
	}

	var got ModelOwnedBy
	if err := db.First(&got, "id = ?", 1001).Error; err != nil {
		t.Fatalf("读取厂商失败: %v", err)
	}
	if got.Slug != "" {
		t.Fatalf("NULL 应读回空串，实际 %q", got.Slug)
	}
}

// 同一 slug 只能归属一个厂商：写入冲突由库上的唯一索引判定，并翻译为 ErrVendorSlugTaken。
func TestModelOwnedBySlugUniqueRejectsDuplicate(t *testing.T) {
	setupSlugTestDB(t, true)

	if err := CreateModelOwnedBy(&ModelOwnedBy{Id: 1001, Name: "OpenAI", Slug: "openai"}); err != nil {
		t.Fatalf("首次插入应成功: %v", err)
	}
	err := CreateModelOwnedBy(&ModelOwnedBy{Id: 1002, Name: "Fake", Slug: "openai"})
	if !errors.Is(err, ErrVendorSlugTaken) {
		t.Fatalf("重复 slug 应收敛为 ErrVendorSlugTaken, got %v", err)
	}
	// 内存快照为空也要拦住：权威判据是数据库约束，不是快照。
	if id := ModelOwnedBysInstance.GetIdBySlug("openai"); id != 1001 {
		t.Fatalf("GetIdBySlug(openai) = %d, want 1001", id)
	}
}

// Migrate → Rollback → Migrate 往返：回滚删掉唯一索引、重跑迁移再把它建回来，
// 保证回滚后能原地再次升级，而不是把库卡在半路。
func TestMigrateModelOwnedBySlugUniqueRoundTrip(t *testing.T) {
	db := setupSlugTestDB(t, false)

	migration := migrateModelOwnedBySlugUnique()
	if err := migration.Migrate(db); err != nil {
		t.Fatalf("首次迁移失败: %v", err)
	}
	if !db.Migrator().HasIndex(&ModelOwnedBy{}, "idx_model_owned_by_slug") {
		t.Fatal("迁移后唯一索引应存在")
	}

	if err := migration.Rollback(db); err != nil {
		t.Fatalf("回滚失败: %v", err)
	}
	if db.Migrator().HasIndex(&ModelOwnedBy{}, "idx_model_owned_by_slug") {
		t.Fatal("回滚后唯一索引应被删除")
	}
	// 回滚后重复 slug 可以写入，正是「约束确实不在了」的证据。
	for _, id := range []int{1001, 1002} {
		if err := db.Exec("INSERT INTO model_owned_by (id, name, icon, slug) VALUES (?, ?, ?, ?)",
			id, "vendor", "", "openai").Error; err != nil {
			t.Fatalf("回滚后插入重复 slug 应成功: %v", err)
		}
	}

	if err := migration.Migrate(db); err != nil {
		t.Fatalf("回滚后重跑迁移失败: %v", err)
	}
	if !db.Migrator().HasIndex(&ModelOwnedBy{}, "idx_model_owned_by_slug") {
		t.Fatal("重跑迁移后唯一索引应恢复")
	}
	// 重跑会先去重：重复组里 id 最小的保留原 slug。
	var kept ModelOwnedBy
	if err := db.First(&kept, "id = ?", 1001).Error; err != nil {
		t.Fatalf("读取厂商失败: %v", err)
	}
	if kept.Slug != "openai" {
		t.Fatalf("重跑迁移后 id 最小的厂商应保留 slug，实际 %q", kept.Slug)
	}
	// 回滚是幂等的：索引已经不在时再回滚一次也不应报错。
	if err := migration.Rollback(db); err != nil {
		t.Fatalf("再次回滚失败: %v", err)
	}
	if err := migration.Rollback(db); err != nil {
		t.Fatalf("索引已删除时回滚应为空操作: %v", err)
	}
}

// 升级路径：历史库里空 slug 是 ''、且可能重复（此前没有唯一约束）。
// 预处理须把 '' 归一为 NULL、每组重复保留 id 最小的厂商，并且不阻断启动。
func TestEnsureModelOwnedBySlugUniqueReadyNormalizesAndDedupes(t *testing.T) {
	db := setupSlugTestDB(t, false)

	rows := []struct {
		id   int
		slug string
	}{{1001, ""}, {1002, ""}, {1003, "openai"}, {1004, " OpenAI "}}
	for _, row := range rows {
		if err := db.Exec("INSERT INTO model_owned_by (id, name, icon, slug) VALUES (?, ?, ?, ?)",
			row.id, "vendor", "", row.slug).Error; err != nil {
			t.Fatalf("构造历史数据失败: %v", err)
		}
	}

	if err := ensureModelOwnedBySlugUniqueReady(db); err != nil {
		t.Fatalf("预处理不应阻断启动: %v", err)
	}
	if err := db.AutoMigrate(&ModelOwnedBy{}); err != nil {
		t.Fatalf("预处理后 AutoMigrate 应能建出唯一索引: %v", err)
	}
	if !db.Migrator().HasIndex(&ModelOwnedBy{}, "idx_model_owned_by_slug") {
		t.Fatal("AutoMigrate 后唯一索引应存在")
	}

	var kept ModelOwnedBy
	if err := db.First(&kept, "id = ?", 1003).Error; err != nil {
		t.Fatalf("读取厂商失败: %v", err)
	}
	if kept.Slug != "openai" {
		t.Fatalf("重复组中 id 最小的厂商应保留 slug，实际 %q", kept.Slug)
	}
	var renamed ModelOwnedBy
	if err := db.First(&renamed, "id = ?", 1004).Error; err != nil {
		t.Fatalf("读取厂商失败: %v", err)
	}
	if renamed.Slug != "openai-1004" {
		t.Fatalf("重复组中其余厂商的 slug 应追加 -{id} 后缀，实际 %q", renamed.Slug)
	}
}
