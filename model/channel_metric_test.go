package model

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupMetricDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "metric.db")), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&ChannelMetric{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldSQLite, oldOn := DB, common.UsingSQLite, config.ChannelMetricsEnabled
	DB, common.UsingSQLite, config.ChannelMetricsEnabled = db, true, true
	t.Cleanup(func() { DB, common.UsingSQLite, config.ChannelMetricsEnabled = oldDB, oldSQLite, oldOn })
	channelMetricPending = map[channelMetricKey]*ChannelMetric{}
}

func TestChannelMetricsFlushAccumulates(t *testing.T) {
	setupMetricDB(t)
	RecordChannelSuccess(1, "m", 2000, 500, 300) // 300 tokens in 1.5s
	RecordChannelSuccess(1, "m", 1000, 0, 0)     // non-stream, no ttft
	RecordChannelFailure(1, "m")
	FlushChannelMetrics()
	// a second flush adds to the existing row instead of inserting another one
	RecordChannelFailure(1, "m")
	RecordChannelSuccess(2, "m", 100, 0, 0)
	FlushChannelMetrics()

	now := time.Now().Unix()
	rows, err := GetChannelMetrics(now-3600, now, 1, "")
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	r := rows[0]
	if r.RequestCount != 4 || r.SuccessCount != 2 || r.LatencyMsSum != 3000 || r.TtftMsSum != 500 || r.TtftCount != 1 ||
		r.OutputTokens != 300 || r.GenerationMs != 1500 {
		t.Fatalf("row = %+v", *r)
	}
	if all, _ := GetChannelMetrics(now-3600, now, 0, ""); len(all) != 2 {
		t.Fatalf("all channels = %d rows", len(all))
	}

	config.ChannelMetricsEnabled = false
	RecordChannelFailure(1, "m")
	if len(channelMetricPending) != 0 {
		t.Fatal("recorded while disabled")
	}
}

func TestCleanupOldChannelMetrics(t *testing.T) {
	setupMetricDB(t)
	old := time.Now().AddDate(0, 0, -40).Unix()
	if err := DB.Create(&ChannelMetric{BucketTs: old, ChannelId: 1, ModelName: "m", RequestCount: 1}).Error; err != nil {
		t.Fatal(err)
	}
	RecordChannelSuccess(1, "m", 10, 0, 0)
	FlushChannelMetrics()
	oldDays := config.ChannelMetricsRetentionDays
	config.ChannelMetricsRetentionDays = 30
	t.Cleanup(func() { config.ChannelMetricsRetentionDays = oldDays })
	if n, err := CleanupOldChannelMetrics(); err != nil || n != 1 {
		t.Fatalf("deleted %d err %v", n, err)
	}
}
