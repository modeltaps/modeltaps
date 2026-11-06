package model

import (
	"encoding/json"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// Every gpt-image generation that has a default price must also have the extra ratio that
// prices its text input tokens, otherwise image requests with a text prompt are billed at the
// image-input rate.
func TestGPTImageDefaultPricesHaveTextTokenRatio(t *testing.T) {
	var extra map[string]map[string]float64
	if err := json.Unmarshal([]byte(GetDefaultExtraRatio()), &extra); err != nil {
		t.Fatalf("default extra ratio is not valid JSON: %v", err)
	}
	prices := defaultPriceByModel(t)

	for _, name := range []string{
		"gpt-image-2.5-flare", "gpt-image-2.5-sunburst",
		"gpt-image-2.6-flare", "gpt-image-2.6-sunburst",
	} {
		p, ok := prices[name]
		if !ok {
			t.Fatalf("missing default price for %s", name)
		}
		if p.ChannelType != config.ChannelTypeOpenAI || p.Input != 4 || p.Output != 15 {
			t.Errorf("%s: unexpected price %+v", name, p)
		}
		if got := extra[name]["input_text_tokens"]; got != 0.625 {
			t.Errorf("%s: input_text_tokens ratio = %v, want 0.625", name, got)
		}
	}
}
