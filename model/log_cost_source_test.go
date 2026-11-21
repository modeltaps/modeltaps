package model

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
)

// The cost source becomes a column and leaves the metadata, which users can read; a log without
// one keeps the column empty.
func TestRecordConsumeLogStoresCostSource(t *testing.T) {
	useUpstreamCostTestDB(t)
	oldEnabled, oldBatch := config.LogConsumeEnabled, config.BatchUpdateEnabled
	config.LogConsumeEnabled, config.BatchUpdateEnabled = true, false
	t.Cleanup(func() { config.LogConsumeEnabled, config.BatchUpdateEnabled = oldEnabled, oldBatch })

	RecordConsumeLog(context.Background(), 1, 7, 10, 5, "a", "token", 600, 400, "", 1, false,
		map[string]any{"group_name": "default", LogMetaCostSource: CostSourceChannelPrice}, "", nil)
	RecordConsumeLog(context.Background(), 1, 7, 10, 5, "a", "token", 600, 0, "", 1, false,
		map[string]any{"group_name": "default"}, "", nil)

	var logs []Log
	if err := DB.Order("id").Find(&logs).Error; err != nil || len(logs) != 2 {
		t.Fatalf("logs = %d, %v", len(logs), err)
	}
	if logs[0].CostSource != CostSourceChannelPrice {
		t.Errorf("cost source = %q, want %q", logs[0].CostSource, CostSourceChannelPrice)
	}
	if stored, _ := json.Marshal(logs[0].Metadata.Data()); strings.Contains(string(stored), "cost_source") {
		t.Errorf("metadata = %s", stored)
	}
	if logs[1].CostSource != "" {
		t.Errorf("a log without a cost source has %q", logs[1].CostSource)
	}

	logs[0].RedactForMember()
	if logs[0].CostSource != "" {
		t.Errorf("a member can read the cost source %q", logs[0].CostSource)
	}
}

// The profit per minute counts the requests whose cost is known. What the others were charged is
// reported apart: counting their unknown cost as zero would show all of it as profit.
func TestRpmTpmStatisticsLeavesOutUnknownCost(t *testing.T) {
	useUpstreamCostTestDB(t)
	now := time.Now().Unix()
	logs := []Log{
		// Charged 1000, cost 600.
		{UserId: 1, Type: LogTypeConsume, CreatedAt: now, Quota: 1000, CostQuota: 600, CostSource: CostSourceChannelPrice},
		// Charged 300 on an upstream that lists the model as free: a known cost of nothing.
		{UserId: 1, Type: LogTypeConsume, CreatedAt: now, Quota: 300, CostQuota: 0, CostSource: CostSourceChannelPrice},
		// Charged 500 on a channel whose cost is not known.
		{UserId: 1, Type: LogTypeConsume, CreatedAt: now, Quota: 500, CostQuota: 0},
		// Too old for the last minute.
		{UserId: 1, Type: LogTypeConsume, CreatedAt: now - 600, Quota: 9000, CostQuota: 1, CostSource: CostSourceOfficial},
	}
	if err := DB.Create(&logs).Error; err != nil {
		t.Fatal(err)
	}

	stats, err := GetRpmTpmStatistics(0)
	if err != nil {
		t.Fatal(err)
	}
	usd := func(quota int) float64 { return float64(quota) / float64(config.QuotaPerUnit) }
	near := func(got, want float64) bool { return math.Abs(got-want) < 1e-9 }
	if stats.RPM != 3 || !near(stats.CPM, usd(1800)) {
		t.Errorf("rpm %d, cpm %v, want 3 requests charged %v", stats.RPM, stats.CPM, usd(1800))
	}
	if !near(stats.PPM, usd(700)) {
		t.Errorf("ppm = %v, want %v (400 + 300, without the request of unknown cost)", stats.PPM, usd(700))
	}
	if !near(stats.CostUnknownCPM, usd(500)) {
		t.Errorf("cost unknown cpm = %v, want %v", stats.CostUnknownCPM, usd(500))
	}
}
