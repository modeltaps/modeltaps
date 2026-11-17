package relay_util

import (
	"testing"

	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"
)

// floorQuota is a Quota for $5 / $30 per million at the given group ratio, with the cost floor on
// and the given channel cost (nil: unknown).
func floorQuota(groupRatio float64, cost *model.Price) *Quota {
	price := model.Price{Type: model.TokensPriceType, Input: 2.5, Output: 15}
	return &Quota{
		price: price, groupRatio: groupRatio,
		inputRatio: price.Input * groupRatio, outputRatio: price.Output * groupRatio,
		costFloor: cost != nil || groupRatio < 0, costPrice: cost,
	}
}

func TestCostFloorCharging(t *testing.T) {
	usage := func() *types.Usage { return &types.Usage{PromptTokens: 1000, CompletionTokens: 1000} }
	// Quote at group ratio 1: 1000×2.5 + 1000×15 = 17500.
	cheap := &model.Price{Type: model.TokensPriceType, Input: 2, Output: 12}
	dear := &model.Price{Type: model.TokensPriceType, Input: 2.75 * 1.05, Output: 16.5 * 1.05}

	q := floorQuota(1, cheap)
	if got := q.GetTotalQuotaByUsage(usage()); got != 17500 || q.billing.basis != billingBasisQuote {
		t.Errorf("cheaper channel: charged %d basis %s, want the quote 17500", got, q.billing.basis)
	}
	if q.billing.cost != 14000 {
		t.Errorf("channel cost = %d, want 14000", q.billing.cost)
	}

	q = floorQuota(1, dear)
	want := 20213 // ceil(1000×2.8875 + 1000×17.325)
	if got := q.GetTotalQuotaByUsage(usage()); got != want || q.billing.basis != billingBasisChannelCost {
		t.Errorf("dearer channel: charged %d basis %s, want the cost %d", got, q.billing.basis, want)
	}

	// A discount group pays at least the channel cost; the cost is not discounted.
	q = floorQuota(0.5, cheap)
	if got := q.GetTotalQuotaByUsage(usage()); got != 14000 || q.billing.quote != 8750 {
		t.Errorf("discount group: charged %d (quote %d), want the cost 14000", got, q.billing.quote)
	}
	// A premium group above the cost pays the quote.
	q = floorQuota(2, dear)
	if got := q.GetTotalQuotaByUsage(usage()); got != 35000 {
		t.Errorf("premium group: charged %d, want 35000", got)
	}

	// Unknown cost: the quote, marked as such.
	q = floorQuota(1, nil)
	q.costFloor = true
	if got := q.GetTotalQuotaByUsage(usage()); got != 17500 || q.billing.basis != billingBasisCostUnknown {
		t.Errorf("unknown cost: charged %d basis %s", got, q.billing.basis)
	}
	if _, ok := q.billingLogMeta()["channel_cost_quota"]; ok {
		t.Error("an unknown cost was logged")
	}

	// Floor off: exactly the quote and no breakdown.
	q = floorQuota(0.5, dear)
	q.costFloor = false
	if got := q.GetTotalQuotaByUsage(usage()); got != 8750 || q.billing != nil {
		t.Errorf("floor off: charged %d billing %+v", got, q.billing)
	}
	if q.GetCostQuotaByUsage(usage()) != 0 {
		t.Error("floor off: the cost changed")
	}
}

func TestCostFloorCostAndReserve(t *testing.T) {
	dear := &model.Price{Type: model.TokensPriceType, Input: 5, Output: 30}
	q := floorQuota(1, dear)
	q.feeRate = 0.05
	usage := &types.Usage{PromptTokens: 1000, CompletionTokens: 1000}
	// The logged cost is the channel cost, not the catalog price times the cost ratio.
	if got := q.GetCostQuotaByUsage(usage); got != 35000 {
		t.Errorf("cost quota = %d, want 35000", got)
	}
	meta := q.billingLogMeta()
	if meta != nil {
		t.Error("breakdown logged before settlement")
	}
	q.GetTotalQuotaByUsage(usage)
	meta = q.GetLogMeta(usage)[model.LogMetaBilling].(map[string]any)
	if meta["basis"] != billingBasisChannelCost || meta["charged_quota"] != 35000 || meta["fee_rate"] != 0.05 {
		t.Errorf("billing meta = %v", meta)
	}

	// The reserve covers the channel cost when it is higher.
	q = floorQuota(1, dear)
	q.promptTokens = 1000
	if got, quote := q.preConsumeQuotaWith(q.costPrice, q.costPrice.GetInput(), q.costPrice.GetOutput()), q.preConsumeQuotaWith(&q.price, q.inputRatio, q.outputRatio); got <= quote {
		t.Errorf("cost reserve %d not above quote reserve %d", got, quote)
	}
}
