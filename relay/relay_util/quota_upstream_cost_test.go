package relay_util

import (
	"math"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"
)

// The upstream's cost is logged only for channel types whose usage.cost is a verified USD
// charge, together with the gateway's own estimate of it.
func TestLogMetaUpstreamCost(t *testing.T) {
	// $2 per million input (ratio 1), $4 per million output (ratio 2).
	price := model.Price{Type: model.TokensPriceType, Input: 1, Output: 2}
	usage := &types.Usage{PromptTokens: 1_000_000, CompletionTokens: 500_000, UpstreamCost: 4.8}

	q := &Quota{price: price, groupRatio: 3, channelType: config.ChannelTypeOpenRouter}
	meta := q.GetLogMeta(usage)
	if meta[model.LogMetaUpstreamCost] != 4.8 {
		t.Fatalf("upstream cost = %v", meta[model.LogMetaUpstreamCost])
	}
	// The listed price, without the group ratio: 1M input at $2 + 0.5M output at $4.
	if expected, _ := meta[model.LogMetaExpectedCost].(float64); math.Abs(expected-4) > 1e-9 {
		t.Errorf("expected cost = %v, want 4", meta[model.LogMetaExpectedCost])
	}

	// A channel cost ratio says the upstream charges that share of the listed price.
	q = &Quota{price: price, groupRatio: 3, costRatio: 0.8, channelType: config.ChannelTypeOpenRouter}
	if expected, _ := q.GetLogMeta(usage)[model.LogMetaExpectedCost].(float64); math.Abs(expected-3.2) > 1e-9 {
		t.Errorf("expected cost at ratio 0.8 = %v, want 3.2", expected)
	}

	// Another upstream's "cost" has an unverified unit and is not logged.
	q = &Quota{price: price, groupRatio: 1, channelType: config.ChannelTypeOpenAI}
	if _, ok := q.GetLogMeta(usage)[model.LogMetaUpstreamCost]; ok {
		t.Error("a cost from an unverified upstream was logged")
	}
	// The estimate is logged all the same: the account's daily report is compared with it.
	if expected, _ := q.GetLogMeta(usage)[model.LogMetaExpectedCost].(float64); math.Abs(expected-4) > 1e-9 {
		t.Errorf("expected cost on an OpenAI channel = %v, want 4", expected)
	}
	// Nothing reported: no upstream cost, but the estimate stays for a cost that is filled in later.
	q = &Quota{price: price, groupRatio: 1, channelType: config.ChannelTypeOpenRouter}
	meta = q.GetLogMeta(&types.Usage{PromptTokens: 1_000_000, CompletionTokens: 500_000})
	if _, ok := meta[model.LogMetaUpstreamCost]; ok {
		t.Error("a request without an upstream cost was logged with one")
	}
	if expected, _ := meta[model.LogMetaExpectedCost].(float64); math.Abs(expected-4) > 1e-9 {
		t.Errorf("expected cost without an upstream cost = %v, want 4", meta[model.LogMetaExpectedCost])
	}
}

// Vercel AI Gateway reports its charge: it is logged with the host that served the request, and
// the cost figures use it (with the channel's fee) over the list-price estimate, as it follows
// the host the gateway actually used. A charge reported only in provider_metadata (messages,
// responses) reaches the usage through the consume snapshot.
func TestGatewayCostAndHost(t *testing.T) {
	price := model.Price{Type: model.TokensPriceType, Input: 1, Output: 2}
	usage := &types.Usage{PromptTokens: 1_000_000, CompletionTokens: 500_000, UpstreamCost: 1.2}
	q := &Quota{price: price, groupRatio: 1, channelType: config.ChannelTypeVercelAIGateway, feeRate: 0.03, upstreamHost: "deepinfra"}
	meta := q.GetLogMeta(usage)
	if meta[model.LogMetaUpstreamCost] != 1.2 || meta["upstream_host"] != "deepinfra" {
		t.Fatalf("meta = %v", meta)
	}
	if got, want := q.GetCostQuotaByUsage(usage), int(math.Round(1.2*1.03*config.QuotaPerUnit)); got != want {
		t.Errorf("cost quota = %d, want %d (the reported charge with the fee)", got, want)
	}
	// An upstream whose cost is not verified keeps the estimate.
	q.channelType = config.ChannelTypeOpenAI
	if got := q.GetCostQuotaByUsage(usage); got == int(math.Round(1.2*1.03*config.QuotaPerUnit)) {
		t.Errorf("cost quota from an unverified upstream cost: %d", got)
	}

	snapUsage := &types.Usage{PromptTokens: 10, CompletionTokens: 5}
	q = &Quota{price: price, groupRatio: 1}
	snap := ConsumeSnapshot{ChannelType: config.ChannelTypeVercelAIGateway, UpstreamHost: "alibaba", GatewayCost: 0.25}
	q.applySnapshotGateway(snap, snapUsage)
	if snapUsage.UpstreamCost != 0.25 || q.upstreamHost != "alibaba" {
		t.Errorf("snapshot: usage cost %v host %q", snapUsage.UpstreamCost, q.upstreamHost)
	}
	// usage.cost, when the response had one, is kept.
	snapUsage = &types.Usage{UpstreamCost: 0.3}
	q.applySnapshotGateway(snap, snapUsage)
	if snapUsage.UpstreamCost != 0.3 {
		t.Errorf("usage.cost replaced: %v", snapUsage.UpstreamCost)
	}
}
