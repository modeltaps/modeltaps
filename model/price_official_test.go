package model

import (
	"math"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func defaultPriceByModel(t *testing.T) map[string]*Price {
	t.Helper()
	byModel := make(map[string]*Price)
	for _, p := range GetDefaultPrice() {
		if _, dup := byModel[p.Model]; dup {
			t.Fatalf("duplicate default price for %q", p.Model)
		}
		byModel[p.Model] = p
	}
	return byModel
}

func TestOfficialTokenPricesAreValid(t *testing.T) {
	for name, o := range officialTokenPrices() {
		if o.input <= 0 || o.output <= 0 {
			t.Errorf("%s: input and output must be positive, got %v / %v", name, o.input, o.output)
		}
		if o.channelType == config.ChannelTypeUnknown {
			t.Errorf("%s: channel type must name the vendor", name)
		}
		for key, usd := range o.extra {
			if _, known := ExtraKeyIsPrompt[key]; !known {
				t.Errorf("%s: unknown extra key %q", name, key)
			}
			if usd <= 0 {
				t.Errorf("%s: extra %q must be positive, got %v", name, key, usd)
			}
		}
		if lc := o.longContext; lc != nil && (lc.Threshold <= 0 || lc.InputRatio <= 0 || lc.OutputRatio <= 0) {
			t.Errorf("%s: invalid long-context tier %+v", name, *lc)
		}
	}
}

func TestDefaultPriceRatioConversion(t *testing.T) {
	type want struct {
		channelType int
		input       float64
		output      float64
		extra       map[string]float64
		longContext *LongContextTier
	}
	cases := map[string]want{
		// $10 / $50, cached $1, cache writes $12.5; >272K: 2x input, 1.5x output.
		"gpt-6-astra": {config.ChannelTypeOpenAI, 5, 25,
			map[string]float64{config.UsageExtraCache: 0.1, config.UsageExtraOpenAICacheWrite: 1.25},
			&LongContextTier{Threshold: 272000, InputRatio: 2, OutputRatio: 1.5}},
		// $2 / $10, cached $0.10 (5% of input).
		"gpt-6.1-sol": {config.ChannelTypeOpenAI, 1, 5,
			map[string]float64{config.UsageExtraCache: 0.05, config.UsageExtraOpenAICacheWrite: 1.25},
			&LongContextTier{Threshold: 272000, InputRatio: 2, OutputRatio: 1.5}},
		// $5 / $30, cached $0.50, no cache-write price.
		"gpt-5.5": {config.ChannelTypeOpenAI, 2.5, 15,
			map[string]float64{config.UsageExtraCache: 0.1},
			&LongContextTier{Threshold: 272000, InputRatio: 2, OutputRatio: 1.5}},
		// $2.50 / $10, cached $1.25 (was 2.5 / 7.5, the 2024-05-13 snapshot price).
		"gpt-4o": {config.ChannelTypeOpenAI, 1.25, 5,
			map[string]float64{config.UsageExtraCache: 0.5}, nil},
		// $4 / $20, 5m $5, 1h $8, hits $0.20.
		"claude-opus-5-5": {config.ChannelTypeAnthropic, 2, 10,
			map[string]float64{config.UsageExtraCachedWrite: 1.25, config.UsageExtraCachedWrite1h: 2, config.UsageExtraCachedRead: 0.05}, nil},
		// $10 / $50, hits $0.25 (0.025x).
		"claude-fable-5-1": {config.ChannelTypeAnthropic, 5, 25,
			map[string]float64{config.UsageExtraCachedWrite: 1.25, config.UsageExtraCachedWrite1h: 2, config.UsageExtraCachedRead: 0.025}, nil},
		// $0.10 / $0.50 up to 100K, 5x over.
		"claude-haiku-5-5": {config.ChannelTypeAnthropic, 0.05, 0.25,
			map[string]float64{config.UsageExtraCachedWrite: 1.25, config.UsageExtraCachedWrite1h: 2, config.UsageExtraCachedRead: 0.1},
			&LongContextTier{Threshold: 100000, InputRatio: 5, OutputRatio: 5}},
		// $2 / $12, caching $0.20; >200k: $4 / $18.
		"gemini-3.1-pro-preview": {config.ChannelTypeGemini, 1, 6,
			map[string]float64{config.UsageExtraCache: 0.1},
			&LongContextTier{Threshold: 200000, InputRatio: 2, OutputRatio: 1.5}},
		// $0.30 text / $1.00 audio input, $2.50 output, caching $0.03.
		"gemini-2.5-flash": {config.ChannelTypeGemini, 0.15, 1.25,
			map[string]float64{config.UsageExtraCache: 0.1, config.UsageExtraInputAudio: 3.333333}, nil},
		// $2 input, $12 text output, $120 image output.
		"gemini-3-pro-image": {config.ChannelTypeGemini, 1, 6,
			map[string]float64{config.UsageExtraOutputImageTokens: 10}, nil},
		// Peak: $1.32 miss / $0.044 hit / $3.96 output.
		"deepseek-v4-pro": {config.ChannelTypeDeepseek, 0.66, 1.98,
			map[string]float64{
				config.UsageExtraCache: 0.033333, config.UsageExtraCachedRead: 0.033333,
				config.UsageExtraCachedWrite: 1, config.UsageExtraCachedWrite1h: 1,
			}, nil},
	}

	prices := defaultPriceByModel(t)
	for name, w := range cases {
		p, ok := prices[name]
		if !ok {
			t.Errorf("%s: missing from default prices", name)
			continue
		}
		if p.Type != TokensPriceType || p.ChannelType != w.channelType {
			t.Errorf("%s: type/channel = %s/%d, want tokens/%d", name, p.Type, p.ChannelType, w.channelType)
		}
		if !nearly(p.Input, w.input) || !nearly(p.Output, w.output) {
			t.Errorf("%s: ratio = %v / %v, want %v / %v", name, p.Input, p.Output, w.input, w.output)
		}
		var extra map[string]float64
		if p.ExtraRatios != nil {
			extra = p.ExtraRatios.Data()
		}
		if len(extra) != len(w.extra) {
			t.Errorf("%s: extra ratios = %v, want %v", name, extra, w.extra)
		}
		for key, ratio := range w.extra {
			if got := p.GetExtraRatio(key); !nearly(got, ratio) {
				t.Errorf("%s: extra %s = %v, want %v", name, key, got, ratio)
			}
		}
		switch {
		case w.longContext == nil && p.LongContext != nil:
			t.Errorf("%s: unexpected long-context tier %+v", name, p.LongContext.Data())
		case w.longContext != nil && (p.LongContext == nil || p.LongContext.Data() != *w.longContext):
			t.Errorf("%s: long-context tier = %v, want %+v", name, p.LongContext, *w.longContext)
		}
	}
}

func nearly(a, b float64) bool {
	return math.Abs(a-b) < 1e-6
}
