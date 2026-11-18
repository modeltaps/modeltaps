package model

import "testing"

// Backups above the quote are held for review; serving ones, accepted ones and supplies added by
// hand are left on.
func TestReviewSuppliesAboveQuote(t *testing.T) {
	useAllocationTestDB(t)
	if err := DB.AutoMigrate(&Option{}); err != nil {
		t.Fatal(err)
	}
	cheap := makeAllocationChannel(t, testChannel{"cheap", 1, "https://cheap.example", 10, 1})
	dear := makeAllocationChannel(t, testChannel{"dear", 1, "https://dear.example", 0, 1})
	dearTop := makeAllocationChannel(t, testChannel{"dear-top", 1, "https://dear-top.example", 10, 1})
	for _, ch := range []*Channel{dear, dearTop} {
		if err := DB.Model(ch).Update("cost_ratio", 1.5).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"backed", "accepted", "by-hand", "served"} {
		makePublicModel(t, name)
	}
	oldPricing := PricingInstance
	PricingInstance = &Pricing{Prices: map[string]*Price{}}
	for _, name := range []string{"backed", "accepted", "by-hand", "served"} {
		PricingInstance.Prices[name] = &Price{Model: name, Type: TokensPriceType, Input: 1, Output: 2}
	}
	t.Cleanup(func() { PricingInstance = oldPricing })

	supply := func(model string, ch *Channel, priority int64, source string, accepted int64) *ModelSupply {
		s := makeSupply(t, model, ch, priority, 1)
		if err := DB.Model(s).Updates(map[string]any{"source": source, "cost_accepted_at": accepted}).Error; err != nil {
			t.Fatal(err)
		}
		return s
	}
	supply("backed", cheap, 10, ModelSupplySourceChannel, 0)
	heldBackup := supply("backed", dear, 0, ModelSupplySourceChannel, 0)
	supply("accepted", cheap, 10, ModelSupplySourceChannel, 0)
	acceptedBackup := supply("accepted", dear, 0, ModelSupplySourceChannel, 1700000000)
	supply("by-hand", cheap, 10, ModelSupplySourceChannel, 0)
	handBackup := supply("by-hand", dear, 0, ModelSupplySourceManual, 0)
	served := supply("served", dearTop, 10, ModelSupplySourceChannel, 0)
	ChannelGroup.Load()
	reloadSupplies()

	// While the default group sells at the official price (no margin), the backup is only reported.
	setDefaultGroupRatio(t, 1)
	unset, err := ReviewSuppliesAboveQuote()
	if err != nil {
		t.Fatal(err)
	}
	if len(unset.Held) != 0 || len(unset.Unheld) != 1 || unset.Unheld[0].Id != heldBackup.Id {
		t.Fatalf("no margin: held %v unheld %v, want only reported", viewIds(unset.Held), viewIds(unset.Unheld))
	}
	if got, _ := GetModelSupply(heldBackup.Id); !got.Enabled || got.Pending {
		t.Fatalf("no margin: backup = %+v, want left on", got)
	}
	// With a margin the default group's price cannot cover a 50% dearer channel either: held.
	setDefaultGroupRatio(t, 1.12)

	review, err := ReviewSuppliesAboveQuote()
	if err != nil {
		t.Fatal(err)
	}
	if len(review.Held) != 1 || review.Held[0].Id != heldBackup.Id {
		t.Fatalf("held = %v, want only the channel-made backup", viewIds(review.Held))
	}
	if len(review.Serving) != 1 || review.Serving[0].Id != served.Id {
		t.Fatalf("serving = %v, want the supply that takes requests", viewIds(review.Serving))
	}
	for id, want := range map[int][2]bool{
		heldBackup.Id:     {false, true},
		acceptedBackup.Id: {true, false},
		handBackup.Id:     {true, false},
		served.Id:         {true, false},
	} {
		got, _ := GetModelSupply(id)
		if got.Enabled != want[0] || got.Pending != want[1] {
			t.Errorf("supply %d enabled=%v pending=%v, want %v/%v", id, got.Enabled, got.Pending, want[0], want[1])
		}
	}

	// Confirming the held supply records the decision: the next review leaves it on.
	held, _ := GetModelSupply(heldBackup.Id)
	held.Enabled = true
	if err := UpdateModelSupply(held); err != nil {
		t.Fatal(err)
	}
	if again, err := ReviewSuppliesAboveQuote(); err != nil || len(again.Held) != 0 {
		t.Fatalf("second review held %v, err %v", viewIds(again.Held), err)
	}
	if got, _ := GetModelSupply(heldBackup.Id); !got.Enabled || got.CostAcceptedAt == 0 {
		t.Fatalf("confirmed supply = %+v, want on with the acceptance recorded", got)
	}
}

func viewIds(views []*ModelSupplyView) []int {
	ids := make([]int, 0, len(views))
	for _, v := range views {
		ids = append(ids, v.Id)
	}
	return ids
}

// setDefaultGroupRatio puts a default group with the ratio in the loaded groups for the test.
func setDefaultGroupRatio(t *testing.T, ratio float64) {
	t.Helper()
	GlobalUserGroupRatio.Lock()
	old := GlobalUserGroupRatio.UserGroup
	groups := map[string]*UserGroup{}
	for k, v := range old {
		groups[k] = v
	}
	groups[DefaultUserGroupSymbol] = &UserGroup{Symbol: DefaultUserGroupSymbol, Name: "Default", Ratio: ratio}
	GlobalUserGroupRatio.UserGroup = groups
	GlobalUserGroupRatio.Unlock()
	t.Cleanup(func() {
		GlobalUserGroupRatio.Lock()
		GlobalUserGroupRatio.UserGroup = old
		GlobalUserGroupRatio.Unlock()
	})
}
