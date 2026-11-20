package relay_util

import (
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"
	"gorm.io/datatypes"
)

func offPeakQuota(at time.Time) *Quota {
	op := datatypes.NewJSONType(model.OffPeakPricing{Ratio: 0.5, Peak: []model.PeakWindow{
		{Days: []int{1, 2, 3, 4, 5}, From: "01:00", To: "04:00"},
	}})
	price := model.Price{Type: model.TokensPriceType, Input: 1, Output: 2, OffPeak: &op}
	cost := price
	return &Quota{
		price: price, inputRatio: 1, outputRatio: 2, groupRatio: 1,
		costPrice: &cost, listPrice: &cost, startedAt: at,
	}
}

// A request that started off peak is charged, costed and expected at half the price; one that
// started in the peak window at the full price, even when it ends after it.
func TestOffPeakRequestsPayTheShare(t *testing.T) {
	usage := &types.Usage{PromptTokens: 1000, CompletionTokens: 1000}
	saturday := time.Date(2026, time.November, 21, 2, 0, 0, 0, time.UTC)
	monday := time.Date(2026, time.November, 16, 3, 59, 0, 0, time.UTC)

	off := offPeakQuota(saturday)
	if got := off.GetTotalQuotaByUsage(usage); got != 1500 {
		t.Errorf("off-peak charge = %d, want 1500", got)
	}
	if got := off.GetCostQuotaByUsage(usage); got != 1500 {
		t.Errorf("off-peak cost = %d, want 1500", got)
	}
	if got := off.ExpectedUpstreamCost(usage); got != 1500/float64(config.QuotaPerUnit) {
		t.Errorf("off-peak expected cost = %v", got)
	}
	meta := off.GetLogMeta(usage)
	if meta["off_peak_ratio"] != 0.5 {
		t.Errorf("log meta off_peak_ratio = %v", meta["off_peak_ratio"])
	}

	peak := offPeakQuota(monday)
	if got := peak.GetTotalQuotaByUsage(usage); got != 3000 {
		t.Errorf("peak charge = %d, want 3000", got)
	}
	if _, ok := peak.GetLogMeta(usage)["off_peak_ratio"]; ok {
		t.Error("peak request logged an off-peak share")
	}
}
