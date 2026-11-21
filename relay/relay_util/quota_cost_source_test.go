package relay_util

import (
	"math"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"
)

// A request's cost comes with where it is from. A cost that is not known is 0 with no source, so
// the profit figures can leave the request out instead of counting it as free.
func TestRequestCostSource(t *testing.T) {
	// Ratio 1 is $2 per million tokens: 1000 input + 1000 output is 3000 quota at the catalog price.
	price := model.Price{Type: model.TokensPriceType, Input: 1, Output: 2}
	list := &model.Price{Type: model.TokensPriceType, Input: 0.5, Output: 1}
	upstreamCharge := int(math.Round(0.01 * 1.055 * config.QuotaPerUnit))

	cases := []struct {
		name     string
		quota    *Quota
		upstream float64
		cost     int
		source   string
	}{
		{"not known", &Quota{price: price}, 0, 0, ""},
		{"cost ratio alone", &Quota{price: price, costRatio: 0.5}, 0, 1500, model.CostSourceCostRatio},
		{"vendor channel", &Quota{price: price, costPrice: &price, costSource: model.CostSourceOfficial}, 0, 3000, model.CostSourceOfficial},
		{"price list", &Quota{price: price, costPrice: list, costSource: model.CostSourceChannelPrice}, 0, 1500, model.CostSourceChannelPrice},
		{"price list without a stated source", &Quota{price: price, costPrice: list}, 0, 1500, model.CostSourceChannelPrice},
		{"upstream's own charge over the price list",
			&Quota{price: price, costPrice: list, costSource: model.CostSourceChannelPrice, feeRate: 0.055, channelType: config.ChannelTypeOpenRouter},
			0.01, upstreamCharge, model.CostSourceUpstream},
		{"a charge in an unverified unit is ignored", &Quota{price: price, channelType: config.ChannelTypeOpenAI}, 0.01, 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			usage := &types.Usage{PromptTokens: 1000, CompletionTokens: 1000, UpstreamCost: tc.upstream}
			cost, source := tc.quota.requestCost(usage)
			if cost != tc.cost || source != tc.source {
				t.Errorf("cost %d from %q, want %d from %q", cost, source, tc.cost, tc.source)
			}
			if got := tc.quota.GetCostQuotaByUsage(usage); got != tc.cost {
				t.Errorf("GetCostQuotaByUsage = %d, want %d", got, tc.cost)
			}
			logged, ok := tc.quota.GetLogMeta(usage)[model.LogMetaCostSource]
			if tc.source == "" {
				if ok {
					t.Errorf("a cost that is not known was logged with source %v", logged)
				}
			} else if logged != tc.source {
				t.Errorf("logged source = %v, want %q", logged, tc.source)
			}
		})
	}
}
