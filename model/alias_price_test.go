package model

import "testing"

// mappingTargetFor：从单条渠道 model_mapping JSON 里取 modelName 的映射目标。
// "+" 前缀表示按传入(别名)计费，此时别名用自己的价，不解析目标 → ok=false。
func TestMappingTargetFor(t *testing.T) {
	cases := []struct {
		mapping   string
		model     string
		wantTgt   string
		wantOk    bool
	}{
		{`{"opus-latest":"claude-opus-4.8"}`, "opus-latest", "claude-opus-4.8", true},
		{`{"opus-latest":"+claude-opus-4.8"}`, "opus-latest", "", false}, // 按传入计费→不解析
		{`{"a":"b"}`, "opus-latest", "", false},                          // 无此 key
		{``, "opus-latest", "", false},                                   // 空
		{`not-json`, "opus-latest", "", false},                           // 非法
	}
	for _, c := range cases {
		tgt, ok := mappingTargetFor(c.mapping, c.model)
		if tgt != c.wantTgt || ok != c.wantOk {
			t.Errorf("mappingTargetFor(%q,%q)=(%q,%v) want (%q,%v)", c.mapping, c.model, tgt, ok, c.wantTgt, c.wantOk)
		}
	}
}

// Billing an upstream alias without its own price uses the canonical model's price; an alias with
// its own price, a non-alias and an alias whose canonical is unpriced are unchanged.
func TestGetBillingPriceFallsBackToCanonical(t *testing.T) {
	setupModelInfoTestDB(t)
	for _, info := range []*ModelInfo{
		{Model: "claude-opus-5"},
		{Model: "anthropic/claude-opus-5", AliasOf: "claude-opus-5"},
		{Model: "vendor/opus-own-price", AliasOf: "claude-opus-5"},
		{Model: "gpt-unpriced"},
		{Model: "openai/gpt-unpriced", AliasOf: "gpt-unpriced"},
	} {
		if err := DB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}
	oldPricing := PricingInstance
	PricingInstance = &Pricing{Prices: map[string]*Price{
		"claude-opus-5":         {Model: "claude-opus-5", Type: TokensPriceType, Input: 5, Output: 25},
		"vendor/opus-own-price": {Model: "vendor/opus-own-price", Type: TokensPriceType, Input: 1, Output: 2},
	}}
	t.Cleanup(func() { PricingInstance = oldPricing })

	if got := PricingInstance.GetBillingPrice("anthropic/claude-opus-5"); got.Unconfigured || got.Input != 5 || got.Output != 25 {
		t.Fatalf("alias without price = %+v, want the canonical price", got)
	}
	if got := PricingInstance.GetBillingPrice("vendor/opus-own-price"); got.Input != 1 {
		t.Fatalf("alias with its own price = %+v", got)
	}
	for _, name := range []string{"openai/gpt-unpriced", "not-in-catalog"} {
		if got := PricingInstance.GetBillingPrice(name); !got.Unconfigured {
			t.Fatalf("%s = %+v, want unconfigured", name, got)
		}
	}
	// Display lookups keep their old behaviour.
	if got := PricingInstance.GetPrice("anthropic/claude-opus-5"); !got.Unconfigured {
		t.Fatalf("GetPrice must not fall back: %+v", got)
	}
}
