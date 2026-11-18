package model

import (
	"math"
	"testing"
	"time"

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
		switch {
		case o.free:
			if o.input != 0 || o.output != 0 || len(o.extra) > 0 {
				t.Errorf("%s: a free model must have no prices, got %+v", name, o)
			}
		case o.input <= 0 || o.output < 0:
			// Output may be zero for models billed on input only (embeddings, TTS).
			t.Errorf("%s: input must be positive and output non-negative, got %v / %v", name, o.input, o.output)
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
		// $8 / $40, 5m $10, 1h $16, hits $0.20 (0.025x).
		"claude-fable-5-2": {config.ChannelTypeAnthropic, 4, 20,
			map[string]float64{config.UsageExtraCachedWrite: 1.25, config.UsageExtraCachedWrite1h: 2, config.UsageExtraCachedRead: 0.025}, nil},
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

// officialPriceByModel is every built-in official price in effect now, keyed by model.
func officialPriceByModel(t *testing.T) map[string]*Price {
	t.Helper()
	byModel := make(map[string]*Price)
	for model, o := range officialTokenPricesAt(time.Now()) {
		byModel[model] = o.toPrice(model)
	}
	for model, o := range officialRequestPrices() {
		if _, dup := byModel[model]; dup {
			t.Fatalf("%s is both a per-token and a per-request official price", model)
		}
		byModel[model] = o.toPrice(model)
	}
	return byModel
}

func samePriceFields(a, b *Price) bool {
	if a.Type != b.Type || a.ChannelType != b.ChannelType || a.Input != b.Input || a.Output != b.Output {
		return false
	}
	var ae, be map[string]float64
	if a.ExtraRatios != nil {
		ae = a.ExtraRatios.Data()
	}
	if b.ExtraRatios != nil {
		be = b.ExtraRatios.Data()
	}
	if len(ae) != len(be) {
		return false
	}
	for k, v := range ae {
		if bv, ok := be[k]; !ok || bv != v {
			return false
		}
	}
	if (a.LongContext == nil) != (b.LongContext == nil) {
		return false
	}
	return a.LongContext == nil || a.LongContext.Data() == b.LongContext.Data()
}

// type=official lists the official tables only, at the prices in effect now.
func TestGetPricesListOfficial(t *testing.T) {
	prices := GetPricesList("official")
	official := officialPriceByModel(t)
	if len(prices) != len(official) {
		t.Fatalf("got %d prices, want %d", len(prices), len(official))
	}
	for _, p := range prices {
		want, ok := official[p.Model]
		if !ok {
			t.Errorf("%s is not in the official table", p.Model)
			continue
		}
		if !samePriceFields(p, want) {
			t.Errorf("%s = %+v, want %+v", p.Model, p, want)
		}
	}
}

// The default prices a new deployment starts with are the official prices plus the gateway default
// prices: every official model has the same price in both, and every other default entry is a gateway
// default.
func TestDefaultPricesMatchOfficial(t *testing.T) {
	defaults := defaultPriceByModel(t)
	official := officialPriceByModel(t)
	gateway := gatewayDefaultPrices()
	if len(defaults) != len(official)+len(gateway) {
		t.Errorf("default has %d models, want %d official + %d gateway", len(defaults), len(official), len(gateway))
	}
	for model, d := range defaults {
		if o, ok := official[model]; ok {
			if !samePriceFields(d, o) {
				t.Errorf("%s: default %+v differs from official %+v", model, d, o)
			}
			continue
		}
		g, ok := gateway[model]
		if !ok {
			t.Errorf("%s has a default price but neither an official nor a gateway price", model)
			continue
		}
		if !samePriceFields(d, g.toPrice(model)) {
			t.Errorf("%s: default %+v differs from gateway %+v", model, d, g)
		}
	}
}

// A model is either officially priced or a gateway default, never both.
func TestGatewayDefaultPricesDoNotOverlapOfficial(t *testing.T) {
	official := officialPriceByModel(t)
	for model := range gatewayDefaultPrices() {
		if _, ok := official[model]; ok {
			t.Errorf("%s is in both the official and the gateway default tables", model)
		}
	}
}

// Official prices (and type=official) never include gateway defaults.
func TestOfficialPricesExcludeGatewayDefaults(t *testing.T) {
	gateway := gatewayDefaultPrices()
	for _, p := range GetOfficialPrices() {
		if _, ok := gateway[p.Model]; ok {
			t.Errorf("GetOfficialPrices includes gateway default %s", p.Model)
		}
	}
	for _, p := range GetPricesList("official") {
		if _, ok := gateway[p.Model]; ok {
			t.Errorf("type=official includes gateway default %s", p.Model)
		}
	}
}

// Supported models without a convertible official price stay preset, at the gateway defaults.
func TestGatewayDefaultPricesPreset(t *testing.T) {
	cases := map[string]struct {
		priceType     string
		channelType   int
		input, output float64
	}{
		"mj_imagine":        {TimesPriceType, config.ChannelTypeMidjourney, 50, 50},
		"mj_relax_upscale":  {TimesPriceType, config.ChannelTypeMidjourney, 25, 25},
		"mj_turbo_upload":   {TimesPriceType, config.ChannelTypeMidjourney, 0, 0},
		"chirp-v3-5":        {TimesPriceType, config.ChannelTypeSuno, 50, 50},
		"suno_lyrics":       {TimesPriceType, config.ChannelTypeSuno, 5, 5},
		"dall-e-2":          {TokensPriceType, config.ChannelTypeOpenAI, 8, 8},
		"dall-e-3":          {TokensPriceType, config.ChannelTypeOpenAI, 20, 20},
		"stable-image-core": {TokensPriceType, config.ChannelTypeStabilityAI, 15, 15},
	}
	defaults := defaultPriceByModel(t)
	mj := 0
	for model := range defaults {
		if len(model) > 3 && model[:3] == "mj_" {
			mj++
		}
	}
	if mj != 48 {
		t.Errorf("got %d Midjourney default prices, want 48", mj)
	}
	for name, w := range cases {
		p, ok := defaults[name]
		if !ok {
			t.Errorf("%s: missing", name)
			continue
		}
		if p.Type != w.priceType || p.ChannelType != w.channelType || p.Input != w.input || p.Output != w.output {
			t.Errorf("%s = %s/%d %v/%v, want %s/%d %v/%v", name, p.Type, p.ChannelType, p.Input, p.Output,
				w.priceType, w.channelType, w.input, w.output)
		}
	}
}

// Retired models and outdated legacy defaults are no longer preset.
func TestDefaultPricesDropLegacyEntries(t *testing.T) {
	defaults := defaultPriceByModel(t)
	for _, model := range []string{
		"gpt-3.5-turbo", "gpt-4", "gpt-4-turbo", "davinci-002", "tts-1-1106",
		"claude-2.1", "claude-3-haiku-20240307", "gemini-1.5-pro",
		"qwen-max", "qwen-plus", "qwen-turbo", "qwen-vl-max",
		"deepseek-chat", "deepseek-coder", "mistral-medium-latest", "command-r",
		"ERNIE-Bot-4", "text-embedding-v1", "@cf/openai/whisper",
	} {
		if _, ok := defaults[model]; ok {
			t.Errorf("%s should not have a default price", model)
		}
	}
}

// Prices that replaced outdated legacy defaults, as published by each vendor.
func TestMigratedOfficialPrices(t *testing.T) {
	cases := map[string]struct {
		priceType     string
		channelType   int
		input, output float64
		extra         map[string]float64
	}{
		// $8 image in, $30 image out, $5 text in.
		"gpt-image-2.6-flare": {TokensPriceType, config.ChannelTypeOpenAI, 4, 15,
			map[string]float64{config.UsageExtraInputTextTokens: 0.625}},
		// $8 image in, $32 image out, $5 text in, $10 text out.
		"gpt-image-1.5": {TokensPriceType, config.ChannelTypeOpenAI, 4, 16,
			map[string]float64{config.UsageExtraInputTextTokens: 0.625, config.UsageExtraOutputTextTokens: 0.3125}},
		"chatgpt-image-latest": {TokensPriceType, config.ChannelTypeOpenAI, 4, 16,
			map[string]float64{config.UsageExtraInputTextTokens: 0.625, config.UsageExtraOutputTextTokens: 0.3125}},
		// $2.50 image in, $8 image out, $2 text in.
		"gpt-image-1-mini": {TokensPriceType, config.ChannelTypeOpenAI, 1.25, 4,
			map[string]float64{config.UsageExtraInputTextTokens: 0.8}},
		// $0.13 / 1M, input only.
		"text-embedding-3-large": {TokensPriceType, config.ChannelTypeOpenAI, 0.065, 0, nil},
		"omni-moderation-latest": {TokensPriceType, config.ChannelTypeOpenAI, 0, 0, nil},
		// $15 / 1M characters.
		"tts-1": {TokensPriceType, config.ChannelTypeOpenAI, 7.5, 0, nil},
		// $0.15 / $0.60, cached $0.015.
		"mistral-small-latest": {TokensPriceType, config.ChannelTypeMistral, 0.075, 0.3,
			map[string]float64{config.UsageExtraCache: 0.1}},
		// $0.70 / $2.10, cached $0.07.
		"mistral-large-latest": {TokensPriceType, config.ChannelTypeMistral, 0.35, 1.05,
			map[string]float64{config.UsageExtraCache: 0.1}},
		"mistral-embed": {TokensPriceType, config.ChannelTypeMistral, 0.05, 0,
			map[string]float64{config.UsageExtraCache: 0.1}},
		// $0.04 / $0.08 per song.
		"lyria-3-clip-preview": {TimesPriceType, config.ChannelTypeGemini, 20, 20, nil},
		"lyria-3-pro-preview":  {TimesPriceType, config.ChannelTypeGemini, 40, 40, nil},
	}
	prices := defaultPriceByModel(t)
	for name, w := range cases {
		p, ok := prices[name]
		if !ok {
			t.Errorf("%s: missing", name)
			continue
		}
		if p.Type != w.priceType || p.ChannelType != w.channelType || !nearly(p.Input, w.input) || !nearly(p.Output, w.output) {
			t.Errorf("%s = %s/%d %v/%v, want %s/%d %v/%v", name, p.Type, p.ChannelType, p.Input, p.Output,
				w.priceType, w.channelType, w.input, w.output)
		}
		var extra map[string]float64
		if p.ExtraRatios != nil {
			extra = p.ExtraRatios.Data()
		}
		if len(extra) != len(w.extra) {
			t.Errorf("%s: extra = %v, want %v", name, extra, w.extra)
		}
		for k, v := range w.extra {
			if !nearly(extra[k], v) {
				t.Errorf("%s: extra %s = %v, want %v", name, k, extra[k], v)
			}
		}
	}
}

// Whisper is an official per-minute price ($0.006 / minute = ratio 3), not a gateway default.
func TestWhisperOfficialMinutesPrice(t *testing.T) {
	var whisper *Price
	for _, p := range GetOfficialPrices() {
		if p.Model == "whisper-1" {
			whisper = p
		}
	}
	if whisper == nil || whisper.Type != MinutesPriceType || whisper.Input != 3 || whisper.ChannelType != config.ChannelTypeOpenAI {
		t.Fatalf("whisper-1 = %+v, want an official minutes price of 3", whisper)
	}
	if _, ok := gatewayDefaultPrices()["whisper-1"]; ok {
		t.Error("whisper-1 must not also be a gateway default")
	}
}

// Live sessions report audio and frame tokens; the Live prices bill them at their own rates
// relative to text (audio in 4x, frames 1.33x, audio out 2.67x for gemini-3.8-live).
func TestGeminiLiveOfficialPrice(t *testing.T) {
	price := officialTokenPrices()["gemini-3.8-live"].toPrice("gemini-3.8-live")
	if price.Input != usdPerMillionToRatio(0.75) || price.Output != usdPerMillionToRatio(4.5) {
		t.Fatalf("base prices: %v / %v", price.Input, price.Output)
	}
	want := map[string]float64{
		config.UsageExtraInputAudio:       4,
		config.UsageExtraInputImageTokens: roundRatioValue(1 / 0.75),
		config.UsageExtraOutputAudio:      roundRatioValue(12 / 4.5),
	}
	for key, ratio := range want {
		if got := price.GetExtraRatio(key); got != ratio {
			t.Errorf("%s: got %v, want %v", key, got, ratio)
		}
	}
	// Audio-only models have no extras: every token is billed at the listed audio price.
	if p := officialTokenPrices()["gemini-3.5-live-translate-preview"]; len(p.extra) != 0 {
		t.Errorf("translate extras: %v", p.extra)
	}
}
