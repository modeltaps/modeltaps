package model

import (
	"path/filepath"
	"testing"

	"github.com/modeltaps/modeltaps/common"

	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestGetChannelCacheStats(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "cache_stats.db")
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&Log{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldSQLite := DB, common.UsingSQLite
	DB, common.UsingSQLite = testDB, true
	t.Cleanup(func() { DB, common.UsingSQLite = oldDB, oldSQLite })

	logs := []Log{
		{ChannelId: 1, Type: LogTypeConsume, CreatedAt: 100, PromptTokens: 1000,
			Metadata: datatypes.NewJSONType(map[string]any{"session_affinity": "bound"})},
		{ChannelId: 1, Type: LogTypeConsume, CreatedAt: 200, PromptTokens: 1000,
			Metadata: datatypes.NewJSONType(map[string]any{"session_affinity": "hit", "cached_read_tokens": 900, "response_model_mismatch": true})},
		{ChannelId: 2, Type: LogTypeConsume, CreatedAt: 200, PromptTokens: 500,
			Metadata: datatypes.NewJSONType(map[string]any{})},
		{ChannelId: 1, Type: LogTypeConsume, CreatedAt: 9999, PromptTokens: 7,
			Metadata: datatypes.NewJSONType(map[string]any{"session_affinity": "hit"})}, // outside range
	}
	if err := DB.Create(&logs).Error; err != nil {
		t.Fatal(err)
	}

	stats, err := GetChannelCacheStats(1, 1000, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 {
		t.Fatalf("stats = %+v", stats)
	}
	got := *stats[0]
	want := ChannelCacheStat{ChannelId: 1, RequestCount: 2, PromptTokens: 2000, CachedReadTokens: 900, AffinityHits: 1, AffinityBinds: 1, ResponseModelMismatches: 1}
	if got != want {
		t.Fatalf("channel 1 = %+v, want %+v", got, want)
	}
	if stats[1].ChannelId != 2 || stats[1].AffinityHits != 0 || stats[1].CachedReadTokens != 0 {
		t.Fatalf("channel 2 = %+v", *stats[1])
	}

	one, err := GetChannelCacheStats(1, 1000, 2)
	if err != nil || len(one) != 1 || one[0].ChannelId != 2 {
		t.Fatalf("channel filter = %+v, %v", one, err)
	}
}
