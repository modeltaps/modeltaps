package model

import (
	"sort"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
)

// The settings preview of the platform markup: for a markup and a payment cost the admin is
// typing, what each channel needs to break even and which supplies would lose money. It reads
// the same supply costs the price check uses (SupplyCost.CostToCatalog) and changes nothing.

// MarkupRiskChannel is one channel in the preview.
type MarkupRiskChannel struct {
	ChannelId int     `json:"channel_id"`
	Name      string  `json:"name"`
	Type      int     `json:"type"`
	FeeRate   float64 `json:"fee_rate"`
	// FeeBreakeven is the markup, in percent, at which the channel's fee alone breaks even:
	// (1 + fee) / (1 - payment cost) - 1. FeeMargin is the margin left at the previewed markup
	// on a model the channel sells at the catalog price, in percent (negative is a loss).
	FeeBreakeven float64 `json:"fee_breakeven"`
	FeeMargin    float64 `json:"fee_margin"`
	// WorstBreakeven is the markup the channel's dearest supply needs, in percent.
	WorstBreakeven float64 `json:"worst_breakeven"`
	Supplies       int     `json:"supplies"`      // supplies with a known cost that take or back up requests
	ServingAbove   int     `json:"serving_above"` // of those, taking requests and losing money
	BackupAbove    int     `json:"backup_above"`  // backups that would lose money (and be held)
	CostUnknown    int     `json:"cost_unknown"`  // supplies taking requests whose cost is unknown
	Pending        int     `json:"pending"`       // supplies of the channel waiting for review
}

// MarkupRiskSupply is a supply that loses money at the previewed markup.
type MarkupRiskSupply struct {
	Model       string  `json:"model"`
	ChannelName string  `json:"channel_name"`
	Traffic     string  `json:"traffic"`
	CostToCat   float64 `json:"cost_to_catalog"`
	Breakeven   float64 `json:"breakeven"` // markup this supply needs, in percent
}

// MarkupRisk is the whole preview.
type MarkupRisk struct {
	Markup       float64             `json:"markup"`
	PaymentCost  float64             `json:"payment_cost"`
	NetFactor    float64             `json:"net_factor"`
	Channels     []MarkupRiskChannel `json:"channels"`
	ServingAbove int                 `json:"serving_above"`
	BackupAbove  int                 `json:"backup_above"`
	CostUnknown  int                 `json:"cost_unknown"`
	// ModelsAtLoss are models every serving supply of which loses money.
	ModelsAtLoss []string           `json:"models_at_loss"`
	Worst        []MarkupRiskSupply `json:"worst"`
	// SupplyRouting is false when supplies do not route requests (ModelSupplyEnabled off): the
	// figures then describe what would happen once they do.
	SupplyRouting bool `json:"supply_routing"`
}

// breakevenMarkup is the markup, in percent, at which a cost-to-catalog ratio breaks even.
func breakevenMarkup(costToCatalog, paymentPercent float64) float64 {
	keep := 1 - paymentPercent/100
	if keep <= 0 {
		return 0
	}
	return (costToCatalog/keep - 1) * 100
}

// PreviewMarkupRisk works out the preview for a markup and a payment cost, in percent.
func PreviewMarkupRisk(markupPercent, paymentPercent float64) (*MarkupRisk, error) {
	views, err := GetModelSupplyViews("", 0)
	if err != nil {
		return nil, err
	}
	line := config.NetRevenueFactorAt(markupPercent, paymentPercent) * aboveQuoteSlack
	out := &MarkupRisk{
		Markup: markupPercent, PaymentCost: paymentPercent,
		NetFactor:     config.NetRevenueFactorAt(markupPercent, paymentPercent),
		SupplyRouting: config.ModelSupplyEnabled,
	}
	byChannel := map[int]*MarkupRiskChannel{}
	servingByModel := map[string][2]int{} // serving supplies, of which losing money
	for _, v := range views {
		ch := byChannel[v.ChannelId]
		if ch == nil {
			fee := 0.0
			if v.Cost != nil {
				fee = v.Cost.FeeRate
			}
			ch = &MarkupRiskChannel{ChannelId: v.ChannelId, Name: v.ChannelName, Type: v.ChannelType, FeeRate: fee}
			byChannel[v.ChannelId] = ch
		}
		if v.Pending {
			ch.Pending++
		}
		serving := v.Traffic == SupplyTrafficPrimary || v.Traffic == SupplyTrafficBalanced
		if v.Traffic == SupplyTrafficNone {
			continue
		}
		if v.Cost == nil || !v.Cost.Known || v.Cost.CostToCatalog <= 0 {
			if serving {
				ch.CostUnknown++
				out.CostUnknown++
			}
			continue
		}
		if v.Cost.FeeRate > ch.FeeRate {
			ch.FeeRate = v.Cost.FeeRate
		}
		ch.Supplies++
		need := breakevenMarkup(v.Cost.CostToCatalog, paymentPercent)
		if need > ch.WorstBreakeven {
			ch.WorstBreakeven = need
		}
		counts := servingByModel[v.Model]
		if serving {
			counts[0]++
		}
		if v.Cost.CostToCatalog > line {
			out.Worst = append(out.Worst, MarkupRiskSupply{Model: v.Model, ChannelName: v.ChannelName, Traffic: v.Traffic, CostToCat: v.Cost.CostToCatalog, Breakeven: need})
			if serving {
				ch.ServingAbove++
				out.ServingAbove++
				counts[1]++
			} else {
				ch.BackupAbove++
				out.BackupAbove++
			}
		}
		servingByModel[v.Model] = counts
	}
	for model, c := range servingByModel {
		if c[0] > 0 && c[0] == c[1] {
			out.ModelsAtLoss = append(out.ModelsAtLoss, model)
		}
	}
	sort.Strings(out.ModelsAtLoss)
	for _, ch := range byChannel {
		ch.FeeBreakeven = breakevenMarkup(1+ch.FeeRate, paymentPercent)
		ch.FeeMargin = (out.NetFactor/(1+ch.FeeRate) - 1) * 100
		out.Channels = append(out.Channels, *ch)
	}
	sort.Slice(out.Channels, func(i, j int) bool {
		a, b := out.Channels[i], out.Channels[j]
		if a.ServingAbove+a.BackupAbove != b.ServingAbove+b.BackupAbove {
			return a.ServingAbove+a.BackupAbove > b.ServingAbove+b.BackupAbove
		}
		return a.Name < b.Name
	})
	sort.Slice(out.Worst, func(i, j int) bool { return out.Worst[i].CostToCat > out.Worst[j].CostToCat })
	if len(out.Worst) > 20 {
		out.Worst = out.Worst[:20]
	}
	return out, nil
}

// ConfirmPendingSupplies confirms every supply of a channel waiting for review: switched on,
// with the admin's acceptance of its cost recorded. It returns how many were confirmed.
func ConfirmPendingSupplies(channelId int) (int64, error) {
	result := DB.Model(&ModelSupply{}).Where("channel_id = ? AND pending = ?", channelId, true).
		Updates(map[string]any{"enabled": true, "pending": false, "cost_accepted_at": time.Now().Unix()})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected > 0 {
		reloadSupplies()
	}
	return result.RowsAffected, nil
}
