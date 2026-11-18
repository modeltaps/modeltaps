package model

import (
	"sort"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
)

// The settings preview of the site's pricing: for the default group's ratio and a payment cost
// the admin is trying, what each channel needs to break even and which supplies would lose money,
// and what every user group brings in. It reads the same supply costs the price check uses
// (SupplyCost.CostToCatalog) and changes nothing.

// PricingRiskChannel is one channel in the preview.
type PricingRiskChannel struct {
	ChannelId int     `json:"channel_id"`
	Name      string  `json:"name"`
	Type      int     `json:"type"`
	FeeRate   float64 `json:"fee_rate"`
	// FeeBreakeven is the group ratio at which the channel's fee alone breaks even:
	// (1 + fee) / (1 - payment cost). FeeMargin is the margin left at the previewed ratio on a
	// model the channel sells at the catalog price, in percent (negative is a loss).
	FeeBreakeven float64 `json:"fee_breakeven"`
	FeeMargin    float64 `json:"fee_margin"`
	// WorstBreakeven is the group ratio the channel's dearest supply needs.
	WorstBreakeven float64 `json:"worst_breakeven"`
	Supplies       int     `json:"supplies"`      // supplies with a known cost that take or back up requests
	ServingAbove   int     `json:"serving_above"` // of those, taking requests and losing money
	BackupAbove    int     `json:"backup_above"`  // backups that would lose money (and be held)
	CostUnknown    int     `json:"cost_unknown"`  // supplies taking requests whose cost is unknown
	Pending        int     `json:"pending"`       // supplies of the channel waiting for review
}

// PricingRiskSupply is a supply that loses money at the previewed ratio.
type PricingRiskSupply struct {
	Model       string  `json:"model"`
	ChannelName string  `json:"channel_name"`
	Traffic     string  `json:"traffic"`
	CostToCat   float64 `json:"cost_to_catalog"`
	Breakeven   float64 `json:"breakeven"` // group ratio this supply needs
}

// PricingRiskGroup is one user group: what its ratio brings in after the payment cost, and how
// many channels lose money on it through their fee alone.
type PricingRiskGroup struct {
	Symbol    string  `json:"symbol"`
	Name      string  `json:"name"`
	Ratio     float64 `json:"ratio"`
	NetFactor float64 `json:"net_factor"`
	// LosingChannels counts the channels whose fee is above what the group brings in.
	LosingChannels int `json:"losing_channels"`
}

// PricingRisk is the whole preview.
type PricingRisk struct {
	// GroupRatio is the default group's ratio previewed: the site's normal price over the
	// catalog price.
	GroupRatio   float64             `json:"group_ratio"`
	PaymentCost  float64             `json:"payment_cost"`
	NetFactor    float64             `json:"net_factor"`
	Channels     []PricingRiskChannel `json:"channels"`
	ServingAbove int                 `json:"serving_above"`
	BackupAbove  int                 `json:"backup_above"`
	CostUnknown  int                 `json:"cost_unknown"`
	// ModelsAtLoss are models every serving supply of which loses money.
	ModelsAtLoss []string           `json:"models_at_loss"`
	Worst        []PricingRiskSupply `json:"worst"`
	Groups       []PricingRiskGroup  `json:"groups"`
	// SupplyRouting is false when supplies do not route requests (ModelSupplyEnabled off): the
	// figures then describe what would happen once they do.
	SupplyRouting bool `json:"supply_routing"`
}

// breakevenRatio is the group ratio at which a cost-to-catalog ratio breaks even.
func breakevenRatio(costToCatalog, paymentPercent float64) float64 {
	keep := 1 - paymentPercent/100
	if keep <= 0 {
		return 0
	}
	return costToCatalog / keep
}

// PreviewPricingRisk works out the preview for a default group ratio and a payment cost (percent).
func PreviewPricingRisk(groupRatio, paymentPercent float64) (*PricingRisk, error) {
	views, err := GetModelSupplyViews("", 0)
	if err != nil {
		return nil, err
	}
	net := NetRevenueLineAt(groupRatio, paymentPercent)
	line := net * aboveQuoteSlack
	out := &PricingRisk{
		GroupRatio: groupRatio, PaymentCost: paymentPercent,
		NetFactor:     net,
		SupplyRouting: config.ModelSupplyEnabled,
	}
	byChannel := map[int]*PricingRiskChannel{}
	servingByModel := map[string][2]int{} // serving supplies, of which losing money
	for _, v := range views {
		ch := byChannel[v.ChannelId]
		if ch == nil {
			fee := 0.0
			if v.Cost != nil {
				fee = v.Cost.FeeRate
			}
			ch = &PricingRiskChannel{ChannelId: v.ChannelId, Name: v.ChannelName, Type: v.ChannelType, FeeRate: fee}
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
		need := breakevenRatio(v.Cost.CostToCatalog, paymentPercent)
		if need > ch.WorstBreakeven {
			ch.WorstBreakeven = need
		}
		counts := servingByModel[v.Model]
		if serving {
			counts[0]++
		}
		if v.Cost.CostToCatalog > line {
			out.Worst = append(out.Worst, PricingRiskSupply{Model: v.Model, ChannelName: v.ChannelName, Traffic: v.Traffic, CostToCat: v.Cost.CostToCatalog, Breakeven: need})
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
		ch.FeeBreakeven = breakevenRatio(1+ch.FeeRate, paymentPercent)
		ch.FeeMargin = (out.NetFactor/(1+ch.FeeRate) - 1) * 100
		out.Channels = append(out.Channels, *ch)
	}
	for symbol, g := range GlobalUserGroupRatio.GetAll() {
		if g == nil {
			continue
		}
		pg := PricingRiskGroup{Symbol: symbol, Name: g.Name, Ratio: g.Ratio, NetFactor: NetRevenueLineAt(g.Ratio, paymentPercent)}
		if symbol == DefaultUserGroupSymbol {
			pg.Ratio, pg.NetFactor = groupRatio, net
		}
		for _, ch := range out.Channels {
			if 1+ch.FeeRate > pg.NetFactor*aboveQuoteSlack {
				pg.LosingChannels++
			}
		}
		out.Groups = append(out.Groups, pg)
	}
	sort.Slice(out.Groups, func(i, j int) bool {
		if out.Groups[i].Symbol == DefaultUserGroupSymbol || out.Groups[j].Symbol == DefaultUserGroupSymbol {
			return out.Groups[i].Symbol == DefaultUserGroupSymbol
		}
		return out.Groups[i].Ratio < out.Groups[j].Ratio
	})
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
