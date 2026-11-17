package e2e

import (
	"math"
	"net/http"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// The platform markup multiplies the charge on top of the group ratio; the log keeps the amount at
// the catalog price, and the expected upstream charge is the channel's list price without its fee.
func TestPlatformMarkupCharging(t *testing.T) {
	oldMarkup := config.PlatformMarkupPercent
	config.PlatformMarkupPercent = 12
	t.Cleanup(func() { config.PlatformMarkupPercent = oldMarkup })

	fake := NewFakeProvider(t)
	fake.SetResponse(FakeResponse{JSON: ChatCompletionJSON("gpt-4o-mini", "ok", 1000, 1000)})
	h := NewHarness(t, HarnessOptions{
		FakeBaseURL: fake.Server.URL, GroupRatio: 0.5,
		Prices: []PriceSpec{{Model: "gpt-4o-mini", Input: billingInputRatio, Output: billingOutputRatio}},
	})
	// $4 / $6 per 1M list price with a 5% fee.
	setChannelCost(t, h.Channel, 0.05, 4, 6)
	before := h.UserQuota(t)
	if w := h.Post("/v1/chat/completions", floorBody); w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	// Catalog 1000 x 2 + 1000 x 3 = 5000, x 0.5 group, x 1.12 markup = 2800.
	if got, want := before-h.UserQuota(t), 2800; got != want {
		t.Fatalf("charged %d, want %d", got, want)
	}
	log := soleConsumeLog(t)
	meta := log.Metadata.Data()
	if meta["base_quota"] != float64(5000) || meta["markup_rate"] == nil || meta["user_group_ratio"] != 0.5 {
		t.Errorf("markup metadata = %v", meta)
	}
	// List: (1000 x 4 + 1000 x 6) / 1M = $0.01, without the fee; cost with the fee $0.0105.
	if math.Abs(log.ExpectedCost-0.01) > 1e-9 {
		t.Errorf("expected cost = %v, want the list price $0.01", log.ExpectedCost)
	}
	if want := int(0.0105 * float64(config.QuotaPerUnit)); log.CostQuota != want {
		t.Errorf("cost = %d, want %d", log.CostQuota, want)
	}
}
