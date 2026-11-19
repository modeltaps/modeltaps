package model

import (
	"reflect"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
)

func TestCostTierIndex(t *testing.T) {
	old := config.SupplyCostMinSavingPercent
	t.Cleanup(func() { config.SupplyCostMinSavingPercent = old })
	cases := []struct {
		saving float64
		costs  []float64
		want   []int
	}{
		{5, []float64{0, 0}, []int{0, 0}},               // nothing known: one tier
		{5, []float64{1.06, 1, 1.0005}, []int{1, 0, 0}}, // 6% dearer: a tier of its own
		{5, []float64{1.04, 1.045, 1}, []int{0, 0, 0}},  // fees of 4% and 4.5%: within 5%
		{5, []float64{0, 1.2, 1}, []int{2, 1, 0}},       // unknown cost last
		{5, []float64{1.1, 1.1}, []int{0, 0}},           // equal: shared
		{0, []float64{1.05, 1.055, 1}, []int{1, 2, 0}},  // no saving asked: 0.1% still splits
		{0, []float64{1.05, 1, 1.0005}, []int{1, 0, 0}}, // within 0.1%: same price
	}
	for _, c := range cases {
		config.SupplyCostMinSavingPercent = c.saving
		if got := costTierIndex(c.costs); !reflect.DeepEqual(got, c.want) {
			t.Errorf("saving %v%%: costTierIndex(%v) = %v, want %v", c.saving, c.costs, got, c.want)
		}
	}
}

// With SupplyCostOrder "priority" (default) cost does not order supplies: a cheaper and a dearer
// supply of one priority share requests by weight. With "cost" the cheaper one takes them all,
// the dearer known cost backing it up before the unknown one.
func TestRouterCostOrderWithinPriority(t *testing.T) {
	useAllocationTestDB(t)
	old := config.SupplyCostOrder
	t.Cleanup(func() { config.SupplyCostOrder = old })
	cheap := makeAllocationChannel(t, testChannel{"cheap", 1, "https://cheap.example", 0, 1})
	dear := makeAllocationChannel(t, testChannel{"dear", 1, "https://dear.example", 0, 1})
	unknown := makeAllocationChannel(t, testChannel{"unknown", 1, "https://unknown.example", 0, 1})
	if err := DB.Model(cheap).Update("cost_ratio", 1.0).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Model(dear).Updates(map[string]any{"cost_ratio": 1.0, "fee_rate": 0.10}).Error; err != nil {
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
	resetSupplyHealth()
	ChannelGroup.Load()

	config.SupplyCostOrder = SupplyCostOrderPriority
	picked := map[int]int{}
	for i := 0; i < 200; i++ {
		ch, err := ChannelGroup.Next("default", "m")
		if err != nil {
			t.Fatal(err)
		}
		picked[ch.Id]++
	}
	if picked[dear.Id] == 0 || picked[unknown.Id] == 0 || picked[cheap.Id] > 50 {
		t.Fatalf("priority first: picks %v, want shares by weight (1 / 100 / 100)", picked)
	}
	views, _ := GetModelSupplyViews("m", 0)
	for _, v := range views {
		if v.Traffic != SupplyTrafficBalanced || v.DearerBackup {
			t.Errorf("priority first, %s: traffic %q dearer %v, want balanced", v.ChannelName, v.Traffic, v.DearerBackup)
		}
	}
	preview, _ := PreviewModelSupply("m", "")
	if len(preview.Tiers) != 1 {
		t.Fatalf("priority first: preview tiers = %d, want one", len(preview.Tiers))
	}

	config.SupplyCostOrder = SupplyCostOrderCost
	for i := 0; i < 20; i++ {
		ch, err := ChannelGroup.Next("default", "m")
		if err != nil || ch.Id != cheap.Id {
			t.Fatalf("cost first, pick %d = %v, %v; want the cheapest channel every time", i, ch, err)
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
	views, _ = GetModelSupplyViews("m", 0)
	for _, v := range views {
		wantTraffic, wantDearer := SupplyTrafficBackup, true
		if v.ChannelId == cheap.Id {
			wantTraffic, wantDearer = SupplyTrafficPrimary, false
		}
		if v.Traffic != wantTraffic || v.DearerBackup != wantDearer {
			t.Errorf("cost first, %s: traffic %q dearer %v, want %q %v", v.ChannelName, v.Traffic, v.DearerBackup, wantTraffic, wantDearer)
		}
	}
	preview, _ = PreviewModelSupply("m", "")
	if len(preview.Tiers) != 3 || preview.Tiers[0].Supplies[0].ChannelId != cheap.Id || preview.Tiers[2].CostTier != 2 {
		t.Fatalf("cost first: preview tiers = %d, want three cost tiers cheapest first", len(preview.Tiers))
	}
}

// A cheap supply that has been failing is not put ahead for its cost: it is ordered as if its cost
// were unknown until it serves well again; with too few attempts it is not judged.
func TestRouterCostFirstNeedsHealth(t *testing.T) {
	useAllocationTestDB(t)
	if err := DB.AutoMigrate(&ChannelMetric{}); err != nil {
		t.Fatal(err)
	}
	oldOrder, oldSuccess := config.SupplyCostOrder, config.SupplyCostMinSuccessPercent
	t.Cleanup(func() { config.SupplyCostOrder, config.SupplyCostMinSuccessPercent = oldOrder, oldSuccess })
	config.SupplyCostOrder, config.SupplyCostMinSuccessPercent = SupplyCostOrderCost, 95
	top := makeAllocationChannel(t, testChannel{"top", 1, "https://top.example", 0, 1})
	cheap := makeAllocationChannel(t, testChannel{"cheap", 1, "https://cheap.example", 0, 1})
	DB.Model(top).Update("cost_ratio", 1.0)
	DB.Model(cheap).Update("cost_ratio", 0.5)
	makePublicModel(t, "m")
	makeSupply(t, "m", top, 10, 1)
	makeSupply(t, "m", cheap, 0, 1)
	oldPricing := PricingInstance
	PricingInstance = &Pricing{Prices: map[string]*Price{"m": {Model: "m", Type: TokensPriceType, Input: 1, Output: 2}}}
	t.Cleanup(func() { PricingInstance = oldPricing })
	InvalidateChannelPriceCache()
	ChannelGroup.Load()

	// Too few attempts to judge: the cheaper supply leads.
	DB.Create(&ChannelMetric{BucketTs: metricBucket(time.Now()), ChannelId: cheap.Id, ModelName: "m", RequestCount: 10, SuccessCount: 2})
	resetSupplyHealth()
	if ch, _ := ChannelGroup.Next("default", "m"); ch == nil || ch.Id != cheap.Id {
		t.Fatalf("not judged yet: picked %v, want the cheaper supply", ch)
	}
	// 30 of 60 succeeded over the last two hours: held back behind the healthy one.
	DB.Create(&ChannelMetric{BucketTs: metricBucket(time.Now()) - 3600, ChannelId: cheap.Id, ModelName: "m", RequestCount: 50, SuccessCount: 28})
	resetSupplyHealth()
	if ch, _ := ChannelGroup.Next("default", "m"); ch == nil || ch.Id != top.Id {
		t.Fatalf("failing: picked %v, want the healthy supply", ch)
	}
	views, _ := GetModelSupplyViews("m", 0)
	for _, v := range views {
		if (v.ChannelId == cheap.Id) != v.LowSuccess {
			t.Errorf("%s: low success %v", v.ChannelName, v.LowSuccess)
		}
	}
	// The check switched off: cost alone decides again.
	config.SupplyCostMinSuccessPercent = 0
	if ch, _ := ChannelGroup.Next("default", "m"); ch == nil || ch.Id != cheap.Id {
		t.Fatalf("check off: picked %v, want the cheaper supply", ch)
	}
}

func TestOrderSupplyTiers(t *testing.T) {
	prio := []int64{10, 10, 0, 0}
	costs := []float64{1.05, 1, 0.6, 0}
	// Priority first: one tier per priority, cost left out.
	if got := orderSupplyTiers(prio, costs, false); !reflect.DeepEqual(got, [][]int{{0, 1}, {2, 3}}) {
		t.Errorf("priority first = %v", got)
	}
	// Cost first: cheapest anywhere first (1 and 1.05 within the 5% saving share a tier); unknown last.
	if got := orderSupplyTiers(prio, costs, true); !reflect.DeepEqual(got, [][]int{{2}, {0, 1}, {3}}) {
		t.Errorf("cost first = %v", got)
	}
	// Same cost across priorities: priority orders them.
	if got := orderSupplyTiers([]int64{0, 10}, []float64{1, 1}, true); !reflect.DeepEqual(got, [][]int{{1}, {0}}) {
		t.Errorf("same cost = %v", got)
	}
}

// With SupplyCostOrder "cost" a cheaper backup of a lower priority takes the requests.
func TestRouterCostFirstAcrossPriorities(t *testing.T) {
	useAllocationTestDB(t)
	old := config.SupplyCostOrder
	t.Cleanup(func() { config.SupplyCostOrder = old })
	top := makeAllocationChannel(t, testChannel{"top", 1, "https://top.example", 0, 1})
	low := makeAllocationChannel(t, testChannel{"low", 1, "https://low.example", 0, 1})
	DB.Model(top).Updates(map[string]any{"cost_ratio": 1.0, "fee_rate": 0.10})
	DB.Model(low).Update("cost_ratio", 1.0)
	makePublicModel(t, "m")
	makeSupply(t, "m", top, 10, 1)
	makeSupply(t, "m", low, 0, 1)
	oldPricing := PricingInstance
	PricingInstance = &Pricing{Prices: map[string]*Price{"m": {Model: "m", Type: TokensPriceType, Input: 1, Output: 2}}}
	t.Cleanup(func() { PricingInstance = oldPricing })
	InvalidateChannelPriceCache()
	resetSupplyHealth()
	ChannelGroup.Load()

	config.SupplyCostOrder = SupplyCostOrderPriority
	if ch, _ := ChannelGroup.Next("default", "m"); ch == nil || ch.Id != top.Id {
		t.Fatalf("priority first picked %v, want the higher priority", ch)
	}
	config.SupplyCostOrder = SupplyCostOrderCost
	if ch, _ := ChannelGroup.Next("default", "m"); ch == nil || ch.Id != low.Id {
		t.Fatalf("cost first picked %v, want the cheaper supply", ch)
	}
	views, _ := GetModelSupplyViews("m", 0)
	for _, v := range views {
		want := SupplyTrafficBackup
		if v.ChannelId == low.Id {
			want = SupplyTrafficPrimary
		}
		if v.Traffic != want || (v.ChannelId == top.Id && !v.DearerBackup) {
			t.Errorf("%s: traffic %q dearer %v", v.ChannelName, v.Traffic, v.DearerBackup)
		}
	}
	preview, _ := PreviewModelSupply("m", "")
	if preview.CostOrder != SupplyCostOrderCost || preview.Tiers[0].Supplies[0].ChannelId != low.Id {
		t.Fatalf("preview order = %s, first %d", preview.CostOrder, preview.Tiers[0].Supplies[0].ChannelId)
	}
}
