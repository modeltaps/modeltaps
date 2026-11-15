package model

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func useUpstreamCostTestDB(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "upstream_costs.db")
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
}

func TestGetChannelUpstreamCosts(t *testing.T) {
	useUpstreamCostTestDB(t)
	logs := []Log{
		{ChannelId: 1, ModelName: "a", Type: LogTypeConsume, CreatedAt: 100, UpstreamCost: 1.2, ExpectedCost: 1},
		{ChannelId: 1, ModelName: "a", Type: LogTypeConsume, CreatedAt: 200, UpstreamCost: 0.6, ExpectedCost: 0.5},
		{ChannelId: 1, ModelName: "b", Type: LogTypeConsume, CreatedAt: 200, UpstreamCost: 0.1, ExpectedCost: 0.1},
		{ChannelId: 2, ModelName: "a", Type: LogTypeConsume, CreatedAt: 200, UpstreamCost: 5, ExpectedCost: 4},
		{ChannelId: 1, ModelName: "a", Type: LogTypeConsume, CreatedAt: 200},                                    // the upstream reported nothing
		{ChannelId: 1, ModelName: "a", Type: LogTypeConsume, CreatedAt: 9999, UpstreamCost: 9, ExpectedCost: 9}, // outside the range
	}
	if err := DB.Create(&logs).Error; err != nil {
		t.Fatal(err)
	}

	rows, err := GetChannelUpstreamCosts(1, 1000, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	a := *rows[0]
	if a.ModelName != "a" || a.RequestCount != 2 || a.UpstreamCost < 1.79 || a.UpstreamCost > 1.81 || a.ExpectedCost != 1.5 {
		t.Errorf("model a = %+v", a)
	}
	if b := *rows[1]; b.ModelName != "b" || b.RequestCount != 1 {
		t.Errorf("model b = %+v", b)
	}

	all, err := GetChannelUpstreamCosts(1, 1000, 0)
	if err != nil || len(all) != 3 || all[0].ChannelId != 2 {
		t.Fatalf("all channels = %+v, %v", all, err)
	}
}

// The costs become columns and leave the metadata, which users can read.
func TestRecordConsumeLogStoresUpstreamCost(t *testing.T) {
	useUpstreamCostTestDB(t)
	oldEnabled, oldBatch := config.LogConsumeEnabled, config.BatchUpdateEnabled
	config.LogConsumeEnabled, config.BatchUpdateEnabled = true, false
	t.Cleanup(func() { config.LogConsumeEnabled, config.BatchUpdateEnabled = oldEnabled, oldBatch })

	meta := map[string]any{"group_name": "default", LogMetaUpstreamCost: 0.0012, LogMetaExpectedCost: 0.001}
	RecordConsumeLog(context.Background(), 1, 7, 10, 5, "a", "token", 600, 0, "", 1, false, meta, "", nil)
	RecordConsumeLog(context.Background(), 1, 7, 10, 5, "a", "token", 600, 0, "", 1, false, map[string]any{"group_name": "default"}, "", nil)

	var logs []Log
	if err := DB.Order("id").Find(&logs).Error; err != nil || len(logs) != 2 {
		t.Fatalf("logs = %d, %v", len(logs), err)
	}
	if logs[0].UpstreamCost != 0.0012 || logs[0].ExpectedCost != 0.001 {
		t.Errorf("costs = %v / %v", logs[0].UpstreamCost, logs[0].ExpectedCost)
	}
	stored, _ := json.Marshal(logs[0].Metadata.Data())
	if strings.Contains(string(stored), "cost") || !strings.Contains(string(stored), "group_name") {
		t.Errorf("metadata = %s", stored)
	}
	if logs[1].UpstreamCost != 0 || logs[1].ExpectedCost != 0 {
		t.Errorf("a log without an upstream cost has %v / %v", logs[1].UpstreamCost, logs[1].ExpectedCost)
	}
}

// A cost that arrives after the log is written fills the log once, and does not replace a cost
// the response itself carried.
func TestEnrichLogByRequestId(t *testing.T) {
	useUpstreamCostTestDB(t)
	oldEnabled, oldBatch := config.LogConsumeEnabled, config.BatchUpdateEnabled
	config.LogConsumeEnabled, config.BatchUpdateEnabled = true, false
	t.Cleanup(func() { config.LogConsumeEnabled, config.BatchUpdateEnabled = oldEnabled, oldBatch })

	if err := EnrichLogByRequestId("req-1", map[string]any{"or_provider": "x"}, 0.5); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("a missing log: err = %v, want ErrRecordNotFound", err)
	}

	// The estimate alone, as logged for a response without a cost.
	RecordConsumeLog(context.Background(), 1, 7, 10, 5, "a", "token", 600, 0, "", 1, false,
		map[string]any{"request_id": "req-1", LogMetaExpectedCost: 0.4}, "", nil)
	RecordConsumeLog(context.Background(), 1, 7, 10, 5, "a", "token", 600, 0, "", 1, false,
		map[string]any{"request_id": "req-2", LogMetaExpectedCost: 0.4, LogMetaUpstreamCost: 0.45}, "", nil)

	if err := EnrichLogByRequestId("req-1", map[string]any{"or_provider": "x"}, 0.5); err != nil {
		t.Fatal(err)
	}
	if err := EnrichLogByRequestId("req-2", nil, 0.9); err != nil {
		t.Fatal(err)
	}

	var logs []Log
	DB.Order("id").Find(&logs)
	if len(logs) != 2 {
		t.Fatalf("logs = %d", len(logs))
	}
	if logs[0].UpstreamCost != 0.5 || logs[0].ExpectedCost != 0.4 || logs[0].Metadata.Data()["or_provider"] != "x" {
		t.Errorf("filled log = %v / %v, metadata %v", logs[0].UpstreamCost, logs[0].ExpectedCost, logs[0].Metadata.Data())
	}
	if logs[1].UpstreamCost != 0.45 {
		t.Errorf("the cost from the response was replaced: %v", logs[1].UpstreamCost)
	}
}
