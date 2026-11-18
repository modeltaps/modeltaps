package model

import (
	"reflect"
	"testing"
)

func TestCostTierIndex(t *testing.T) {
	cases := []struct {
		costs []float64
		want  []int
	}{
		{[]float64{0, 0}, []int{0, 0}},               // nothing known: one tier
		{[]float64{1.05, 1, 1.0005}, []int{1, 0, 0}}, // within 0.1%: same price
		{[]float64{1.05, 1.055, 1}, []int{1, 2, 0}},  // Requesty 5% vs OpenRouter 5.5%
		{[]float64{0, 1.05, 1}, []int{2, 1, 0}},      // unknown cost last
		{[]float64{1.1, 1.1}, []int{0, 0}},           // equal: shared
	}
	for _, c := range cases {
		if got := costTierIndex(c.costs); !reflect.DeepEqual(got, c.want) {
			t.Errorf("costTierIndex(%v) = %v, want %v", c.costs, got, c.want)
		}
	}
}

// Supplies of one priority are tried cheapest first: a dearer one only takes requests when the
// cheaper one is unavailable, even with balance and a larger weight.
func TestRouterPrefersCheaperSupply(t *testing.T) {
	useAllocationTestDB(t)
	cheap := makeAllocationChannel(t, testChannel{"cheap", 1, "https://cheap.example", 0, 1})
	dear := makeAllocationChannel(t, testChannel{"dear", 1, "https://dear.example", 0, 1})
	unknown := makeAllocationChannel(t, testChannel{"unknown", 1, "https://unknown.example", 0, 1})
	if err := DB.Model(cheap).Update("cost_ratio", 1.0).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Model(dear).Updates(map[string]any{"cost_ratio": 1.0, "fee_rate": 0.05}).Error; err != nil {
		t.Fatal(err)
	}
	makePublicModel(t, "m")
	makeSupply(t, "m", cheap, 0, 1)
	makeSupply(t, "m", dear, 0, 100)
	makeSupply(t, "m", unknown, 0, 100)
	oldPricing := PricingInstance
	PricingInstance = &Pricing{Prices: map[string]*Price{"m": {Model: "m", Type: TokensPriceType, Input: 1, Output: 2}}}
	t.Cleanup(func() { PricingInstance = oldPricing })
	InvalidateChannelPriceCache()
	ChannelGroup.Load()

	for i := 0; i < 20; i++ {
		ch, err := ChannelGroup.Next("default", "m")
		if err != nil || ch.Id != cheap.Id {
			t.Fatalf("pick %d = %v, %v; want the cheapest channel every time", i, ch, err)
		}
	}
	ch, err := ChannelGroup.Next("default", "m", FilterChannelId([]int{cheap.Id}))
	if err != nil || ch.Id != dear.Id {
		t.Fatalf("cheapest unavailable: got %v, %v; want the dearer known cost before the unknown", ch, err)
	}
	ch, err = ChannelGroup.Next("default", "m", FilterChannelId([]int{cheap.Id, dear.Id}))
	if err != nil || ch.Id != unknown.Id {
		t.Fatalf("known costs unavailable: got %v, %v; want the unknown cost last", ch, err)
	}

	views, err := GetModelSupplyViews("m", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range views {
		wantTraffic, wantDearer := SupplyTrafficBackup, true
		if v.ChannelId == cheap.Id {
			wantTraffic, wantDearer = SupplyTrafficPrimary, false
		}
		if v.Traffic != wantTraffic || v.DearerBackup != wantDearer {
			t.Errorf("supply on %s: traffic %q dearer %v, want %q %v", v.ChannelName, v.Traffic, v.DearerBackup, wantTraffic, wantDearer)
		}
	}
	preview, err := PreviewModelSupply("m", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Tiers) != 3 || preview.Tiers[0].Supplies[0].ChannelId != cheap.Id || preview.Tiers[2].CostTier != 2 {
		t.Fatalf("preview tiers = %d, want three cost tiers cheapest first", len(preview.Tiers))
	}
}
