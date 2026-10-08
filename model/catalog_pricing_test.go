package model

import (
	"encoding/json"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// models.dev api.json：顶层按 provider id 分组，cost 单位为 USD/1M tokens。
// 倍率 = USD每1M ÷ 2（= USD每token × 500000）。cache_read 映射为相对 input 的 ExtraRatios。
// BuildCatalogPrices 应：① 仅处理已映射的 provider；② provider→ChannelType 正确；
// ③ 跳过 openrouter（由直连处理）与未映射 provider。

const catalogFixture = `{
  "openai":     {"models": {"gpt-4o":   {"id": "gpt-4o",   "cost": {"input": 2.5, "output": 10, "cache_read": 1.25}}}},
  "anthropic":  {"models": {"claude-x": {"id": "claude-x", "cost": {"input": 5,   "output": 25, "cache_read": 0.5}}}},
  "openrouter": {"models": {"foo/bar":  {"id": "foo/bar",  "cost": {"input": 1,   "output": 1}}}},
  "made-up":    {"models": {"zzz":      {"id": "zzz",      "cost": {"input": 1,   "output": 1}}}}
}`

func TestRatioFromUSDPerMillion(t *testing.T) {
	cases := []struct{ usd, want float64 }{
		{3, 1.5}, {10, 5}, {0, 0}, {2.5, 1.25},
	}
	for _, c := range cases {
		if got := RatioFromUSDPerMillion(c.usd); !floatEqM(got, c.want) {
			t.Errorf("RatioFromUSDPerMillion(%g) = %g, want %g", c.usd, got, c.want)
		}
	}
}

func TestBuildCatalogPrices(t *testing.T) {
	data := map[string]catalogProvider{}
	if err := json.Unmarshal([]byte(catalogFixture), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	prices := BuildCatalogPrices(data)

	byModel := map[string]*Price{}
	for _, p := range prices {
		byModel[p.Model] = p
	}

	if byModel["foo/bar"] != nil {
		t.Error("openrouter 模型应跳过（由直连处理）")
	}
	if byModel["zzz"] != nil {
		t.Error("未映射 provider 的模型应跳过")
	}

	gpt := byModel["gpt-4o"]
	if gpt == nil {
		t.Fatal("缺少 gpt-4o")
	}
	if gpt.ChannelType != config.ChannelTypeOpenAI {
		t.Errorf("gpt-4o ChannelType = %d, want %d", gpt.ChannelType, config.ChannelTypeOpenAI)
	}
	if !floatEqM(gpt.Input, 1.25) || !floatEqM(gpt.Output, 5) {
		t.Errorf("gpt-4o in/out = %g/%g, want 1.25/5", gpt.Input, gpt.Output)
	}
	if gpt.ExtraRatios == nil || !floatEqM(gpt.ExtraRatios.Data()[config.UsageExtraCachedRead], 0.5) {
		t.Errorf("gpt-4o cached_read 应为 0.5（1.25/2.5）")
	}

	claude := byModel["claude-x"]
	if claude == nil || claude.ChannelType != config.ChannelTypeAnthropic {
		t.Fatalf("claude-x 缺失或 ChannelType 错")
	}
	if !floatEqM(claude.Input, 2.5) || !floatEqM(claude.Output, 12.5) {
		t.Errorf("claude-x in/out = %g/%g, want 2.5/12.5", claude.Input, claude.Output)
	}
}

func TestBuildCatalogModelInfos(t *testing.T) {
	const fx = `{
      "openai": {"models": {"gpt-4o": {"id":"gpt-4o","name":"GPT-4o","limit":{"context":128000,"output":16384},"modalities":{"input":["text","image"],"output":["text"]},"cost":{"input":2.5,"output":10}}}},
      "made-up": {"models": {"zzz": {"id":"zzz","name":"Z","cost":{"input":1,"output":1}}}}
    }`
	data := map[string]catalogProvider{}
	if err := json.Unmarshal([]byte(fx), &data); err != nil {
		t.Fatal(err)
	}
	infos := BuildCatalogModelInfos(data)
	byModel := map[string]*ModelInfo{}
	for _, m := range infos {
		byModel[m.Model] = m
	}
	if byModel["zzz"] != nil {
		t.Error("未映射 provider 的模型应跳过")
	}
	m := byModel["gpt-4o"]
	if m == nil {
		t.Fatal("缺少 gpt-4o")
	}
	if m.Name != "GPT-4o" || m.ContextLength != 128000 || m.MaxTokens != 16384 {
		t.Errorf("元数据错: %+v", m)
	}
	if m.InputModalities != `["text","image"]` || m.OutputModalities != `["text"]` {
		t.Errorf("模态错: in=%q out=%q", m.InputModalities, m.OutputModalities)
	}
}

// models.dev 三个布尔字段 → capabilities（缺失按不支持，三项都不支持写 "[]"）。
func TestBuildCatalogModelInfosCapabilities(t *testing.T) {
	const fx = `{
      "openai": {"models": {
        "all":     {"id":"all","tool_call":true,"reasoning":true,"structured_output":true},
        "none":    {"id":"none","tool_call":false,"reasoning":false,"structured_output":false},
        "partial": {"id":"partial","tool_call":true,"structured_output":true},
        "missing": {"id":"missing"}
      }}
    }`
	data := map[string]catalogProvider{}
	if err := json.Unmarshal([]byte(fx), &data); err != nil {
		t.Fatal(err)
	}
	byModel := map[string]*ModelInfo{}
	for _, m := range BuildCatalogModelInfos(data) {
		byModel[m.Model] = m
	}

	cases := []struct{ model, want string }{
		{"all", `["tool_call","reasoning","structured_output"]`},
		{"none", `[]`},
		{"partial", `["tool_call","structured_output"]`},
		{"missing", `[]`},
	}
	for _, c := range cases {
		m := byModel[c.model]
		if m == nil {
			t.Fatalf("缺少 %s", c.model)
		}
		if m.Capabilities != c.want {
			t.Errorf("%s capabilities = %q, want %q", c.model, m.Capabilities, c.want)
		}
	}
}

// 已映射 provider 的全量刷新：来源没给价的模型不写价格（不静默写 0），音频模型不写 token 价。
func TestBuildCatalogPricesSkipsMissingAndAudio(t *testing.T) {
	const fx = `{
      "openai": {"models": {
        "priced":  {"id":"priced","cost":{"input":1,"output":2}},
        "free":    {"id":"free","cost":{"input":0,"output":0}},
        "nocost":  {"id":"nocost"},
        "tts":     {"id":"tts","modalities":{"input":["text"],"output":["audio"]},"cost":{"input":0.6,"output":12}}
      }}
    }`
	data := map[string]catalogProvider{}
	if err := json.Unmarshal([]byte(fx), &data); err != nil {
		t.Fatal(err)
	}
	byModel := map[string]*Price{}
	for _, p := range BuildCatalogPrices(data) {
		byModel[p.Model] = p
	}
	if byModel["priced"] == nil || byModel["free"] == nil {
		t.Fatalf("priced / free 应写入: %v", byModel)
	}
	if byModel["nocost"] != nil {
		t.Error("没有 cost 的模型不应写价格")
	}
	if byModel["tts"] != nil {
		t.Error("音频模型不应按 token 价写入")
	}
}

// 渠道模型名在 models.dev 全量里匹配：带厂商前缀的 id 去前缀后能对上；不限映射 provider；
// 同名冲突第一方优先；音频价格与无价列入 unconvertible；匹配不到列入 unmatched。
func TestBuildChannelCatalogEntries(t *testing.T) {
	const fx = `{
      "openai": {"models": {
        "gpt-4o-mini-tts": {"id":"gpt-4o-mini-tts","name":"GPT-4o mini TTS","modalities":{"input":["text"],"output":["audio"]},"cost":{"input":0.6,"output":12}},
        "gpt-4o": {"id":"gpt-4o","name":"GPT-4o","limit":{"context":128000},"modalities":{"input":["text","image"],"output":["text"]},"cost":{"input":2.5,"output":10}}
      }},
      "cheap-host": {"models": {
        "gpt-4o": {"id":"gpt-4o","name":"Host GPT-4o","cost":{"input":0.1,"output":0.1}}
      }},
      "vercel": {"models": {
        "openai/gpt-4o-transcribe": {"id":"openai/gpt-4o-transcribe","modalities":{"input":["audio"],"output":["text"]},"cost":{"input":2.5,"output":10}},
        "zai/glm-x": {"id":"zai/glm-x","name":"GLM X","modalities":{"input":["text"],"output":["text"]}}
      }},
      "openrouter": {"models": {
        "only/on-openrouter": {"id":"only/on-openrouter","cost":{"input":1,"output":1}}
      }}
    }`
	data := map[string]catalogProvider{}
	if err := json.Unmarshal([]byte(fx), &data); err != nil {
		t.Fatal(err)
	}
	names := []string{"openai/gpt-4o-mini-tts", "openai/gpt-4o", "gpt-4o-transcribe", "glm-x", "only/on-openrouter", "nobody/knows"}
	infos, prices, unconvertible, unmatched := BuildChannelCatalogEntries(data, names)

	infoBy := map[string]*ModelInfo{}
	for _, m := range infos {
		infoBy[m.Model] = m
	}
	priceBy := map[string]*Price{}
	for _, p := range prices {
		priceBy[p.Model] = p
	}
	reasonBy := map[string]string{}
	for _, u := range unconvertible {
		reasonBy[u.Model] = u.Reason
	}

	tts := infoBy["openai/gpt-4o-mini-tts"]
	if tts == nil || tts.Name != "GPT-4o mini TTS" || tts.Endpoints != `["audio.speech"]` {
		t.Errorf("前缀 TTS 匹配或接口能力错: %+v", tts)
	}
	if reasonBy["openai/gpt-4o-mini-tts"] != UnconvertibleAudioTokenPricing {
		t.Errorf("TTS 应列为换算不了, got %q", reasonBy["openai/gpt-4o-mini-tts"])
	}
	if priceBy["openai/gpt-4o-mini-tts"] != nil {
		t.Error("换算不了的 TTS 不应写价格")
	}

	gpt := infoBy["openai/gpt-4o"]
	if gpt == nil || gpt.Name != "GPT-4o" || gpt.ContextLength != 128000 {
		t.Errorf("同名冲突应取第一方 openai: %+v", gpt)
	}
	if p := priceBy["openai/gpt-4o"]; p == nil || !floatEqM(p.Input, 1.25) || p.ChannelType != config.ChannelTypeOpenAI {
		t.Errorf("openai/gpt-4o 价格错: %+v", p)
	}

	stt := infoBy["gpt-4o-transcribe"]
	if stt == nil || stt.Endpoints != `["audio.transcription"]` {
		t.Errorf("去前缀索引应匹配 STT: %+v", stt)
	}
	if reasonBy["gpt-4o-transcribe"] != UnconvertibleAudioTokenPricing {
		t.Errorf("STT 应列为换算不了, got %q", reasonBy["gpt-4o-transcribe"])
	}

	if infoBy["glm-x"] == nil || reasonBy["glm-x"] != UnconvertibleNoPrice {
		t.Errorf("未映射 provider 也应匹配元信息，无价列为 no_price: info=%v reason=%q", infoBy["glm-x"], reasonBy["glm-x"])
	}

	if len(unmatched) != 2 || unmatched[0] != "only/on-openrouter" || unmatched[1] != "nobody/knows" {
		t.Errorf("unmatched = %v, want [only/on-openrouter nobody/knows]（openrouter 由直连负责）", unmatched)
	}
}

// 补齐只针对缺价格或缺目录行的模型；别名行跳过。
func TestCatalogMissingModels(t *testing.T) {
	setupModelInfoTestDB(t)
	if err := DB.AutoMigrate(&Price{}); err != nil {
		t.Fatal(err)
	}
	for _, info := range []*ModelInfo{{Model: "full"}, {Model: "no-price"}, {Model: "alias", AliasOf: "full"}} {
		if err := DB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}
	oldPricing := PricingInstance
	PricingInstance = &Pricing{Prices: map[string]*Price{
		"full":    {Model: "full"},
		"no-info": {Model: "no-info"},
	}}
	t.Cleanup(func() { PricingInstance = oldPricing })

	got := CatalogMissingModels([]string{"full", "no-price", "no-info", "alias", "brand-new"})
	want := []string{"no-price", "no-info", "brand-new"}
	if len(got) != len(want) {
		t.Fatalf("missing = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("missing = %v, want %v", got, want)
		}
	}
}

// 渠道自带列表（OpenRouter）来源优先：models.dev 补齐不覆盖 OpenRouter 写入的元信息与价格。
func TestFilterChannelSourced(t *testing.T) {
	setupModelInfoTestDB(t)
	if err := DB.Create(&ModelInfo{Model: "or/model", Source: ModelInfoSourceOpenRouter}).Error; err != nil {
		t.Fatal(err)
	}
	oldPricing := PricingInstance
	PricingInstance = &Pricing{Prices: map[string]*Price{
		"or/model": {Model: "or/model", ChannelType: config.ChannelTypeOpenRouter},
		"other":    {Model: "other", ChannelType: config.ChannelTypeOpenAI},
	}}
	t.Cleanup(func() { PricingInstance = oldPricing })

	infos, prices := filterChannelSourced(
		[]*ModelInfo{{Model: "or/model"}, {Model: "other"}},
		[]*Price{{Model: "or/model"}, {Model: "other"}},
	)
	if len(infos) != 1 || infos[0].Model != "other" {
		t.Errorf("infos = %+v, want only other", infos)
	}
	if len(prices) != 1 || prices[0].Model != "other" {
		t.Errorf("prices = %+v, want only other", prices)
	}
}

func floatEqM(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-9
}
