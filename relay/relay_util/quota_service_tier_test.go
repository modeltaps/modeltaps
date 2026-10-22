package relay_util

import (
	"testing"

	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"
)

// TestGetTotalQuotaByUsageServiceTier: the tier the upstream reports scales the token
// charge and the upstream cost; the standard tier leaves both unchanged.
func TestGetTotalQuotaByUsageServiceTier(t *testing.T) {
	q := &Quota{
		price:      model.Price{Type: model.TokensPriceType, Input: 1, Output: 2},
		inputRatio: 1, outputRatio: 2, groupRatio: 1, costRatio: 0.5,
	}
	cases := []struct {
		tier, geo string
		quota     int
		cost      int
	}{
		{"", "", 2000, 1000},           // 1000*1 + 500*2
		{"default", "", 2000, 1000},    // standard
		{"priority", "", 4000, 2000},   // 2x
		{"fast", "", 4000, 2000},       // 2x
		{"ultrafast", "", 12000, 6000}, // 6x
		{"flex", "", 1000, 500},        // 0.5x
		{"fast", "us", 4400, 2200},     // 2x * 1.1
		{"", "us", 2200, 1100},         // 1.1x
	}
	for _, tc := range cases {
		usage := &types.Usage{PromptTokens: 1000, CompletionTokens: 500, ServiceTier: tc.tier, InferenceGeo: tc.geo}
		if got := q.GetTotalQuotaByUsage(usage); got != tc.quota {
			t.Errorf("tier=%q geo=%q quota = %d, want %d", tc.tier, tc.geo, got, tc.quota)
		}
		if got := q.GetCostQuotaByUsage(usage); got != tc.cost {
			t.Errorf("tier=%q geo=%q cost = %d, want %d", tc.tier, tc.geo, got, tc.cost)
		}
	}
}

// TestGetLogMetaServiceTier: a reported tier and its ratio land in the log metadata;
// a standard request writes none of the keys.
func TestGetLogMetaServiceTier(t *testing.T) {
	q := &Quota{price: model.Price{Type: model.TokensPriceType, Input: 1, Output: 1}}

	meta := q.GetLogMeta(&types.Usage{PromptTokens: 1, CompletionTokens: 1, ServiceTier: "fast", InferenceGeo: "us"})
	if meta["service_tier"] != "fast" || meta["inference_geo"] != "us" {
		t.Fatalf("meta = %v", meta)
	}
	if ratio, _ := meta["service_tier_ratio"].(float64); ratio < 2.199 || ratio > 2.201 {
		t.Fatalf("service_tier_ratio = %v, want 2.2", meta["service_tier_ratio"])
	}

	meta = q.GetLogMeta(&types.Usage{PromptTokens: 1, CompletionTokens: 1})
	for _, key := range []string{"service_tier", "inference_geo", "service_tier_ratio"} {
		if _, ok := meta[key]; ok {
			t.Errorf("standard request wrote %s", key)
		}
	}
}
