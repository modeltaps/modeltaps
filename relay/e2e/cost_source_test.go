package e2e

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// setChannelPrice stores the upstream list price of a model on a channel, in USD per million
// tokens. The harness does not migrate the channel prices table, so this does.
func setChannelPrice(t *testing.T, channelId int, modelName string, input, output float64) {
	t.Helper()
	if err := model.DB.AutoMigrate(&model.ChannelPrice{}); err != nil {
		t.Fatalf("migrate channel prices: %v", err)
	}
	if err := model.DB.Create(&model.ChannelPrice{ChannelId: channelId, Model: modelName, Input: input, Output: output}).Error; err != nil {
		t.Fatalf("store channel price: %v", err)
	}
	model.InvalidateChannelPriceCache()
	t.Cleanup(model.InvalidateChannelPriceCache)
}

// Every consume log says where its cost comes from. A request whose cost is not known is logged
// with a cost of 0 and no source, so it is not mistaken for a request that cost nothing.
func TestCostSourceIsLogged(t *testing.T) {
	const promptTokens, completionTokens = 1000, 500
	body := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`

	run := func(t *testing.T, channelType int, prepare func(h *Harness), response map[string]any) model.Log {
		t.Helper()
		fake := NewFakeProvider(t)
		fake.SetResponse(FakeResponse{JSON: response})
		h := NewHarness(t, HarnessOptions{
			GroupRatio: 1.5,
			Channels:   []ChannelSpec{{Name: "upstream", Type: channelType, BaseURL: fake.Server.URL}},
			// Ratio 1 is $2 per million tokens: input $2, output $4.
			Prices: []PriceSpec{{Model: "gpt-4o-mini", Input: 1, Output: 2}},
		})
		if prepare != nil {
			prepare(h)
		}
		if w := h.Post("/v1/chat/completions", body); w.Code != http.StatusOK {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		log := soleConsumeLog(t)
		if stored, _ := json.Marshal(log.Metadata.Data()); strings.Contains(string(stored), "cost_source") {
			t.Errorf("the cost source stayed in the metadata users can read: %s", stored)
		}
		return log
	}
	plain := func() map[string]any {
		return ChatCompletionJSON("gpt-4o-mini", "hello", promptTokens, completionTokens)
	}

	t.Run("unknown", func(t *testing.T) {
		// A relay at its own address with no price list and no cost ratio.
		log := run(t, config.ChannelTypeOpenAI, nil, plain())
		if log.CostQuota != 0 || log.CostSource != "" {
			t.Errorf("cost %d from %q, want 0 and no source", log.CostQuota, log.CostSource)
		}
	})

	t.Run("cost ratio", func(t *testing.T) {
		log := run(t, config.ChannelTypeOpenAI, func(h *Harness) {
			if err := model.DB.Model(&model.Channel{}).Where("id = ?", h.Channel.Id).Update("cost_ratio", 0.5).Error; err != nil {
				t.Fatalf("set cost ratio: %v", err)
			}
			model.ChannelGroup.Load()
		}, plain())
		// (1000 × 1 + 500 × 2) × 0.5.
		if log.CostQuota != 1000 || log.CostSource != model.CostSourceCostRatio {
			t.Errorf("cost %d from %q, want 1000 from the cost ratio", log.CostQuota, log.CostSource)
		}
	})

	t.Run("channel price", func(t *testing.T) {
		log := run(t, config.ChannelTypeOpenAI, func(h *Harness) {
			// $1.50 / $3 per million: ratios 0.75 and 1.5.
			setChannelPrice(t, h.Channel.Id, "gpt-4o-mini", 1.5, 3)
		}, plain())
		// 1000 × 0.75 + 500 × 1.5.
		if log.CostQuota != 1500 || log.CostSource != model.CostSourceChannelPrice {
			t.Errorf("cost %d from %q, want 1500 from the channel price", log.CostQuota, log.CostSource)
		}
	})

	t.Run("upstream", func(t *testing.T) {
		// OpenRouter says what it charged; that figure wins over any price list.
		log := run(t, config.ChannelTypeOpenRouter, nil, withUsageCost(plain(), 0.0048))
		if log.CostQuota <= 0 || log.CostSource != model.CostSourceUpstream {
			t.Errorf("cost %d from %q, want the upstream's charge", log.CostQuota, log.CostSource)
		}
	})
}

// Requests that do not go through the main relay path get their channel cost the same way: the
// decision call of a routing policy is priced from its channel's price list.
func TestDecisionCallHasChannelCost(t *testing.T) {
	decision := NewFakeProvider(t)
	decision.SetResponse(FakeResponse{JSON: decisionJSON("small", 0.9)})
	target := NewFakeProvider(t)
	target.SetResponse(FakeResponse{JSON: ChatCompletionJSON(routingCheapModel, "done", 100, 50)})
	h := newRoutingHarness(t, decision.Server.URL, target.Server.URL, 0)
	// $1 per million input and output: ratio 0.5, half the catalog price.
	setChannelPrice(t, h.Channels[0].Id, routingDecisionName, 1, 1)

	if w := h.Post("/v1/chat/completions", chatBodyFor(routingPolicyName, "add a log line")); w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	logs := consumeLogsByModel(t)
	decisionLog, ok := logs[routingDecisionName]
	if !ok {
		t.Fatalf("no log for the decision call, have %v", logKeys(logs))
	}
	// The decision reply used 20 + 8 tokens: 28 × 0.5.
	if decisionLog.CostQuota != 14 || decisionLog.CostSource != model.CostSourceChannelPrice {
		t.Errorf("decision call cost %d from %q, want 14 from the channel price", decisionLog.CostQuota, decisionLog.CostSource)
	}
	// The target's channel has no price list and no cost ratio.
	if targetLog := logs[routingCheapModel]; targetLog.CostQuota != 0 || targetLog.CostSource != "" {
		t.Errorf("target cost %d from %q, want 0 and no source", targetLog.CostQuota, targetLog.CostSource)
	}
}
