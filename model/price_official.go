package model

import (
	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/datatypes"
)

// officialTokenPrice is a vendor's published standard (non-batch, non-priority) list price, in USD
// per 1M tokens, exactly as the pricing page shows it. toPrice converts it into ratios:
//
//	Input / Output   = USD per 1M tokens / 2   (ratio 1 = $0.002 / 1K tokens = $2 / 1M tokens)
//	ExtraRatios[key] = USD per 1M tokens of that token kind / input (or output) USD per 1M tokens
//	LongContext      = multipliers applied to the whole request once input tokens exceed Threshold
//
// The billing side adds (ratio - 1) x tokens of each extra kind on top of the prompt or completion
// tokens (relay/relay_util/quota_calc.go), so an extra ratio is always relative to the plain input or
// output price of the same model.
//
// ChannelType is the model's vendor. It only drives vendor attribution (owned_by and the vendor
// fallback in relay/model.go) and is not used for billing: prices are keyed by model name, so the
// same price applies on every channel that serves the model (direct API, Codex, Claude Code, Gemini
// CLI, Antigravity, Bedrock, Vertex AI).
type officialTokenPrice struct {
	channelType int
	input       float64
	output      float64
	// extra maps a usage extra key (config.UsageExtra*) to its USD price per 1M tokens.
	extra       map[string]float64
	longContext *LongContextTier
}

// Long-context tiers as published by each vendor.
var (
	// OpenAI: prompts with more than 272K input tokens are priced at 2x input and cache rates and
	// 1.5x output for the full request.
	openAILongContext = &LongContextTier{Threshold: 272000, InputRatio: 2, OutputRatio: 1.5}
	// Gemini Pro: prompts over 200k tokens pay 2x input and cache rates and 1.5x output.
	geminiProLongContext = &LongContextTier{Threshold: 200000, InputRatio: 2, OutputRatio: 1.5}
	// Claude Haiku 5.5: prompts over 100,000 tokens pay 5x on every price.
	claudeHaiku55LongContext = &LongContextTier{Threshold: 100000, InputRatio: 5, OutputRatio: 5}
)

func openAIPrice(input, cached, cacheWrite, output float64, longContext *LongContextTier) officialTokenPrice {
	extra := map[string]float64{}
	if cached > 0 {
		extra[config.UsageExtraCache] = cached
	}
	if cacheWrite > 0 {
		extra[config.UsageExtraOpenAICacheWrite] = cacheWrite
	}
	return officialTokenPrice{
		channelType: config.ChannelTypeOpenAI,
		input:       input,
		output:      output,
		extra:       extra,
		longContext: longContext,
	}
}

func claudePrice(input, write5m, write1h, read, output float64) officialTokenPrice {
	return officialTokenPrice{
		channelType: config.ChannelTypeAnthropic,
		input:       input,
		output:      output,
		extra: map[string]float64{
			config.UsageExtraCachedWrite:   write5m,
			config.UsageExtraCachedWrite1h: write1h,
			config.UsageExtraCachedRead:    read,
		},
	}
}

// geminiPrice: audio is the audio input price when it differs from text input (0 otherwise),
// image is the image output price for image generation models (0 otherwise).
func geminiPrice(input, cached, audio, output, image float64, longContext *LongContextTier) officialTokenPrice {
	extra := map[string]float64{}
	if cached > 0 {
		extra[config.UsageExtraCache] = cached
	}
	if audio > 0 {
		extra[config.UsageExtraInputAudio] = audio
	}
	if image > 0 {
		extra[config.UsageExtraOutputImageTokens] = image
	}
	return officialTokenPrice{
		channelType: config.ChannelTypeGemini,
		input:       input,
		output:      output,
		extra:       extra,
		longContext: longContext,
	}
}

// deepseekPrice: DeepSeek has no cache write fee. Cache hits arrive as cached_tokens on the OpenAI
// format and as cache_read_input_tokens on the Anthropic format, and cache creation tokens on the
// Anthropic format are billed as cache misses.
func deepseekPrice(cacheMiss, cacheHit, output float64) officialTokenPrice {
	return officialTokenPrice{
		channelType: config.ChannelTypeDeepseek,
		input:       cacheMiss,
		output:      output,
		extra: map[string]float64{
			config.UsageExtraCache:         cacheHit,
			config.UsageExtraCachedRead:    cacheHit,
			config.UsageExtraCachedWrite:   cacheMiss,
			config.UsageExtraCachedWrite1h: cacheMiss,
		},
	}
}

// officialTokenPrices lists current models at their official list prices (checked 2026-10-15).
func officialTokenPrices() map[string]officialTokenPrice {
	return map[string]officialTokenPrice{
		// OpenAI, USD per 1M tokens: input, cached input, cache writes, output.
		// https://developers.openai.com/api/docs/pricing
		// Long-context tiers from https://developers.openai.com/api/docs/models/<model>.
		"gpt-6-astra":        openAIPrice(10, 1, 12.5, 50, openAILongContext),
		"gpt-6.1-sol":        openAIPrice(2, 0.1, 2.5, 10, openAILongContext),
		"gpt-6-sol":          openAIPrice(2, 0.2, 2.5, 10, openAILongContext),
		"gpt-6-luna":         openAIPrice(0.1, 0.01, 0.125, 0.5, openAILongContext),
		"gpt-5.6-sol":        openAIPrice(4, 0.4, 5, 20, openAILongContext), // promotional price, at least through 2026-11-21
		"gpt-5.6":            openAIPrice(4, 0.4, 5, 20, openAILongContext), // alias of gpt-5.6-sol
		"gpt-5.6-terra":      openAIPrice(2, 0.2, 2.5, 12, openAILongContext),
		"gpt-5.6-luna":       openAIPrice(0.2, 0.02, 0.25, 1.2, openAILongContext),
		"gpt-5.5":            openAIPrice(5, 0.5, 0, 30, openAILongContext),
		"gpt-5.5-pro":        openAIPrice(30, 0, 0, 180, nil),
		"gpt-5.4":            openAIPrice(2.5, 0.25, 0, 15, openAILongContext),
		"gpt-5.4-pro":        openAIPrice(30, 0, 0, 180, openAILongContext),
		"gpt-5.4-mini":       openAIPrice(0.75, 0.075, 0, 4.5, nil),
		"gpt-5.4-nano":       openAIPrice(0.2, 0.02, 0, 1.25, nil),
		"gpt-5.3-codex":      openAIPrice(1.75, 0.175, 0, 14, nil),
		"gpt-5.2-codex":      openAIPrice(1.75, 0.175, 0, 14, nil),
		"gpt-5.2":            openAIPrice(1.75, 0.175, 0, 14, nil),
		"gpt-5.2-pro":        openAIPrice(21, 0, 0, 168, nil),
		"gpt-5.1":            openAIPrice(1.25, 0.125, 0, 10, nil),
		"gpt-5.1-codex":      openAIPrice(1.25, 0.125, 0, 10, nil),
		"gpt-5.1-codex-max":  openAIPrice(1.25, 0.125, 0, 10, nil),
		"gpt-5.1-codex-mini": openAIPrice(0.25, 0.025, 0, 2, nil),
		"gpt-5":              openAIPrice(1.25, 0.125, 0, 10, nil),
		"gpt-5-codex":        openAIPrice(1.25, 0.125, 0, 10, nil),
		"gpt-5-chat-latest":  openAIPrice(1.25, 0.125, 0, 10, nil),
		"gpt-5-mini":         openAIPrice(0.25, 0.025, 0, 2, nil),
		"gpt-5-nano":         openAIPrice(0.05, 0.005, 0, 0.4, nil),
		"gpt-5-pro":          openAIPrice(15, 0, 0, 120, nil),
		"chat-latest":        openAIPrice(5, 0.5, 0, 30, nil),
		"codex-mini-latest":  openAIPrice(1.5, 0.375, 0, 6, nil),
		"gpt-4.1":            openAIPrice(2, 0.5, 0, 8, nil),
		"gpt-4.1-mini":       openAIPrice(0.4, 0.1, 0, 1.6, nil),
		"gpt-4.1-nano":       openAIPrice(0.1, 0.025, 0, 0.4, nil),
		"gpt-4o":             openAIPrice(2.5, 1.25, 0, 10, nil),
		"gpt-4o-2024-05-13":  openAIPrice(5, 0, 0, 15, nil),
		"gpt-4o-mini":        openAIPrice(0.15, 0.075, 0, 0.6, nil),
		"o3":                 openAIPrice(2, 0.5, 0, 8, nil),
		"o3-pro":             openAIPrice(20, 0, 0, 80, nil),
		"o3-mini":            openAIPrice(1.1, 0.55, 0, 4.4, nil),
		"o4-mini":            openAIPrice(1.1, 0.275, 0, 4.4, nil),
		"o1":                 openAIPrice(15, 7.5, 0, 60, nil),
		"o1-pro":             openAIPrice(150, 0, 0, 600, nil),

		// Anthropic, USD per 1M tokens: base input, 5m cache writes, 1h cache writes, cache hits, output.
		// https://platform.claude.com/docs/en/about-claude/pricing
		// Model IDs from https://platform.claude.com/docs/en/about-claude/models/overview and
		// https://platform.claude.com/docs/en/about-claude/model-deprecations.
		"claude-fable-5-1":           claudePrice(10, 12.5, 20, 0.25, 50),
		"claude-fable-5":             claudePrice(10, 12.5, 20, 1, 50),
		"claude-opus-5-5":            claudePrice(4, 5, 8, 0.2, 20),
		"claude-opus-5":              claudePrice(5, 6.25, 10, 0.5, 25),
		"claude-opus-4-8":            claudePrice(5, 6.25, 10, 0.5, 25),
		"claude-opus-4-7":            claudePrice(5, 6.25, 10, 0.5, 25),
		"claude-opus-4-6":            claudePrice(5, 6.25, 10, 0.5, 25),
		"claude-opus-4-5":            claudePrice(5, 6.25, 10, 0.5, 25),
		"claude-opus-4-5-20251101":   claudePrice(5, 6.25, 10, 0.5, 25),
		"claude-opus-4-1-20250805":   claudePrice(15, 18.75, 30, 1.5, 75), // retired except on Bedrock and Google Cloud
		"claude-opus-4-20250514":     claudePrice(15, 18.75, 30, 1.5, 75), // retired except on Google Cloud
		"claude-sonnet-5-5":          claudePrice(2, 2.5, 4, 0.1, 10),
		"claude-sonnet-5":            claudePrice(2, 2.5, 4, 0.2, 10),
		"claude-sonnet-4-6":          claudePrice(3, 3.75, 6, 0.3, 15),
		"claude-sonnet-4-5":          claudePrice(3, 3.75, 6, 0.3, 15),
		"claude-sonnet-4-5-20250929": claudePrice(3, 3.75, 6, 0.3, 15),
		"claude-sonnet-4-20250514":   claudePrice(3, 3.75, 6, 0.3, 15), // retired except on Bedrock and Google Cloud
		"claude-haiku-4-5":           claudePrice(1, 1.25, 2, 0.1, 5),
		"claude-haiku-4-5-20251001":  claudePrice(1, 1.25, 2, 0.1, 5),
		"claude-3-5-haiku-20241022":  claudePrice(0.8, 1, 1.6, 0.08, 4), // retired except on Google Cloud
		// Prompts up to 100,000 tokens; over that every price is 5x ($0.50 / $0.625 / $1 / $0.05 / $2.50).
		"claude-haiku-5-5": func() officialTokenPrice {
			p := claudePrice(0.1, 0.125, 0.2, 0.01, 0.5)
			p.longContext = claudeHaiku55LongContext
			return p
		}(),
		// Antigravity model names that providers/antigravity/chat.go (alias2ModelName) maps to
		// Claude Sonnet 4.5 / Claude Opus 4.5; billed at those models' prices.
		"gemini-claude-sonnet-4-5":          claudePrice(3, 3.75, 6, 0.3, 15),
		"gemini-claude-sonnet-4-5-thinking": claudePrice(3, 3.75, 6, 0.3, 15),
		"gemini-claude-opus-4-5-thinking":   claudePrice(5, 6.25, 10, 0.5, 25),

		// Gemini API paid tier, USD per 1M tokens: input, context caching, audio input, output,
		// image output. https://ai.google.dev/gemini-api/docs/pricing
		"gemini-4-argon":                     geminiPrice(2, 0.1, 0, 10, 0, nil),        // through 2026-11-30; $4 / $0.20 / $20 from 2026-12-01
		"gemini-3.8-flash":                   geminiPrice(0.75, 0.075, 0, 3.75, 0, nil), // through 2026-12-31; $1.50 / $0.15 / $7.50 from 2027-01-01
		"gemini-3.6-flash":                   geminiPrice(0.75, 0.075, 0, 3.75, 0, nil), // through 2026-12-31; $1.50 / $0.15 / $7.50 from 2027-01-01
		"gemini-3.5-flash-lite":              geminiPrice(0.3, 0.03, 0, 2.5, 0, nil),
		"gemini-3.1-flash-lite":              geminiPrice(0.25, 0.025, 0.5, 1.5, 0, nil),
		"gemini-3.1-pro-preview":             geminiPrice(2, 0.2, 0, 12, 0, geminiProLongContext),
		"gemini-3.1-pro-preview-customtools": geminiPrice(2, 0.2, 0, 12, 0, geminiProLongContext),
		"gemini-3-flash-preview":             geminiPrice(0.5, 0.05, 1, 3, 0, nil),
		"gemini-2.5-pro":                     geminiPrice(1.25, 0.125, 0, 10, 0, geminiProLongContext),
		"gemini-2.5-flash":                   geminiPrice(0.3, 0.03, 1, 2.5, 0, nil),
		"gemini-2.5-flash-lite":              geminiPrice(0.1, 0.01, 0.3, 0.4, 0, nil),
		"gemini-3-pro-image":                 geminiPrice(2, 0, 0, 12, 120, nil),
		// Antigravity model name that alias2ModelName maps to gemini-3-pro-image.
		"gemini-3-pro-image-preview":  geminiPrice(2, 0, 0, 12, 120, nil),
		"gemini-3.1-flash-image":      geminiPrice(0.5, 0, 0, 3, 60, nil),
		"gemini-3.1-flash-lite-image": geminiPrice(0.25, 0, 0, 1.5, 30, nil),
		"gemini-nano-banana-2.1":      geminiPrice(1.5, 0, 0, 7.5, 30, nil),

		// DeepSeek, USD per 1M tokens at peak hours: cache miss input, cache hit input, output.
		// Off-peak rates are half; a static price cannot follow the time of day, so the peak rate
		// is used to never bill below cost. https://api-docs.deepseek.com/quick_start/pricing
		"deepseek-flash": deepseekPrice(0.3, 0.006, 1.2),
		// Legacy names served by DeepSeek-V4.1-Flash and billed at the Flash price.
		"deepseek-v4-flash":            deepseekPrice(0.3, 0.006, 1.2),
		"deepseek-v4-flash-vision-exp": deepseekPrice(0.3, 0.006, 1.2),
		"deepseek-v4-pro":              deepseekPrice(1.32, 0.044, 3.96),
	}
}

// toPrice converts an official USD price into a ratio-based Price.
func (o officialTokenPrice) toPrice(modelName string) *Price {
	price := &Price{
		Model:       modelName,
		Type:        TokensPriceType,
		ChannelType: o.channelType,
		Input:       usdPerMillionToRatio(o.input),
		Output:      usdPerMillionToRatio(o.output),
	}

	if len(o.extra) > 0 {
		extra := make(map[string]float64, len(o.extra))
		for key, usd := range o.extra {
			base := o.output
			if GetExtraPriceIsPrompt(key) {
				base = o.input
			}
			if base > 0 {
				extra[key] = roundRatioValue(usd / base)
			}
		}
		jsonExtra := datatypes.NewJSONType(extra)
		price.ExtraRatios = &jsonExtra
	}

	if o.longContext != nil {
		jsonLC := datatypes.NewJSONType(*o.longContext)
		price.LongContext = &jsonLC
	}

	return price
}

// usdPerMillionToRatio converts USD per 1M tokens into a ratio (ratio 1 = $2 / 1M tokens).
func usdPerMillionToRatio(usd float64) float64 {
	return roundRatioValue(usd / (DollarRate * 1000))
}
