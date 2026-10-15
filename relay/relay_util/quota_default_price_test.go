package relay_util

import (
	"math"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"
)

// TestDefaultPriceQuotaMatchesOfficialUSD bills sample requests with the built-in default prices
// and checks that the quota equals the cost computed by hand from the vendor's published USD prices
// (QuotaPerUnit quota = $1).
func TestDefaultPriceQuotaMatchesOfficialUSD(t *testing.T) {
	prices := make(map[string]model.Price)
	for _, p := range model.GetDefaultPrice() {
		prices[p.Model] = *p
	}

	cases := []struct {
		name    string
		model   string
		usage   *types.Usage
		wantUSD float64
	}{
		{
			// >272K: 200,000 uncached x $20 + 100,000 cached x $2 + 10,000 output x $75 (per 1M).
			name:  "openai long context with cache hits",
			model: "gpt-6-astra",
			usage: &types.Usage{
				PromptTokens: 300000, CompletionTokens: 10000,
				PromptTokensDetails: types.PromptTokensDetails{CachedTokens: 100000},
			},
			wantUSD: 4.0 + 0.2 + 0.75,
		},
		{
			// 8,000 uncached x $2 + 2,000 cache writes x $2.50 + 10,000 cached x $0.10 + 1,000 x $10.
			name:  "openai cache write",
			model: "gpt-6.1-sol",
			usage: &types.Usage{
				PromptTokens: 20000, CompletionTokens: 1000,
				PromptTokensDetails: types.PromptTokensDetails{CachedTokens: 10000, OpenAICacheWriteTokens: 2000},
			},
			wantUSD: 0.016 + 0.005 + 0.001 + 0.01,
		},
		{
			// 1,000 x $4 + 2,000 5m writes x $5 + 1,000 1h writes x $8 + 10,000 hits x $0.20 + 500 x $20.
			name:  "anthropic cache writes and reads",
			model: "claude-opus-5-5",
			usage: &types.Usage{
				PromptTokens: 14000, CompletionTokens: 500,
				PromptTokensDetails: types.PromptTokensDetails{
					CachedWriteTokens: 2000, CachedWrite1hTokens: 1000, CachedReadTokens: 10000,
				},
			},
			wantUSD: 0.004 + 0.01 + 0.008 + 0.002 + 0.01,
		},
		{
			// Over 100,000 tokens: 100,000 x $0.50 + 50,000 hits x $0.05 + 1,000 x $2.50.
			name:  "claude haiku 5.5 long context",
			model: "claude-haiku-5-5",
			usage: &types.Usage{
				PromptTokens: 150000, CompletionTokens: 1000,
				PromptTokensDetails: types.PromptTokensDetails{CachedReadTokens: 50000},
			},
			wantUSD: 0.05 + 0.0025 + 0.0025,
		},
		{
			// >200k: 200,000 x $4 + 50,000 cached x $0.40 + 2,000 x $18.
			name:  "gemini pro long context",
			model: "gemini-3.1-pro-preview",
			usage: &types.Usage{
				PromptTokens: 250000, CompletionTokens: 2000,
				PromptTokensDetails: types.PromptTokensDetails{CachedTokens: 50000},
			},
			wantUSD: 0.8 + 0.02 + 0.036,
		},
		{
			// 6,000 text x $0.30 + 4,000 audio x $1.00 + 1,000 x $2.50.
			name:  "gemini audio input",
			model: "gemini-2.5-flash",
			usage: &types.Usage{
				PromptTokens: 10000, CompletionTokens: 1000,
				PromptTokensDetails: types.PromptTokensDetails{AudioTokens: 4000},
			},
			wantUSD: 0.0018 + 0.004 + 0.0025,
		},
		{
			// 1,000 x $2 + 200 text output x $12 + 1,120 image output x $120.
			name:  "gemini image output",
			model: "gemini-3-pro-image",
			usage: &types.Usage{
				PromptTokens: 1000, CompletionTokens: 1320,
				CompletionTokensDetails: types.CompletionTokensDetails{ImageTokens: 1120},
			},
			wantUSD: 0.002 + 0.0024 + 0.1344,
		},
		{
			// Peak: 6,000 misses x $1.32 + 4,000 hits x $0.044 + 1,000 x $3.96.
			name:  "deepseek cache hits",
			model: "deepseek-v4-pro",
			usage: &types.Usage{
				PromptTokens: 10000, CompletionTokens: 1000,
				PromptTokensDetails: types.PromptTokensDetails{CachedTokens: 4000},
			},
			wantUSD: 0.00792 + 0.000176 + 0.00396,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			price, ok := prices[tc.model]
			if !ok {
				t.Fatalf("%s has no default price", tc.model)
			}
			q := &Quota{price: price, inputRatio: price.GetInput(), outputRatio: price.GetOutput(), groupRatio: 1}
			got := q.GetTotalQuotaByUsage(tc.usage)
			want := tc.wantUSD * config.QuotaPerUnit
			// Ratios are rounded to 6 decimals and token adjustments truncate, so allow 0.1%.
			if diff := math.Abs(float64(got) - want); diff > math.Max(2, want*0.001) {
				t.Fatalf("quota = %d, want %.0f ($%.6f)", got, want, tc.wantUSD)
			}
		})
	}
}
