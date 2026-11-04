package model

import (
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"

	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupRoutingStatsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&Log{}, &LogDetail{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	oldDB, oldUsing, oldPricing := DB, common.UsingSQLite, PricingInstance
	DB, common.UsingSQLite = testDB, true
	PricingInstance = &Pricing{Prices: map[string]*Price{
		"big-model":   {Model: "big-model", Type: TokensPriceType, Input: 10, Output: 20},
		"cheap-model": {Model: "cheap-model", Type: TokensPriceType, Input: 1, Output: 2},
	}, Match: []string{}}
	t.Cleanup(func() { DB, common.UsingSQLite, PricingInstance = oldDB, oldUsing, oldPricing })
	return testDB
}

func routedLog(createdAt int64, modelName string, prompt, completion, quota int, meta map[string]any) *Log {
	return &Log{Type: LogTypeConsume, CreatedAt: createdAt, ModelName: modelName, PromptTokens: prompt, CompletionTokens: completion, Quota: quota, Metadata: datatypes.NewJSONType(meta)}
}

// TestGetRoutingPolicyStats 原因 / 标签分布、兜底数、平均决策耗时、决策成本与按兜底价估算的节省
func TestGetRoutingPolicyStats(t *testing.T) {
	db := setupRoutingStatsTestDB(t)
	now := time.Now().Unix()
	policy := &RoutingPolicy{Name: "auto/t", FallbackModel: "big-model"}
	base := map[string]any{"routing_policy": "auto/t", "group_ratio": 1.0}
	with := func(extra map[string]any) map[string]any {
		m := map[string]any{}
		for k, v := range base {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	logs := []*Log{
		// 两条分类到 cheap：1000 入 / 100 出，cheap 价 1/2 → 1200 每条
		routedLog(now-100, "cheap-model", 1000, 100, 1200, with(map[string]any{"routing_label": "small", "routing_target": "cheap-model", "routing_reason": "classified", "routing_confidence": 0.9, "routing_decision_ms": 120})),
		routedLog(now-90, "cheap-model", 1000, 100, 1200, with(map[string]any{"routing_label": "small", "routing_target": "cheap-model", "routing_reason": "pinned"})),
		// 一条低于阈值走兜底（带标签 large 但目标是兜底）
		routedLog(now-80, "big-model", 500, 50, 6000, with(map[string]any{"routing_label": "large", "routing_target": "big-model", "routing_reason": "below_threshold", "routing_confidence": 0.3, "routing_decision_ms": 80})),
		// 一条决策失败走兜底，没有标签
		routedLog(now-70, "big-model", 500, 50, 6000, with(map[string]any{"routing_target": "big-model", "routing_reason": "decision_error", "routing_decision_ms": 3000})),
		// 决策调用两条
		routedLog(now-100, "decider", 20, 8, 28, map[string]any{"routing_policy": "auto/t", "routing_decision": true}),
		routedLog(now-80, "decider", 20, 8, 28, map[string]any{"routing_policy": "auto/t", "routing_decision": true}),
		// 别的策略与 30 天外的不算
		routedLog(now-60, "cheap-model", 1, 1, 1, map[string]any{"routing_policy": "auto/other", "routing_label": "x", "routing_target": "cheap-model", "routing_reason": "classified"}),
		routedLog(now-40*24*3600, "cheap-model", 1, 1, 1, with(map[string]any{"routing_label": "small", "routing_target": "cheap-model", "routing_reason": "classified"})),
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("写日志失败: %v", err)
	}

	stats, err := GetRoutingPolicyStats(policy, 7)
	if err != nil {
		t.Fatalf("统计失败: %v", err)
	}
	if stats.Requests != 4 {
		t.Fatalf("请求数期望 4，实际 %d", stats.Requests)
	}
	if stats.ByReason["classified"] != 1 || stats.ByReason["pinned"] != 1 || stats.ByReason["below_threshold"] != 1 || stats.ByReason["decision_error"] != 1 {
		t.Fatalf("原因分布不对: %v", stats.ByReason)
	}
	if stats.FallbackRequests != 1 {
		t.Fatalf("无标签的兜底请求应为 1，实际 %d", stats.FallbackRequests)
	}
	if len(stats.Labels) != 3 || stats.Labels[0].Label != "small" || stats.Labels[0].Requests != 2 || stats.Labels[0].Quota != 2400 {
		t.Fatalf("标签分布不对: %+v", stats.Labels)
	}
	// 决策耗时：120、80、3000 → 平均 1066.7
	if stats.AvgDecisionMs != 1066.7 {
		t.Fatalf("平均决策耗时期望 1066.7，实际 %v", stats.AvgDecisionMs)
	}
	if stats.DecisionCalls != 2 || stats.DecisionQuota != 56 {
		t.Fatalf("决策调用期望 2 次 56 额度，实际 %d / %d", stats.DecisionCalls, stats.DecisionQuota)
	}
	// 实际 1200*2 + 6000*2 = 14400；全走兜底：cheap 两条按 big 价 (1000*10+100*20)=12000 各 → 24000，big 两条原样 12000 → 36000
	if stats.ActualQuota != 14400 || stats.FallbackQuota != 36000 {
		t.Fatalf("实际 / 估算费用期望 14400 / 36000，实际 %d / %d", stats.ActualQuota, stats.FallbackQuota)
	}
	if !stats.FallbackPriceKnown || stats.SavedQuota != 36000-14400-56 {
		t.Fatalf("节省期望 %d，实际 %d (known=%v)", 36000-14400-56, stats.SavedQuota, stats.FallbackPriceKnown)
	}

	// 30 天窗口也不含 40 天前的那条
	stats30, err := GetRoutingPolicyStats(policy, 30)
	if err != nil || stats30.Requests != 4 || stats30.Days != 30 {
		t.Fatalf("30 天统计不对: %+v err=%v", stats30, err)
	}

	// 兜底没有价格：不估算
	PricingInstance = &Pricing{Prices: map[string]*Price{}, Match: []string{}}
	noPrice, err := GetRoutingPolicyStats(policy, 7)
	if err != nil || noPrice.FallbackPriceKnown || noPrice.SavedQuota != 0 || noPrice.ActualQuota != 14400 {
		t.Fatalf("无兜底价时不应估算: %+v err=%v", noPrice, err)
	}
}

// TestGetRoutingPolicyLogSamples 只返回留存了请求明细的被路由日志，新的在前，不含决策调用
func TestGetRoutingPolicyLogSamples(t *testing.T) {
	db := setupRoutingStatsTestDB(t)
	now := time.Now().Unix()
	meta := map[string]any{"routing_policy": "auto/t", "routing_label": "small", "routing_target": "cheap-model", "routing_reason": "classified", "routing_confidence": 0.8}
	l1 := routedLog(now-100, "cheap-model", 1, 1, 1, meta)
	l1.RequestId = "r1"
	l2 := routedLog(now-50, "cheap-model", 1, 1, 1, meta)
	l2.RequestId = "r2"
	l3 := routedLog(now-10, "cheap-model", 1, 1, 1, meta) // 无明细
	d := routedLog(now-5, "decider", 1, 1, 1, map[string]any{"routing_policy": "auto/t", "routing_decision": true})
	if err := db.Create(&[]*Log{l1, l2, l3, d}).Error; err != nil {
		t.Fatalf("写日志失败: %v", err)
	}
	details := []*LogDetail{
		{LogId: l1.Id, RequestBody: `{"messages":[{"role":"user","content":"first"}]}`},
		{LogId: l2.Id, RequestBody: `{"messages":[{"role":"user","content":"second"}]}`},
		{LogId: d.Id, RequestBody: `{"messages":[{"role":"user","content":"decision"}]}`},
	}
	if err := db.Create(&details).Error; err != nil {
		t.Fatalf("写明细失败: %v", err)
	}
	samples, err := GetRoutingPolicyLogSamples("auto/t", 10)
	if err != nil {
		t.Fatalf("取样本失败: %v", err)
	}
	if len(samples) != 2 || samples[0].RequestId != "r2" || samples[1].RequestId != "r1" {
		t.Fatalf("样本应为 r2, r1，实际 %+v", samples)
	}
	if samples[0].Label != "small" || samples[0].Confidence != 0.8 || samples[0].RequestBody == "" {
		t.Fatalf("样本字段不对: %+v", samples[0])
	}
}
