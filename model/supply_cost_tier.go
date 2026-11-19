package model

import (
	"sort"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
)

// How cost orders supplies. With SupplyCostOrder "priority" (default) it does not: the admin's
// priorities and weights decide, and a cheaper supply in the same tier shares requests by weight
// like any other. With "cost" the cheapest supplies of a model take the requests whatever their
// priority, a dearer one being reached only when every cheaper one is unavailable, and supplies
// of unknown cost come last; priority and weight order supplies of about the same cost. Two
// guards keep a cheap but poor supply from taking a model's traffic on price alone:
//
//   - a supply must be cheaper by at least SupplyCostMinSavingPercent to be put ahead; closer
//     costs count as the same (costTierIndex);
//   - a supply whose recent success rate is below SupplyCostMinSuccessPercent is ordered as if its
//     cost were unknown (supply_health.go), until it serves well again.
//
// Users pay the catalog price times their group ratio either way, so cost only keeps the site's
// cost down; it never changes a price.

// Values of SupplyCostOrder.
const (
	SupplyCostOrderPriority = "priority"
	SupplyCostOrderCost     = "cost"
)

func costFirst() bool { return config.SupplyCostOrder == SupplyCostOrderCost }

// orderSupplyTiers groups supplies into the tiers they are tried in, first tier first. prio is
// each supply's priority (higher first), costs its cost over catalog (0 unknown). It returns the
// supplies' indexes per tier: one tier per priority, or with byCost one per cost tier, priority
// ordering the supplies inside it.
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
		for i, p := range prio {
			keys[i] = key{-p, 0}
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

// costTierMargin is how much cheaper (relative) a cost must be to make a tier of its own:
// SupplyCostMinSavingPercent, at least 0.1% so rounding in synced prices never splits a tier.
func costTierMargin() float64 {
	m := config.SupplyCostMinSavingPercent / 100
	if m < 0.001 {
		m = 0.001
	}
	return m
}

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
	// Start of each tier: a cost the margin above the tier's first, so the tier's first is at
	// least that much cheaper than the next tier.
	margin := costTierMargin()
	starts := []float64{known[0]}
	for _, c := range known[1:] {
		if c*(1-margin) >= starts[len(starts)-1]*(1+1e-9) {
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
	byCost := costFirst()
	var ids []int
	var prio []int64
	var costs []float64
	for p, tier := range priorities {
		for _, id := range tier {
			ids = append(ids, id)
			prio = append(prio, int64(len(priorities)-p))
			cost := 0.0
			if choice, ok := cc.Channels[id]; ok && byCost {
				cost = supplyRouteCost(modelName, supplies[id], choice.Channel)
				if cost > 0 && supplies[id] != nil && !supplyServesWell(id, supplyUpstreamName(supplies[id])) {
					cost = 0
				}
			}
			costs = append(costs, cost)
		}
	}
	groups := orderSupplyTiers(prio, costs, byCost)
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

// supplyViewRankCost is the cost a supply view is ordered by: its cost under the "cost"
// SupplyCostOrder unless it serves poorly (then 0, like an unknown cost, and LowSuccess is set);
// 0 under "priority", where cost does not order supplies.
func supplyViewRankCost(v *ModelSupplyView) float64 {
	if !costFirst() {
		return 0
	}
	cost := supplyViewCost(v)
	if cost > 0 && v.ModelSupply != nil && !supplyServesWell(v.ChannelId, supplyUpstreamName(v.ModelSupply)) {
		v.LowSuccess = true
		return 0
	}
	return cost
}
