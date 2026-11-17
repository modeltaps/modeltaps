package e2e

// Cost floor billing (SupplyCostFloorBilling): a request is charged the higher of the quote
// (catalog price × group ratio) and the channel cost (channel price × (1 + fee rate)).
// The catalog price here is Input 2 / Output 3 (ratio units), $4 / $6 per million tokens.

import (
	"math"
	"net/http"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

const floorBody = `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`

// setChannelCost gives a channel a fee rate and an upstream price for gpt-4o-mini in USD per
// million tokens, then reloads the channels.
func setChannelCost(t *testing.T, ch *model.Channel, feeRate, inputUSD, outputUSD float64) {
	t.Helper()
	if err := model.DB.AutoMigrate(&model.ChannelPrice{}); err != nil {
		t.Fatal(err)
	}
	if err := model.DB.Model(&model.Channel{}).Where("id = ?", ch.Id).Updates(map[string]any{"fee_rate": feeRate, "fee_note": "test fee"}).Error; err != nil {
		t.Fatal(err)
	}
	if inputUSD > 0 || outputUSD > 0 {
		if err := model.ReplaceChannelPrices(ch.Id, []*model.ChannelPrice{{Model: "gpt-4o-mini", Input: inputUSD, Output: outputUSD}}); err != nil {
			t.Fatal(err)
		}
	}
	model.InvalidateChannelPriceCache()
	model.ChannelGroup.Load()
}

func floorBilling(t *testing.T, log model.Log) map[string]any {
	t.Helper()
	billing, ok := log.Metadata.Data()[model.LogMetaBilling].(map[string]any)
	if !ok {
		t.Fatalf("no billing breakdown in %v", log.Metadata.Data())
	}
	return billing
}

func TestCostFloorBilling(t *testing.T) {
	const prompt, completion = 1000, 1000
	run := func(t *testing.T, groupRatio, feeRate, inUSD, outUSD float64) (int, model.Log) {
		fake := NewFakeProvider(t)
		fake.SetResponse(FakeResponse{JSON: ChatCompletionJSON("gpt-4o-mini", "ok", prompt, completion)})
		h := NewHarness(t, HarnessOptions{
			FakeBaseURL: fake.Server.URL, GroupRatio: groupRatio,
			Prices: []PriceSpec{{Model: "gpt-4o-mini", Input: billingInputRatio, Output: billingOutputRatio}},
		})
		setAndRestore(t, &config.SupplyCostFloorBilling, true)
		setChannelCost(t, h.Channel, feeRate, inUSD, outUSD)
		before := h.UserQuota(t)
		if w := h.Post("/v1/chat/completions", floorBody); w.Code != http.StatusOK {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		return before - h.UserQuota(t), soleConsumeLog(t)
	}
	// Channel cost in quota units for the usage above.
	cost := func(feeRate, inUSD, outUSD float64) int {
		return int(math.Ceil(prompt*inUSD/2*(1+feeRate) + completion*outUSD/2*(1+feeRate)))
	}

	t.Run("channel cheaper than the quote", func(t *testing.T) {
		charged, log := run(t, 1, 0.05, 3, 5)
		if want := expectQuota(prompt, completion, 1); charged != want {
			t.Fatalf("charged %d, want the quote %d", charged, want)
		}
		if b := floorBilling(t, log); b["basis"] != "quote" {
			t.Errorf("basis = %v", b["basis"])
		}
		if want := cost(0.05, 3, 5); log.CostQuota != want {
			t.Errorf("cost quota = %d, want the channel cost %d", log.CostQuota, want)
		}
	})

	t.Run("channel dearer than the quote", func(t *testing.T) {
		charged, log := run(t, 1, 0.05, 4.4, 6.6)
		want := cost(0.05, 4.4, 6.6)
		if charged != want || log.Quota != want {
			t.Fatalf("charged %d (log %d), want the channel cost %d", charged, log.Quota, want)
		}
		b := floorBilling(t, log)
		if b["basis"] != "channel_cost" || b["fee_rate"] != 0.05 {
			t.Errorf("billing = %v", b)
		}
		// Users see the cost and the fee rate of a request charged at cost, never the admin note.
		log.RedactForMember()
		member := log.Metadata.Data()[model.LogMetaBilling].(map[string]any)
		if _, ok := member["_admin_fee_note"]; ok || member["channel_cost_quota"] == nil {
			t.Errorf("member billing = %v", member)
		}
	})

	t.Run("discount group pays at least the cost", func(t *testing.T) {
		charged, _ := run(t, 0.5, 0, 3, 5)
		if want := cost(0, 3, 5); charged != want {
			t.Fatalf("charged %d, want the channel cost %d", charged, want)
		}
	})

	t.Run("unknown cost charges the quote", func(t *testing.T) {
		charged, log := run(t, 0.5, 0.05, 0, 0)
		if want := expectQuota(prompt, completion, 0.5); charged != want {
			t.Fatalf("charged %d, want the quote %d", charged, want)
		}
		if b := floorBilling(t, log); b["basis"] != "cost_unknown" {
			t.Errorf("basis = %v", b["basis"])
		}
	})
}

// The floor follows the channel that served the request: a retry to a dearer channel is charged
// at that channel's cost.
func TestCostFloorFollowsRetry(t *testing.T) {
	failing := NewFakeProvider(t)
	failing.SetResponse(FakeResponse{Status: http.StatusInternalServerError, JSON: OpenAIErrorJSON("boom", "server_error", "internal_error")})
	dear := NewFakeProvider(t)
	dear.SetResponse(FakeResponse{JSON: ChatCompletionJSON("gpt-4o-mini", "ok", 1000, 1000)})
	h := NewHarness(t, HarnessOptions{
		Channels: []ChannelSpec{
			{Name: "cheap", BaseURL: failing.Server.URL, Priority: 10},
			{Name: "dear", BaseURL: dear.Server.URL, Priority: 1},
		},
		Prices: []PriceSpec{{Model: "gpt-4o-mini", Input: billingInputRatio, Output: billingOutputRatio}},
	})
	setAndRestore(t, &config.RetryTimes, 3)
	setAndRestore(t, &config.SupplyCostFloorBilling, true)
	setChannelCost(t, h.Channels[0], 0, 1, 1)
	setChannelCost(t, h.Channels[1], 0, 8, 12)

	before := h.UserQuota(t)
	if w := h.Post("/v1/chat/completions", floorBody); w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if got, want := before-h.UserQuota(t), 1000*4+1000*6; got != want {
		t.Fatalf("charged %d, want the second channel's cost %d", got, want)
	}
}

// With the setting off, a channel price changes nothing.
func TestCostFloorOff(t *testing.T) {
	fake := NewFakeProvider(t)
	fake.SetResponse(FakeResponse{JSON: ChatCompletionJSON("gpt-4o-mini", "ok", 1000, 1000)})
	h := NewHarness(t, HarnessOptions{
		FakeBaseURL: fake.Server.URL, GroupRatio: 0.5,
		Prices: []PriceSpec{{Model: "gpt-4o-mini", Input: billingInputRatio, Output: billingOutputRatio}},
	})
	setChannelCost(t, h.Channel, 0.05, 40, 60)
	before := h.UserQuota(t)
	if w := h.Post("/v1/chat/completions", floorBody); w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if got, want := before-h.UserQuota(t), expectQuota(1000, 1000, 0.5); got != want {
		t.Fatalf("charged %d, want %d", got, want)
	}
	log := soleConsumeLog(t)
	if _, ok := log.Metadata.Data()[model.LogMetaBilling]; ok || log.CostQuota != 0 {
		t.Errorf("floor off left a breakdown or a cost: %v cost=%d", log.Metadata.Data(), log.CostQuota)
	}
}
