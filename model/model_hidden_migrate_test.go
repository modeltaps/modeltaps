package model

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupHiddenMigrateDB 建只含 model_info / 目录审计的库，模拟存量部署升级。
func setupHiddenMigrateDB(t *testing.T, withAudit bool) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "hidden_migrate.db") + "?_busy_timeout=5000&_journal_mode=WAL"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	tables := []any{&ModelInfo{}}
	if withAudit {
		tables = append(tables, &ModelCatalogAuditLog{})
	}
	if err := db.AutoMigrate(tables...); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	return db
}

func seedAuditSnapshot(t *testing.T, snapshot legacySeedAuditSnapshot) string {
	t.Helper()
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// 之前被明确下架过的行保持隐藏，其余一律可见；仅因新行默认未发布的行不算下架。重复执行结果不变。
func TestMigrateModelInfoHidden(t *testing.T) {
	db := setupHiddenMigrateDB(t, true)
	for _, name := range []string{
		"never-audited", "unpublished", "republished", "seed-unpublished",
		"seed-then-published", "unpublished-then-seed-published", "dry-run-only",
	} {
		if err := db.Create(&ModelInfo{Model: name}).Error; err != nil {
			t.Fatalf("预置目录行失败: %v", err)
		}
	}
	logs := []*ModelCatalogAuditLog{
		{Action: ModelCatalogActionLegacyUnpublish, Model: "unpublished", CreatedTime: 1},
		{Action: ModelCatalogActionLegacyUnpublish, Model: "republished", CreatedTime: 2},
		{Action: ModelCatalogActionLegacyPublish, Model: "republished", CreatedTime: 3},
		{Action: ModelCatalogActionLegacyUnpublish, Model: "unpublished-then-seed-published", CreatedTime: 4},
		{Action: ModelCatalogActionApplySeed, CreatedTime: 5, After: seedAuditSnapshot(t, legacySeedAuditSnapshot{
			UnpublishedRows: []string{"seed-unpublished", "seed-then-published"},
			PublishedModels: []string{"unpublished-then-seed-published"},
		})},
		{Action: ModelCatalogActionApplySeed, CreatedTime: 6, After: seedAuditSnapshot(t, legacySeedAuditSnapshot{
			DryRun:          true,
			UnpublishedRows: []string{"dry-run-only"},
		})},
		{Action: ModelCatalogActionLegacyPublish, Model: "seed-then-published", CreatedTime: 7},
	}
	for _, log := range logs {
		if err := db.Create(log).Error; err != nil {
			t.Fatalf("预置审计失败: %v", err)
		}
	}

	want := map[string]bool{
		"never-audited":                   false,
		"unpublished":                     true,
		"republished":                     false,
		"seed-unpublished":                true,
		"seed-then-published":             false,
		"unpublished-then-seed-published": false,
		"dry-run-only":                    false,
	}
	migration := migrateModelInfoHidden()
	for round := 1; round <= 2; round++ {
		if err := migration.Migrate(db); err != nil {
			t.Fatalf("第 %d 次迁移失败: %v", round, err)
		}
		for name, hidden := range want {
			var got ModelInfo
			if err := db.Where("model = ?", name).First(&got).Error; err != nil {
				t.Fatalf("读取 %s 失败: %v", name, err)
			}
			if got.Hidden != hidden {
				t.Fatalf("第 %d 次迁移后 %s.hidden = %v, want %v", round, name, got.Hidden, hidden)
			}
		}
	}
}

// 没有审计表（极早期库）时所有行一律可见，不报错。
func TestMigrateModelInfoHiddenWithoutAuditTable(t *testing.T) {
	db := setupHiddenMigrateDB(t, false)
	if err := db.Create(&ModelInfo{Model: "gpt-5", Hidden: true}).Error; err != nil {
		t.Fatalf("预置目录行失败: %v", err)
	}
	if err := migrateModelInfoHidden().Migrate(db); err != nil {
		t.Fatalf("无审计表时迁移失败: %v", err)
	}
	var got ModelInfo
	if err := db.Where("model = ?", "gpt-5").First(&got).Error; err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if got.Hidden {
		t.Fatal("无审计记录时不应隐藏任何模型")
	}
}

// 只修正未锁定、非 manual 且仍等于旧版推导的行；锁定 / 管理员手填 / 已被改过的行不动。重复执行结果不变。
func TestMigrateModelInfoAudioSpeechEndpoints(t *testing.T) {
	db := setupHiddenMigrateDB(t, false)
	legacy := `["chat","responses","audio.speech"]`
	rows := []*ModelInfo{
		{Model: "openai/gpt-audio", Endpoints: legacy, InputModalities: `["text","audio"]`, OutputModalities: `["text","audio"]`, Source: ModelInfoSourceOpenRouter},
		{Model: "lyria-3-pro-preview", Endpoints: legacy, InputModalities: `["text","image"]`, OutputModalities: `["text","audio"]`, Source: ModelInfoSourceModelsDev},
		{Model: "gemini-2.5-flash-preview-tts", Endpoints: legacy, InputModalities: `["text"]`, OutputModalities: `["audio"]`, Source: ModelInfoSourceModelsDev},
		{Model: "hexgrad/kokoro-82m", Endpoints: `["audio.speech"]`, InputModalities: `["text"]`, OutputModalities: `["speech"]`, Source: ModelInfoSourceOpenRouter},
		{Model: "locked-audio", Endpoints: legacy, InputModalities: `["text","audio"]`, OutputModalities: `["text","audio"]`, Source: ModelInfoSourceModelsDev, Locked: true},
		{Model: "manual-audio", Endpoints: legacy, InputModalities: `["text","audio"]`, OutputModalities: `["text","audio"]`, Source: ModelInfoSourceManual},
		{Model: "edited-audio", Endpoints: `["chat","audio.speech"]`, InputModalities: `["text","audio"]`, OutputModalities: `["text","audio"]`, Source: ModelInfoSourceModelsDev},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("预置目录行失败: %v", err)
		}
	}
	want := map[string]string{
		"openai/gpt-audio":             `["chat","responses"]`,
		"lyria-3-pro-preview":          `["chat","responses"]`,
		"gemini-2.5-flash-preview-tts": `["audio.speech"]`,
		"hexgrad/kokoro-82m":           `["audio.speech"]`,
		"locked-audio":                 legacy,
		"manual-audio":                 legacy,
		"edited-audio":                 `["chat","audio.speech"]`,
	}
	migration := migrateModelInfoAudioSpeechEndpoints()
	for round := 1; round <= 2; round++ {
		if err := migration.Migrate(db); err != nil {
			t.Fatalf("第 %d 次迁移失败: %v", round, err)
		}
		for name, endpoints := range want {
			var got ModelInfo
			if err := db.Where("model = ?", name).First(&got).Error; err != nil {
				t.Fatalf("读取 %s 失败: %v", name, err)
			}
			if got.Endpoints != endpoints {
				t.Fatalf("第 %d 次迁移后 %s.endpoints = %s, want %s", round, name, got.Endpoints, endpoints)
			}
		}
	}
}
