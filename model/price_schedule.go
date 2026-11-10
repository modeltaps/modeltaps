package model

import (
	"math"
	"sort"
	"time"

	"github.com/modeltaps/modeltaps/common/logger"
)

// scheduledOfficialPrice is an announced change of a vendor's list price: from the start of From
// (UTC) the model is billed at Price instead of the entry in officialTokenPrices.
type scheduledOfficialPrice struct {
	Model string
	From  time.Time
	Price officialTokenPrice
}

func priceChangeDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// scheduledOfficialPrices lists announced price changes, oldest first per model. When a vendor's
// pricing page quotes an introductory price "through <date>" and a new price "starting <date>",
// the current price stays in officialTokenPrices and the new one goes here, so the switch happens
// on the day without a release.
func scheduledOfficialPrices() []scheduledOfficialPrice {
	return []scheduledOfficialPrice{
		// https://ai.google.dev/gemini-api/docs/pricing (checked 2026-11-05)
		{"gemini-4-argon", priceChangeDate(2026, time.December, 1), geminiPrice(4, 0.2, 0, 20, 0, nil)},
		{"gemini-3.8-flash", priceChangeDate(2027, time.January, 1), geminiPrice(1.5, 0.15, 0, 7.5, 0, nil)},
		{"gemini-3.6-flash", priceChangeDate(2027, time.January, 1), geminiPrice(1.5, 0.15, 0, 7.5, 0, nil)},
	}
}

// officialTokenPricesAt returns the official list prices in effect at now: the base table with
// every scheduled change that has started applied on top.
func officialTokenPricesAt(now time.Time) map[string]officialTokenPrice {
	prices := officialTokenPrices()
	for _, change := range sortedScheduledPrices() {
		if !now.Before(change.From) {
			prices[change.Model] = change.Price
		}
	}
	return prices
}

func sortedScheduledPrices() []scheduledOfficialPrice {
	changes := scheduledOfficialPrices()
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].From.Before(changes[j].From) })
	return changes
}

// ApplyScheduledPriceChanges moves stored prices to the announced official price once its date has
// passed. A stored price is only changed while it still equals the official price that was in
// effect just before the change, so prices an administrator edited or locked are left alone, and
// running it again is a no-op. It returns the models that were updated.
func ApplyScheduledPriceChanges(now time.Time) ([]string, error) {
	base := officialTokenPrices()
	var updated []string
	for _, change := range sortedScheduledPrices() {
		if now.Before(change.From) {
			continue
		}
		previous, ok := base[change.Model]
		base[change.Model] = change.Price
		if !ok {
			continue
		}

		var stored Price
		result := DB.Where("model = ?", change.Model).Limit(1).Find(&stored)
		if result.Error != nil {
			return updated, result.Error
		}
		if result.RowsAffected == 0 || stored.Locked || !samePrice(&stored, previous.toPrice(change.Model)) {
			continue
		}

		next := change.Price.toPrice(change.Model)
		if err := next.Update(change.Model); err != nil {
			return updated, err
		}
		updated = append(updated, change.Model)
		logger.SysLog("scheduled price change applied: " + change.Model + " from " + change.From.Format("2006-01-02"))
	}

	if len(updated) > 0 && PricingInstance != nil {
		if err := PricingInstance.Init(); err != nil {
			return updated, err
		}
	}
	return updated, nil
}

// samePrice reports whether two token prices bill identically.
func samePrice(a, b *Price) bool {
	if a.Type != b.Type || !sameRatio(a.Input, b.Input) || !sameRatio(a.Output, b.Output) {
		return false
	}
	aExtra, bExtra := map[string]float64{}, map[string]float64{}
	if a.ExtraRatios != nil {
		aExtra = a.ExtraRatios.Data()
	}
	if b.ExtraRatios != nil {
		bExtra = b.ExtraRatios.Data()
	}
	if len(aExtra) != len(bExtra) {
		return false
	}
	for key, value := range aExtra {
		other, ok := bExtra[key]
		if !ok || !sameRatio(value, other) {
			return false
		}
	}
	var aLC, bLC LongContextTier
	if a.LongContext != nil {
		aLC = a.LongContext.Data()
	}
	if b.LongContext != nil {
		bLC = b.LongContext.Data()
	}
	return aLC.Threshold == bLC.Threshold && sameRatio(aLC.InputRatio, bLC.InputRatio) && sameRatio(aLC.OutputRatio, bLC.OutputRatio)
}

func sameRatio(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
