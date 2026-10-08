package model

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/gorm"
)

//go:embed catalog_seed.json
var catalogSeedRaw []byte

// CatalogSeedEntry 是策展白名单里的一条：主名 + 厂商 + 别名（现有上游模型名）+ 接口能力
// + 模态与能力（前端的能力分节按这三项筛选，缺了就整节显示「暂无可用模型」）。
type CatalogSeedEntry struct {
	Canonical        string   `json:"canonical"`
	VendorSlug       string   `json:"vendor_slug"`
	Aliases          []string `json:"aliases"`
	Endpoints        []string `json:"endpoints"`
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
	Capabilities     []string `json:"capabilities"`
	Reason           string   `json:"reason"`
}

// 种子模态词表：与 models.dev 的 modalities 取值一致。
var seedModalityVocabulary = []string{"text", "image", "audio", "video", "file"}

// unknownSeedModalities 返回模态数组里词表外的取值（保持入参顺序、去重）。
func unknownSeedModalities(modalities []string) []string {
	seen := make(map[string]bool, len(modalities))
	unknown := make([]string, 0)
	for _, modality := range modalities {
		if seen[modality] {
			continue
		}
		seen[modality] = true
		known := false
		for _, want := range seedModalityVocabulary {
			if modality == want {
				known = true
				break
			}
		}
		if !known {
			unknown = append(unknown, modality)
		}
	}
	return unknown
}

type CatalogSeed struct {
	Version string             `json:"version"`
	Models  []CatalogSeedEntry `json:"models"`
}

// SeedChannelChange 记录一个渠道上的改动：新加入的主名与新写入的 model_mapping 条目。
type SeedChannelChange struct {
	ChannelId     int               `json:"channel_id"`
	Name          string            `json:"name"`
	AddedModels   []string          `json:"added_models"`
	AddedMappings map[string]string `json:"added_mappings"`
}

// SeedApplyResult 是 apply_seed 的变更摘要；dry_run 与实跑结构一致。
type SeedApplyResult struct {
	DryRun            bool                `json:"dry_run"`
	Version           string              `json:"version"`
	Created           int                 `json:"created"`
	SkippedLocked     int                 `json:"skipped_locked"`
	ChannelsUpdated   int                 `json:"channels_updated"`
	CreatedModels     []string            `json:"created_models"`
	AliasedModels     []string            `json:"aliased_models"`
	SkippedLockedRows []string            `json:"skipped_locked_rows"`
	Channels          []SeedChannelChange `json:"channels"`
	MissingAliases    []string            `json:"missing_aliases"`
	UnknownVendors    []string            `json:"unknown_vendors"`
}

// LoadCatalogSeed 解析内置种子并做基本校验：主名唯一、接口能力属于词表。
func LoadCatalogSeed() (*CatalogSeed, error) {
	return parseCatalogSeed(catalogSeedRaw)
}

// parseCatalogSeed 解析并校验种子内容。接口能力在归一化之前按原始取值校验：
// ModelEndpointsJSON 会静默丢弃词表外的值，放在它之后校验等于什么都不校验。
func parseCatalogSeed(raw []byte) (*CatalogSeed, error) {
	seed := &CatalogSeed{}
	if err := json.Unmarshal(raw, seed); err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(seed.Models))
	for _, entry := range seed.Models {
		if entry.Canonical == "" {
			return nil, errors.New("catalog seed: empty canonical model name")
		}
		if seen[entry.Canonical] {
			return nil, fmt.Errorf("catalog seed: duplicated canonical model %s", entry.Canonical)
		}
		seen[entry.Canonical] = true
		if len(entry.Endpoints) == 0 {
			return nil, fmt.Errorf("catalog seed: %s declares no endpoints", entry.Canonical)
		}
		if unknown := UnknownModelEndpoints(entry.Endpoints); len(unknown) > 0 {
			return nil, fmt.Errorf("catalog seed: unknown endpoints %v for %s", unknown, entry.Canonical)
		}
		if len(entry.InputModalities) == 0 || len(entry.OutputModalities) == 0 {
			return nil, fmt.Errorf("catalog seed: %s declares no input/output modalities", entry.Canonical)
		}
		if unknown := unknownSeedModalities(entry.InputModalities); len(unknown) > 0 {
			return nil, fmt.Errorf("catalog seed: unknown input modalities %v for %s", unknown, entry.Canonical)
		}
		if unknown := unknownSeedModalities(entry.OutputModalities); len(unknown) > 0 {
			return nil, fmt.Errorf("catalog seed: unknown output modalities %v for %s", unknown, entry.Canonical)
		}
		if unknown := unknownSeedCapabilities(entry.Capabilities); len(unknown) > 0 {
			return nil, fmt.Errorf("catalog seed: unknown capabilities %v for %s", unknown, entry.Canonical)
		}
	}
	return seed, nil
}

// unknownSeedCapabilities 返回能力数组里词表外的取值：ModelCapabilitiesJSON 会静默丢弃它们，
// 放在归一化之后校验等于什么都不校验。
func unknownSeedCapabilities(caps []string) []string {
	unknown := make([]string, 0)
	for _, c := range caps {
		switch c {
		case ModelCapabilityToolCall, ModelCapabilityReasoning, ModelCapabilityStructuredOutput:
		default:
			unknown = append(unknown, c)
		}
	}
	return unknown
}

// ApplyCatalogSeed 幂等地把策展白名单应用到目录与承接渠道：
// 主名缺行则新建（元数据继承别名行）；主名补齐厂商 / 接口能力 / 模态 / 能力；别名行写 alias_of=主名；
// 含别名的启用渠道补上主名与 model_mapping[主名]=别名。hidden 只由管理员维护，种子不写，
// 白名单之外的目录行也不动。
// locked=true 的目录行（管理员手工维护）一律完全跳过，只计入 skipped_locked。
// dryRun=true 时只计算摘要、不落库。
// 实跑是全或无：目录行与渠道改写在同一个事务里，中途失败一律回滚，
// 内存索引与渠道分组的刷新放在提交之后，避免半应用状态被缓存住。
func ApplyCatalogSeed(dryRun bool) (*SeedApplyResult, error) {
	if dryRun {
		result, _, _, err := applyCatalogSeedTx(DB, true)
		return result, err
	}

	var (
		result          *SeedApplyResult
		modelsChanged   bool
		channelsChanged bool
	)
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		result, modelsChanged, channelsChanged, err = applyCatalogSeedTx(tx, false)
		return err
	})
	if err != nil {
		return nil, err
	}

	if modelsChanged {
		refreshCatalogCaches()
	}
	if channelsChanged {
		ChannelGroup.Load()
	}
	return result, nil
}

// applyCatalogSeedTx 在给定事务（dry_run 时为只读的 DB 句柄）上计算并落地白名单，
// 返回摘要与「目录 / 渠道是否有改动」，由调用方在提交后刷新对应缓存。
func applyCatalogSeedTx(tx *gorm.DB, dryRun bool) (*SeedApplyResult, bool, bool, error) {
	seed, err := LoadCatalogSeed()
	if err != nil {
		return nil, false, false, err
	}
	var infos []*ModelInfo
	if err := tx.Model(&ModelInfo{}).Order("id desc").Find(&infos).Error; err != nil {
		return nil, false, false, err
	}
	byModel := make(map[string]*ModelInfo, len(infos))
	for _, info := range infos {
		byModel[info.Model] = info
	}

	result := &SeedApplyResult{
		DryRun:            dryRun,
		Version:           seed.Version,
		CreatedModels:     []string{},
		AliasedModels:     []string{},
		SkippedLockedRows: []string{},
		Channels:          []SeedChannelChange{},
		MissingAliases:    []string{},
		UnknownVendors:    []string{},
	}
	// aliasByCanonical 保存每条主名在目录里实际存在的别名，供渠道步骤复用。
	aliasByCanonical := make(map[string][]string, len(seed.Models))
	unknownVendors := make(map[string]bool)
	// skippedLocked 去重记录被跳过的锁定行。
	skippedLocked := make(map[string]bool)
	modelsChanged := false

	for _, entry := range seed.Models {
		vendorID := ModelOwnedBysInstance.GetIdBySlug(entry.VendorSlug)
		if vendorID == 0 && entry.VendorSlug != "" {
			unknownVendors[entry.VendorSlug] = true
		}
		endpointsJSON := ModelEndpointsJSON(entry.Endpoints)

		aliases := make([]string, 0, len(entry.Aliases))
		for _, alias := range entry.Aliases {
			if alias == "" || alias == entry.Canonical {
				continue
			}
			if byModel[alias] == nil {
				result.MissingAliases = append(result.MissingAliases, alias)
				continue
			}
			aliases = append(aliases, alias)
		}
		aliasByCanonical[entry.Canonical] = aliases

		row := byModel[entry.Canonical]
		if row != nil && row.Locked {
			skippedLocked[entry.Canonical] = true
		} else if row == nil {
			created := newSeedCanonicalRow(entry, vendorID, endpointsJSON, aliases, byModel)
			result.CreatedModels = append(result.CreatedModels, entry.Canonical)
			if !dryRun {
				if err := tx.Create(created).Error; err != nil {
					return nil, false, false, err
				}
				modelsChanged = true
			}
			byModel[entry.Canonical] = created
		} else {
			updates := map[string]any{}
			if row.AliasOf != "" {
				updates["alias_of"] = ""
			}
			if vendorID != 0 && row.VendorID != vendorID {
				updates["vendor_id"] = vendorID
			}
			if endpointsJSON != "" && row.Endpoints != endpointsJSON {
				updates["endpoints"] = endpointsJSON
			}
			// 模态与能力以白名单为准：同步来源与旧行常常是空的，留着空值等于前端能力面没数据。
			if inputJSON := catalogModalitiesToJSON(entry.InputModalities); inputJSON != "" && row.InputModalities != inputJSON {
				updates["input_modalities"] = inputJSON
			}
			if outputJSON := catalogModalitiesToJSON(entry.OutputModalities); outputJSON != "" && row.OutputModalities != outputJSON {
				updates["output_modalities"] = outputJSON
			}
			if capsJSON := ModelCapabilitiesJSON(entry.Capabilities); row.Capabilities != capsJSON {
				updates["capabilities"] = capsJSON
			}
			if len(updates) > 0 && !dryRun {
				if err := tx.Model(&ModelInfo{}).Where("id = ?", row.Id).Updates(updates).Error; err != nil {
					return nil, false, false, err
				}
				modelsChanged = true
			}
		}

		for _, alias := range aliases {
			aliasRow := byModel[alias]
			if aliasRow.Locked {
				skippedLocked[alias] = true
				continue
			}
			if aliasRow.AliasOf == entry.Canonical {
				continue
			}
			result.AliasedModels = append(result.AliasedModels, alias)
			if !dryRun {
				if err := tx.Model(&ModelInfo{}).Where("id = ?", aliasRow.Id).Update("alias_of", entry.Canonical).Error; err != nil {
					return nil, false, false, err
				}
				modelsChanged = true
			}
		}
	}

	channelsChanged, err := applySeedToChannels(tx, seed, aliasByCanonical, dryRun, result)
	if err != nil {
		return nil, false, false, err
	}

	for slug := range unknownVendors {
		result.UnknownVendors = append(result.UnknownVendors, slug)
	}
	for name := range skippedLocked {
		result.SkippedLockedRows = append(result.SkippedLockedRows, name)
	}
	sort.Strings(result.UnknownVendors)
	sort.Strings(result.MissingAliases)
	sort.Strings(result.SkippedLockedRows)
	result.Created = len(result.CreatedModels)
	result.SkippedLocked = len(result.SkippedLockedRows)
	result.ChannelsUpdated = len(result.Channels)
	return result, modelsChanged, channelsChanged, nil
}

// newSeedCanonicalRow 为缺失的主名建行：元数据继承第一个存在的别名行，
// 模态与能力由白名单决定（别名行的这些字段通常是空的）。
func newSeedCanonicalRow(entry CatalogSeedEntry, vendorID int, endpointsJSON string, aliases []string, byModel map[string]*ModelInfo) *ModelInfo {
	row := &ModelInfo{
		Model:     entry.Canonical,
		Name:      entry.Canonical,
		VendorID:  vendorID,
		Endpoints: endpointsJSON,
		Source:    ModelInfoSourceManual,
	}
	for _, alias := range aliases {
		src := byModel[alias]
		if src == nil {
			continue
		}
		if src.Name != "" {
			row.Name = src.Name
		}
		row.Description = src.Description
		row.ContextLength = src.ContextLength
		row.MaxTokens = src.MaxTokens
		row.InputModalities = src.InputModalities
		row.OutputModalities = src.OutputModalities
		row.Tags = src.Tags
		row.SupportUrl = src.SupportUrl
		row.Capabilities = src.Capabilities
		if row.VendorID == 0 {
			row.VendorID = src.VendorID
		}
		if row.Endpoints == "" {
			row.Endpoints = src.Endpoints
		}
		break
	}
	if inputJSON := catalogModalitiesToJSON(entry.InputModalities); inputJSON != "" {
		row.InputModalities = inputJSON
	}
	if outputJSON := catalogModalitiesToJSON(entry.OutputModalities); outputJSON != "" {
		row.OutputModalities = outputJSON
	}
	row.Capabilities = ModelCapabilitiesJSON(entry.Capabilities)
	return row
}

// applySeedToChannels 把主名补进每个「配置了对应别名」的启用渠道，并写入 model_mapping[主名]=别名；
// 已有的 models 条目与 mapping 键一律跳过，保证重复执行零变更。
// 返回是否有渠道被改写，渠道分组的重载由调用方在事务提交后执行。
func applySeedToChannels(tx *gorm.DB, seed *CatalogSeed, aliasByCanonical map[string][]string, dryRun bool, result *SeedApplyResult) (bool, error) {
	var channels []*Channel
	if err := tx.Model(&Channel{}).Order("id desc").Find(&channels).Error; err != nil {
		return false, err
	}
	changed := false
	for _, channel := range channels {
		if channel.Status != config.ChannelStatusEnabled {
			continue
		}
		models := splitChannelModels(channel.Models)
		has := make(map[string]bool, len(models))
		for _, name := range models {
			has[name] = true
		}
		mapping, err := parseChannelModelMapping(channel.GetModelMapping())
		if err != nil {
			return false, fmt.Errorf("channel %d: invalid model_mapping: %w", channel.Id, err)
		}

		change := SeedChannelChange{
			ChannelId:     channel.Id,
			Name:          channel.Name,
			AddedModels:   []string{},
			AddedMappings: map[string]string{},
		}
		for _, entry := range seed.Models {
			alias := firstPresent(aliasByCanonical[entry.Canonical], has)
			if alias == "" {
				continue
			}
			if !has[entry.Canonical] {
				models = append(models, entry.Canonical)
				has[entry.Canonical] = true
				change.AddedModels = append(change.AddedModels, entry.Canonical)
			}
			if _, ok := mapping[entry.Canonical]; !ok {
				mapping[entry.Canonical] = alias
				change.AddedMappings[entry.Canonical] = alias
			}
		}
		if len(change.AddedModels) == 0 && len(change.AddedMappings) == 0 {
			continue
		}
		result.Channels = append(result.Channels, change)
		if dryRun {
			continue
		}
		encoded, err := json.Marshal(mapping)
		if err != nil {
			return false, err
		}
		updates := map[string]any{
			"models":        strings.Join(models, ","),
			"model_mapping": string(encoded),
		}
		if err := tx.Model(&Channel{}).Where("id = ?", channel.Id).Updates(updates).Error; err != nil {
			return false, err
		}
		changed = true
	}
	return changed, nil
}

func splitChannelModels(raw string) []string {
	models := make([]string, 0)
	for _, name := range strings.Split(raw, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			models = append(models, name)
		}
	}
	return models
}

// parseChannelModelMapping 解析渠道的 model_mapping：空串视为空映射。
func parseChannelModelMapping(raw string) (map[string]string, error) {
	mapping := map[string]string{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return mapping, nil
	}
	if err := json.Unmarshal([]byte(raw), &mapping); err != nil {
		return nil, err
	}
	return mapping, nil
}

// firstPresent 返回第一个出现在渠道 models 里的别名，没有则返回空串。
func firstPresent(aliases []string, has map[string]bool) string {
	for _, alias := range aliases {
		if has[alias] {
			return alias
		}
	}
	return ""
}
