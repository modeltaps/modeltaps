package model

import (
	"testing"
	"time"
)

func officialCatalogEntry(in float64, out *float64) catalogModel {
	return catalogModel{Cost: catalogCost{Input: &in, Output: out}}
}

func f64(v float64) *float64 { return &v }

func officialCheckData() map[string]catalogProvider {
	return map[string]catalogProvider{
		"openai": {Models: map[string]catalogModel{
			"gpt-5.5":     officialCatalogEntry(6, f64(30)), // input 20% higher: listed
			"gpt-image-2": officialCatalogEntry(5, f64(40)), // image tokens: not compared
		}},
		"google": {Models: map[string]catalogModel{
			"gemini-4-argon": officialCatalogEntry(4, f64(20)), // the scheduled price, shown early
		}},
		"deepseek": {Models: map[string]catalogModel{
			"deepseek-v4-pro": officialCatalogEntry(0.66, f64(1.98)), // the off-peak price
		}},
		"mistral": {Models: map[string]catalogModel{
			"mistral-large-latest": officialCatalogEntry(0.72, nil), // within 10%, no output price
		}},
	}
}

var officialCheckNow = time.Date(2026, time.November, 20, 0, 0, 0, 0, time.UTC)

func TestOfficialPriceDiffsListsOnlyRealDifferences(t *testing.T) {
	diffs := officialPriceDiffs(officialCheckData(), officialCheckNow)
	if len(diffs) != 1 {
		t.Fatalf("diffs = %+v, want gpt-5.5 only", diffs)
	}
	d := diffs[0]
	if d.Model != "gpt-5.5" || d.OfficialInput != 5 || d.CatalogInput != 6 || d.item() != "official|gpt-5.5|5|30|6|30" {
		t.Fatalf("diff = %+v (%s)", d, d.item())
	}
}

func TestOfficialPriceDiffsComparesInputOnlyWithoutOutputPrice(t *testing.T) {
	data := map[string]catalogProvider{"mistral": {Models: map[string]catalogModel{
		"mistral-large-latest": officialCatalogEntry(1, nil),
	}}}
	diffs := officialPriceDiffs(data, officialCheckNow)
	if len(diffs) != 1 || diffs[0].CatalogOutput != diffs[0].OfficialOutput {
		t.Fatalf("diffs = %+v", diffs)
	}
}

func TestOfficialPriceDiffsWantsTheOffPeakShareExactly(t *testing.T) {
	data := map[string]catalogProvider{"deepseek": {Models: map[string]catalogModel{
		"deepseek-v4-pro": officialCatalogEntry(0.9, f64(3)), // neither the list nor the off-peak price
	}}}
	if diffs := officialPriceDiffs(data, officialCheckNow); len(diffs) != 1 {
		t.Fatalf("diffs = %+v", diffs)
	}
}

func TestCheckOfficialPricesKeepsItsAlertThroughTheHourlyReview(t *testing.T) {
	usePriceSourceTestDB(t)
	CheckOfficialPrices(officialCheckData(), officialCheckNow)
	open := func() []string {
		t.Helper()
		items, err := AlertItems(PriceSourceAlertKind, OfficialPriceSource)
		if err != nil {
			t.Fatal(err)
		}
		return items
	}
	if got := open(); len(got) != 1 || got[0] != "official|gpt-5.5|5|30|6|30" {
		t.Fatalf("alert items = %v", got)
	}
	if _, err := ReviewPriceSources(nil); err != nil {
		t.Fatal(err)
	}
	if got := open(); len(got) != 1 {
		t.Fatalf("the hourly review closed the official alert: %v", got)
	}
	// Fixed on either side: the alert closes.
	data := officialCheckData()
	data["openai"].Models["gpt-5.5"] = officialCatalogEntry(5, f64(30))
	CheckOfficialPrices(data, officialCheckNow)
	if got := open(); len(got) != 0 {
		t.Fatalf("alert still open: %v", got)
	}
}
