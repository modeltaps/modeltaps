package model

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"gorm.io/datatypes"
)

// Channel cost: what the channel's provider charges for a request, as a price list in the units
// the gateway bills with. It is never charged to users (they pay the catalog price times their
// group ratio): it orders supplies (supply_cost_tier.go), drives the price check and the cost
// figures.

// Where a channel cost comes from, in this order: the channel's upstream price list, the cost ratio
// set on the channel (an admin who knows the real discount), and the official list price for a
// vendor channel at its own address (CostSourceOfficial).
const (
	CostSourceChannelPrice = "channel_price" // the channel's upstream price list
	CostSourceCostRatio    = "cost_ratio"    // the catalog price times the channel's cost ratio
)

// CostSourceUpstream is the cost of one request taken from what its upstream said it charged
// (the consume log's cost_source). A supply never has it: its cost is a price list.
const CostSourceUpstream = "upstream"

// SupplyCostPrice returns the price list of what the channel charges for upstreamModel, in the
// same ratio units as the catalog price, fee included and without any group ratio. Parts the
// channel's price does not list (1h cache writes, long-context tiers) keep the catalog's ratios.
// known is false when the cost cannot be told: no channel price and no cost ratio, or a channel
// price in a unit the catalog price does not use.
func SupplyCostPrice(catalog *Price, ch *Channel, cp *ChannelPrice, upstreamModel string) (price *Price, source string, known bool) {
	price, source, known = SupplyListPrice(catalog, ch, cp, upstreamModel)
	if !known {
		return nil, "", false
	}
	fee := 1 + ch.GetFeeRate()
	price.Input *= fee
	price.Output *= fee
	return price, source, true
}

// SupplyListPrice is the channel's list price for upstreamModel, without its fee: what the
// upstream deducts from the account's credits, the fee having been paid when the credits were
// bought. It is the side of the reconciliation that the upstream's own charge is compared with.
func SupplyListPrice(catalog *Price, ch *Channel, cp *ChannelPrice, upstreamModel string) (price *Price, source string, known bool) {
	if catalog == nil || ch == nil {
		return nil, "", false
	}
	if cp != nil {
		if p, ok := priceFromChannelPrice(catalog, cp); ok {
			return p, CostSourceChannelPrice, true
		}
	}
	if ratio := ch.GetCostRatio(); ratio > 0 {
		p := clonePrice(catalog)
		p.Input = catalog.GetInput() * ratio
		p.Output = catalog.GetOutput() * ratio
		return p, CostSourceCostRatio, true
	}
	if p := OfficialChannelCost(catalog, ch, upstreamModel); p != nil {
		return p, CostSourceOfficial, true
	}
	return nil, "", false
}

// priceFromChannelPrice converts a channel's upstream price to the catalog's price type and
// units. A channel price in another unit (per image, per second of video) is not converted.
func priceFromChannelPrice(catalog *Price, cp *ChannelPrice) (*Price, bool) {
	if cp.Other {
		return nil, false
	}
	p := clonePrice(catalog)
	// The channel's own list has no off-peak discount of the catalog price's vendor.
	p.OffPeak = nil
	switch catalog.Type {
	case TimesPriceType:
		if cp.Request <= 0 {
			return listedFreePrice(p, cp)
		}
		p.Input, p.Output = cp.Request/DollarRate, 0
		return p, true
	case MinutesPriceType:
		if cp.AudioSecond <= 0 {
			return listedFreePrice(p, cp)
		}
		p.Input, p.Output = cp.AudioSecond*60/DollarRate, 0
		return p, true
	}
	if cp.Input <= 0 && cp.Output <= 0 {
		return listedFreePrice(p, cp)
	}
	// USD per million tokens to ratio units (1 = $2 / 1M tokens).
	p.Input = cp.Input / modelsDevCostPerMillionBase
	p.Output = cp.Output / modelsDevCostPerMillionBase
	if cp.Input > 0 && (cp.CacheRead > 0 || cp.CacheWrite > 0) {
		extra := map[string]float64{}
		if catalog.ExtraRatios != nil {
			for k, v := range catalog.ExtraRatios.Data() {
				extra[k] = v
			}
		}
		if cp.CacheRead > 0 {
			extra[config.UsageExtraCachedRead] = cp.CacheRead / cp.Input
		}
		if cp.CacheWrite > 0 {
			extra[config.UsageExtraCachedWrite] = cp.CacheWrite / cp.Input
			extra[config.UsageExtraOpenAICacheWrite] = cp.CacheWrite / cp.Input
		}
		jt := datatypes.NewJSONType(extra)
		p.ExtraRatios = &jt
	}
	return p, true
}

// listedFreePrice is a zero price when the upstream lists the model as free, and no price otherwise.
func listedFreePrice(p *Price, cp *ChannelPrice) (*Price, bool) {
	if !cp.isFree() {
		return nil, false
	}
	p.Input, p.Output = 0, 0
	return p, true
}

// isFree reports a model the upstream lists at zero for every unit.
func (cp *ChannelPrice) isFree() bool {
	return !cp.Other && cp.Input == 0 && cp.Output == 0 && cp.CacheRead == 0 && cp.CacheWrite == 0 &&
		cp.Image == 0 && cp.Request == 0 && cp.VideoSecond == 0 && cp.AudioSecond == 0
}

func clonePrice(p *Price) *Price {
	c := *p
	c.ModelInfo = nil
	c.Unconfigured = false
	return &c
}

// Channel prices are read on every billed request, so lookups are cached. Any write to the
// channel prices table drops the whole cache; entries also expire, for writes made by another node.
const channelPriceCacheTTL = 10 * time.Minute

type channelPriceCacheEntry struct {
	price *ChannelPrice // nil: the channel has no price for the model
	gen   uint64
	at    time.Time
}

var (
	channelPriceCache    sync.Map // channelPriceKey -> channelPriceCacheEntry
	channelPriceCacheGen atomic.Uint64
)

// InvalidateChannelPriceCache drops every cached channel price lookup.
func InvalidateChannelPriceCache() {
	channelPriceCacheGen.Add(1)
}

// CachedChannelPrice returns the stored upstream price of a model on a channel, nil when the
// channel has none.
func CachedChannelPrice(channelID int, upstreamModel string) *ChannelPrice {
	if channelID == 0 || upstreamModel == "" || DB == nil {
		return nil
	}
	key := channelPriceKey{channelID: channelID, model: upstreamModel}
	gen := channelPriceCacheGen.Load()
	if v, ok := channelPriceCache.Load(key); ok {
		e := v.(channelPriceCacheEntry)
		if e.gen == gen && time.Since(e.at) < channelPriceCacheTTL {
			return e.price
		}
	}
	var rows []*ChannelPrice
	if err := DB.Where("channel_id = ? AND model = ?", channelID, upstreamModel).Limit(1).Find(&rows).Error; err != nil {
		return nil
	}
	var price *ChannelPrice
	if len(rows) > 0 {
		price = rows[0]
	}
	channelPriceCache.Store(key, channelPriceCacheEntry{price: price, gen: gen, at: time.Now()})
	return price
}
