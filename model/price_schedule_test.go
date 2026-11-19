package model

import (
	"testing"
	"time"
)

func TestScheduledOfficialPricesAreValid(t *testing.T) {
	base := officialTokenPrices()
	for _, change := range scheduledOfficialPrices() {
		if _, ok := base[change.Model]; !ok {
			t.Errorf("%s: a scheduled change needs the current price in officialTokenPrices", change.Model)
		}
		if change.From.Location() != time.UTC || change.From.Hour() != 0 {
			t.Errorf("%s: change must start at 00:00 UTC, got %v", change.Model, change.From)
		}
		if change.Price.input <= 0 || change.Price.output <= 0 || change.Price.channelType != base[change.Model].channelType {
			t.Errorf("%s: invalid scheduled price %+v", change.Model, change.Price)
		}
	}
}

func TestOfficialTokenPricesAt(t *testing.T) {
	before := officialTokenPricesAt(time.Date(2026, time.November, 30, 23, 59, 59, 0, time.UTC))
	after := officialTokenPricesAt(time.Date(2026, time.December, 1, 0, 0, 0, 0, time.UTC))
	if got := before["gemini-4-argon"]; got.input != 2 || got.output != 10 {
		t.Fatalf("gemini-4-argon before 2026-12-01 should be $2 / $10, got %v / %v", got.input, got.output)
	}
	if got := after["gemini-4-argon"]; got.input != 4 || got.output != 20 {
		t.Fatalf("gemini-4-argon from 2026-12-01 should be $4 / $20, got %v / %v", got.input, got.output)
	}
	if got := after["gemini-3.8-flash"]; got.input != 0.75 {
		t.Fatalf("gemini-3.8-flash changes only in 2027, got %v", got.input)
	}
	if got := officialTokenPricesAt(time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC))["gemini-3.8-flash"]; got.input != 1.5 || got.output != 7.5 {
		t.Fatalf("gemini-3.8-flash from 2027-01-01 should be $1.50 / $7.50, got %v / %v", got.input, got.output)
	}
}

func TestApplyScheduledPriceChanges(t *testing.T) {
	setupModelInfoTestDB(t)
	if err := DB.AutoMigrate(&Price{}); err != nil {
		t.Fatal(err)
	}
	oldPricing := PricingInstance
	PricingInstance = &Pricing{Prices: map[string]*Price{}}
	t.Cleanup(func() { PricingInstance = oldPricing })

	base := officialTokenPrices()
	// Still at the old official price: follows the change.
	if err := base["gemini-4-argon"].toPrice("gemini-4-argon").Insert(); err != nil {
		t.Fatal(err)
	}
	// Edited by an administrator: left alone.
	edited := base["gemini-3.8-flash"].toPrice("gemini-3.8-flash")
	edited.Input = 0.5
	if err := edited.Insert(); err != nil {
		t.Fatal(err)
	}
	// Locked at the old price: left alone.
	locked := base["gemini-3.6-flash"].toPrice("gemini-3.6-flash")
	locked.Locked = true
	if err := locked.Insert(); err != nil {
		t.Fatal(err)
	}

	updated, err := ApplyScheduledPriceChanges(time.Date(2026, time.November, 30, 12, 0, 0, 0, time.UTC))
	if err != nil || len(updated) != 0 {
		t.Fatalf("nothing should change before the date, got %v err=%v", updated, err)
	}

	updated, err = ApplyScheduledPriceChanges(time.Date(2027, time.January, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(updated) != 1 || updated[0] != "gemini-4-argon" {
		t.Fatalf("only the untouched gemini-4-argon row should change, got %v", updated)
	}

	stored := map[string]Price{}
	var rows []Price
	DB.Find(&rows)
	for _, row := range rows {
		stored[row.Model] = row
	}
	argon := stored["gemini-4-argon"]
	if !samePrice(&argon, geminiPrice(4, 0.2, 0, 20, 0, nil).toPrice("gemini-4-argon")) {
		t.Fatalf("gemini-4-argon should now bill $4 / $20, got input=%v output=%v", argon.Input, argon.Output)
	}
	if got := stored["gemini-3.8-flash"]; got.Input != 0.5 {
		t.Fatalf("an edited price must not be overwritten, got input=%v", got.Input)
	}
	if got := stored["gemini-3.6-flash"]; !got.Locked || got.Input != locked.Input {
		t.Fatalf("a locked price must not be overwritten, got %+v", got)
	}
	if PricingInstance.GetPrice("gemini-4-argon").Input != argon.Input {
		t.Fatalf("the pricing cache should reload after a change")
	}

	if again, err := ApplyScheduledPriceChanges(time.Date(2027, time.January, 2, 1, 0, 0, 0, time.UTC)); err != nil || len(again) != 0 {
		t.Fatalf("a second run should be a no-op, got %v err=%v", again, err)
	}
}

func TestOfficialPriceReviewsAreValid(t *testing.T) {
	base := officialTokenPrices()
	for _, review := range officialPriceReviews() {
		if _, ok := base[review.Model]; !ok {
			t.Errorf("%s: a review needs the current price in officialTokenPrices", review.Model)
		}
		if review.On.Location() != time.UTC || review.On.Hour() != 0 || review.Source == "" {
			t.Errorf("%s: a review starts at 00:00 UTC and cites the vendor's page, got %+v", review.Model, review)
		}
	}
}

func TestScheduledPriceItems(t *testing.T) {
	all := func(string) bool { return true }
	if got := scheduledPriceItems(time.Date(2026, time.November, 30, 23, 0, 0, 0, time.UTC), all); len(got) != 0 {
		t.Fatalf("nothing is due before the dates, got %v", got)
	}
	got := scheduledPriceItems(time.Date(2026, time.December, 1, 0, 7, 0, 0, time.UTC), all)
	want := map[string]bool{
		"scheduled|gemini-4-argon|2|10|4|20":                                                      true,
		"review|gpt-5.6-sol|2026-12-01|https://developers.openai.com/api/docs/models/gpt-5.6-sol": true,
		"review|gpt-5.6|2026-12-01|https://developers.openai.com/api/docs/models/gpt-5.6-sol":     true,
	}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for _, item := range got {
		if !want[item] {
			t.Fatalf("unexpected item %q in %v", item, got)
		}
	}
	// A change leaves the alert after two weeks; a review stays until a release replaces it.
	later := scheduledPriceItems(time.Date(2026, time.December, 20, 0, 0, 0, 0, time.UTC), all)
	if len(later) != 2 || later[0][:7] != "review|" {
		t.Fatalf("only the reviews should be left, got %v", later)
	}
	// Models nobody is served are not told.
	if got := scheduledPriceItems(time.Date(2026, time.December, 1, 1, 0, 0, 0, time.UTC), func(m string) bool { return m == "gpt-5.6-sol" }); len(got) != 1 {
		t.Fatalf("only the served model should be told, got %v", got)
	}
}

func TestReportScheduledPriceDatesTellsOnce(t *testing.T) {
	setupModelInfoTestDB(t)
	if err := DB.AutoMigrate(&Alert{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.December, 1, 0, 7, 0, 0, time.UTC)
	first, err := ReportScheduledPriceDates(now)
	if err != nil || len(first) == 0 {
		t.Fatalf("the first run should report the dates, got %v err=%v", first, err)
	}
	again, err := ReportScheduledPriceDates(now.Add(time.Hour))
	if err != nil || len(again) != 0 {
		t.Fatalf("an hour later nothing is new, got %v err=%v", again, err)
	}
}
