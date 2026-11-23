package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// Supplies of one model under cost routing: the top priority holds two at the catalog price (shared
// by balance), a dearer aggregator and one of unknown cost (both backups, being dearer), with one
// backup below. The quote is $5 / $30 per million.
func TestSupplyCostViews(t *testing.T) {
	usePreviewTestDB(t)
	oldOrder := config.SupplyCostOrder
	config.SupplyCostOrder = SupplyCostOrderCost
	t.Cleanup(func() { config.SupplyCostOrder = oldOrder })
	resetSupplyHealth()
	oldPricing := PricingInstance
	PricingInstance = &Pricing{Prices: map[string]*Price{
		"m": {Model: "m", Type: TokensPriceType, Input: 2.5, Output: 15},
	}}
	t.Cleanup(func() { PricingInstance = oldPricing })
	if err := DB.Create(&ModelInfo{Model: "m", Status: ModelStatusPublished}).Error; err != nil {
		t.Fatal(err)
	}

	official := previewChannel(t, "official", "default", config.ChannelStatusEnabled)
	aggregator := previewChannel(t, "aggregator", "default", config.ChannelStatusEnabled)
	backup := previewChannel(t, "backup", "default", config.ChannelStatusEnabled)
	relay := previewChannel(t, "relay", "default", config.ChannelStatusEnabled)
	DB.Model(aggregator).Update("fee_rate", 0.05)
	DB.Model(backup).Update("fee_rate", 0.05)
	DB.Model(relay).Update("cost_ratio", 1.0)
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
	if v := byName["aggregator"]; v.Traffic != SupplyTrafficBackup || !v.DearerBackup {
		t.Errorf("aggregator: traffic %q dearer %v, want a dearer backup", v.Traffic, v.DearerBackup)
	}
	if v := byName["relay"]; v.Traffic != SupplyTrafficBalanced {
		t.Errorf("relay: traffic %q, want balanced with official", v.Traffic)
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
	if summary == nil || summary.AboveQuote != 0 || summary.AboveQuoteBackup != 2 || summary.CostUnknown != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	// The cheapest available supply of known cost is at or below the catalog price.
	if summary.CheapestToCatalog <= 0 || summary.CheapestToCatalog > 1 || summary.CheapestAboveQuote {
		t.Errorf("cheapest: %v above %v", summary.CheapestToCatalog, summary.CheapestAboveQuote)
	}

	// A model only the aggregator supplies takes its traffic above the quote.
	if err := DB.Create(&ModelInfo{Model: "n", Status: ModelStatusPublished}).Error; err != nil {
		t.Fatal(err)
	}
	PricingInstance.Prices["n"] = &Price{Model: "n", Type: TokensPriceType, Input: 2.5, Output: 15}
	previewSupply(t, "n", aggregator, 10, 1, true)
	nViews, _ := GetModelSupplyViews("n", 0)
	nSummary := summarizeSupplyCosts(nViews)["n"]
	if nSummary == nil || nSummary.AboveQuote != 1 || *nSummary.MaxOutput < 34.65-1e-9 || *nSummary.MaxOutput > 34.65+1e-9 {
		t.Fatalf("serving above the quote: summary = %+v", nSummary)
	}
	if nSummary.CheapestToCatalog <= 1 || !nSummary.CheapestAboveQuote {
		t.Errorf("only supply above the quote: cheapest %v above %v", nSummary.CheapestToCatalog, nSummary.CheapestAboveQuote)
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
