package model

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
)

// A price change is worth telling when the input or output price moves by at least this share
// of the old price. Smaller moves are still applied or held as the sync policy says, quietly.
const priceAlertThreshold = 0.10

// priceAlertPending is the alert kind that keeps, per source, the significant changes the last
// sync found: a change is told once the next sync still finds it, so an upstream price that
// goes up and comes back between two syncs is never told.
const priceAlertPending = "price_change_pending"

// significantPriceChange reports whether the input or output price moved by priceAlertThreshold
// or more (any move from a price of 0 counts).
func significantPriceChange(c PriceChange) bool {
	moved := func(old, now float64) bool {
		if old <= 0 {
			return now > 0
		}
		return math.Abs(now-old)/old >= priceAlertThreshold
	}
	return moved(c.OldInput, c.NewInput) || moved(c.OldOutput, c.NewOutput)
}

// priceAlertItem names one change: tag|model|old input|old output|new input|new output. The
// same change from the next sync gives the same item, and an applied increase can be read back.
func priceAlertItem(tag string, c PriceChange) string {
	return strings.Join([]string{tag, c.Model, num(c.OldInput), num(c.OldOutput), num(c.NewInput), num(c.NewOutput)}, "|")
}

func num(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// parsePriceAlertItem reads an item back; ok is false for an item of another shape.
func parsePriceAlertItem(item string) (tag string, c PriceChange, ok bool) {
	parts := strings.Split(item, "|")
	if len(parts) != 6 {
		return "", c, false
	}
	vals := make([]float64, 4)
	for i, p := range parts[2:] {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return "", c, false
		}
		vals[i] = v
	}
	return parts[0], PriceChange{Model: parts[1], Kind: PriceChangeRaised, OldInput: vals[0], OldOutput: vals[1], NewInput: vals[2], NewOutput: vals[3]}, true
}

func sameAmount(a, b float64) bool { return math.Abs(a-b) <= 1e-6*math.Max(1, math.Abs(a)) }

// priceAlertPlan is what one sync means for the admin, worked out from the report and the
// changes the previous sync left pending.
type priceAlertPlan struct {
	pending []string // significant changes of this sync, to confirm at the next one
	raised  []PriceChange
	held    []PriceChange
	locked  []PriceChange
	// revert are held price cuts that bring a model back to its price before an increase the
	// previous sync applied: the increase was a passing spike, so it is undone quietly.
	revert []*Price
}

// planPriceAlert decides what a sync tells. Only models users are served count, and only
// changes of priceAlertThreshold or more. Held changes and applied increases are told when the
// next sync still finds them (an increase stands when the next sync does not bring the price
// back); locked models now priced below the upstream lose money on every request and are told
// at once.
func planPriceAlert(report *PriceSyncReport, served func(string) bool, prevPending []string) priceAlertPlan {
	var plan priceAlertPlan
	keep := func(c PriceChange) bool { return served(c.Model) && significantPriceChange(c) }
	cuts := map[string]PriceChange{}
	for _, c := range append(append([]PriceChange(nil), report.Held...), report.Applied...) {
		if c.Kind == PriceChangeLowered {
			cuts[c.Model] = c
		}
	}
	reverted := map[string]bool{}
	for _, item := range prevPending {
		tag, r, ok := parsePriceAlertItem(item)
		if !ok || tag != "raised" {
			continue
		}
		if cut, ok := cuts[r.Model]; ok && sameAmount(cut.NewInput, r.OldInput) && sameAmount(cut.NewOutput, r.OldOutput) {
			reverted[r.Model] = true
			if slices.ContainsFunc(report.Held, func(h PriceChange) bool { return h.Model == r.Model }) && cut.price != nil {
				plan.revert = append(plan.revert, cut.price)
			}
			continue
		}
		plan.raised = append(plan.raised, r)
	}
	for _, c := range report.Applied {
		if c.Kind == PriceChangeRaised && keep(c) {
			plan.pending = append(plan.pending, priceAlertItem("raised", c))
		}
	}
	for _, c := range report.Held {
		if reverted[c.Model] || !keep(c) {
			continue
		}
		item := priceAlertItem("held", c)
		plan.pending = append(plan.pending, item)
		if slices.Contains(prevPending, item) {
			plan.held = append(plan.held, c)
		}
	}
	for _, c := range report.Locked {
		if served(c.Model) {
			plan.locked = append(plan.locked, c)
		}
	}
	return plan
}

func (plan priceAlertPlan) items() []string {
	var out []string
	for _, c := range plan.raised {
		out = append(out, priceAlertItem("raised", c))
	}
	for _, c := range plan.held {
		out = append(out, priceAlertItem("held", c))
	}
	for _, c := range plan.locked {
		out = append(out, priceAlertItem("locked", c))
	}
	return out
}

// notifyPriceSync tells admins what a sync changed for the models users are served (see
// planPriceAlert): increases that stood, cuts waiting for review and locked prices below the
// upstream. The told changes of a source are kept as its price alert, so each is told once,
// also across restarts, and a source with nothing left to say closes its alert.
func notifyPriceSync(report *PriceSyncReport) {
	subject := "source:" + report.Source
	prev, err := AlertItems(priceAlertPending, subject)
	if err != nil {
		logger.SysError("price change alert failed: " + err.Error())
	}
	plan := planPriceAlert(report, servedModelSet(), prev)
	if len(plan.revert) > 0 && PricingInstance != nil {
		if err := PricingInstance.UpsertPrices(plan.revert); err != nil {
			logger.SysError("price spike revert failed: " + err.Error())
		} else {
			logger.SysLog(fmt.Sprintf("price sync %s: %d upstream increase(s) came back before they were confirmed; restored the earlier prices", report.Source, len(plan.revert)))
		}
	}
	if _, err := ReportAlert(priceAlertPending, subject, plan.pending); err != nil {
		logger.SysError("price change alert failed: " + err.Error())
	}
	items := plan.items()
	fresh, err := ReportAlert(notify.KindPriceChange, subject, items)
	if err != nil {
		logger.SysError("price change alert failed: " + err.Error())
		fresh = items
	}
	if len(fresh) == 0 {
		return
	}
	var b strings.Builder
	writeChanges(&b, "Price increases applied", plan.raised)
	writeChanges(&b, "Price changes waiting for review (apply them from the channel's price sync)", plan.held)
	writeChanges(&b, "Locked prices below the upstream price (requests on these models may lose money)", plan.locked)
	logger.SysLog(fmt.Sprintf("price sync %s: %d increases applied, %d held, %d locked below upstream",
		report.Source, len(plan.raised), len(plan.held), len(plan.locked)))
	notify.SendKind(notify.KindPriceChange, "Upstream price changes: "+report.Source, b.String())
}
