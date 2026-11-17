package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/modeltaps/modeltaps/common/notify"
)

// A supply that a channel adds on its own (a new channel, or a new upstream model found by the
// daily check) never starts routing when it costs more than the model's quoted price: it waits
// for an admin, whatever the intake rules say, and admins are told. Accepting it, raising the
// quote or keeping it off is theirs to decide. A supply whose cost cannot be told follows the
// rules as before (most relays have no price list), and the console says the cost is unknown.

// IntakeRuleAboveQuote is the rule name an outcome carries when the price check, not an intake
// rule, decided it.
const IntakeRuleAboveQuote = "Costs more than the quoted price"

// supplyCostOverQuote is a new supply held back because its channel costs more than the quote.
type supplyCostOverQuote struct {
	model, upstream string
	cost            *SupplyCost
	quoteIn         float64
	quoteOut        float64
	quoteFlat       float64
}

// markCostAboveQuote sets costAbove on the candidates whose channel cost is above the model's
// quoted price. It reads prices through the global connection, so it runs before any transaction.
func markCostAboveQuote(channel *Channel, candidates []autoSupplyCandidate) {
	for i := range candidates {
		c := &candidates[i]
		cost := supplyCostOf(c.model, c.upstream, channel, CachedChannelPrice(channel.Id, c.upstream))
		if cost.Known && cost.AboveQuote {
			c.costAbove = cost
		}
	}
}

func overQuoteOf(c autoSupplyCandidate) supplyCostOverQuote {
	held := supplyCostOverQuote{model: c.model, upstream: c.upstream, cost: c.costAbove}
	if PricingInstance != nil {
		if quote := PricingInstance.GetBillingPrice(c.model); quote != nil {
			switch quote.Type {
			case TimesPriceType, MinutesPriceType:
				held.quoteFlat = quote.GetInput() * DollarRate
			default:
				held.quoteIn = *tokenPricePerMillion(quote.GetInput())
				held.quoteOut = *tokenPricePerMillion(quote.GetOutput())
			}
		}
	}
	return held
}

// notifyHeldAboveQuote tells admins about new supplies of a channel that wait for review because
// they cost more than the quote.
func notifyHeldAboveQuote(channel *Channel, held []supplyCostOverQuote) {
	if channel == nil || len(held) == 0 {
		return
	}
	lines := make([]string, 0, len(held))
	for _, h := range held {
		name := h.model
		if h.upstream != "" && h.upstream != h.model {
			name += " (" + h.upstream + ")"
		}
		lines = append(lines, "- "+name+": "+describeHeldCost(h))
	}
	sort.Strings(lines)
	body := fmt.Sprintf("Channel \"%s\" (#%d) now offers these models, but at more than their quoted price, "+
		"so the new supplies are switched off and wait for review on each model's Supplies tab. "+
		"Confirm a supply to accept the cost (with the cost floor on, its requests are charged the channel "+
		"cost; otherwise the difference is lost), raise the quote first, or keep it off.\n\n%s",
		channel.Name, channel.Id, strings.Join(lines, "\n"))
	notify.SendKind(notify.KindSupplyAboveQuote, "New supplies above the quoted price wait for review", body)
}

func describeHeldCost(h supplyCostOverQuote) string {
	c := h.cost
	fee := ""
	if c.FeeRate > 0 {
		fee = fmt.Sprintf(" incl. %.4g%% fee", c.FeeRate*100)
	}
	if c.Flat != nil {
		return fmt.Sprintf("costs $%.4g%s, quoted $%.4g", *c.Flat, fee, h.quoteFlat)
	}
	in, out := 0.0, 0.0
	if c.Input != nil {
		in = *c.Input
	}
	if c.Output != nil {
		out = *c.Output
	}
	return fmt.Sprintf("costs $%.4g / $%.4g per 1M%s, quoted $%.4g / $%.4g", in, out, fee, h.quoteIn, h.quoteOut)
}
