package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// Supplies of one model: two in the top tier by balance, one backup below. The quote is
// $5 / $30 per million.
func TestSupplyCostViews(t *testing.T) {
	usePreviewTestDB(t)
	oldPricing := PricingInstance
	PricingInstance = &Pricing{Prices: map[string]*Price{
		"m": {Model: "m", Type: TokensPriceType, Input: 2.5, Output: 15},
	}}
	t.Cleanup(func() { PricingInstance = oldPricing })
	if err := DB.Create(&ModelInfo{Model: "m", Status: ModelStatusPublished, SupplyMode: SupplyModeBalance}).Error; err != nil {
		t.Fatal(err)
	}

	official := previewChannel(t, "official", "default", config.ChannelStatusEnabled)
	aggregator := previewChannel(t, "aggregator", "default", config.ChannelStatusEnabled)
	backup := previewChannel(t, "backup", "default", config.ChannelStatusEnabled)
	relay := previewChannel(t, "relay", "default", config.ChannelStatusEnabled)
	DB.Model(aggregator).Update("fee_rate", 0.05)
	DB.Model(backup).Update("fee_rate", 0.05)
	DB.Model(relay).Update("cost_ratio", 0.3)
	for _, cp := range []struct {
		ch      *Channel
		in, out float64
	}{{official, 5, 30}, {aggregator, 5.5, 33}, {backup, 5.5, 33}} {
		if err := ReplaceChannelPrices(cp.ch.Id, []*ChannelPrice{{Model: "up/" + cp.ch.Name, Input: cp.in, Output: cp.out}}); err != nil {
			t.Fatal(err)
		}
	}
	previewSupply(t, "m", official, 10, 1, true)
	previewSupply(t, "m", aggregator, 10, 1, true)
	previewSupply(t, "m", relay, 10, 1, true)
	previewSupply(t, "m", backup, 1, 1, true)
	unknown := previewChannel(t, "unknown", "default", config.ChannelStatusEnabled)
	previewSupply(t, "m", unknown, 10, 1, true)

	views, err := GetModelSupplyViews("m", 0)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*ModelSupplyView{}
	for _, v := range views {
		byName[v.ChannelName] = v
	}
	if c := byName["official"].Cost; !c.Known || c.AboveQuote || *c.Input != 5 || byName["official"].Traffic != SupplyTrafficBalanced {
		t.Errorf("official: %+v traffic %q", c, byName["official"].Traffic)
	}
	if c := byName["aggregator"].Cost; !c.AboveQuote || *c.Input < 5.775-1e-9 || *c.Input > 5.775+1e-9 || c.FeeRate != 0.05 {
		t.Errorf("aggregator: %+v", c)
	}
	if c := byName["relay"].Cost; !c.Known || c.Source != CostSourceCostRatio || c.AboveQuote {
		t.Errorf("relay: %+v", c)
	}
	if byName["backup"].Traffic != SupplyTrafficBackup || !byName["backup"].Cost.AboveQuote {
		t.Errorf("backup: traffic %q cost %+v", byName["backup"].Traffic, byName["backup"].Cost)
	}
	if byName["unknown"].Cost.Known {
		t.Error("a channel without a price or cost ratio has a known cost")
	}

	summary := summarizeSupplyCosts(views)["m"]
	if summary == nil || summary.AboveQuote != 1 || summary.AboveQuoteBackup != 1 || summary.CostUnknown != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if *summary.MaxOutput < 34.65-1e-9 || *summary.MaxOutput > 34.65+1e-9 {
		t.Errorf("max output = %v", *summary.MaxOutput)
	}

	// Failover: the heaviest of the top tier takes the traffic, the others are backups.
	DB.Model(&ModelInfo{}).Where("model = ?", "m").Update("supply_mode", SupplyModeFailover)
	modelInfoCache.Lock()
	modelInfoCache.loaded = false
	modelInfoCache.Unlock()
	DB.Model(&ModelSupply{}).Where("channel_id = ?", aggregator.Id).Update("weight", 5)
	views, _ = GetModelSupplyViews("m", 0)
	for _, v := range views {
		want := SupplyTrafficBackup
		if v.ChannelName == "aggregator" {
			want = SupplyTrafficPrimary
		}
		if v.Traffic != want {
			t.Errorf("failover %s: traffic %q, want %q", v.ChannelName, v.Traffic, want)
		}
	}
}

func TestSupplyTrafficSkipsUnavailable(t *testing.T) {
	usePreviewTestDB(t)
	if err := DB.Create(&ModelInfo{Model: "m", Status: ModelStatusPublished}).Error; err != nil {
		t.Fatal(err)
	}
	off := previewChannel(t, "off", "default", config.ChannelStatusManuallyDisabled)
	on := previewChannel(t, "on", "default", config.ChannelStatusEnabled)
	previewSupply(t, "m", off, 10, 1, true)
	previewSupply(t, "m", on, 1, 1, true)
	views, _ := GetModelSupplyViews("m", 0)
	for _, v := range views {
		if v.ChannelName == "off" && v.Traffic != SupplyTrafficNone {
			t.Errorf("a disabled channel takes traffic: %q", v.Traffic)
		}
		if v.ChannelName == "on" && v.Traffic != SupplyTrafficPrimary {
			t.Errorf("the only available supply: %q", v.Traffic)
		}
	}
}
