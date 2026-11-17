package model

import "github.com/modeltaps/modeltaps/common/config"

// Gateway default prices.
//
// These are NOT vendor list prices. They are defaults the gateway presets for models it still
// supports but whose vendor publishes no per-token API price that converts into the official price
// table: per-request and task models (Midjourney, Suno), and models priced per image
// while the gateway bills them by tokens (DALL-E, Stability AI). Without a preset, a new
// deployment with unpriced_model_policy=block would reject these models outright. Administrators
// should adjust them to their own cost.
//
// The values are the long-standing gateway defaults, unchanged. They are part of GetDefaultPrice but
// never of GetOfficialPrices or /api/prices?type=official, and a model name may not appear both here
// and in the official tables (price_official.go).

type gatewayDefaultPrice struct {
	priceType   string
	channelType int
	input       float64
	output      float64
}

func gatewayTimesPrice(channelType int, ratio float64) gatewayDefaultPrice {
	return gatewayDefaultPrice{priceType: TimesPriceType, channelType: channelType, input: ratio, output: ratio}
}

func gatewayTokensPrice(channelType int, input, output float64) gatewayDefaultPrice {
	return gatewayDefaultPrice{priceType: TokensPriceType, channelType: channelType, input: input, output: output}
}

func gatewayDefaultPrices() map[string]gatewayDefaultPrice {
	prices := map[string]gatewayDefaultPrice{
		// DALL-E: officially priced per image ($0.016-$0.020 for DALL-E 2, $0.040-$0.120 for DALL-E 3);
		// no longer listed on the OpenAI pricing page.
		"dall-e-2": gatewayTokensPrice(config.ChannelTypeOpenAI, 8, 8),
		"dall-e-3": gatewayTokensPrice(config.ChannelTypeOpenAI, 20, 20),

		// Stability AI: officially priced in credits per image.
		"sd3":               gatewayTokensPrice(config.ChannelTypeStabilityAI, 32.5, 32.5),
		"sd3-turbo":         gatewayTokensPrice(config.ChannelTypeStabilityAI, 20, 20),
		"stable-image-core": gatewayTokensPrice(config.ChannelTypeStabilityAI, 15, 15),

		// Suno (relay/task/suno): per request; no official public API price.
		"suno_lyrics": gatewayTimesPrice(config.ChannelTypeSuno, 5),
		"chirp-v3-0":  gatewayTimesPrice(config.ChannelTypeSuno, 50),
		"chirp-v3-5":  gatewayTimesPrice(config.ChannelTypeSuno, 50),
	}

	// Midjourney (relay/midjourney): per action, for the fast, turbo and relax modes; Midjourney has
	// no official API price. inpaint and custom_zoom only open the upstream's input dialog and are
	// never charged, whatever their price: the edit itself is submitted and billed as modal, priced
	// like zoom and variation.
	mjActions := map[string]float64{
		"imagine":        50,
		"variation":      50,
		"reroll":         50,
		"blend":          50,
		"modal":          50,
		"zoom":           50,
		"shorten":        50,
		"high_variation": 50,
		"low_variation":  50,
		"pan":            50,
		"inpaint":        0,
		"custom_zoom":    0,
		"describe":       25,
		"upscale":        25,
		"swap_face":      25,
		"upload":         0,
	}
	for _, mode := range []string{"mj_", "mj_turbo_", "mj_relax_"} {
		for action, ratio := range mjActions {
			prices[mode+action] = gatewayTimesPrice(config.ChannelTypeMidjourney, ratio)
		}
	}

	return prices
}

func (g gatewayDefaultPrice) toPrice(modelName string) *Price {
	return &Price{
		Model:       modelName,
		Type:        g.priceType,
		ChannelType: g.channelType,
		Input:       g.input,
		Output:      g.output,
	}
}
