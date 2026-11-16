package relay_util

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"
)

// A minutes price of rate 3 is $0.006 / min. One minute is 1000*3 = 3000 quota
// ($0.006 at 500000 quota per dollar).
func TestMinutesQuota(t *testing.T) {
	cases := []struct {
		name    string
		seconds float64
		ratio   float64
		want    int
	}{
		{"one minute", 60, 3, 3000},
		{"half a minute", 30, 3, 1500},
		{"rounds up to whole seconds", 60.2, 3, 3050}, // billed as 61 s
		{"one hour", 3600, 3, 180000},
		{"no audio", 0, 3, 0},
		{"free model", 120, 0, 0},
	}
	for _, tc := range cases {
		if got := minutesQuota(tc.seconds, tc.ratio); got != tc.want {
			t.Errorf("%s: minutesQuota(%v, %v) = %d, want %d", tc.name, tc.seconds, tc.ratio, got, tc.want)
		}
	}
}

func TestCalcQuotaMinutes(t *testing.T) {
	prev := config.EmptyResponseBillingEnabled
	config.EmptyResponseBillingEnabled = false
	defer func() { config.EmptyResponseBillingEnabled = prev }()

	q := &Quota{price: model.Price{Type: model.MinutesPriceType}}

	// Bills on seconds, ignoring tokens; the empty-response and zero-token gates do not apply.
	q.audioSeconds = 90
	if got := q.calcQuota(0, 0, 3, 0, 1); got != 4500 {
		t.Errorf("90 s at rate 3 = %d, want 4500", got)
	}
	// No audio length means nothing was transcribed.
	q.audioSeconds = 0
	if got := q.calcQuota(10, 20, 3, 0, 1); got != 0 {
		t.Errorf("no audio = %d, want 0", got)
	}
}

// The total and the cost both read the length from the usage, at their own ratios.
func TestMinutesUsageQuota(t *testing.T) {
	q := &Quota{
		price:      model.Price{Type: model.MinutesPriceType, Input: 3},
		groupRatio: 0.5,
		inputRatio: 1.5,
		costRatio:  0.5,
	}
	usage := &types.Usage{AudioSeconds: 120, CompletionTokens: 40}
	if got := q.GetTotalQuotaByUsage(usage); got != 3000 { // 2 min * 1000 * 1.5
		t.Errorf("total = %d, want 3000", got)
	}
	if got := q.GetCostQuotaByUsage(usage); got != 3000 { // 2 min * 1000 * 3 * 0.5
		t.Errorf("cost = %d, want 3000", got)
	}
}

func TestMinutesPreConsumeAmount(t *testing.T) {
	for _, tc := range []struct {
		estimate float64
		want     int
	}{
		{45, 2250},   // probed file length
		{0, 3000},    // unknown length: one minute
		{600, 30000}, // ten minutes
	} {
		q := &Quota{price: model.Price{Type: model.MinutesPriceType, Input: 3}, inputRatio: 3, audioSecondsEstimate: tc.estimate}
		if got := q.minutesPreConsumeQuota(); got != tc.want {
			t.Errorf("estimate %v: pre-consume = %d, want %d", tc.estimate, got, tc.want)
		}
	}
}

func TestMinutesPriceHasNoOutputOrLongContext(t *testing.T) {
	p := model.Price{Type: model.MinutesPriceType, Input: 3, Output: 7}
	if p.GetOutput() != 0 {
		t.Errorf("GetOutput = %v, want 0", p.GetOutput())
	}
	if in, out := p.GetLongContextMultiplier(1 << 30); in != 1 || out != 1 {
		t.Errorf("long context = %v, %v, want 1, 1", in, out)
	}
}
