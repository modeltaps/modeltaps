package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// GetPrice 未命中时不再返回误导性的「30」，而是返回 Unconfigured 标记价(0/0)。
// 计费侧据 unpriced_model_policy 处理：block 拒绝、zero 放行不计费、default 用可配默认倍率。

func TestGetPrice_UnconfiguredFallback(t *testing.T) {
	p := &Pricing{Prices: make(map[string]*Price), Match: []string{}}
	price := p.GetPrice("does/not-exist")
	if price == nil {
		t.Fatal("GetPrice 返回 nil")
	}
	if !price.Unconfigured {
		t.Error("未命中应标记 Unconfigured=true")
	}
	if price.Input != 0 || price.Output != 0 {
		t.Errorf("未命中兜底不应再是 30，应为 0/0，实际 %g/%g", price.Input, price.Output)
	}
	if price.ChannelType != config.ChannelTypeUnknown {
		t.Errorf("ChannelType 应为 Unknown，实际 %d", price.ChannelType)
	}
}

func TestApplyUnpricedPolicy(t *testing.T) {
	// 已配置价格：策略不应改动它，且不拒绝
	configured := &Price{Input: 1.5, Output: 7.5}
	if reject := configured.ApplyUnpricedPolicy("block", 30); reject {
		t.Error("已配置价格不应被拒绝")
	}
	if configured.Input != 1.5 {
		t.Error("已配置价格不应被改动")
	}

	// 未配置 + block → 拒绝
	if reject := (&Price{Unconfigured: true}).ApplyUnpricedPolicy("block", 30); !reject {
		t.Error("block 策略应拒绝未配置模型")
	}

	// 未配置 + zero → 不拒绝，保持 0
	zp := &Price{Unconfigured: true}
	if reject := zp.ApplyUnpricedPolicy("zero", 30); reject {
		t.Error("zero 策略不应拒绝")
	}
	if zp.Input != 0 || zp.Output != 0 {
		t.Error("zero 策略应保持 0 计费")
	}

	// 未配置 + default → 不拒绝，用默认倍率
	dp := &Price{Unconfigured: true}
	if reject := dp.ApplyUnpricedPolicy("default", 30); reject {
		t.Error("default 策略不应拒绝")
	}
	if dp.Input != 30 || dp.Output != 30 {
		t.Errorf("default 策略应用默认倍率 30，实际 %g/%g", dp.Input, dp.Output)
	}

	// 未知策略 → 退回 block(拒绝)，安全优先
	if reject := (&Price{Unconfigured: true}).ApplyUnpricedPolicy("bogus", 30); !reject {
		t.Error("未知策略应退回 block(拒绝)")
	}
}
