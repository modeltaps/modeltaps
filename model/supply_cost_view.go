package model

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/notify"
)

// How much of a model's traffic a supply takes, as the console shows it next to the channel cost.
const (
	SupplyTrafficPrimary  = "primary"  // takes every request of its model (failover, or the only one)
	SupplyTrafficBalanced = "balanced" // shares requests with others of its tier by weight
	SupplyTrafficBackup   = "backup"   // available, but only reached when the supplies above fail
	SupplyTrafficNone     = ""         // disabled, paused, or its channel is off
)

// SupplyCost is what a supply's channel charges for its model, in USD with the channel's fee,
// next to the model's quoted price. It is the site's cost only: users pay the quoted price.
type SupplyCost struct {
	Known bool `json:"known"`
	// Source is where the cost comes from: CostSourceChannelPrice, CostSourceCostRatio or
	// CostSourceOfficial.
	Source  string  `json:"source,omitempty"`
	FeeRate float64 `json:"fee_rate,omitempty"`
	// Input and Output are USD per million tokens; Flat is USD per request or per minute for
	// models priced that way. Only the ones that apply are set.
	Input  *float64 `json:"input,omitempty"`
	Output *float64 `json:"output,omitempty"`
	Flat   *float64 `json:"flat,omitempty"`
	// CacheRead is what cache reads cost on the channel and QuoteCacheRead what the catalog charges
	// for them, in USD per million tokens; both are set when the catalog prices cache reads apart
	// from input (cacheReadPrices). CacheReadAbove is true when cache reads cost more than they
	// bring in. The price details say so, but it is not part of AboveQuote and holds no supply
	// back: cache reads are a share of the input, and a channel that charges more for them often
	// charges much less for the rest, so its requests still earn. Judging a supply by this one
	// share held back supplies whose input cost a ninth of the quote.
	CacheRead      *float64 `json:"cache_read,omitempty"`
	QuoteCacheRead *float64 `json:"quote_cache_read,omitempty"`
	CacheReadAbove bool     `json:"cache_read_above,omitempty"`
	// AboveQuote is true when some part of the cost (input or output) is above what the model's
	// price brings in: the catalog price times the default group's ratio, less the cost of taking
	// payments (NetRevenueLine). Requests served by this supply lose money.
	AboveQuote bool `json:"above_quote"`
	// CostToCatalog is the cost over the catalog price, the highest of its parts (1.05: the
	// channel costs 5% more than the catalog price). The settings preview compares it with other
	// markups. 0 when the catalog price is 0.
	CostToCatalog float64 `json:"cost_to_catalog,omitempty"`
}

// cacheReadPrices returns what a cache read costs on the channel and what the catalog charges for
// it, in ratio units (the input price times the cache read ratio). known is false when the
// catalog does not price cache reads apart from input: a cache hit is then charged, and counted
// as costing, the input price.
//
// Cache reads are most of the input of an agent's requests, so admins are shown the two prices
// side by side (SupplyCost.CacheRead, QuoteCacheRead).
func cacheReadPrices(cost, catalog *Price) (costRead, quoteRead float64, known bool) {
	if catalog.ExtraRatios == nil || catalog.Type == TimesPriceType || catalog.Type == MinutesPriceType {
		return 0, 0, false
	}
	listed := catalog.ExtraRatios.Data()
	for _, key := range cacheReadKeys {
		ratio, ok := listed[key]
		quote := catalog.GetInput() * ratio
		if !ok || quote <= 0 {
			continue
		}
		c := cost.GetInput() * cost.GetExtraRatio(key)
		// A price with both kinds (DeepSeek) is judged by the one that costs the most over its quote.
		if !known || c/quote > costRead/quoteRead {
			costRead, quoteRead, known = c, quote, true
		}
	}
	return costRead, quoteRead, known
}

// costToCatalog is the highest part of cost over catalog.
func costToCatalog(cost, catalog *Price) float64 {
	ratio := 0.0
	over := func(c, q float64) {
		if q > 0 && c/q > ratio {
			ratio = c / q
		}
	}
	over(cost.GetInput(), catalog.GetInput())
	if catalog.Type != TimesPriceType && catalog.Type != MinutesPriceType {
		over(cost.GetOutput(), catalog.GetOutput())
	}
	return ratio
}

// aboveQuoteSlack ignores rounding in synced prices.
const aboveQuoteSlack = 1.000001

// supplyCostOf prices a supply's channel cost against the model's billing price.
func supplyCostOf(modelName, upstreamModel string, ch *Channel, cp *ChannelPrice) *SupplyCost {
	out := &SupplyCost{}
	if PricingInstance == nil || ch == nil {
		return out
	}
	catalog := PricingInstance.GetBillingPrice(modelName)
	if catalog == nil || catalog.Unconfigured {
		return out
	}
	cost, source, ok := SupplyCostPrice(catalog, ch, cp, upstreamModel)
	if !ok {
		return out
	}
	out.Known, out.Source, out.FeeRate = true, source, ch.GetFeeRate()
	line := NetRevenueLine() * aboveQuoteSlack
	switch catalog.Type {
	case TimesPriceType, MinutesPriceType:
		out.Flat = ptrFloat(cost.GetInput() * DollarRate)
		out.AboveQuote = cost.GetInput() > catalog.GetInput()*line
	default:
		out.Input = tokenPricePerMillion(cost.GetInput())
		out.Output = tokenPricePerMillion(cost.GetOutput())
		out.AboveQuote = cost.GetInput() > catalog.GetInput()*line ||
			cost.GetOutput() > catalog.GetOutput()*line
		if c, q, ok := cacheReadPrices(cost, catalog); ok {
			out.CacheRead, out.QuoteCacheRead = tokenPricePerMillion(c), tokenPricePerMillion(q)
			out.CacheReadAbove = c > q*line
		}
	}
	out.CostToCatalog = costToCatalog(cost, catalog)
	return out
}

func ptrFloat(v float64) *float64 { return &v }

// markSupplyTraffic sets each view's Traffic: within a model, the cheapest available supplies of
// the first priority tier with one take the requests (all of it by weight with balance, the
// heaviest with failover); dearer supplies of that tier and available supplies of the tiers below
// are backups.
func markSupplyTraffic(views []*ModelSupplyView) {
	byModel := map[string][]*ModelSupplyView{}
	for _, v := range views {
		byModel[v.Model] = append(byModel[v.Model], v)
	}
	allocation := allocationInForce()
	now := time.Now().Unix()
	for name, list := range byModel {
		var available []*ModelSupplyView
		for _, v := range list {
			if _, ok := supplyStateOf(v, name, now); ok {
				available = append(available, v)
			}
		}
		if len(available) == 0 {
			continue
		}
		sort.SliceStable(available, func(i, j int) bool {
			return available[i].EffectiveWeight() > available[j].EffectiveWeight()
		})
		failover := false
		if info := lookupModelInfo(name); info != nil {
			failover = routingSupplyMode(info, allocation) == SupplyModeFailover
		}
		// The first tier the router tries takes the requests (supply_cost_tier.go).
		prio := make([]int64, len(available))
		costs := make([]float64, len(available))
		for i, v := range available {
			prio[i], costs[i] = v.Priority, supplyViewRankCost(v)
		}
		groups := orderSupplyTiers(prio, costs, costFirst())
		var tier []*ModelSupplyView
		for _, i := range groups[0] {
			tier = append(tier, available[i])
		}
		top := available[groups[0][0]].Priority
		for _, idx := range groups[1:] {
			for _, i := range idx {
				v := available[i]
				v.Traffic = SupplyTrafficBackup
				// Behind only for its cost: it ranks as high as the supplies that take requests.
				v.DearerBackup = v.Priority >= top
			}
		}
		heaviest := tier[0].EffectiveWeight()
		for _, v := range tier {
			switch {
			case len(tier) == 1:
				v.Traffic = SupplyTrafficPrimary
			case failover && v.EffectiveWeight() < heaviest:
				v.Traffic = SupplyTrafficBackup
			case failover:
				v.Traffic = SupplyTrafficPrimary
			default:
				v.Traffic = SupplyTrafficBalanced
			}
		}
	}
}

// ModelCostSummary sums up the channel costs of a public model's supplies for the catalog list.
type ModelCostSummary struct {
	// AboveQuote counts supplies that take traffic (primary or balanced) and cost more than the
	// quote; AboveQuoteBackup counts backups that do.
	AboveQuote       int `json:"above_quote"`
	AboveQuoteBackup int `json:"above_quote_backup"`
	// CostUnknown counts supplies taking traffic whose cost is not known: the price check
	// cannot judge them.
	CostUnknown int `json:"cost_unknown"`
	// MaxInput and MaxOutput are the highest channel cost per million tokens among the supplies
	// above the quote that take traffic; MaxFlat the same for per-request or per-minute models.
	MaxInput  *float64 `json:"max_input,omitempty"`
	MaxOutput *float64 `json:"max_output,omitempty"`
	MaxFlat   *float64 `json:"max_flat,omitempty"`
	// CheapestToCatalog is the lowest cost over the catalog price (SupplyCost.CostToCatalog) among
	// the available supplies whose cost is known, 0 when none is: how the cheapest supply compares
	// with the model's base price. CheapestAboveQuote says even that one is above the quote.
	CheapestToCatalog  float64 `json:"cheapest_to_catalog,omitempty"`
	CheapestAboveQuote bool    `json:"cheapest_above_quote,omitempty"`
}

// ModelCostSummaries returns the summary of every public model that has a supply worth noting
// (above the quote or of unknown cost) or an available supply of known cost; models with neither
// are left out.
func ModelCostSummaries() (map[string]*ModelCostSummary, error) {
	views, err := GetModelSupplyViews("", 0)
	if err != nil {
		return nil, err
	}
	return summarizeSupplyCosts(views), nil
}

func summarizeSupplyCosts(views []*ModelSupplyView) map[string]*ModelCostSummary {
	out := map[string]*ModelCostSummary{}
	get := func(name string) *ModelCostSummary {
		if s := out[name]; s != nil {
			return s
		}
		s := &ModelCostSummary{}
		out[name] = s
		return s
	}
	maxOf := func(cur **float64, v *float64) {
		if v != nil && (*cur == nil || *v > **cur) {
			*cur = ptrFloat(*v)
		}
	}
	for _, v := range views {
		if v.Traffic == SupplyTrafficNone || v.Cost == nil {
			continue
		}
		serving := v.Traffic != SupplyTrafficBackup
		if v.Cost.Known && v.Cost.CostToCatalog > 0 {
			if s := get(v.Model); s.CheapestToCatalog == 0 || v.Cost.CostToCatalog < s.CheapestToCatalog {
				s.CheapestToCatalog, s.CheapestAboveQuote = v.Cost.CostToCatalog, v.Cost.AboveQuote
			}
		}
		switch {
		case !v.Cost.Known && serving:
			get(v.Model).CostUnknown++
		case v.Cost.AboveQuote && serving:
			s := get(v.Model)
			s.AboveQuote++
			maxOf(&s.MaxInput, v.Cost.Input)
			maxOf(&s.MaxOutput, v.Cost.Output)
			maxOf(&s.MaxFlat, v.Cost.Flat)
		case v.Cost.AboveQuote:
			get(v.Model).AboveQuoteBackup++
		}
	}
	return out
}

// supplyCostChannelColumns are the channel columns supplyCostOf reads.
var supplyCostChannelColumns = []string{"fee_rate", "cost_ratio"}

// AboveQuoteSnapshot returns the supplies that take traffic and cost more than the quote, by
// supply id, to compare before and after a price sync. nil when the supplies cannot be read.
func AboveQuoteSnapshot() map[int]*ModelSupplyView {
	views, err := GetModelSupplyViews("", 0)
	if err != nil {
		return nil
	}
	out := map[int]*ModelSupplyView{}
	for _, v := range views {
		if v.Cost != nil && v.Cost.AboveQuote && (v.Traffic == SupplyTrafficPrimary || v.Traffic == SupplyTrafficBalanced) {
			out[v.Id] = v
		}
	}
	return out
}

// NotifyNewAboveQuote tells admins about supplies that take traffic and cost more than the quote
// now but did not in before (a snapshot taken by AboveQuoteSnapshot before a price sync).
func NotifyNewAboveQuote(before map[int]*ModelSupplyView) {
	if before == nil {
		return
	}
	after := AboveQuoteSnapshot()
	var lines []string
	for id, v := range after {
		if before[id] != nil {
			continue
		}
		lines = append(lines, "- "+v.Model+" via "+v.ChannelName+": "+describeSupplyCost(v))
	}
	if len(lines) == 0 {
		return
	}
	sort.Strings(lines)
	body := "These supplies now cost more than the model's quoted price and take requests. " +
		"Users pay the quoted price whatever supply serves them, so each of those requests loses " +
		"the difference. Raise the quote, move the supply down, or use a cheaper upstream.\n\n" +
		strings.Join(lines, "\n")
	notify.SendKind(notify.KindSupplyAboveQuote, "Supplies above the quoted price", body)
}

func describeSupplyCost(v *ModelSupplyView) string {
	c := v.Cost
	fee := ""
	if c.FeeRate > 0 {
		fee = fmt.Sprintf(" incl. %.4g%% fee", c.FeeRate*100)
	}
	if c.Flat != nil {
		return fmt.Sprintf("$%.4g%s", *c.Flat, fee)
	}
	in, out := 0.0, 0.0
	if c.Input != nil {
		in = *c.Input
	}
	if c.Output != nil {
		out = *c.Output
	}
	quote := ""
	if v.SellInputPrice != nil && v.SellOutputPrice != nil {
		quote = fmt.Sprintf(" (quote $%.4g / $%.4g)", *v.SellInputPrice, *v.SellOutputPrice)
	}
	return fmt.Sprintf("$%.4g / $%.4g per 1M%s%s", in, out, fee, quote)
}
