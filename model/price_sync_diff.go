package model

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
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
		case existing.Type != price.Type || math.Abs(priceCacheRead(existing)-priceCacheRead(price)) > priceEpsilon:
			change.Kind = PriceChangeOther
		default:
			change.Kind = PriceChangeUnchanged
		}
		changes = append(changes, change)
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Model < changes[j].Model })
	return changes
}

// UpsertSelectedPrices writes only the named models from prices (locked rows are kept).
func (p *Pricing) UpsertSelectedPrices(prices []*Price, models []string) (int, error) {
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

var (
	priceSyncNotifyMu   sync.Mutex
	priceSyncNotifySent = map[string]string{} // source -> signature of the last notice
)

// notifyPriceSync tells admins about applied increases, held changes and underpriced locked
// models that an enabled channel serves (catalog lists carry thousands of unused models).
// The same set of changes from one source is notified once.
func notifyPriceSync(report *PriceSyncReport) {
	served := servedModelSet()
	var raised []PriceChange
	for _, c := range report.Applied {
		if c.Kind == PriceChangeRaised && served(c.Model) {
			raised = append(raised, c)
		}
	}
	held := filterServed(report.Held, served)
	locked := filterServed(report.Locked, served)
	if len(raised) == 0 && len(held) == 0 && len(locked) == 0 {
		return
	}
	var b strings.Builder
	writeChanges(&b, "Price increases applied", raised)
	writeChanges(&b, "Price changes waiting for review (apply them from the channel's price sync)", held)
	writeChanges(&b, "Locked prices below the upstream price (requests on these models may lose money)", locked)
	body := b.String()

	priceSyncNotifyMu.Lock()
	if priceSyncNotifySent[report.Source] == body {
		priceSyncNotifyMu.Unlock()
		return
	}
	priceSyncNotifySent[report.Source] = body
	priceSyncNotifyMu.Unlock()

	logger.SysLog(fmt.Sprintf("price sync %s: %d increases applied, %d held, %d locked below upstream",
		report.Source, len(raised), len(held), len(locked)))
	notify.Send("Upstream price changes: "+report.Source, body)
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

// servedModelSet reports whether an enabled channel serves the model. When the channel list
// cannot be read every model counts as served, so no notice is lost.
func servedModelSet() func(string) bool {
	names, err := EnabledChannelModelNames()
	if err != nil {
		return func(string) bool { return true }
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
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
	for _, symbol := range strings.Split(channel.Group, ",") {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}
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
