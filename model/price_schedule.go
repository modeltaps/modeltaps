package model

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
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

// officialPriceReview is a day on which a vendor's current list price ends without a published
// successor, as an introductory price "until <date>" whose regular price is not announced. From
// that day admins are told to check the vendor's page, and the price lists are fetched again.
// The entry goes once the new price is known: into scheduledOfficialPrices or the base table.
type officialPriceReview struct {
	Model  string
	On     time.Time
	Source string
}

func officialPriceReviews() []officialPriceReview {
	// Promotional pricing "available until November 30, 2026"; the regular price is not
	// published (checked 2026-11-19).
	const gpt56Sol = "https://developers.openai.com/api/docs/models/gpt-5.6-sol"
	return []officialPriceReview{
		{"gpt-5.6-sol", priceChangeDate(2026, time.December, 1), gpt56Sol},
		{"gpt-5.6", priceChangeDate(2026, time.December, 1), gpt56Sol},
	}
}

// scheduledPriceSubject is the alert subject of official price dates, under the price change kind.
const scheduledPriceSubject = "source:official schedule"

// scheduledPriceWindow is how long a scheduled change that took effect stays in the alert.
const scheduledPriceWindow = 14 * 24 * time.Hour

// scheduledPriceItems lists the official price dates reached for the models users are served:
// "scheduled|model|old in|old out|new in|new out" (USD per 1M tokens) for a change of the last
// scheduledPriceWindow, and "review|model|day|source" for a price that ended with no successor
// published, kept until a release replaces the review.
func scheduledPriceItems(now time.Time, served func(string) bool) []string {
	var items []string
	base := officialTokenPrices()
	for _, change := range sortedScheduledPrices() {
		previous, ok := base[change.Model]
		base[change.Model] = change.Price
		if !ok || now.Before(change.From) || now.Sub(change.From) >= scheduledPriceWindow || !served(change.Model) {
			continue
		}
		items = append(items, strings.Join([]string{"scheduled", change.Model,
			num(previous.input), num(previous.output), num(change.Price.input), num(change.Price.output)}, "|"))
	}
	for _, review := range officialPriceReviews() {
		if now.Before(review.On) || !served(review.Model) {
			continue
		}
		items = append(items, strings.Join([]string{"review", review.Model, review.On.Format("2006-01-02"), review.Source}, "|"))
	}
	return items
}

// ReportScheduledPriceDates keeps the alert of official price dates reached and tells admins
// about the new ones. It returns the new items: when there are any, the caller fetches the price
// lists again, since upstream lists follow a vendor's change on the day.
func ReportScheduledPriceDates(now time.Time) ([]string, error) {
	items := scheduledPriceItems(now, servedModelSet())
	fresh, err := ReportAlert(notify.KindPriceChange, scheduledPriceSubject, items)
	if err != nil || len(fresh) == 0 {
		return fresh, err
	}
	var b strings.Builder
	for _, item := range fresh {
		parts := strings.Split(item, "|")
		switch parts[0] {
		case "scheduled":
			b.WriteString("- " + parts[1] + ": official price changed on schedule, input $" + parts[2] + " → $" + parts[4] +
				", output $" + parts[3] + " → $" + parts[5] + " per 1M tokens\n")
		case "review":
			b.WriteString("- " + parts[1] + ": the official price ended on " + parts[2] +
				" and no new price is published yet; check " + parts[3] + " and set the price by hand if it changed\n")
		}
	}
	notify.SendKind(notify.KindPriceChange, "Official price dates reached", b.String())
	return fresh, nil
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
	return aLC.Threshold == bLC.Threshold && sameRatio(aLC.InputRatio, bLC.InputRatio) && sameRatio(aLC.OutputRatio, bLC.OutputRatio) &&
		sameOffPeak(a, b)
}

func sameRatio(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
