package model

import (
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"gorm.io/datatypes"
)

func offPeakPrice(o OffPeakPricing) *Price {
	op := datatypes.NewJSONType(o)
	return &Price{Model: "m", Type: TokensPriceType, Input: 1, Output: 2, OffPeak: &op}
}

func weekAt(day, hour, minute int) time.Time {
	// 2026-11-16 is a Monday.
	return time.Date(2026, time.November, 16+day, hour, minute, 0, 0, time.UTC)
}

func TestOffPeakMultiplierFollowsDeepSeekPeakHours(t *testing.T) {
	p := offPeakPrice(*deepseekOffPeak)
	cases := []struct {
		name string
		at   time.Time
		want float64
	}{
		{"Monday 01:00 starts the first window", weekAt(0, 1, 0), 1},
		{"Monday 03:59 still peak", weekAt(0, 3, 59), 1},
		{"Monday 04:00 ends it", weekAt(0, 4, 0), 0.5},
		{"Monday 05:30 between windows", weekAt(0, 5, 30), 0.5},
		{"Friday 09:59 peak", weekAt(4, 9, 59), 1},
		{"Friday 10:00 off peak", weekAt(4, 10, 0), 0.5},
		{"Saturday 02:00 weekend", weekAt(5, 2, 0), 0.5},
		{"Sunday 07:00 weekend", weekAt(6, 7, 0), 0.5},
		{"another time zone, same instant", weekAt(0, 2, 0).In(time.FixedZone("UTC+8", 8*3600)), 1},
		{"unknown start time", time.Time{}, 1},
	}
	for _, c := range cases {
		if got := p.OffPeakMultiplier(c.at); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestOffPeakMultiplierChargesFullPriceForBadSchedules(t *testing.T) {
	sunday := weekAt(6, 12, 0)
	bad := []OffPeakPricing{
		{Ratio: 0, Peak: deepseekOffPeak.Peak},
		{Ratio: 1.2, Peak: deepseekOffPeak.Peak},
		{Ratio: 0.5},
		{Ratio: 0.5, Peak: []PeakWindow{{Days: []int{1}, From: "10:00", To: "09:00"}}},
		{Ratio: 0.5, Peak: []PeakWindow{{Days: []int{7}, From: "01:00", To: "02:00"}}},
		{Ratio: 0.5, Peak: []PeakWindow{{Days: []int{1}, From: "1am", To: "02:00"}}},
		{Ratio: 0.5, Peak: []PeakWindow{{From: "01:00", To: "02:00"}}},
	}
	for i, o := range bad {
		if got := offPeakPrice(o).OffPeakMultiplier(sunday); got != 1 {
			t.Errorf("schedule %d: %v, want 1", i, got)
		}
	}
	flat := offPeakPrice(*deepseekOffPeak)
	flat.Type = TimesPriceType
	if got := flat.OffPeakMultiplier(sunday); got != 1 {
		t.Errorf("per-call price: %v, want 1", got)
	}
	if got := (&Price{Type: TokensPriceType}).OffPeakMultiplier(sunday); got != 1 {
		t.Errorf("no schedule: %v, want 1", got)
	}
}

func TestRequestMultipliersCombineLongContextAndOffPeak(t *testing.T) {
	p := offPeakPrice(*deepseekOffPeak)
	lc := datatypes.NewJSONType(LongContextTier{Threshold: 100, InputRatio: 2, OutputRatio: 1.5})
	p.LongContext = &lc
	in, out := p.RequestMultipliers(200, weekAt(5, 2, 0))
	if in != 1 || out != 0.75 {
		t.Fatalf("multipliers = %v, %v, want 1, 0.75", in, out)
	}
}

func TestDeepSeekOfficialPricesCarryTheOffPeakSchedule(t *testing.T) {
	for _, name := range []string{"deepseek-flash", "deepseek-v4-pro"} {
		var found *Price
		for _, p := range GetOfficialPrices() {
			if p.Model == name {
				found = p
			}
		}
		if found == nil || found.OffPeak == nil || found.OffPeakMultiplier(weekAt(5, 2, 0)) != 0.5 {
			t.Fatalf("%s: %+v", name, found)
		}
	}
	for _, p := range GetOfficialPrices() {
		if p.OffPeak != nil && p.ChannelType != config.ChannelTypeDeepseek {
			t.Errorf("%s has an off-peak schedule", p.Model)
		}
	}
}

func TestDiffPricesSeesAnOffPeakChange(t *testing.T) {
	stored := &Price{Model: "deepseek-v4-pro", Type: TokensPriceType, Input: 0.66, Output: 1.98}
	p := &Pricing{Prices: map[string]*Price{stored.Model: stored}}
	next := offPeakPrice(*deepseekOffPeak)
	next.Model, next.Input, next.Output = stored.Model, stored.Input, stored.Output
	changes := p.DiffPrices([]*Price{next})
	if len(changes) != 1 || changes[0].Kind != PriceChangeOther {
		t.Fatalf("changes = %+v", changes)
	}
	if !samePrice(next, clonePrice(next)) || samePrice(next, stored) {
		t.Fatal("samePrice ignores the off-peak schedule")
	}
}

func TestChannelPriceListDropsTheCatalogOffPeak(t *testing.T) {
	catalog := offPeakPrice(*deepseekOffPeak)
	p, ok := priceFromChannelPrice(catalog, &ChannelPrice{Model: "deepseek/deepseek-v4-pro", Input: 1.5, Output: 4})
	if !ok || p.OffPeak != nil {
		t.Fatalf("price = %+v, %v", p, ok)
	}
	if catalog.OffPeak == nil {
		t.Fatal("the catalog price lost its schedule")
	}
}

func TestBackfillOfficialOffPeak(t *testing.T) {
	useSupplyTestDB(t)
	rows := []*Price{
		{Model: "deepseek-v4-pro", Type: TokensPriceType, Input: 0.66, Output: 1.98},
		{Model: "deepseek-flash", Type: TokensPriceType, Input: 1, Output: 1, Locked: true},
		{Model: "gpt-5.5", Type: TokensPriceType, Input: 2.5, Output: 15},
	}
	if err := DB.Create(rows).Error; err != nil {
		t.Fatal(err)
	}
	if err := backfillOfficialOffPeak().Migrate(DB); err != nil {
		t.Fatal(err)
	}
	read := func(name string) *Price {
		var p Price
		if err := DB.Where("model = ?", name).First(&p).Error; err != nil {
			t.Fatal(err)
		}
		return &p
	}
	if p := read("deepseek-v4-pro"); p.OffPeakMultiplier(weekAt(5, 2, 0)) != 0.5 {
		t.Errorf("deepseek-v4-pro not backfilled: %+v", p.OffPeak)
	}
	if p := read("deepseek-flash"); p.OffPeak != nil {
		t.Error("a locked price was changed")
	}
	if p := read("gpt-5.5"); p.OffPeak != nil {
		t.Error("gpt-5.5 got a schedule")
	}
}
