package model

import (
	"reflect"
	"testing"
	"time"
)

func pickerByModel(result *ModelPickerResult) map[string]ModelPickerItem {
	out := make(map[string]ModelPickerItem, len(result.Models))
	for _, item := range result.Models {
		out[item.Model] = item
	}
	return out
}

// 每个家族只标一个最新版本：变体、浮动别名、已过下架时间的不参与；同版本正式版优先于预览版，
// 不带快照的别名优先于日期快照。
func TestBuildModelPickerLatest(t *testing.T) {
	setupModelInfoTestDB(t)
	past := time.Now().Add(-time.Hour).Unix()
	if err := DB.Create(&ModelInfo{Model: "vendor/old-model-9", ExpiresAt: past}).Error; err != nil {
		t.Fatal(err)
	}
	models := []string{
		"anthropic/claude-opus-5.5", "anthropic/claude-opus-5.5:batch", "anthropic/claude-opus-5", "anthropic/claude-opus-4.8",
		"~anthropic/claude-opus-latest",
		"google/gemini-3.1-pro-preview", "google/gemini-2.5-pro",
		"google/gemini-3-flash-preview", "google/gemini-3-flash",
		"gpt-4o-2024-11-20", "gpt-4o",
		"vendor/old-model-9", "vendor/old-model-8",
		"only/thing:free",
	}
	result, err := BuildModelPicker(models, nil)
	if err != nil {
		t.Fatal(err)
	}
	latest := []string{}
	for _, item := range result.Models {
		if item.Latest {
			latest = append(latest, item.Model)
		}
	}
	want := []string{"anthropic/claude-opus-5.5", "google/gemini-3.1-pro-preview", "google/gemini-3-flash", "gpt-4o", "vendor/old-model-8", "only/thing:free"}
	if !reflect.DeepEqual(latest, want) {
		t.Errorf("latest = %v, want %v", latest, want)
	}
	for _, family := range result.Families {
		if family.Family == "claude-opus" && (family.Count != 5 || family.Latest != "anthropic/claude-opus-5.5") {
			t.Errorf("claude-opus family = %+v", family)
		}
	}
}

// 目录元数据按完整 id 查，查不到按去掉厂商前缀的 id 查；查不到时字段为空数组 / nil 而不是缺失。
func TestBuildModelPickerCatalog(t *testing.T) {
	setupModelInfoTestDB(t)
	yes := true
	if err := DB.Create(&ModelInfo{
		Model: "claude-opus-5-5", VendorID: 3, ContextLength: 200000, ReleasedAt: 1790000000,
		InputModalities: `["text","image"]`, OutputModalities: `["text"]`, Capabilities: `["tool_call","reasoning"]`,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&ModelInfo{Model: "qwen/qwen3.8-27b", OpenWeights: &yes}).Error; err != nil {
		t.Fatal(err)
	}
	result, err := BuildModelPicker([]string{"claude-opus-5-5", "qwen/qwen3.8-27b", "unknown-model-1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := pickerByModel(result)
	opus := got["claude-opus-5-5"]
	if opus.VendorID != 3 || opus.ContextLength != 200000 || opus.ReleasedAt != 1790000000 ||
		!reflect.DeepEqual(opus.InputModalities, []string{"text", "image"}) ||
		!reflect.DeepEqual(opus.Capabilities, []string{"tool_call", "reasoning"}) {
		t.Errorf("opus = %+v", opus)
	}
	if qwen := got["qwen/qwen3.8-27b"]; qwen.OpenWeights == nil || !*qwen.OpenWeights {
		t.Errorf("qwen open weights = %+v", qwen.OpenWeights)
	}
	unknown := got["unknown-model-1"]
	if unknown.OpenWeights != nil || unknown.Capabilities == nil || len(unknown.Capabilities) != 0 || unknown.InputPrice != nil {
		t.Errorf("unknown = %+v", unknown)
	}
}

// 家族排序：有本站用量的按请求数排在最前；其余按兜底档位，同档新发布的在前。
func TestBuildModelPickerFamilyRank(t *testing.T) {
	setupModelInfoTestDB(t)
	for _, info := range []ModelInfo{
		{Model: "vendor-a/qwen3.8-flash", ReleasedAt: 100},
		{Model: "vendor-a/deepseek-v4-flash", ReleasedAt: 200},
	} {
		if err := DB.Create(&info).Error; err != nil {
			t.Fatal(err)
		}
	}
	models := []string{"vendor-a/acme-1", "vendor-a/qwen3.8-flash", "vendor-a/deepseek-v4-flash", "vendor-a/claude-sonnet-5", "vendor-a/glm-5"}
	result, err := BuildModelPicker(models, map[string]int64{"glm": 40, "acme": 3})
	if err != nil {
		t.Fatal(err)
	}
	order := []string{}
	for _, family := range result.Families {
		order = append(order, family.Family)
	}
	want := []string{"glm", "acme", "claude-sonnet", "deepseek-v-flash", "qwen-flash"}
	if !reflect.DeepEqual(order, want) {
		t.Errorf("family order = %v, want %v", order, want)
	}
	if result.Families[0].Rank != 1 || result.Families[0].Requests != 40 {
		t.Errorf("top family = %+v", result.Families[0])
	}
}

// 家族热度：statistics 近 30 天按家族汇总，窗口外的不算；不同写法的同一家族合并。
func TestFamilyRequestCounts(t *testing.T) {
	setupModelInfoTestDB(t)
	if err := DB.AutoMigrate(&Statistics{}); err != nil {
		t.Fatal(err)
	}
	today := time.Now()
	rows := []Statistics{
		{Date: today, UserId: 1, ChannelId: 1, ModelName: "claude-opus-5-5", RequestCount: 10},
		{Date: today.AddDate(0, 0, -3), UserId: 2, ChannelId: 1, ModelName: "anthropic/claude-opus-4.8", RequestCount: 5},
		{Date: today.AddDate(0, 0, -40), UserId: 1, ChannelId: 1, ModelName: "claude-opus-4-5", RequestCount: 100},
		{Date: today, UserId: 1, ChannelId: 2, ModelName: "gpt-5.5", RequestCount: 7},
	}
	for _, row := range rows {
		if err := DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	counts, err := FamilyRequestCounts(ModelPickerHeatDays)
	if err != nil {
		t.Fatal(err)
	}
	if counts["claude-opus"] != 15 || counts["gpt"] != 7 || len(counts) != 2 {
		t.Errorf("counts = %v", counts)
	}
}

// 售价：按 token 计费的价格换算成 USD / 1M tokens；按次计费与未配置的为 null。
func TestBuildModelPickerPrices(t *testing.T) {
	setupModelInfoTestDB(t)
	oldPricing := PricingInstance
	PricingInstance = &Pricing{Prices: map[string]*Price{
		"claude-opus-5-5": {Model: "claude-opus-5-5", Type: TokensPriceType, Input: 2.5, Output: 12.5},
		"dall-e-9":        {Model: "dall-e-9", Type: TimesPriceType, Input: 20, Output: 20},
	}}
	t.Cleanup(func() { PricingInstance = oldPricing })

	result, err := BuildModelPicker([]string{"claude-opus-5-5", "dall-e-9", "no-price-1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := pickerByModel(result)
	opus := got["claude-opus-5-5"]
	if opus.InputPrice == nil || *opus.InputPrice != 5 || opus.OutputPrice == nil || *opus.OutputPrice != 25 {
		t.Errorf("opus prices = %v / %v", opus.InputPrice, opus.OutputPrice)
	}
	if got["dall-e-9"].InputPrice != nil || got["no-price-1"].InputPrice != nil {
		t.Errorf("times / unpriced models must have no token price")
	}
}
