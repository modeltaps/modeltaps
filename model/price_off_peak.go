package model

import (
	"strconv"
	"strings"
	"time"
)

// OffPeakPricing is a price charged at a share of itself outside the vendor's peak hours, as
// DeepSeek charges half price off peak. The peak hours are weekly windows in UTC; any other time
// is off peak. A request is priced by the time it started.
//
// Public holidays a vendor also counts as off peak are not modeled: they are billed at the full
// price, which is never below the vendor's.
type OffPeakPricing struct {
	// Ratio is the share of the price charged off peak, between 0 and 1 (0.5 = half price).
	Ratio float64 `json:"ratio"`
	// Peak lists the peak windows; a price without one is never discounted.
	Peak []PeakWindow `json:"peak"`
}

// PeakWindow is a daily window of peak hours on the given weekdays, in UTC.
type PeakWindow struct {
	// Days are the weekdays it applies to, 0 = Sunday … 6 = Saturday.
	Days []int `json:"days"`
	// From and To are "HH:MM" in UTC; To is not included ("10:00" ends at 09:59:59).
	From string `json:"from"`
	To   string `json:"to"`
}

// parseClock reads "HH:MM" as minutes after midnight; "24:00" is the end of the day.
func parseClock(s string) (int, bool) {
	h, m, ok := strings.Cut(strings.TrimSpace(s), ":")
	if !ok {
		return 0, false
	}
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)
	if err1 != nil || err2 != nil || hh < 0 || mm < 0 || mm > 59 || hh*60+mm > 24*60 {
		return 0, false
	}
	return hh*60 + mm, true
}

// Valid reports whether the schedule can be applied: a ratio between 0 and 1 and at least one
// window, every window with weekdays and a From before its To.
func (o OffPeakPricing) Valid() bool {
	if o.Ratio <= 0 || o.Ratio >= 1 || len(o.Peak) == 0 {
		return false
	}
	for _, w := range o.Peak {
		from, ok1 := parseClock(w.From)
		to, ok2 := parseClock(w.To)
		if !ok1 || !ok2 || from >= to || len(w.Days) == 0 {
			return false
		}
		for _, d := range w.Days {
			if d < 0 || d > 6 {
				return false
			}
		}
	}
	return true
}

// IsPeak reports whether at falls in a peak window.
func (o OffPeakPricing) IsPeak(at time.Time) bool {
	at = at.UTC()
	minute := at.Hour()*60 + at.Minute()
	day := int(at.Weekday())
	for _, w := range o.Peak {
		from, _ := parseClock(w.From)
		to, _ := parseClock(w.To)
		if minute < from || minute >= to {
			continue
		}
		for _, d := range w.Days {
			if d == day {
				return true
			}
		}
	}
	return false
}

// OffPeakMultiplier is the share of the price charged for a request that started at: the
// off-peak ratio outside peak hours, else 1. Prices per call or per minute, a schedule that is
// not valid and an unknown start time (zero) pay the full price.
func (price *Price) OffPeakMultiplier(at time.Time) float64 {
	if price == nil || price.OffPeak == nil || at.IsZero() || price.Type == TimesPriceType || price.Type == MinutesPriceType {
		return 1
	}
	schedule := price.OffPeak.Data()
	if !schedule.Valid() || schedule.IsPeak(at) {
		return 1
	}
	return schedule.Ratio
}

// RequestMultipliers are the multipliers on the input and output side of a request with
// inputTokens input tokens that started at: the long-context tier times the off-peak share.
func (price *Price) RequestMultipliers(inputTokens int, at time.Time) (float64, float64) {
	in, out := price.GetLongContextMultiplier(inputTokens)
	share := price.OffPeakMultiplier(at)
	return in * share, out * share
}

// sameOffPeak reports whether two prices have the same off-peak schedule (none counts as one).
func sameOffPeak(a, b *Price) bool {
	var ao, bo OffPeakPricing
	if a.OffPeak != nil {
		ao = a.OffPeak.Data()
	}
	if b.OffPeak != nil {
		bo = b.OffPeak.Data()
	}
	if !ao.Valid() || !bo.Valid() {
		return ao.Valid() == bo.Valid()
	}
	if !sameRatio(ao.Ratio, bo.Ratio) || len(ao.Peak) != len(bo.Peak) {
		return false
	}
	for i := range ao.Peak {
		x, y := ao.Peak[i], bo.Peak[i]
		if x.From != y.From || x.To != y.To || len(x.Days) != len(y.Days) {
			return false
		}
		for j := range x.Days {
			if x.Days[j] != y.Days[j] {
				return false
			}
		}
	}
	return true
}
