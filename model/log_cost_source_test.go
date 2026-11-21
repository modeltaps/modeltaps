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

// An upstream cost that arrives after the log is written becomes the request's cost, with the
// fee the request was logged with: until then the log held an estimate from the price list.
func TestEnrichLogSetsCostFromUpstream(t *testing.T) {
	useUpstreamCostTestDB(t)
	oldEnabled, oldBatch := config.LogConsumeEnabled, config.BatchUpdateEnabled
	config.LogConsumeEnabled, config.BatchUpdateEnabled = true, false
	t.Cleanup(func() { config.LogConsumeEnabled, config.BatchUpdateEnabled = oldEnabled, oldBatch })

	// Estimated from the price list at 2000 quota, on a channel with a 5% fee.
	RecordConsumeLog(context.Background(), 1, 7, 10, 5, "a", "token", 600, 2000, "", 1, false,
		map[string]any{"request_id": "req-fee", LogMetaExpectedCost: 0.004, LogMetaCostSource: CostSourceChannelPrice, LogMetaChannelFeeRate: 0.05}, "", nil)
	// The same on a channel without a fee.
	RecordConsumeLog(context.Background(), 1, 7, 10, 5, "a", "token", 600, 2000, "", 1, false,
		map[string]any{"request_id": "req-plain", LogMetaExpectedCost: 0.004, LogMetaCostSource: CostSourceChannelPrice}, "", nil)
	// The response itself carried the cost: a later figure does not replace it.
	RecordConsumeLog(context.Background(), 1, 7, 10, 5, "a", "token", 600, 2100, "", 1, false,
		map[string]any{"request_id": "req-known", LogMetaUpstreamCost: 0.0042, LogMetaCostSource: CostSourceUpstream}, "", nil)

	for _, id := range []string{"req-fee", "req-plain", "req-known"} {
		if err := EnrichLogByRequestId(id, nil, 0.005); err != nil {
			t.Fatal(err)
		}
	}
	var logs []Log
	if err := DB.Order("id").Find(&logs).Error; err != nil || len(logs) != 3 {
		t.Fatalf("logs = %d, %v", len(logs), err)
	}
	// $0.005 with the 5% fee is 2625 quota.
	if logs[0].CostQuota != 2625 || logs[0].CostSource != CostSourceUpstream || logs[0].UpstreamCost != 0.005 {
		t.Errorf("log with a fee: cost %d from %q, upstream %v; want 2625 from upstream", logs[0].CostQuota, logs[0].CostSource, logs[0].UpstreamCost)
	}
	if logs[1].CostQuota != 2500 || logs[1].CostSource != CostSourceUpstream {
		t.Errorf("log without a fee: cost %d from %q, want 2500 from upstream", logs[1].CostQuota, logs[1].CostSource)
	}
	if logs[2].CostQuota != 2100 || logs[2].UpstreamCost != 0.0042 {
		t.Errorf("a cost from the response was replaced: cost %d, upstream %v", logs[2].CostQuota, logs[2].UpstreamCost)
	}

	// The fee rate stays in the metadata for admins and is hidden from members.
	if logs[0].Metadata.Data()[LogMetaChannelFeeRate] != 0.05 {
		t.Errorf("fee rate in the metadata = %v", logs[0].Metadata.Data()[LogMetaChannelFeeRate])
	}
	logs[0].RedactForMember()
	if _, ok := logs[0].Metadata.Data()[LogMetaChannelFeeRate]; ok {
		t.Error("a member can read the channel's fee rate")
	}
}
