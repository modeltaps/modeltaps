package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func TestDescribeChannelPrices(t *testing.T) {
	vercel := &ChannelPrice{Model: "anthropic/claude-sonnet-4.5"}
	router := &ChannelPrice{Model: "anthropic/claude-sonnet-4.5"}
	DescribeChannelPrices(config.ChannelTypeVercelAIGateway, vercel)
	DescribeChannelPrices(config.ChannelTypeOpenRouter, router)
	if vercel.Source != "Vercel AI Gateway" || vercel.PageURL != "https://vercel.com/ai-gateway/models/claude-sonnet-4.5" || !vercel.PageExact {
		t.Fatalf("vercel = %+v", vercel)
	}
	if router.PageURL != "https://openrouter.ai/anthropic/claude-sonnet-4.5" {
		t.Fatalf("openrouter = %+v", router)
	}
	for name, want := range map[string]string{
		"anthropic/claude-haiku-4-5": "https://www.requesty.ai/models/anthropic/claude-haiku-4-5",
		"claude-haiku-4-5":           "https://www.requesty.ai/models",
	} {
		p := &ChannelPrice{Model: name}
		DescribeChannelPrices(config.ChannelTypeRequesty, p)
		if p.PageURL != want || p.PageExact != (name != "claude-haiku-4-5") {
			t.Fatalf("requesty %s = %+v", name, p)
		}
	}
	none := &ChannelPrice{Model: "m"}
	DescribeChannelPrices(config.ChannelTypeOpenAI, none)
	if none.Source != "" || none.PageURL != "" {
		t.Fatalf("a type without a source = %+v", none)
	}
	if got, _ := PricingPage("https://x.test/pricing")("a/b"); got != "https://x.test/pricing" {
		t.Fatalf("pricing page = %s", got)
	}
	if got := fillPageTemplate("https://x.test/{id}", "a b/c"); got != "https://x.test/a%20b/c" {
		t.Fatalf("escaped = %s", got)
	}
}
