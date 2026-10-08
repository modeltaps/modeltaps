package model

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/gorm"
)

// 模型元信息来源：空值表示历史数据未知，下一次同步自然补齐。
const (
	ModelInfoSourceManual     = "manual"
	ModelInfoSourceModelsDev  = "models.dev"
	ModelInfoSourceOpenRouter = "openrouter"
)

// 模型能力词表：仅用于展示与筛选，不参与路由与计费。
const (
	ModelCapabilityToolCall         = "tool_call"
	ModelCapabilityReasoning        = "reasoning"
	ModelCapabilityStructuredOutput = "structured_output"
)

// modelCapabilityOrder 决定序列化顺序，便于比对与测试。
var modelCapabilityOrder = []string{
	ModelCapabilityToolCall,
	ModelCapabilityReasoning,
	ModelCapabilityStructuredOutput,
}

// ModelCapabilitiesJSON 按词表固定顺序把能力集合序列化为 JSON 数组字符串；
// 词表外的值丢弃，一项都没有时返回 "[]"（明确表示三项都不支持，区别于空串的未知）。
func ModelCapabilitiesJSON(caps []string) string {
	has := make(map[string]bool, len(caps))
	for _, c := range caps {
		has[c] = true
	}
	ordered := make([]string, 0, len(modelCapabilityOrder))
	for _, c := range modelCapabilityOrder {
		if has[c] {
			ordered = append(ordered, c)
		}
	}
	b, err := json.Marshal(ordered)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// IsValidModelCapabilities 校验后台写入的能力字段：空串（未知）视为合法，
// 其余必须是 JSON 数组且元素全部属于词表。
func IsValidModelCapabilities(raw string) bool {
	if raw == "" {
		return true
	}
	var caps []string
	if err := json.Unmarshal([]byte(raw), &caps); err != nil {
		return false
	}
	for _, c := range caps {
		switch c {
		case ModelCapabilityToolCall, ModelCapabilityReasoning, ModelCapabilityStructuredOutput:
		default:
			return false
		}
	}
	return true
}

type ModelInfo struct {
	Id               int    `json:"id" gorm:"index"`
	Model            string `json:"model" gorm:"type:varchar(100);index"`
	Name             string `json:"name" gorm:"type:varchar(100)"`
	Description      string `json:"description" gorm:"type:text"`
	ContextLength    int    `json:"context_length"`
	MaxTokens        int    `json:"max_tokens"`
	InputModalities  string `json:"input_modalities" gorm:"type:text"`
	OutputModalities string `json:"output_modalities" gorm:"type:text"`
	Tags             string `json:"tags" gorm:"type:text"`
	SupportUrl       string `json:"support_url" gorm:"type:text"`
	// Capabilities 能力数组（JSON 字符串）：空串 = 未知，"[]" = 三项均不支持。
	Capabilities string `json:"capabilities" gorm:"type:text"`
	// Mode 任务类型（chat / image / chat_image，空 = 未设置）：只由管理员维护，同步不写；
	// 为空时由 Endpoints 推导（见 EffectiveMode）。
	Mode string `json:"mode" gorm:"type:varchar(32);default:''"`
	// VendorID 厂商（model_owned_by.id），0 = 未知。厂商由元数据决定，不再等同于渠道类型。
	VendorID int `json:"vendor_id" gorm:"index;default:0"`
	// Hidden 管理员隐藏：默认 false（有启用渠道可路由即对外可见），只由管理员维护，同步不写。
	Hidden bool `json:"hidden" gorm:"default:false"`
	// Endpoints 接口能力数组（JSON 字符串）：空串 = 未设置。同步只在为空时填入，不覆盖已有值。
	Endpoints string `json:"endpoints" gorm:"type:text"`
	// SupportedVoices 文字转语音音色 id 数组（JSON 字符串）：空串 = 未同步。只由同步维护。
	SupportedVoices string `json:"supported_voices" gorm:"type:text"`
	// AliasOf 非空表示本行是别名，值为主名模型标识；隐藏状态与厂商随主名。
	AliasOf   string `json:"alias_of" gorm:"type:varchar(255);index"`
	Source    string `json:"source" gorm:"type:varchar(32);default:''"`
	SyncedAt  int64  `json:"synced_at"`
	Locked    bool   `json:"locked" gorm:"default:false"` // 锁定后同步不再更新该行
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

type ModelInfoResponse struct {
	Model            string   `json:"model"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	ContextLength    int      `json:"context_length"`
	MaxTokens        int      `json:"max_tokens"`
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
	Tags             []string `json:"tags"`
	SupportUrl       []string `json:"support_url"`
	Capabilities     []string `json:"capabilities"`
	Endpoints        []string `json:"endpoints"`
	Mode             string   `json:"mode"`
	VendorID         int      `json:"vendor_id"`
	Hidden           bool     `json:"hidden"`
	AliasOf          string   `json:"alias_of"`
	CreatedAt        int64    `json:"created_at"`
	UpdatedAt        int64    `json:"updated_at"`
}

// EffectiveMode 返回生效的任务类型：管理员显式设置的 mode 优先，
// 为空时由 endpoints（配合输出模态）推导，仍推不出则为空串。
func (m *ModelInfo) EffectiveMode() string {
	if m.Mode != "" {
		return m.Mode
	}
	return ModelModeFromEndpoints(m.Endpoints, m.OutputModalities)
}

func (m *ModelInfo) ToResponse() *ModelInfoResponse {
	res := &ModelInfoResponse{
		Model:         m.Model,
		Name:          m.Name,
		Description:   m.Description,
		ContextLength: m.ContextLength,
		MaxTokens:     m.MaxTokens,
		Mode:          m.EffectiveMode(),
		VendorID:      m.VendorID,
		Hidden:        m.Hidden,
		AliasOf:       m.AliasOf,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}

	res.Endpoints, _ = utils.UnmarshalString[[]string](m.Endpoints)
	if res.Endpoints == nil {
		res.Endpoints = []string{}
	}

	res.InputModalities, _ = utils.UnmarshalString[[]string](m.InputModalities)
	res.OutputModalities, _ = utils.UnmarshalString[[]string](m.OutputModalities)
	res.Tags, _ = utils.UnmarshalString[[]string](m.Tags)

	res.Capabilities, _ = utils.UnmarshalString[[]string](m.Capabilities)
	if res.Capabilities == nil {
		res.Capabilities = []string{}
	}

	var err error
	res.SupportUrl, err = utils.UnmarshalString[[]string](m.SupportUrl)
	if err != nil {
		if m.SupportUrl != "" {
			res.SupportUrl = []string{m.SupportUrl}
		} else {
			res.SupportUrl = []string{}
		}
	}

	return res
}

func (m *ModelInfo) TableName() string {
	return "model_info"
}

func CreateModelInfo(modelInfo *ModelInfo) error {
	err := DB.Create(modelInfo).Error
	if err != nil {
		return err
	}
	refreshCatalogCaches()
	return nil
}

// UpdateModelInfo 用于后台手工编辑：synced_at / supported_voices 由同步流程维护，这里不覆盖。
func UpdateModelInfo(modelInfo *ModelInfo) error {
	err := DB.Omit("id", "created_at", "synced_at", "supported_voices").Save(modelInfo).Error
	if err != nil {
		return err
	}
	refreshCatalogCaches()
	return nil
}

// UpsertModelInfos 按 model 名 upsert 元信息：已存在则更新元数据字段、不存在则插入。
// 供价格同步（OpenRouter / models.dev）顺带写入模型元信息，使模型详情有上下文/描述/模态。
// 已锁定（locked）的行整行跳过；同步值为空时不覆盖库里已有的非空值。
// hidden / alias_of 属管理员维护字段，同步既不覆盖也不在新行上写入非零值；
// endpoints 只在库里为空时填入，已有值（含管理员改过的）保持不变。
// source 由调用方显式给出并覆盖 info 上的取值，来源缺失视为调用错误。
func UpsertModelInfos(infos []*ModelInfo, source string) error {
	if source == "" {
		return errors.New("model info source is required")
	}
	now := time.Now().Unix()
	for _, info := range infos {
		if info == nil || info.Model == "" {
			continue
		}
		info.Source = source
		var existing ModelInfo
		err := DB.Where("model = ?", info.Model).First(&existing).Error
		if err == nil {
			if existing.Locked {
				continue
			}
			updates := map[string]any{"synced_at": now, "source": source}
			putStr := func(column, next, prev string) {
				if next != "" || prev == "" {
					updates[column] = next
				}
			}
			putInt := func(column string, next, prev int) {
				if next != 0 || prev == 0 {
					updates[column] = next
				}
			}
			putStr("name", info.Name, existing.Name)
			putStr("description", info.Description, existing.Description)
			putStr("input_modalities", info.InputModalities, existing.InputModalities)
			putStr("output_modalities", info.OutputModalities, existing.OutputModalities)
			putStr("capabilities", info.Capabilities, existing.Capabilities)
			putStr("supported_voices", info.SupportedVoices, existing.SupportedVoices)
			putInt("context_length", info.ContextLength, existing.ContextLength)
			putInt("max_tokens", info.MaxTokens, existing.MaxTokens)
			putInt("vendor_id", info.VendorID, existing.VendorID)
			// endpoints 只在库里为空时填入：已有值可能是管理员改过的，同步不得覆盖。
			// hidden / alias_of 只由管理员维护，同步一概不写。
			if existing.Endpoints == "" && info.Endpoints != "" {
				updates["endpoints"] = info.Endpoints
			}
			if e := DB.Model(&ModelInfo{}).Where("id = ?", existing.Id).Updates(updates).Error; e != nil {
				return e
			}
			continue
		}
		info.SyncedAt = now
		// mode 只由管理员维护，同步来源即便带了值也不落库。
		info.Mode = ""
		// 新行一律不隐藏、非别名：隐藏与别名只由管理员决定。
		info.Hidden = false
		info.AliasOf = ""
		if e := DB.Create(info).Error; e != nil {
			return e
		}
	}
	refreshCatalogCaches()
	return nil
}

func GetModelInfo(id int) (*ModelInfo, error) {
	modelInfo := &ModelInfo{}
	err := DB.Where("id = ?", id).First(modelInfo).Error
	if err != nil {
		return nil, err
	}
	return modelInfo, nil
}

func GetModelInfoByModel(model string) (*ModelInfo, error) {
	modelInfo := &ModelInfo{}
	err := DB.Where("model = ?", model).First(modelInfo).Error
	if err != nil {
		return nil, err
	}
	return modelInfo, nil
}

func GetAllModelInfo() ([]*ModelInfo, error) {
	var modelInfos []*ModelInfo
	err := DB.Order("id desc").Find(&modelInfos).Error
	if err != nil {
		return nil, err
	}
	return modelInfos, nil
}

// SetModelInfoHidden 批量切换隐藏状态；ids 为空时什么都不做。
func SetModelInfoHidden(ids []int, hidden bool) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := DB.Model(&ModelInfo{}).Where("id IN ?", ids).Update("hidden", hidden)
	if result.Error != nil {
		return 0, result.Error
	}
	refreshCatalogCaches()
	return result.RowsAffected, nil
}

// 别名写入的校验错误：别名只允许单跳，且必须指向一条真实存在的主名行。
var (
	ErrAliasSelfReference = errors.New("alias_of must differ from model")
	ErrAliasTargetMissing = errors.New("alias_of target does not exist")
	ErrAliasTargetIsAlias = errors.New("alias_of target is itself an alias")
)

// ValidateAliasTarget 校验一条别名行的 alias_of：不能指向自己、目标必须存在、目标本身不能是别名。
// 三条合起来保证别名图只有一跳、不成环，modelAliasMaxDepth 退化为纯防御。
// 判据直接查库而非内存索引：索引可能滞后，别名写入必须以库为准。
func ValidateAliasTarget(modelName, aliasOf string) error {
	if aliasOf == "" {
		return nil
	}
	if aliasOf == modelName {
		return ErrAliasSelfReference
	}
	target, err := GetModelInfoByModel(aliasOf)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAliasTargetMissing
		}
		return err
	}
	if target.AliasOf != "" {
		return ErrAliasTargetIsAlias
	}
	return nil
}

// ModelAliasReferences 列出把 modelName 当作主名的别名标识，用于在改名 / 删除主名时拒绝并说明原因。
func ModelAliasReferences(modelName string) ([]string, error) {
	if modelName == "" {
		return nil, nil
	}
	var names []string
	if err := DB.Model(&ModelInfo{}).Where("alias_of = ?", modelName).
		Order("model asc").Pluck("model", &names).Error; err != nil {
		return nil, err
	}
	return names, nil
}

func DeleteModelInfo(id int) error {
	err := DB.Delete(&ModelInfo{}, id).Error
	if err != nil {
		return err
	}
	refreshCatalogCaches()
	return nil
}

// modelInfoCache 是 model_info 的内存索引：目录解析（别名、隐藏、厂商）在列表接口里
// 会按模型逐个查询，必须避免每次都打 DB。未加载时所有查询直接回落 DB（如单测）。
var modelInfoCache = struct {
	sync.RWMutex
	loaded  bool
	byModel map[string]*ModelInfo
	// aliasesByModel 主名到别名列表的反查索引（已排序），随 byModel 一起重建。
	aliasesByModel map[string][]string
}{}

// ReloadModelInfoCache 全量重建 model_info 内存索引。
func ReloadModelInfoCache() error {
	infos, err := GetAllModelInfo()
	if err != nil {
		return err
	}
	byModel := make(map[string]*ModelInfo, len(infos))
	for _, info := range infos {
		byModel[info.Model] = info
	}

	aliasesByModel := make(map[string][]string)
	for _, info := range infos {
		if info.AliasOf == "" {
			continue
		}
		canonical := resolveCanonicalIn(byModel, info.Model)
		if canonical == info.Model {
			continue
		}
		aliasesByModel[canonical] = append(aliasesByModel[canonical], info.Model)
	}
	for canonical := range aliasesByModel {
		sort.Strings(aliasesByModel[canonical])
	}

	modelInfoCache.Lock()
	defer modelInfoCache.Unlock()
	modelInfoCache.byModel = byModel
	modelInfoCache.aliasesByModel = aliasesByModel
	modelInfoCache.loaded = true
	return nil
}

// resolveCanonicalIn 在给定索引内顺着 alias_of 解析主名，供 ReloadModelInfoCache 在持锁前使用。
func resolveCanonicalIn(byModel map[string]*ModelInfo, name string) string {
	canonical := name
	for i := 0; i < modelAliasMaxDepth; i++ {
		info := byModel[canonical]
		if info == nil || info.AliasOf == "" || info.AliasOf == canonical {
			break
		}
		canonical = info.AliasOf
	}
	return canonical
}

// refreshModelInfoCache 在写入后刷新索引；索引尚未启用时什么都不做。
func refreshModelInfoCache() {
	modelInfoCache.RLock()
	loaded := modelInfoCache.loaded
	modelInfoCache.RUnlock()
	if !loaded {
		return
	}
	if err := ReloadModelInfoCache(); err != nil {
		logger.SysError("Failed to reload model info cache: " + err.Error())
	}
}

// refreshCatalogCaches 在目录写入后刷新所有依赖 model_info 的进程内缓存：
// 内存索引，以及价格表上的 model_info 快照。二者都刷新，隐藏 / 别名 / 种子变更
// 才会立刻反映到 /api/available_model、/v1/models 与公开价格页，而不必等重启或下一次同步。
func refreshCatalogCaches() {
	refreshModelInfoCache()
	refreshPricingModelInfo()
}

// refreshPricingModelInfo 重建价格实例：Init 会把 model_info 重新挂到价格条目上，
// 读 price.ModelInfo 的接口才能看到最新元信息。价格实例未初始化时（如单测）什么都不做。
func refreshPricingModelInfo() {
	if PricingInstance == nil {
		return
	}
	if err := PricingInstance.Init(); err != nil {
		logger.SysError("Failed to refresh pricing after catalog change: " + err.Error())
	}
}

// lookupModelInfo 按模型标识精确查找目录行，命中不了返回 nil。
func lookupModelInfo(modelName string) *ModelInfo {
	if modelName == "" {
		return nil
	}
	modelInfoCache.RLock()
	if modelInfoCache.loaded {
		info := modelInfoCache.byModel[modelName]
		modelInfoCache.RUnlock()
		return info
	}
	modelInfoCache.RUnlock()

	if DB == nil {
		return nil
	}
	info, err := GetModelInfoByModel(modelName)
	if err != nil {
		return nil
	}
	return info
}

// modelAliasMaxDepth 限制别名跳数，避免脏数据构成的环把解析拖死。
const modelAliasMaxDepth = 8

// ResolveCanonicalModel 顺着 alias_of 解析到主名；无目录行或非别名时原样返回。
// isAlias 表示入参本身是别名（解析结果与入参不同）。
func ResolveCanonicalModel(name string) (canonical string, isAlias bool) {
	canonical = name
	for i := 0; i < modelAliasMaxDepth; i++ {
		info := lookupModelInfo(canonical)
		if info == nil || info.AliasOf == "" || info.AliasOf == canonical {
			break
		}
		canonical = info.AliasOf
	}
	return canonical, canonical != name
}

// IsModelHidden 判断模型是否被管理员隐藏；别名随主名，没有目录行一律不隐藏。
func IsModelHidden(name string) bool {
	canonical, _ := ResolveCanonicalModel(name)
	info := lookupModelInfo(canonical)
	return info != nil && info.Hidden
}

// HasCatalogRow 判断目录里是否存在该模型标识的行（别名行也算）。
// 任务型 relay 据此区分「内部伪模型名」与「目录管辖的模型」。
func HasCatalogRow(name string) bool {
	return lookupModelInfo(name) != nil
}

// IsCatalogVisible 判断一个可路由模型能否出现在对外可见面（公开模型列表、模型检索）：
// 别名行一律不可见（可调用不可见），主名未被隐藏即可见，没有目录行同样可见。
// 「可路由」由调用方保证（列表取自渠道分组）。catalog.enforce_hidden 关闭时一律可见，
// 退回「所有可路由模型」的行为。公开面的可见性判定只此一处，避免各接口口径漂移。
func IsCatalogVisible(name string) bool {
	if !config.CatalogEnforceHidden() {
		return true
	}
	if _, isAlias := ResolveCanonicalModel(name); isAlias {
		return false
	}
	return !IsModelHidden(name)
}

// ModelAliases 返回指向该主名的别名列表（已排序）；索引未加载时回落 DB 只查直接别名。
// 无别名返回空切片，便于公开接口直接序列化为 []。
func ModelAliases(canonical string) []string {
	if canonical == "" {
		return []string{}
	}

	modelInfoCache.RLock()
	if modelInfoCache.loaded {
		aliases := modelInfoCache.aliasesByModel[canonical]
		modelInfoCache.RUnlock()
		return append([]string{}, aliases...)
	}
	modelInfoCache.RUnlock()

	if DB == nil {
		return []string{}
	}
	var infos []*ModelInfo
	if err := DB.Where("alias_of = ?", canonical).Order("model asc").Find(&infos).Error; err != nil {
		return []string{}
	}
	aliases := make([]string, 0, len(infos))
	for _, info := range infos {
		if info.Model != canonical {
			aliases = append(aliases, info.Model)
		}
	}
	return aliases
}

// ModelEndpoints 返回模型声明的接口能力；别名随主名，未设置或无目录行返回空切片。
func ModelEndpoints(name string) []string {
	canonical, _ := ResolveCanonicalModel(name)
	info := lookupModelInfo(canonical)
	if info == nil {
		return []string{}
	}
	endpoints, _ := utils.UnmarshalString[[]string](info.Endpoints)
	if endpoints == nil {
		return []string{}
	}
	return endpoints
}

// ModelCreatedAt 返回模型目录行的创建时间（unix 秒）；别名随主名，无目录行返回 0。
func ModelCreatedAt(name string) int64 {
	canonical, _ := ResolveCanonicalModel(name)
	info := lookupModelInfo(canonical)
	if info == nil {
		return 0
	}
	return info.CreatedAt
}

// ResolveVendorID 取模型在目录里的厂商：本行 vendor_id 优先，为空则取别名主名的厂商；
// 都没有返回 0（未知），由调用方继续回退。
func ResolveVendorID(name string) int {
	info := lookupModelInfo(name)
	if info == nil {
		return 0
	}
	if info.VendorID != 0 {
		return info.VendorID
	}
	if canonical, isAlias := ResolveCanonicalModel(name); isAlias {
		if target := lookupModelInfo(canonical); target != nil {
			return target.VendorID
		}
	}
	return 0
}

// VendorIDFromModelName 按 "厂商slug/模型名" 前缀匹配厂商（OpenRouter 风格标识）；
// 无前缀或 slug 未登记返回 0。OpenRouter 的 latest 别名（~anthropic/...）先去掉开头的 ~。
func VendorIDFromModelName(modelName string) int {
	modelName = strings.TrimPrefix(modelName, "~")
	idx := strings.Index(modelName, "/")
	if idx <= 0 {
		return 0
	}
	return ModelOwnedBysInstance.GetIdBySlug(modelName[:idx])
}
