package model

import (
	"sort"
	"sync"
	"time"
)

// Supplies are tried cheapest first. Within one priority tier, supplies are split by what their
// channel costs for the model (list price with the channel's fee, over the catalog price): only
// the cheapest share requests (by weight, or the heaviest with failover), a dearer one is reached
// only when every cheaper one is unavailable, and supplies of unknown cost come last. Users pay
// the catalog price times their group ratio either way, so the split only keeps the site's cost
// down; it never changes a price. Priorities still come first: an admin who ranks a dearer supply
// above a cheaper one gets that order.

// costTierTolerance treats costs this close (relative) as the same price: synced prices carry
// rounding, and a supply 0.1% dearer is not worth taking traffic away from.
const costTierTolerance = 0.001

// costTierIndex gives each cost (cost over catalog, 0 when unknown) its tier: 0 for the
// cheapest, counting up with each dearer price, and the last index for unknown costs. When no
// cost is known every supply is in tier 0, as before costs were known.
func costTierIndex(costs []float64) []int {
	out := make([]int, len(costs))
	known := make([]float64, 0, len(costs))
	for _, c := range costs {
		if c > 0 {
			known = append(known, c)
		}
	}
	if len(known) == 0 {
		return out
	}
	sort.Float64s(known)
	// Start of each tier: a cost more than the tolerance above the tier's first.
	starts := []float64{known[0]}
	for _, c := range known[1:] {
		if c > starts[len(starts)-1]*(1+costTierTolerance) {
			starts = append(starts, c)
		}
	}
	for i, c := range costs {
		if c <= 0 {
			out[i] = len(starts)
			continue
		}
		tier := 0
		for t, s := range starts {
			if c >= s {
				tier = t
			}
		}
		out[i] = tier
	}
	return out
}

// routeCostTTL is how long a supply's cost is reused by the router; prices change rarely.
const routeCostTTL = time.Minute

type routeCostKey struct {
	model     string
	channelId int
	upstream  string
}

type routeCostEntry struct {
	cost float64
	gen  uint64
	at   time.Time
}

var routeCostCache sync.Map // routeCostKey -> routeCostEntry

// supplyRouteCost is the cost over catalog of serving modelName through the supply on channel,
// 0 when unknown. Cached for a minute, and dropped with the channel price cache.
func supplyRouteCost(modelName string, supply *ModelSupply, channel *Channel) float64 {
	if supply == nil || channel == nil {
		return 0
	}
	upstream := supplyUpstreamName(supply)
	key := routeCostKey{modelName, channel.Id, upstream}
	gen := channelPriceCacheGen.Load()
	if v, ok := routeCostCache.Load(key); ok {
		e := v.(routeCostEntry)
		if e.gen == gen && time.Since(e.at) < routeCostTTL {
			return e.cost
		}
	}
	cost := 0.0
	if c := supplyCostOf(modelName, upstream, channel, CachedChannelPrice(channel.Id, upstream)); c.Known {
		cost = c.CostToCatalog
	}
	routeCostCache.Store(key, routeCostEntry{cost: cost, gen: gen, at: time.Now()})
	return cost
}

// costTiers splits one priority tier of the routing table into cost tiers, cheapest first. The
// caller holds the read lock. Without supply routing the tier is returned whole.
func (cc *ChannelsChooser) costTiers(modelName string, ids []int) [][]int {
	if cc.Supplies == nil || len(ids) < 2 {
		return [][]int{ids}
	}
	supplies := cc.Supplies[modelName]
	costs := make([]float64, len(ids))
	for i, id := range ids {
		if choice, ok := cc.Channels[id]; ok {
			costs[i] = supplyRouteCost(modelName, supplies[id], choice.Channel)
		}
	}
	index := costTierIndex(costs)
	n := 0
	for _, t := range index {
		n = max(n, t+1)
	}
	if n <= 1 {
		return [][]int{ids}
	}
	out := make([][]int, n)
	for i, id := range ids {
		out[index[i]] = append(out[index[i]], id)
	}
	tiers := out[:0]
	for _, t := range out {
		if len(t) > 0 {
			tiers = append(tiers, t)
		}
	}
	return tiers
}

// supplyViewCost is the cost over catalog of a supply view, 0 when unknown.
func supplyViewCost(v *ModelSupplyView) float64 {
	if v == nil || v.Cost == nil || !v.Cost.Known {
		return 0
	}
	return v.Cost.CostToCatalog
}
