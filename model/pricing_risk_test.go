package model

import (
	"math"
	"testing"
)

func TestPreviewPricingRiskAndConfirm(t *testing.T) {
	useAllocationTestDB(t)
	direct := makeAllocationChannel(t, testChannel{"direct", 1, "https://direct.example", 10, 1})
	feeChannel := makeAllocationChannel(t, testChannel{"fee5", 1, "https://fee.example", 0, 1})
	if err := DB.Model(direct).Update("cost_ratio", 1.0).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Model(feeChannel).Updates(map[string]any{"cost_ratio": 1.0, "fee_rate": 0.05}).Error; err != nil {
		t.Fatal(err)
	}
	makePublicModel(t, "m1")
	makeSupply(t, "m1", direct, 10, 1)
	backup := makeSupply(t, "m1", feeChannel, 0, 1)
	if err := DB.Model(backup).Updates(map[string]any{"enabled": false, "pending": true}).Error; err != nil {
		t.Fatal(err)
	}
	makePublicModel(t, "m2")
	makeSupply(t, "m2", feeChannel, 10, 1)
	ChannelGroup.Load()
	oldPricing := PricingInstance
	PricingInstance = &Pricing{Prices: map[string]*Price{
		"m1": {Model: "m1", Type: TokensPriceType, Input: 1, Output: 2},
		"m2": {Model: "m2", Type: TokensPriceType, Input: 1, Output: 2},
	}}
	t.Cleanup(func() { PricingInstance = oldPricing })
	reloadSupplies()

	// Default group at the official price: the 5% channel serving m2 loses money, m2 has no
	// other supply.
	risk, err := PreviewPricingRisk(1, 4)
	if err != nil {
		t.Fatal(err)
	}
	if risk.ServingAbove != 2 || len(risk.ModelsAtLoss) != 2 {
		t.Fatalf("ratio 1: serving above %d, models at loss %v", risk.ServingAbove, risk.ModelsAtLoss)
	}
	var fee PricingRiskChannel
	for _, ch := range risk.Channels {
		if ch.ChannelId == feeChannel.Id {
			fee = ch
		}
	}
	// 1.05 / 0.96 = 1.09375 to break even; at 1.12: 1.12 x 0.96 / 1.05 - 1 = 2.4%.
	if math.Abs(fee.FeeBreakeven-1.09375) > 1e-6 || fee.Pending != 1 {
		t.Fatalf("fee channel = %+v", fee)
	}
	risk, err = PreviewPricingRisk(1.12, 4)
	if err != nil {
		t.Fatal(err)
	}
	if risk.ServingAbove != 0 || len(risk.ModelsAtLoss) != 0 {
		t.Fatalf("ratio 1.12: serving above %d, models at loss %v", risk.ServingAbove, risk.ModelsAtLoss)
	}
	for _, ch := range risk.Channels {
		if ch.ChannelId == feeChannel.Id && math.Abs(ch.FeeMargin-2.4) > 1e-6 {
			t.Fatalf("fee margin at 1.12 = %v, want 2.4", ch.FeeMargin)
		}
	}

	n, err := ConfirmPendingSupplies(feeChannel.Id)
	if err != nil || n != 1 {
		t.Fatalf("confirmed %d, %v", n, err)
	}
	if got, _ := GetModelSupply(backup.Id); !got.Enabled || got.Pending || got.CostAcceptedAt == 0 {
		t.Fatalf("confirmed supply = %+v", got)
	}
}
