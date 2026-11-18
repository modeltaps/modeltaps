package model

import (
	"sort"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
)

// Supplies are tried cheapest first. Supplies are split by what their channel costs for the model
// (list price with the channel's fee, over the catalog price): only the cheapest share requests
// (by weight, or the heaviest with failover), a dearer one is reached only when every cheaper one
// is unavailable, and supplies of unknown cost come last. Users pay the catalog price times their
// group ratio either way, so the split only keeps the site's cost down; it never changes a price.
//
// SupplyCostOrder sets how far cost reaches. "priority" (default): within one priority only, so an
// admin who ranks a dearer supply above a cheaper one gets that order. "cost": across priorities,
// priority only ordering supplies of the same cost.

// Values of SupplyCostOrder.
const (
	SupplyCostOrderPriority = "priority"
	SupplyCostOrderCost     = "cost"
)

func costFirst() bool { return config.SupplyCostOrder == SupplyCostOrderCost }

// orderSupplyTiers groups supplies into the tiers they are tried in, first tier first. prio is
// each supply's priority (higher first), costs its cost over catalog (0 unknown). It returns the
// supplies' indexes per tier.
func orderSupplyTiers(prio []int64, costs []float64, byCost bool) [][]int {
	type key struct {
		first, second int64
	}
	keys := make([]key, len(prio))
	if byCost {
		tiers := costTierIndex(costs)
		for i := range prio {
			keys[i] = key{int64(tiers[i]), -prio[i]}
		}
	} else {
		byPrio := map[int64][]int{}
		for i, p := range prio {
			byPrio[p] = append(byPrio[p], i)
		}
		for p, idx := range byPrio {
			sub := make([]float64, len(idx))
			for j, i := range idx {
				sub[j] = costs[i]
			}
			for j, t := range costTierIndex(sub) {
				keys[idx[j]] = key{-p, int64(t)}
			}
		}
	}
	groups := map[key][]int{}
	var order []key
	for i, k := range keys {
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], i)
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].first != order[j].first {
			return order[i].first < order[j].first
		}
		return order[i].second < order[j].second
	})
	out := make([][]int, len(order))
	for i, k := range order {
		out[i] = groups[k]
	}
	return out
}

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

// routeTiers turns a model's priority tiers from the routing table into the tiers requests try,
// cheapest first as SupplyCostOrder says. The caller holds the read lock. Without supply routing
// the priority tiers are returned as they are.
func (cc *ChannelsChooser) routeTiers(modelName string, priorities [][]int) [][]int {
	if cc.Supplies == nil {
		return priorities
	}
	supplies := cc.Supplies[modelName]
	var ids []int
	var prio []int64
	var costs []float64
	for p, tier := range priorities {
		for _, id := range tier {
			ids = append(ids, id)
			prio = append(prio, int64(len(priorities)-p))
			cost := 0.0
			if choice, ok := cc.Channels[id]; ok {
				cost = supplyRouteCost(modelName, supplies[id], choice.Channel)
			}
			costs = append(costs, cost)
		}
	}
	groups := orderSupplyTiers(prio, costs, costFirst())
	out := make([][]int, len(groups))
	for g, idx := range groups {
		for _, i := range idx {
			out[g] = append(out[g], ids[i])
		}
	}
	return out
}

// supplyViewCost is the cost over catalog of a supply view, 0 when unknown.
func supplyViewCost(v *ModelSupplyView) float64 {
	if v == nil || v.Cost == nil || !v.Cost.Known {
		return 0
	}
	return v.Cost.CostToCatalog
}
