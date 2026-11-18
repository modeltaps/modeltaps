package preset

import (
	"net/url"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// pricePages gives, per platform, the web page where a person can check the price of a model the
// channel sends upstream: a page for the model where the platform has one that can be built from
// the API name, else the platform's pricing page. Checked by hand on 2026-11-15. Together,
// Cerebras and Baseten have model pages, but under marketing slugs that the API name does not
// give, so they get the pricing page. Nebius' catalog was behind a bot check and is unverified.
var pricePages = map[int]func(string) (string, bool){
	config.ChannelTypeTogether:  model.PricingPage("https://www.together.ai/pricing"),
	config.ChannelTypeFireworks: fireworksPage,
	config.ChannelTypeDeepInfra: model.ExactPage("https://deepinfra.com/{id}"),
	config.ChannelTypeCerebras:  model.PricingPage("https://www.cerebras.ai/pricing"),
	config.ChannelTypeNebius:    model.PricingPage("https://tokenfactory.nebius.com/model-catalog.md"),
	// "deepseek/deepseek-v4.1-flash" → model-detail/deepseek-deepseek-v4.1-flash
	config.ChannelTypeNovita:    slugPage("https://novita.ai/models/model-detail/", "", strings.NewReplacer("/", "-"), false),
	config.ChannelTypeSambaNova: model.PricingPage("https://cloud.sambanova.ai/pricing"),
	// NVIDIA's and ModelScope's model pages list no per-token price: both are free to call.
	config.ChannelTypeNvidiaNIM:   model.ExactPage("https://build.nvidia.com/{id}"),
	config.ChannelTypeModelScope:  model.ExactPage("https://www.modelscope.cn/models/{id}"),
	config.ChannelTypeQiniu:       model.PricingPage("https://www.qiniu.com/ai/models"),
	config.ChannelTypeHuggingFace: huggingFacePage,
	// "moonshotai/Kimi-K2.6-TEE" → chute/chutes-moonshotai-kimi-k2-6-tee
	config.ChannelTypeChutes:     slugPage("https://chutes.ai/app/chute/", "chutes-", strings.NewReplacer("/", "-", ".", "-"), true),
	config.ChannelTypeBaseten:    model.PricingPage("https://www.baseten.co/pricing/"),
	config.ChannelTypeStepFun:    model.PricingPage("https://platform.stepfun.com/docs/zh/pricing/details"),
	config.ChannelTypeXiaomiMiMo: model.PricingPage("https://mimo.mi.com/docs/zh-CN/price/pay-as-you-go"),
	config.ChannelTypeLongCat:    slugPage("https://longcat.chat/platform/docs/pricing/", "", strings.NewReplacer(), true),
	config.ChannelTypeVolcengine: model.PricingPage("https://docs.volcengine.com/docs/ark/model-pricing"),
	config.ChannelTypeBytePlus:   model.PricingPage("https://docs.byteplus.com/en/docs/modelark/model-pricing"),
}

// The preset platforms' channel prices come from their own list where they publish one
// (ownPriceLists), else from models.dev. Runs after presets.go's init (files initialise in name
// order), so Presets is filled.
func init() {
	for t, p := range Presets {
		via := "models.dev"
		if ownPriceLists[t] != nil {
			via = ""
		}
		model.SetChannelPriceSource(t, model.ChannelPriceSource{Name: p.Name, Via: via, Page: pricePages[t]})
	}
}

// Fireworks names models "accounts/fireworks/models/<name>"; the page is models/fireworks/<name>.
// Only serverless models show a price there.
func fireworksPage(name string) (string, bool) {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return "https://fireworks.ai/models/fireworks/" + url.PathEscape(name), true
}

// Hugging Face lists every provider's price for a model in its filtered inference table.
func huggingFacePage(name string) (string, bool) {
	return "https://huggingface.co/inference/models?search=" + url.QueryEscape(name), true
}

func slugPage(base, prefix string, replace *strings.Replacer, lower bool) func(string) (string, bool) {
	return func(name string) (string, bool) {
		slug := replace.Replace(name)
		if lower {
			slug = strings.ToLower(slug)
		}
		return base + prefix + url.PathEscape(slug), true
	}
}
