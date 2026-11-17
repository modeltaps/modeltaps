package model

import (
	"math"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"gorm.io/datatypes"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestSupplyCostPrice(t *testing.T) {
	lc := datatypes.NewJSONType(LongContextTier{Threshold: 200000, InputRatio: 2, OutputRatio: 1.5})
	extra := datatypes.NewJSONType(map[string]float64{config.UsageExtraCachedRead: 0.1, config.UsageExtraCachedWrite1h: 2})
	// Catalog: $5 / $30 per million.
	catalog := &Price{Model: "gpt-5.5", Type: TokensPriceType, Input: 2.5, Output: 15, ExtraRatios: &extra, LongContext: &lc}

	t.Run("channel price with fee", func(t *testing.T) {
		ch := &Channel{FeeRate: floatPtr(0.05)}
		cp := &ChannelPrice{Input: 5.5, Output: 33, CacheRead: 0.55, CacheWrite: 6.875}
		p, source, ok := SupplyCostPrice(catalog, ch, cp, "")
		if !ok || source != CostSourceChannelPrice {
			t.Fatalf("known=%v source=%q", ok, source)
		}
		if !near(p.Input, 5.5/2*1.05) || !near(p.Output, 33.0/2*1.05) {
			t.Errorf("input/output = %v/%v", p.Input, p.Output)
		}
		if !near(p.GetExtraRatio(config.UsageExtraCachedRead), 0.1) || !near(p.GetExtraRatio(config.UsageExtraCachedWrite), 1.25) {
			t.Errorf("cache ratios = %v", p.ExtraRatios.Data())
		}
		// Parts the channel does not list keep the catalog ratios.
		if !near(p.GetExtraRatio(config.UsageExtraCachedWrite1h), 2) || p.LongContext == nil {
			t.Error("catalog ratios for unlisted parts were dropped")
		}
		// The catalog price is left alone.
		if catalog.Input != 2.5 || !near(catalog.GetExtraRatio(config.UsageExtraCachedWrite), 1.25) {
			t.Error("catalog price was modified")
		}
	})

	t.Run("cost ratio without a channel price", func(t *testing.T) {
		p, source, ok := SupplyCostPrice(catalog, &Channel{CostRatio: floatPtr(0.3)}, nil, "")
		if !ok || source != CostSourceCostRatio || !near(p.Input, 0.75) || !near(p.Output, 4.5) {
			t.Fatalf("got %+v %q %v", p, source, ok)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		if _, _, ok := SupplyCostPrice(catalog, &Channel{FeeRate: floatPtr(0.05)}, nil, ""); ok {
			t.Error("a channel with neither a price nor a cost ratio has a known cost")
		}
		// A price in another unit cannot be compared.
		if _, _, ok := SupplyCostPrice(catalog, &Channel{}, &ChannelPrice{Image: 0.04}, ""); ok {
			t.Error("a per-image price was used for a token price")
		}
		if _, _, ok := SupplyCostPrice(catalog, &Channel{}, &ChannelPrice{Other: true}, ""); ok {
			t.Error("an unlisted price kind was used")
		}
	})

	t.Run("free on the upstream", func(t *testing.T) {
		p, _, ok := SupplyCostPrice(catalog, &Channel{FeeRate: floatPtr(0.05)}, &ChannelPrice{}, "")
		if !ok || p.Input != 0 || p.Output != 0 {
			t.Fatalf("got %+v %v", p, ok)
		}
	})

	t.Run("per-request price", func(t *testing.T) {
		times := &Price{Type: TimesPriceType, Input: 20}
		p, _, ok := SupplyCostPrice(times, &Channel{FeeRate: floatPtr(0.1)}, &ChannelPrice{Request: 0.05}, "")
		if !ok || !near(p.Input, 0.05/DollarRate*1.1) || p.Output != 0 {
			t.Fatalf("got %+v %v", p, ok)
		}
	})

	t.Run("per-minute price", func(t *testing.T) {
		minutes := &Price{Type: MinutesPriceType, Input: 3}
		p, _, ok := SupplyCostPrice(minutes, &Channel{}, &ChannelPrice{AudioSecond: 0.0001}, "")
		if !ok || !near(p.Input, 0.006/DollarRate) {
			t.Fatalf("got %+v %v", p, ok)
		}
	})
}

func TestRedactBilling(t *testing.T) {
	billing := func(basis string) map[string]any {
		return map[string]any{"basis": basis, "quote_quota": 100, "charged_quota": 120, "channel_cost_quota": 120,
			"fee_rate": 0.05, "_admin_fee_note": "x", "_admin_cost_source": CostSourceChannelPrice}
	}
	kept := redactBilling(billing("channel_cost"))
	if kept["channel_cost_quota"] != 120 || kept["fee_rate"] != 0.05 {
		t.Errorf("charged at cost: %v", kept)
	}
	for k := range kept {
		if k[0] == '_' {
			t.Errorf("admin key %s kept", k)
		}
	}
	kept = redactBilling(billing("quote"))
	if _, ok := kept["channel_cost_quota"]; ok {
		t.Error("the channel cost of a request charged the quote was shown")
	}
}

func TestSupplyCostPriceOfficialForVendorChannel(t *testing.T) {
	catalog := &Price{Model: "gpt-image-2.6-sunburst", Type: TokensPriceType, Input: 9, Output: 9}
	openai := &Channel{Type: config.ChannelTypeOpenAI, FeeRate: floatPtr(0.1)}

	p, source, ok := SupplyCostPrice(catalog, openai, nil, "gpt-image-2.6-sunburst")
	if !ok || source != CostSourceOfficial {
		t.Fatalf("vendor channel: ok=%v source=%q, want the official price", ok, source)
	}
	// $8 image in, $30 image out (ratios 4 / 15), with the 10% fee; text input keeps its own rate.
	if !nearly(p.Input, 4.4) || !nearly(p.Output, 16.5) {
		t.Errorf("official cost = %v / %v, want 4.4 / 16.5", p.Input, p.Output)
	}
	if p.ExtraRatios == nil || !nearly(p.ExtraRatios.Data()[config.UsageExtraInputTextTokens], 0.625) {
		t.Errorf("official cost extra = %v, want the text input rate", p.ExtraRatios)
	}

	relay := &Channel{Type: config.ChannelTypeOpenAI, BaseURL: strPtr("https://relay.example.com")}
	if _, _, ok := SupplyCostPrice(catalog, relay, nil, "gpt-image-2.6-sunburst"); ok {
		t.Error("a relay at another address is not the vendor: its cost is unknown")
	}
	other := &Channel{Type: config.ChannelTypeOpenRouter}
	if _, _, ok := SupplyCostPrice(catalog, other, nil, "gpt-image-2.6-sunburst"); ok {
		t.Error("another channel type is not the vendor")
	}
	ratio := &Channel{Type: config.ChannelTypeOpenAI, CostRatio: floatPtr(0.5)}
	if _, source, _ := SupplyCostPrice(catalog, ratio, nil, "gpt-image-2.6-sunburst"); source != CostSourceCostRatio {
		t.Errorf("a cost ratio set on the channel wins over the official price, got %q", source)
	}
	if _, source, _ := SupplyCostPrice(catalog, openai, &ChannelPrice{Input: 1, Output: 1}, "gpt-image-2.6-sunburst"); source != CostSourceChannelPrice {
		t.Errorf("a channel price wins over the official price, got %q", source)
	}
	if _, _, ok := SupplyCostPrice(catalog, openai, nil, "not-an-official-model"); ok {
		t.Error("a model without an official price has no official cost")
	}
}

func TestOfficialChannelPriceForDisplay(t *testing.T) {
	openai := &Channel{Type: config.ChannelTypeOpenAI}
	cp := OfficialChannelPrice(openai, "gpt-image-2.6-sunburst")
	if cp == nil || !cp.Official || !nearly(cp.Input, 8) || !nearly(cp.Output, 30) {
		t.Fatalf("official channel price = %+v, want $8 / $30 per 1M", cp)
	}
	if OfficialChannelPrice(&Channel{Type: config.ChannelTypeAnthropic}, "gpt-image-2.6-sunburst") != nil {
		t.Error("an Anthropic channel is not the vendor of an OpenAI model")
	}
}
