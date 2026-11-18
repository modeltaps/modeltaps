package relay_util

import (
	"testing"

	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"
)

// A request is charged the catalog price times the group ratio whatever its channel costs; the
// channel cost (list price with its fee) only goes to the cost figures.
func TestChargeIgnoresChannelCost(t *testing.T) {
	usage := func() *types.Usage { return &types.Usage{PromptTokens: 1000, CompletionTokens: 1000} }
	price := model.Price{Type: model.TokensPriceType, Input: 2.5, Output: 15}
	list := &model.Price{Type: model.TokensPriceType, Input: 2.75, Output: 16.5}
	cost := &model.Price{Type: model.TokensPriceType, Input: 2.75 * 1.05, Output: 16.5 * 1.05}
	q := &Quota{
		price: price, groupRatio: 1.12,
		inputRatio: price.Input * 1.12, outputRatio: price.Output * 1.12,
		listPrice: list, costPrice: cost, feeRate: 0.05,
	}
	// 1000×2.5 + 1000×15 = 17500 at the catalog price; × 1.12 = 19600.
	if got := q.GetTotalQuotaByUsage(usage()); got != 19600 {
		t.Errorf("charged %d, want 19600 (catalog × group ratio) even on a dearer channel", got)
	}
	if q.baseQuota != 17500 {
		t.Errorf("base quota = %d, want 17500", q.baseQuota)
	}
	// Cost: (2750 + 16500) × 1.05 = 20212.5.
	if got := q.GetCostQuotaByUsage(usage()); got < 20212 || got > 20213 {
		t.Errorf("cost quota = %d, want about 20212", got)
	}
}
