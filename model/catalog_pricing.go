package model

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
)

// models.dev api.json 结构：顶层按 provider id 分组 → models → 每个含 cost(USD/1M)。
// 用指针区分「来源没给价」与「明确免费(0)」，前者不写价格。
type catalogCost struct {
	Input     *float64 `json:"input"`
	Output    *float64 `json:"output"`
	CacheRead *float64 `json:"cache_read"`
}

// 同步时换算不了的价格原因（列入同步结果，不静默写 0）。
const (
	UnconvertibleNoPrice           = "no_price"
	UnconvertiblePerRequest        = "per_request"
	UnconvertibleAudioDuration     = "audio_duration"
	UnconvertibleAudioOutputTokens = "audio_output_tokens"
	UnconvertibleAudioTokenPricing = "audio_token_pricing"
)

// UnconvertiblePrice 是同步时来源有该模型、但价格无法换算进价格表的一项。
type UnconvertiblePrice struct {
	Model  string `json:"model"`
	Reason string `json:"reason"`
}

// CatalogSyncResult 是 models.dev 同步结果。
type CatalogSyncResult struct {
	Synced        int                  `json:"synced"`
	Unconvertible []UnconvertiblePrice `json:"unconvertible"`
	Unmatched     []string             `json:"unmatched"`
}

func costValue(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

type catalogLimit struct {
	Context int `json:"context"`
	Output  int `json:"output"`
}

type catalogModalities struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

type catalogModel struct {
	Id               string            `json:"id"`
	Name             string            `json:"name"`
	Cost             catalogCost       `json:"cost"`
	Limit            catalogLimit      `json:"limit"`
	Modalities       catalogModalities `json:"modalities"`
	ToolCall         bool              `json:"tool_call"`
	Reasoning        bool              `json:"reasoning"`
	StructuredOutput bool              `json:"structured_output"`
}

type catalogProvider struct {
	Models map[string]catalogModel `json:"models"`
}

// catalogProviderToChannelType 把 models.dev 的 provider id 映射到我们的 ChannelType。
// openrouter 故意不在此——它由直连 /v1/models 更新（更实时），目录里跳过避免覆盖。
var catalogProviderToChannelType = map[string]int{
	"openai":        config.ChannelTypeOpenAI,
	"azure":         config.ChannelTypeAzure,
	"anthropic":     config.ChannelTypeAnthropic,
	"alibaba":       config.ChannelTypeAli,
	"google":        config.ChannelTypeGemini,
	"google-vertex": config.ChannelTypeVertexAI,
	"deepseek":      config.ChannelTypeDeepseek,
	"moonshotai":    config.ChannelTypeMoonshot,
	"mistral":       config.ChannelTypeMistral,
	"groq":          config.ChannelTypeGroq,
	"cohere":        config.ChannelTypeCohere,
}

// isAudioEndpoints 判断推导出的接口能力是否只有音频专用接口（TTS / STT）。
func isAudioEndpoints(endpointsJSON string) bool {
	return endpointsJSON == ModelEndpointsJSON([]string{ModelEndpointAudioSpeech}) ||
		endpointsJSON == ModelEndpointsJSON([]string{ModelEndpointAudioTranscription})
}

// catalogPrice 把一条 models.dev 模型换算为价格；换算不了时返回原因。
// models.dev 的音频模型按 token 计价，而网关 TTS 按输入字符、STT 按转写文本计费，口径对不上。
func catalogPrice(modelName string, channelType int, m catalogModel) (*Price, string) {
	if isAudioEndpoints(ModelEndpointsFromModalities(modelName, m.Modalities.Input, m.Modalities.Output)) {
		return nil, UnconvertibleAudioTokenPricing
	}
	if m.Cost.Input == nil {
		return nil, UnconvertibleNoPrice
	}
	input := costValue(m.Cost.Input)
	cacheRead := costValue(m.Cost.CacheRead)
	cacheReadRatio := 0.0
	if input > 0 && cacheRead > 0 {
		cacheReadRatio = cacheRead / input
	}
	return newRatioPrice(modelName, channelType,
		RatioFromUSDPerMillion(input),
		RatioFromUSDPerMillion(costValue(m.Cost.Output)),
		cacheReadRatio), ""
}

// BuildCatalogPrices 纯函数：把 models.dev 数据转为 []*Price。
// 仅处理已映射的 provider；未映射(含 openrouter) 一律跳过；无价或音频模型不写。
func BuildCatalogPrices(data map[string]catalogProvider) []*Price {
	var prices []*Price
	for providerID, prov := range data {
		channelType, ok := catalogProviderToChannelType[providerID]
		if !ok {
			continue
		}
		for modelID, m := range prov.Models {
			id := m.Id
			if id == "" {
				id = modelID
			}
			if price, _ := catalogPrice(id, channelType, m); price != nil {
				prices = append(prices, price)
			}
		}
	}
	return prices
}

// catalogModalitiesToJSON 把模态数组序列化为 JSON 字符串（与 ModelInfo 存储一致）；空则空串。
func catalogModalitiesToJSON(m []string) string {
	if len(m) == 0 {
		return ""
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}

// catalogCapabilities 把 models.dev 的三个布尔字段映射为能力数组（字段缺失按不支持）。
func catalogCapabilities(m catalogModel) string {
	caps := make([]string, 0, 3)
	if m.ToolCall {
		caps = append(caps, ModelCapabilityToolCall)
	}
	if m.Reasoning {
		caps = append(caps, ModelCapabilityReasoning)
	}
	if m.StructuredOutput {
		caps = append(caps, ModelCapabilityStructuredOutput)
	}
	return ModelCapabilitiesJSON(caps)
}

// catalogProviderToVendorSlug 把 models.dev 的 provider id 映射到厂商 slug。
// azure / google-vertex 只是承载渠道，模型厂商仍是 openai / google。
var catalogProviderToVendorSlug = map[string]string{
	"openai":        "openai",
	"azure":         "openai",
	"anthropic":     "anthropic",
	"alibaba":       "qwen",
	"google":        "google",
	"google-vertex": "google",
	"deepseek":      "deepseek",
	"moonshotai":    "moonshotai",
	"mistral":       "mistralai",
	"groq":          "groq",
	"cohere":        "cohere",
}

// catalogModelInfo 把一条 models.dev 模型转为以 modelName 为键的目录元信息。
func catalogModelInfo(modelName string, vendorID int, m catalogModel) *ModelInfo {
	return &ModelInfo{
		Model:            modelName,
		Name:             m.Name,
		ContextLength:    m.Limit.Context,
		MaxTokens:        m.Limit.Output,
		InputModalities:  catalogModalitiesToJSON(m.Modalities.Input),
		OutputModalities: catalogModalitiesToJSON(m.Modalities.Output),
		Capabilities:     catalogCapabilities(m),
		VendorID:         vendorID,
		Endpoints:        ModelEndpointsFromModalities(modelName, m.Modalities.Input, m.Modalities.Output),
	}
}

// BuildCatalogModelInfos 纯函数：把 models.dev 数据转为 []*ModelInfo（仅已映射的 provider）。
// 厂商按 provider 映射到厂商 slug 再查 model_owned_by；接口能力按模态推导。
func BuildCatalogModelInfos(data map[string]catalogProvider) []*ModelInfo {
	var infos []*ModelInfo
	for providerID, prov := range data {
		if _, ok := catalogProviderToChannelType[providerID]; !ok {
			continue
		}
		vendorID := ModelOwnedBysInstance.GetIdBySlug(catalogProviderToVendorSlug[providerID])
		for modelID, m := range prov.Models {
			id := m.Id
			if id == "" {
				id = modelID
			}
			infos = append(infos, catalogModelInfo(id, vendorID, m))
		}
	}
	return infos
}

// catalogEntry 是 models.dev 全量索引里的一条（某 provider 下的某模型）。
type catalogEntry struct {
	Provider string
	ID       string
	Model    catalogModel
}

// catalogIndex 是 models.dev 全量（所有 provider）按模型 id 建的索引：
// exact 以完整 id（小写）为键，bare 以去掉 "厂商/" 前缀后的 id（小写）为键。
type catalogIndex struct {
	exact map[string]catalogEntry
	bare  map[string]catalogEntry
}

// bareModelID 去掉 "厂商/" 前缀（取最后一个斜杠之后的部分）。
func bareModelID(id string) string {
	if idx := strings.LastIndex(id, "/"); idx >= 0 {
		return id[idx+1:]
	}
	return id
}

// preferCatalogEntry 同名冲突消解：第一方官方源优先，其次已映射 provider，再次带价的一份，
// 仍打平取 provider 名较小者（遍历已排序，保留先到者即可）。
func preferCatalogEntry(current, next catalogEntry) bool {
	currentFP := isFirstPartyModelsDevProvider(current.Provider)
	nextFP := isFirstPartyModelsDevProvider(next.Provider)
	if currentFP != nextFP {
		return nextFP
	}
	_, currentMapped := catalogProviderToChannelType[current.Provider]
	_, nextMapped := catalogProviderToChannelType[next.Provider]
	if currentMapped != nextMapped {
		return nextMapped
	}
	currentPriced := current.Model.Cost.Input != nil
	nextPriced := next.Model.Cost.Input != nil
	if currentPriced != nextPriced {
		return nextPriced
	}
	return false
}

// buildCatalogIndex 纯函数：把 models.dev 全量数据建成索引（不限映射 provider，openrouter 除外——
// 它由直连 /v1/models 负责）。遍历按 provider / 模型名排序，结果确定。
func buildCatalogIndex(data map[string]catalogProvider) *catalogIndex {
	idx := &catalogIndex{exact: map[string]catalogEntry{}, bare: map[string]catalogEntry{}}
	providers := make([]string, 0, len(data))
	for p := range data {
		if p == "openrouter" {
			continue
		}
		providers = append(providers, p)
	}
	sort.Strings(providers)
	put := func(m map[string]catalogEntry, key string, e catalogEntry) {
		if cur, ok := m[key]; !ok || preferCatalogEntry(cur, e) {
			m[key] = e
		}
	}
	for _, p := range providers {
		models := data[p].Models
		names := make([]string, 0, len(models))
		for name := range models {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			m := models[name]
			id := m.Id
			if id == "" {
				id = name
			}
			e := catalogEntry{Provider: p, ID: id, Model: m}
			put(idx.exact, strings.ToLower(id), e)
			put(idx.bare, strings.ToLower(bareModelID(id)), e)
		}
	}
	return idx
}

// match 按「完整 id → 去前缀 id 精确 → 去前缀 id 对齐去前缀索引」的顺序匹配渠道模型名，
// 使 OpenRouter 风格的 "openai/gpt-4o-mini-tts" 也能对上 models.dev 的 "gpt-4o-mini-tts"。
func (idx *catalogIndex) match(name string) (catalogEntry, bool) {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return catalogEntry{}, false
	}
	if e, ok := idx.exact[lower]; ok {
		return e, true
	}
	bare := bareModelID(lower)
	if e, ok := idx.exact[bare]; ok {
		return e, true
	}
	if e, ok := idx.bare[bare]; ok {
		return e, true
	}
	return catalogEntry{}, false
}

// catalogEntryVendorID 渠道模型名自带厂商前缀优先，其次 provider 映射 / provider id 本身，再次条目 id 的前缀。
func catalogEntryVendorID(modelName string, e catalogEntry) int {
	if id := VendorIDFromModelName(modelName); id != 0 {
		return id
	}
	slug := catalogProviderToVendorSlug[e.Provider]
	if slug == "" {
		slug = e.Provider
	}
	if id := ModelOwnedBysInstance.GetIdBySlug(slug); id != 0 {
		return id
	}
	return VendorIDFromModelName(e.ID)
}

// BuildChannelCatalogEntries 纯函数：按渠道模型名在 models.dev 全量里匹配，产出以渠道模型名为键的
// 元信息与价格；价格换算不了的列入 unconvertible，匹配不到的列入 unmatched。
func BuildChannelCatalogEntries(data map[string]catalogProvider, names []string) ([]*ModelInfo, []*Price, []UnconvertiblePrice, []string) {
	idx := buildCatalogIndex(data)
	var infos []*ModelInfo
	var prices []*Price
	var unconvertible []UnconvertiblePrice
	var unmatched []string
	for _, name := range names {
		e, ok := idx.match(name)
		if !ok {
			unmatched = append(unmatched, name)
			continue
		}
		infos = append(infos, catalogModelInfo(name, catalogEntryVendorID(name, e), e.Model))
		price, reason := catalogPrice(name, catalogProviderToChannelType[e.Provider], e.Model)
		if price == nil {
			unconvertible = append(unconvertible, UnconvertiblePrice{Model: name, Reason: reason})
			continue
		}
		prices = append(prices, price)
	}
	return infos, prices, unconvertible, unmatched
}

// EnabledChannelModelNames 返回所有启用渠道的模型名（去空白、去重、排序）。
func EnabledChannelModelNames() ([]string, error) {
	channels, err := GetAllChannels()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, ch := range channels {
		if ch.Status != config.ChannelStatusEnabled {
			continue
		}
		for _, name := range SplitChannelModels(ch.Models) {
			seen[name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// SplitChannelModels 把渠道的逗号分隔模型列表拆成去空白、去重的名字。
func SplitChannelModels(csv string) []string {
	seen := map[string]bool{}
	var names []string
	for _, name := range strings.Split(csv, ",") {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

// priceSnapshot 读取价格表里某模型的精确行（不走通配 / 兜底）。
func (p *Pricing) priceSnapshot(name string) (*Price, bool) {
	if p == nil {
		return nil, false
	}
	p.RLock()
	defer p.RUnlock()
	price, ok := p.Prices[name]
	return price, ok
}

// CatalogMissingModels 返回缺价格或缺目录行的模型名（别名行跳过：别名沿用主名价格与元信息）。
func CatalogMissingModels(names []string) []string {
	var missing []string
	for _, name := range names {
		info := lookupModelInfo(name)
		if info != nil && info.AliasOf != "" {
			continue
		}
		if _, ok := PricingInstance.priceSnapshot(name); ok && info != nil {
			continue
		}
		missing = append(missing, name)
	}
	return missing
}

// filterChannelSourced 去掉已由渠道自带列表（OpenRouter）提供的元信息 / 价格：渠道来源优先于 models.dev。
func filterChannelSourced(infos []*ModelInfo, prices []*Price) ([]*ModelInfo, []*Price) {
	keptInfos := make([]*ModelInfo, 0, len(infos))
	for _, info := range infos {
		if existing := lookupModelInfo(info.Model); existing != nil && existing.Source == ModelInfoSourceOpenRouter {
			continue
		}
		keptInfos = append(keptInfos, info)
	}
	keptPrices := make([]*Price, 0, len(prices))
	for _, price := range prices {
		if existing, ok := PricingInstance.priceSnapshot(price.Model); ok && existing.ChannelType == config.ChannelTypeOpenRouter {
			continue
		}
		keptPrices = append(keptPrices, price)
	}
	return keptInfos, keptPrices
}

// syncCatalogChannelModels 把渠道模型名按 models.dev 全量匹配结果 upsert（不覆盖 locked 与渠道来源）。
func syncCatalogChannelModels(data map[string]catalogProvider, names []string) (*CatalogSyncResult, error) {
	infos, prices, unconvertible, unmatched := BuildChannelCatalogEntries(data, names)
	infos, prices = filterChannelSourced(infos, prices)
	if len(prices) > 0 {
		if err := PricingInstance.UpsertPrices(prices); err != nil {
			return nil, err
		}
	}
	if len(infos) > 0 {
		if err := UpsertModelInfos(infos, ModelInfoSourceModelsDev); err != nil {
			logger.SysError("catalog channel model info upsert failed: " + err.Error())
		}
	}
	return &CatalogSyncResult{Synced: len(prices), Unconvertible: unconvertible, Unmatched: unmatched}, nil
}

// SyncCatalogForModels 拉取 models.dev 并只为给定模型名补齐元信息与价格（渠道保存钩子用）。
func SyncCatalogForModels(url string, names []string) (*CatalogSyncResult, error) {
	if len(names) == 0 {
		return &CatalogSyncResult{}, nil
	}
	data, err := fetchCatalogData(url)
	if err != nil {
		return nil, err
	}
	return syncCatalogChannelModels(data, names)
}

const defaultCatalogURL = "https://models.dev/api.json"

// fetchCatalogData 拉取并解析 models.dev 目录（一次取回，供价格 + 元信息共用）。
func fetchCatalogData(url string) (map[string]catalogProvider, error) {
	if url == "" {
		url = defaultCatalogURL
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch catalog: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read catalog: %w", err)
	}

	data := map[string]catalogProvider{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	return data, nil
}

// FetchCatalogPrices 拉取 models.dev 目录并转为 []*Price。url 为空时用默认源。
func FetchCatalogPrices(url string) ([]*Price, error) {
	data, err := fetchCatalogData(url)
	if err != nil {
		return nil, err
	}
	return BuildCatalogPrices(data), nil
}

// SyncCatalogPricing 拉取 models.dev 目录并 upsert 价格 + 模型元信息（不覆盖 Locked）。
// 无需 gin.Context，可直接用于 cron。
func SyncCatalogPricing(url string) (int, error) {
	result, err := SyncCatalogPricingDetailed(url)
	if err != nil {
		return 0, err
	}
	return result.Synced, nil
}

// SyncCatalogPricingDetailed 同 SyncCatalogPricing，并返回换算不了 / 匹配不到的渠道模型：
// 先按已映射 provider 全量刷新，再按所有启用渠道的模型名在 models.dev 全量里匹配补齐。
func SyncCatalogPricingDetailed(url string) (*CatalogSyncResult, error) {
	data, err := fetchCatalogData(url)
	if err != nil {
		return nil, err
	}
	prices := BuildCatalogPrices(data)
	if err := PricingInstance.UpsertPrices(prices); err != nil {
		return nil, err
	}
	if err := UpsertModelInfos(BuildCatalogModelInfos(data), ModelInfoSourceModelsDev); err != nil {
		logger.SysError("catalog model info upsert failed: " + err.Error())
	}
	result := &CatalogSyncResult{Synced: len(prices)}
	names, err := EnabledChannelModelNames()
	if err != nil {
		logger.SysError("catalog sync: list channel models failed: " + err.Error())
	} else if channelResult, err := syncCatalogChannelModels(data, names); err != nil {
		logger.SysError("catalog sync: channel models upsert failed: " + err.Error())
	} else {
		result.Synced += channelResult.Synced
		result.Unconvertible = channelResult.Unconvertible
		result.Unmatched = channelResult.Unmatched
	}
	logger.SysLog(fmt.Sprintf("Catalog synced from models.dev: %d prices, %d unconvertible, %d unmatched channel models",
		result.Synced, len(result.Unconvertible), len(result.Unmatched)))
	return result, nil
}
