package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func seedPriceTable(t *testing.T) *Pricing {
	t.Helper()
	setupPriceTestDB(t)
	rows := []Price{
		{Model: "up", Type: TokensPriceType, Input: 1, Output: 2},
		{Model: "down", Type: TokensPriceType, Input: 4, Output: 8},
		{Model: "same", Type: TokensPriceType, Input: 1, Output: 1},
		{Model: "locked", Type: TokensPriceType, Input: 1, Output: 1, Locked: true},
	}
	for i := range rows {
		if err := DB.Create(&rows[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	p := &Pricing{Prices: make(map[string]*Price), Match: []string{}}
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}
	return p
}

func upstreamPrices() []*Price {
	return []*Price{
		{Model: "up", Type: TokensPriceType, Input: 1.5, Output: 2},
		{Model: "down", Type: TokensPriceType, Input: 2, Output: 8},
		{Model: "same", Type: TokensPriceType, Input: 1, Output: 1},
		{Model: "locked", Type: TokensPriceType, Input: 3, Output: 3},
		{Model: "fresh", Type: TokensPriceType, Input: 0.5, Output: 0.5},
	}
}

func TestDiffPrices(t *testing.T) {
	p := seedPriceTable(t)
	got := map[string]PriceChange{}
	for _, c := range p.DiffPrices(upstreamPrices()) {
		got[c.Model] = c
	}
	want := map[string]string{"up": PriceChangeRaised, "down": PriceChangeLowered, "same": PriceChangeUnchanged,
		"locked": PriceChangeRaised, "fresh": PriceChangeNew}
	for model, kind := range want {
		if got[model].Kind != kind {
			t.Errorf("%s kind = %q, want %q", model, got[model].Kind, kind)
		}
	}
	if !got["locked"].Locked {
		t.Error("locked flag missing")
	}
	// ratio 1 = $2 per 1M tokens
	if c := got["up"]; c.OldInput != 2 || c.NewInput != 3 || c.OldOutput != 4 || c.NewOutput != 4 {
		t.Errorf("up prices = %+v", c)
	}
}

func TestApplyAutoPriceSyncPolicies(t *testing.T) {
	old := config.PriceSyncAutoApply
	t.Cleanup(func() { config.PriceSyncAutoApply = old })

	cases := []struct {
		policy           string
		upInput, dnInput float64
		held             int
	}{
		{PriceSyncAutoApplyIncreases, 1.5, 4, 1}, // increase written, decrease held
		{PriceSyncAutoApplyAll, 1.5, 2, 0},
		{PriceSyncAutoApplyNone, 1, 4, 2},
	}
	for _, tc := range cases {
		config.PriceSyncAutoApply = tc.policy
		p := seedPriceTable(t)
		report, err := p.ApplyAutoPriceSync(upstreamPrices(), "test-"+tc.policy)
		if err != nil {
			t.Fatal(err)
		}
		if p.Prices["up"].Input != tc.upInput || p.Prices["down"].Input != tc.dnInput {
			t.Errorf("%s: up=%v down=%v", tc.policy, p.Prices["up"].Input, p.Prices["down"].Input)
		}
		if _, ok := p.Prices["fresh"]; !ok {
			t.Errorf("%s: new model not inserted", tc.policy)
		}
		if p.Prices["locked"].Input != 1 {
			t.Errorf("%s: locked price overwritten", tc.policy)
		}
		if len(report.Held) != tc.held || len(report.Locked) != 1 {
			t.Errorf("%s: held=%d locked=%d", tc.policy, len(report.Held), len(report.Locked))
		}
	}
}

func TestUpsertSelectedPrices(t *testing.T) {
	p := seedPriceTable(t)
	n, err := p.UpsertSelectedPrices(upstreamPrices(), []string{"down"})
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if p.Prices["down"].Input != 2 || p.Prices["up"].Input != 1 {
		t.Fatalf("down=%v up=%v", p.Prices["down"].Input, p.Prices["up"].Input)
	}
	if _, ok := p.Prices["fresh"]; ok {
		t.Fatal("unselected model inserted")
	}
}

func TestChannelGroupLosses(t *testing.T) {
	old := GlobalUserGroupRatio.UserGroup
	t.Cleanup(func() { GlobalUserGroupRatio.UserGroup = old })
	GlobalUserGroupRatio.UserGroup = map[string]*UserGroup{
		"default": {Symbol: "default", Ratio: 1},
		"vip":     {Symbol: "vip", Ratio: 0.8},
	}

	cost := 0.9
	losses := ChannelGroupLosses(&Channel{Group: "default,vip", CostRatio: &cost})
	if len(losses) != 1 || losses[0].Group != "vip" || losses[0].GroupRatio != 0.8 {
		t.Fatalf("losses = %+v", losses)
	}
	if ChannelGroupLosses(&Channel{Group: "default,vip"}) != nil {
		t.Fatal("a channel without a cost ratio has no known loss")
	}
}
