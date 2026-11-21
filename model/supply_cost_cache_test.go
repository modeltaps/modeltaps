package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"gorm.io/datatypes"
)

// A supply's cost says what cache reads cost on its channel next to what the catalog charges for
// them, and whether they cost more than they bring in. That is told to admins and does not make
// the supply above the quote: cache reads are a share of the input, and a channel that charges
// more for them often charges much less for the rest.
func TestSupplyCostCacheRead(t *testing.T) {
	extraOf := func(ratios map[string]float64) *datatypes.JSONType[map[string]float64] {
		extra := datatypes.NewJSONType(ratios)
		return &extra
	}
	oldPricing, oldPayment := PricingInstance, config.PaymentCostPercent
	PricingInstance = &Pricing{Prices: map[string]*Price{
		// $3 / $15 per million, cache reads $0.30 (the Anthropic usage kind).
		"claude": {Model: "claude", Type: TokensPriceType, Input: 1.5, Output: 7.5,
			ExtraRatios: extraOf(map[string]float64{config.UsageExtraCachedRead: 0.1})},
		// $2 / $8 per million, cache hits $0.20 (the OpenAI usage kind).
		"gpt": {Model: "gpt", Type: TokensPriceType, Input: 1, Output: 4,
			ExtraRatios: extraOf(map[string]float64{config.UsageExtraCache: 0.1})},
		// No cache price of its own: a cache hit is charged as input.
		"plain": {Model: "plain", Type: TokensPriceType, Input: 1, Output: 4},
	}}
	config.PaymentCostPercent = 0
	t.Cleanup(func() { PricingInstance, config.PaymentCostPercent = oldPricing, oldPayment })

	t.Run("cache reads dearer than their quote, input and output below theirs", func(t *testing.T) {
		c := supplyCostOf("claude", "claude", &Channel{}, &ChannelPrice{Input: 2.7, Output: 13.5, CacheRead: 0.9})
		if !c.Known || !c.CacheReadAbove {
			t.Fatalf("cost %+v, want cache reads marked dearer than their quote", c)
		}
		if !near(*c.Input, 2.7) || !near(*c.CacheRead, 0.9) || !near(*c.QuoteCacheRead, 0.3) {
			t.Errorf("input %v, cache read %v, quoted %v", *c.Input, *c.CacheRead, *c.QuoteCacheRead)
		}
		// The supply is not held back for it, and is still ordered by its input and output cost.
		if c.AboveQuote || !near(c.CostToCatalog, 0.9) {
			t.Errorf("above quote %v, cost to catalog %v; want a supply below the quote at 0.9", c.AboveQuote, c.CostToCatalog)
		}
	})

	t.Run("the channel's one cache read price covers the OpenAI usage kind", func(t *testing.T) {
		c := supplyCostOf("gpt", "gpt", &Channel{}, &ChannelPrice{Input: 1.8, Output: 7.2, CacheRead: 0.6})
		if !c.CacheReadAbove || !near(*c.CacheRead, 0.6) || !near(*c.QuoteCacheRead, 0.2) {
			t.Errorf("cost %+v, want cache reads at $0.60 above the $0.20 quote", c)
		}
	})

	t.Run("a channel that lists no cache price keeps the catalog's discount", func(t *testing.T) {
		c := supplyCostOf("claude", "claude", &Channel{}, &ChannelPrice{Input: 2.7, Output: 13.5})
		if c.AboveQuote || c.CacheReadAbove || !near(*c.CacheRead, 0.27) || !near(c.CostToCatalog, 0.9) {
			t.Errorf("cost %+v, want cache reads at $0.27 and nothing above the quote", c)
		}
	})

	t.Run("the same prices as the catalog are not above it", func(t *testing.T) {
		c := supplyCostOf("claude", "claude", &Channel{}, &ChannelPrice{Input: 3, Output: 15, CacheRead: 0.3})
		if c.AboveQuote || c.CacheReadAbove || !near(c.CostToCatalog, 1) {
			t.Errorf("cost %+v, want nothing above the quote", c)
		}
		// With a fee every part is above a quote sold at no margin.
		fee := supplyCostOf("claude", "claude", &Channel{FeeRate: floatPtr(0.05)}, &ChannelPrice{Input: 3, Output: 15, CacheRead: 0.3})
		if !fee.AboveQuote || !fee.CacheReadAbove || !near(*fee.CacheRead, 0.315) {
			t.Errorf("cost with a fee %+v, want every part above the quote", fee)
		}
	})

	t.Run("a model without a cache price is not judged by one", func(t *testing.T) {
		c := supplyCostOf("plain", "plain", &Channel{}, &ChannelPrice{Input: 1.8, Output: 7.2, CacheRead: 1.8})
		if c.AboveQuote || c.CacheRead != nil || c.QuoteCacheRead != nil || !near(c.CostToCatalog, 0.9) {
			t.Errorf("cost %+v, want no cache read part", c)
		}
	})

	t.Run("the model list does not count it as a supply above the quote", func(t *testing.T) {
		dearer := supplyCostOf("claude", "claude", &Channel{}, &ChannelPrice{Input: 2.7, Output: 13.5, CacheRead: 0.9})
		views := []*ModelSupplyView{{ModelSupply: &ModelSupply{Model: "claude"}, Traffic: SupplyTrafficPrimary, Cost: dearer}}
		if summary := summarizeSupplyCosts(views)["claude"]; summary == nil || summary.AboveQuote != 0 || summary.CheapestAboveQuote {
			t.Errorf("summary = %+v, want no supply above the quote", summary)
		}
	})
}
