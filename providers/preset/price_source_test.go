package preset

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

func TestPresetPricePages(t *testing.T) {
	for typ := range Presets {
		if pricePages[typ] == nil {
			t.Errorf("preset type %d has no price page", typ)
		}
	}
	cases := []struct {
		typ   int
		model string
		want  string
		exact bool
	}{
		{config.ChannelTypeNovita, "deepseek/deepseek-v4.1-flash", "https://novita.ai/models/model-detail/deepseek-deepseek-v4.1-flash", true},
		{config.ChannelTypeChutes, "moonshotai/Kimi-K2.6-TEE", "https://chutes.ai/app/chute/chutes-moonshotai-kimi-k2-6-tee", true},
		{config.ChannelTypeFireworks, "accounts/fireworks/models/llama-v3p3-70b-instruct", "https://fireworks.ai/models/fireworks/llama-v3p3-70b-instruct", true},
		{config.ChannelTypeHuggingFace, "openai/gpt-oss-120b", "https://huggingface.co/inference/models?search=openai%2Fgpt-oss-120b", true},
		{config.ChannelTypeLongCat, "LongCat-2.5", "https://longcat.chat/platform/docs/pricing/longcat-2.5", true},
		{config.ChannelTypeTogether, "meta-llama/Llama-3.3-70B-Instruct-Turbo", "https://www.together.ai/pricing", false},
	}
	for _, c := range cases {
		p := &model.ChannelPrice{Model: c.model}
		model.DescribeChannelPrices(c.typ, p)
		if p.PageURL != c.want || p.PageExact != c.exact || p.Via != "models.dev" || p.Source == "" {
			t.Errorf("%d %s = %+v", c.typ, c.model, p)
		}
	}
}
