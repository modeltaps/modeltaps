package model

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
)

// Price change kinds between the price table and an upstream price list.
const (
	PriceChangeNew       = "new"       // not in the price table yet
	PriceChangeRaised    = "raised"    // upstream input or output price went up
	PriceChangeLowered   = "lowered"   // upstream prices went down (none went up)
	PriceChangeOther     = "changed"   // billing type or cache price differs, token prices equal
	PriceChangeUnchanged = "unchanged" // identical
)

// PriceChange compares one upstream price with the price table. Prices are in USD per
// million tokens (per call for times billing) so the admin reads the provider's numbers.
type PriceChange struct {
	Model     string  `json:"model"`
	Kind      string  `json:"kind"`
	Locked    bool    `json:"locked"`
	OldInput  float64 `json:"old_input"`
	OldOutput float64 `json:"old_output"`
	NewInput  float64 `json:"new_input"`
	NewOutput float64 `json:"new_output"`

	price *Price
}

// PriceSyncAutoApply* are the values of config.PriceSyncAutoApply, which decides what an
// automatic sync (startup, daily cron, channel save) writes for models already priced.
const (
	PriceSyncAutoApplyAll       = "all"       // write every change
	PriceSyncAutoApplyIncreases = "increases" // write increases, hold decreases for review
	PriceSyncAutoApplyNone      = "none"      // write new models only, hold every change
)

func ratioToUSDPerMillion(ratio float64) float64 {
	return math.Round(ratio*DollarRate*1000*1e6) / 1e6
}

func priceCacheRead(p *Price) float64 {
	if p == nil || p.ExtraRatios == nil {
		return 0
	}
	return p.ExtraRatios.Data()[config.UsageExtraCachedRead]
}

const priceEpsilon = 1e-9

// DiffPrices compares upstream prices with the price table, sorted by model name.
func (p *Pricing) DiffPrices(prices []*Price) []PriceChange {
	changes := make([]PriceChange, 0, len(prices))
	for _, price := range prices {
		if price == nil || price.Model == "" {
			continue
		}
		change := PriceChange{
			Model:     price.Model,
			NewInput:  ratioToUSDPerMillion(price.Input),
			NewOutput: ratioToUSDPerMillion(price.Output),
			price:     price,
		}
		existing, ok := p.priceSnapshot(price.Model)
		if !ok {
			change.Kind = PriceChangeNew
			changes = append(changes, change)
			continue
		}
		change.Locked = existing.Locked
		change.OldInput = ratioToUSDPerMillion(existing.Input)
		change.OldOutput = ratioToUSDPerMillion(existing.Output)
		inDelta, outDelta := price.Input-existing.Input, price.Output-existing.Output
		switch {
		case inDelta > priceEpsilon || outDelta > priceEpsilon:
			change.Kind = PriceChangeRaised
		case inDelta < -priceEpsilon || outDelta < -priceEpsilon:
			change.Kind = PriceChangeLowered
		case existing.Type != price.Type || math.Abs(priceCacheRead(existing)-priceCacheRead(price)) > priceEpsilon ||
			!sameOffPeak(existing, price):
			change.Kind = PriceChangeOther
		default:
			change.Kind = PriceChangeUnchanged
		}
		changes = append(changes, change)
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Model < changes[j].Model })
	return changes
}

// SelectPrices keeps the prices of the named models.
func SelectPrices(prices []*Price, models []string) []*Price {
	want := make(map[string]bool, len(models))
	for _, m := range models {
		want[m] = true
	}
	selected := make([]*Price, 0, len(models))
	for _, price := range prices {
		if price != nil && want[price.Model] {
			selected = append(selected, price)
		}
	}
	return selected
}

// UpsertSelectedPrices writes only the named models from prices (locked rows are kept).
func (p *Pricing) UpsertSelectedPrices(prices []*Price, models []string) (int, error) {
	selected := SelectPrices(prices, models)
	if len(selected) == 0 {
		return 0, nil
	}
	return len(selected), p.UpsertPrices(selected)
}

// PriceSyncReport is what an automatic sync wrote and what it held back for review.
type PriceSyncReport struct {
	Source  string
	Applied []PriceChange
	Held    []PriceChange
	Locked  []PriceChange // locked rows the upstream now prices higher than the table
}

// ApplyAutoPriceSync writes an automatic price sync under config.PriceSyncAutoApply: new
// models always, existing unlocked models as the policy allows. Changes it does not write
// and locked models whose upstream price rose are notified (once per distinct set).
func (p *Pricing) ApplyAutoPriceSync(prices []*Price, source string) (*PriceSyncReport, error) {
	report := &PriceSyncReport{Source: source}
	var write []*Price
	for _, change := range p.DiffPrices(prices) {
		switch {
		case change.Kind == PriceChangeUnchanged:
		case change.Locked:
			if change.Kind == PriceChangeRaised {
				report.Locked = append(report.Locked, change)
			}
		case change.Kind == PriceChangeNew || autoApplies(change.Kind):
			report.Applied = append(report.Applied, change)
			write = append(write, change.price)
		default:
			report.Held = append(report.Held, change)
		}
	}
	if len(write) > 0 {
		if err := p.UpsertPrices(write); err != nil {
			return report, err
		}
	}
	notifyPriceSync(report)
	return report, nil
}

func autoApplies(kind string) bool {
	switch config.PriceSyncAutoApply {
	case PriceSyncAutoApplyAll:
		return true
	case PriceSyncAutoApplyNone:
		return false
	default:
		return kind == PriceChangeRaised
	}
}

func writeChanges(b *strings.Builder, title string, changes []PriceChange) {
	if len(changes) == 0 {
		return
	}
	fmt.Fprintf(b, "**%s** (%d)\n\n", title, len(changes))
	for i, c := range changes {
		if i == 30 {
			fmt.Fprintf(b, "- … and %d more\n", len(changes)-i)
			break
		}
		fmt.Fprintf(b, "- %s: input $%g → $%g, output $%g → $%g per 1M tokens\n",
			c.Model, c.OldInput, c.NewInput, c.OldOutput, c.NewOutput)
	}
	b.WriteString("\n")
}

// servedModelSet reports whether users are served the model (see ServedModelNames). When that
// cannot be read every model counts as served, so no notice is lost.
func servedModelSet() func(string) bool {
	set, err := ServedModelNames()
	if err != nil {
		return func(string) bool { return true }
	}
	return func(m string) bool { return set[m] }
}

func filterServed(changes []PriceChange, served func(string) bool) []PriceChange {
	var out []PriceChange
	for _, c := range changes {
		if served(c.Model) {
			out = append(out, c)
		}
	}
	return out
}

// writeSyncedPrices writes a synced price list: an automatic sync follows PriceSyncAutoApply,
// a sync an admin started writes everything (locked rows are always kept).
func writeSyncedPrices(prices []*Price, source string, auto bool) error {
	if !auto {
		return PricingInstance.UpsertPrices(prices)
	}
	_, err := PricingInstance.ApplyAutoPriceSync(prices, source)
	return err
}

// GroupLoss is a user group whose price multiplier is below the channel's cost ratio: every
// request that group sends through the channel is sold for less than it costs.
type GroupLoss struct {
	Group      string  `json:"group"`
	GroupRatio float64 `json:"group_ratio"`
	CostRatio  float64 `json:"cost_ratio"`
}

// ChannelGroupLosses lists the channel's groups priced below its cost ratio. A channel
// without a cost ratio has no known cost and reports nothing.
func ChannelGroupLosses(channel *Channel) []GroupLoss {
	if channel == nil || channel.CostRatio == nil || *channel.CostRatio <= 0 {
		return nil
	}
	cost := *channel.CostRatio
	var losses []GroupLoss
	for _, symbol := range ExpandChannelGroup(channel.Group) {
		group := GlobalUserGroupRatio.GetBySymbol(symbol)
		if group == nil {
			continue
		}
		if group.Ratio+priceEpsilon < cost {
			losses = append(losses, GroupLoss{Group: symbol, GroupRatio: group.Ratio, CostRatio: cost})
		}
	}
	return losses
}
