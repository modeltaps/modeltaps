package model

import (
	"math"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// OpenRouter 返回的是 USD/token（字符串数值）。我们的内部计费用「倍率」：
// 1 倍率 = $0.002 / 1K tokens = $0.000002 / token，故 倍率 = USD每token × 500000。
// 这些测试守护换算正确 + Price 落库字段（ChannelType/Type/ExtraRatios）符合预期。

func floatEq(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestRatioFromUSDPerToken(t *testing.T) {
	cases := []struct {
		usdPerToken float64
		wantRatio   float64
	}{
		{0.000003, 1.5},   // $3 / 1M  → Claude 3.5 Sonnet 输入
		{0.000015, 7.5},   // $15 / 1M → Claude 3.5 Sonnet 输出
		{0.0000005, 0.25}, // $0.5 / 1M
		{0, 0},            // 免费
	}
	for _, c := range cases {
		got := RatioFromUSDPerToken(c.usdPerToken)
		if !floatEq(got, c.wantRatio) {
			t.Errorf("RatioFromUSDPerToken(%g) = %g, want %g", c.usdPerToken, got, c.wantRatio)
		}
	}
}

func TestPriceFromOpenRouter_WithCache(t *testing.T) {
	p := PriceFromOpenRouter("anthropic/claude-3.5-sonnet", 0.000003, 0.000015, 0.0000003)
	if p == nil {
		t.Fatal("PriceFromOpenRouter 返回 nil")
	}
	if p.Model != "anthropic/claude-3.5-sonnet" {
		t.Errorf("Model = %q", p.Model)
	}
	if p.Type != TokensPriceType {
		t.Errorf("Type = %q, want %q", p.Type, TokensPriceType)
	}
	if p.ChannelType != config.ChannelTypeOpenRouter {
		t.Errorf("ChannelType = %d, want %d", p.ChannelType, config.ChannelTypeOpenRouter)
	}
	if !floatEq(p.Input, 1.5) {
		t.Errorf("Input = %g, want 1.5", p.Input)
	}
	if !floatEq(p.Output, 7.5) {
		t.Errorf("Output = %g, want 7.5", p.Output)
	}
	// 缓存读取是相对 input 的倍数：0.0000003 / 0.000003 = 0.1
	if p.ExtraRatios == nil {
		t.Fatal("ExtraRatios 为 nil，应含 cached_read_tokens")
	}
	got := p.ExtraRatios.Data()[config.UsageExtraCachedRead]
	if !floatEq(got, 0.1) {
		t.Errorf("ExtraRatios[%q] = %g, want 0.1", config.UsageExtraCachedRead, got)
	}
}

// 倍率不应带浮点尾噪（如 1.2500000000000002）——断言精确相等而非容差。
func TestPriceRatioNoFloatNoise(t *testing.T) {
	p := PriceFromOpenRouter("x", 0.0000025, 0.00001, 0) // $2.5/1M、$10/1M
	if p.Input != 1.25 {
		t.Errorf("Input 含浮点噪声: %v (want 精确 1.25)", p.Input)
	}
	if p.Output != 5 {
		t.Errorf("Output 含浮点噪声: %v (want 精确 5)", p.Output)
	}
	c := PriceFromOpenRouter("y", 0.000003, 0.000015, 0.0000003) // cache 0.1
	if c.ExtraRatios.Data()[config.UsageExtraCachedRead] != 0.1 {
		t.Errorf("cache 倍率含噪声: %v (want 精确 0.1)", c.ExtraRatios.Data()[config.UsageExtraCachedRead])
	}
}

func TestPriceFromOpenRouter_NoCache(t *testing.T) {
	p := PriceFromOpenRouter("openai/gpt-4o", 0.0000025, 0.00001, 0)
	if p == nil {
		t.Fatal("PriceFromOpenRouter 返回 nil")
	}
	if !floatEq(p.Input, 1.25) { // $2.5/1M
		t.Errorf("Input = %g, want 1.25", p.Input)
	}
	if !floatEq(p.Output, 5) { // $10/1M
		t.Errorf("Output = %g, want 5", p.Output)
	}
	if p.ExtraRatios != nil {
		t.Errorf("无缓存价时 ExtraRatios 应为 nil，实际 %+v", p.ExtraRatios.Data())
	}
}
