package model

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// ModelPickerHeatDays 是家族热度统计的窗口：近 30 天本站请求数。
const ModelPickerHeatDays = 30

// ModelPickerItem 是渠道模型挑选里的一行：上游模型 id、家族拆解、目录元数据与本站售价。
type ModelPickerItem struct {
	Model string `json:"model"`
	ModelFamily
	// Latest 表示该行是本家族默认勾选的最新版本。
	Latest bool `json:"latest"`
	// OtherChannels 是除当前渠道外，已在 models 里配置了该模型的渠道数。
	OtherChannels    int      `json:"other_channels"`
	VendorID         int      `json:"vendor_id"`
	OpenWeights      *bool    `json:"open_weights"`
	ReleasedAt       int64    `json:"released_at"`
	ExpiresAt        int64    `json:"expires_at"`
	ContextLength    int      `json:"context_length"`
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
	Capabilities     []string `json:"capabilities"`
	// InputPrice / OutputPrice 本站按 token 计费的售价（USD / 1M tokens）；未配置或非 token 计费为 null。
	InputPrice  *float64 `json:"input_price"`
	OutputPrice *float64 `json:"output_price"`
}

// ModelPickerFamily 是一个家族的汇总，按 Rank 从热到冷排列。
type ModelPickerFamily struct {
	Family string `json:"family"`
	// Rank 从 1 开始；Requests 是近 ModelPickerHeatDays 天本站该家族的请求数。
	Rank     int    `json:"rank"`
	Requests int64  `json:"requests"`
	Latest   string `json:"latest"`
	Count    int    `json:"count"`
}

type ModelPickerResult struct {
	Models   []ModelPickerItem   `json:"models"`
	Families []ModelPickerFamily `json:"families"`
}

// familyTiers 是没有本站用量时的家族兜底顺序（按家族首词，数字越小越靠前）：
// 主流闭源家族在前，其后是常用开放权重家族；不在表里的排最后。同一档内新发布的家族在前。
var familyTiers = map[string]int{
	"claude": 0, "gpt": 0, "o": 0, "gemini": 0,
	"deepseek": 1, "qwen": 1, "grok": 1,
	"kimi": 2, "glm": 2, "minimax": 2,
	"mistral": 3, "llama": 3, "gemma": 3, "codestral": 3, "devstral": 3,
}

const familyTierUnknown = 9

func familyTier(family string) int {
	head, _, _ := strings.Cut(family, "-")
	if tier, ok := familyTiers[head]; ok {
		return tier
	}
	return familyTierUnknown
}

// newerModel 判断 a 是否比 b 更适合作为家族的「最新版本」：
// 版本号大者优先；同版本正式版优先于预览版、不带快照的别名优先于带日期的快照、快照日期新者优先；
// 最后比发布时间。
func newerModel(a, b *ModelPickerItem) bool {
	if c := CompareModelVersions(a.Version, b.Version); c != 0 {
		return c > 0
	}
	if a.Preview != b.Preview {
		return !a.Preview
	}
	if (a.Snapshot == "") != (b.Snapshot == "") {
		return a.Snapshot == ""
	}
	if a.Snapshot != b.Snapshot {
		return a.Snapshot > b.Snapshot
	}
	return a.ReleasedAt > b.ReleasedAt
}

func decodeStringArray(raw string) []string {
	var out []string
	if raw == "" || json.Unmarshal([]byte(raw), &out) != nil {
		return []string{}
	}
	return out
}

// tokenPricePerMillion 把倍率换算成 USD / 1M tokens（1 倍率 = $0.002 / 1K tokens）。
func tokenPricePerMillion(ratio float64) *float64 {
	usd := ratio * DollarRate * 1000
	return &usd
}

// FamilyRequestCounts 汇总近 days 天本站各家族的请求数（来自 statistics 日统计表）。
func FamilyRequestCounts(days int) (map[string]int64, error) {
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	var rows []struct {
		ModelName    string
		RequestCount int64
	}
	err := DB.Table("statistics").
		Select("model_name, sum(request_count) as request_count").
		Where("date >= ?", since).
		Group("model_name").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int64)
	for _, row := range rows {
		counts[ParseModelFamily(row.ModelName).Family] += row.RequestCount
	}
	return counts, nil
}

// ModelChannelCounts 统计每个模型被多少个渠道的 models 配置，excludeID 对应的渠道不计（编辑中的渠道自己）。
func ModelChannelCounts(excludeID int) (map[string]int, error) {
	var channels []struct {
		Id     int
		Models string
	}
	if err := DB.Model(&Channel{}).Select("id, models").Where("id <> ?", excludeID).Find(&channels).Error; err != nil {
		return nil, err
	}
	counts := make(map[string]int)
	for _, channel := range channels {
		seen := make(map[string]bool)
		for _, name := range strings.Split(channel.Models, ",") {
			name = strings.TrimSpace(name)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			counts[name]++
		}
	}
	return counts, nil
}

// BuildModelPicker 把渠道上游的模型 id 列表加工成挑选表：拆家族、补目录元数据与售价、
// 标出每个家族的最新版本，并按热度给家族排序。目录按完整 id 查，查不到再按去掉厂商前缀的 id 查。
func BuildModelPicker(models []string, familyRequests map[string]int64, channelCounts map[string]int) (*ModelPickerResult, error) {
	names := make([]string, 0, len(models)*2)
	for _, name := range models {
		names = append(names, name, bareModelID(name))
	}
	var infos []*ModelInfo
	if len(names) > 0 {
		if err := DB.Where("model IN ?", names).Find(&infos).Error; err != nil {
			return nil, err
		}
	}
	catalog := make(map[string]*ModelInfo, len(infos))
	for _, info := range infos {
		catalog[info.Model] = info
	}

	items := make([]ModelPickerItem, 0, len(models))
	for _, name := range models {
		item := ModelPickerItem{
			Model:            name,
			ModelFamily:      ParseModelFamily(name),
			OtherChannels:    channelCounts[name],
			InputModalities:  []string{},
			OutputModalities: []string{},
			Capabilities:     []string{},
		}
		info := catalog[name]
		if info == nil {
			info = catalog[bareModelID(name)]
		}
		if info != nil {
			item.VendorID = info.VendorID
			item.OpenWeights = info.OpenWeights
			item.ReleasedAt = info.ReleasedAt
			item.ExpiresAt = info.ExpiresAt
			item.ContextLength = info.ContextLength
			item.InputModalities = decodeStringArray(info.InputModalities)
			item.OutputModalities = decodeStringArray(info.OutputModalities)
			item.Capabilities = decodeStringArray(info.Capabilities)
		}
		if item.VendorID == 0 {
			item.VendorID = VendorIDFromModelName(name)
		}
		if PricingInstance != nil {
			if price := PricingInstance.GetPrice(name); price != nil && !price.Unconfigured && price.Type == TokensPriceType {
				item.InputPrice = tokenPricePerMillion(price.Input)
				item.OutputPrice = tokenPricePerMillion(price.Output)
			}
		}
		items = append(items, item)
	}

	// 每个家族选一个最新版本：只在具体版本里选（变体 :batch / :free、浮动别名、已过下架时间的不算）；
	// 家族里只有这些时退而在全部成员里选。
	now := time.Now().Unix()
	latest := make(map[string]int)
	fallback := make(map[string]int)
	stats := make(map[string]*ModelPickerFamily)
	order := make([]string, 0)
	for i := range items {
		item := &items[i]
		family := item.Family
		if stats[family] == nil {
			stats[family] = &ModelPickerFamily{Family: family, Requests: familyRequests[family]}
			order = append(order, family)
		}
		stats[family].Count++
		if j, ok := fallback[family]; !ok || newerModel(item, &items[j]) {
			fallback[family] = i
		}
		expired := item.ExpiresAt != 0 && item.ExpiresAt <= now
		if item.Variant != "" || item.Alias || expired {
			continue
		}
		if j, ok := latest[family]; !ok || newerModel(item, &items[j]) {
			latest[family] = i
		}
	}
	familyReleased := make(map[string]int64)
	for _, family := range order {
		i, ok := latest[family]
		if !ok {
			i = fallback[family]
		}
		items[i].Latest = true
		stats[family].Latest = items[i].Model
		familyReleased[family] = items[i].ReleasedAt
	}

	// 家族排序：有本站用量的按请求数；其余按兜底档位，同档新发布的在前，最后按名字。
	sort.SliceStable(order, func(a, b int) bool {
		fa, fb := stats[order[a]], stats[order[b]]
		if fa.Requests != fb.Requests {
			return fa.Requests > fb.Requests
		}
		if ta, tb := familyTier(fa.Family), familyTier(fb.Family); ta != tb {
			return ta < tb
		}
		if ra, rb := familyReleased[fa.Family], familyReleased[fb.Family]; ra != rb {
			return ra > rb
		}
		return fa.Family < fb.Family
	})
	families := make([]ModelPickerFamily, 0, len(order))
	for i, family := range order {
		stats[family].Rank = i + 1
		families = append(families, *stats[family])
	}
	return &ModelPickerResult{Models: items, Families: families}, nil
}
