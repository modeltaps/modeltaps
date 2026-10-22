package types

import (
	"math"
	"testing"
)

func TestUsageTierMultiplier(t *testing.T) {
	cases := []struct {
		tier, geo string
		want      float64
	}{
		{"", "", 1}, {"default", "", 1}, {"auto", "", 1}, {"scale", "", 1},
		{"priority", "", 2}, {"FAST", "", 2}, {"ultrafast", "", 6}, {"flex", "", 0.5},
		{"", "us", 1.1}, {"", "global", 1}, {"fast", "US", 2.2},
	}
	for _, tc := range cases {
		u := &Usage{ServiceTier: tc.tier, InferenceGeo: tc.geo}
		if got := u.TierMultiplier(); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("tier=%q geo=%q multiplier = %v, want %v", tc.tier, tc.geo, got, tc.want)
		}
	}
	var nilUsage *Usage
	if nilUsage.TierMultiplier() != 1 {
		t.Error("nil usage multiplier != 1")
	}
}
