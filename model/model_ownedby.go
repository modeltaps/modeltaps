package model

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"

	"gorm.io/gorm"
)

var UnknownOwnedBy = "Unknown"

const ModelOwnedByReserveID = 1000

// 默认模型图标（未知厂商时使用）：空串表示交给前端按厂商名自动解析，解析不到显示首字母占位。
const DefaultModelIcon = ""

// ErrVendorSlugTaken 表示 slug 已被其他厂商占用，由数据库唯一约束判定，接口据此返回 400。
var ErrVendorSlugTaken = errors.New("slug already exists")

// NullableSlug 是 model_owned_by.slug 的列类型，语义与 NullableEmail 一致：Go 侧仍是字符串
// （空串表示"该厂商没有公开标识"），落库时空串写为 NULL，从而让普通唯一索引同时满足
// "一个 slug 只对应一个厂商"与"任意多个无 slug 厂商共存"（三种方言的唯一索引均不约束 NULL）。
type NullableSlug string

// Value 空串落库为 NULL，其余原样写入。
func (s NullableSlug) Value() (driver.Value, error) {
	if s == "" {
		return nil, nil
	}
	return string(s), nil
}

// Scan NULL 读回为空串，与历史上的 ” 语义一致。
func (s *NullableSlug) Scan(value any) error {
	switch v := value.(type) {
	case nil:
		*s = ""
	case string:
		*s = NullableSlug(v)
	case []byte:
		*s = NullableSlug(v)
	default:
		return fmt.Errorf("cannot parse %T as a vendor slug", value)
	}
	return nil
}

// GormDataType 保持与原 string 字段一致的列类型推导（各方言的默认字符串列）。
func (NullableSlug) GormDataType() string {
	return "string"
}

type ModelOwnedBy struct {
	Id   int    `json:"id" gorm:"index"`
	Name string `json:"name" gorm:"type:varchar(100)"`
	// Icon 取值见 NormalizeVendorIcon：空串（自动解析）/ brand:{key} / upload:{assetId}。
	Icon string `json:"icon" gorm:"type:text"`
	// IconLegacy 迁移前的外链图标原值，供回滚恢复；不对外输出，也不随编辑覆盖。
	IconLegacy string `json:"-" gorm:"type:text"`
	// Slug 厂商小写标识（openai / anthropic / x-ai …），用于把同步来源的
	// provider id、模型名前缀映射到厂商行；空串表示该厂商没有稳定的公开标识。
	Slug NullableSlug `json:"slug" gorm:"type:varchar(64);uniqueIndex:idx_model_owned_by_slug"`
}

func (m *ModelOwnedBy) TableName() string {
	return "model_owned_by"
}

func CreateModelOwnedBy(modelOwnedBy *ModelOwnedBy) error {
	err := DB.Create(modelOwnedBy).Error
	if err != nil {
		return mapVendorSlugConflict(err)
	}

	ModelOwnedBysInstance.Load()

	return nil
}

func UpdateModelOwnedBy(modelOwnedBy *ModelOwnedBy) error {
	err := DB.Omit("id", "icon_legacy").Save(modelOwnedBy).Error
	if err != nil {
		return mapVendorSlugConflict(err)
	}

	ModelOwnedBysInstance.Load()

	return nil
}

// mapVendorSlugConflict 把唯一索引冲突翻译成 ErrVendorSlugTaken：内存快照只是加速判重，
// 并发写入下唯一的权威判据是数据库约束本身。
func mapVendorSlugConflict(err error) error {
	if err == nil || !isUniqueViolation(err) {
		return err
	}
	return ErrVendorSlugTaken
}

// isUniqueViolation 判定唯一约束冲突。本仓库未开启 gorm 的 TranslateError，
// 三种方言的驱动只给出各自的原始错误文本，故按文本兜底。
func isUniqueViolation(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") || // sqlite / postgres
		strings.Contains(msg, "duplicate key value") || // postgres
		strings.Contains(msg, "duplicate entry") // mysql
}

func GetModelOwnedBy(id int) (*ModelOwnedBy, error) {
	modelOwnedBy := &ModelOwnedBy{}
	err := DB.Where("id = ?", id).First(modelOwnedBy).Error
	if err != nil {
		return nil, err
	}
	return modelOwnedBy, nil
}

func GetAllModelOwnedBy() ([]*ModelOwnedBy, error) {
	var modelOwnedBies []*ModelOwnedBy
	err := DB.Find(&modelOwnedBies).Error
	if err != nil {
		return nil, err
	}
	return modelOwnedBies, nil
}

func DeleteModelOwnedBy(id int) error {
	err := DB.Delete(&ModelOwnedBy{}, id).Error
	if err != nil {
		return err
	}

	ModelOwnedBysInstance.Load()

	return nil
}

type ModelOwnedBys struct {
	sync.RWMutex
	ModelOwnedBy map[int]*ModelOwnedBy
	// idBySlug 厂商 slug（小写）到厂商 id 的反查索引，随 Load 一起重建。
	idBySlug map[string]int
}

var ModelOwnedBysInstance *ModelOwnedBys

func NewModelOwnedBys() {
	ModelOwnedBysInstance = &ModelOwnedBys{}
	err := ModelOwnedBysInstance.Load()
	if err != nil {
		logger.SysError("Failed to initialize ModelOwnedBys:" + err.Error())
		return
	}

	logger.SysLog("Checking for ModelOwned updates")
	modelOwnedBies := GetDefaultModelOwnedBy()
	ModelOwnedBysInstance.SyncModelOwnedBy(modelOwnedBies)
	logger.SysLog("ModelOwnedBys initialized")
}

func (m *ModelOwnedBys) Load() error {
	modelOwnedBies, err := GetAllModelOwnedBy()
	if err != nil {
		return err
	}

	newModelOwnedBy := make(map[int]*ModelOwnedBy)
	newIdBySlug := make(map[string]int)
	for _, modelOwnedBy := range modelOwnedBies {
		newModelOwnedBy[modelOwnedBy.Id] = modelOwnedBy
		if slug := strings.ToLower(string(modelOwnedBy.Slug)); slug != "" {
			newIdBySlug[slug] = modelOwnedBy.Id
		}
	}

	m.Lock()
	defer m.Unlock()

	m.ModelOwnedBy = newModelOwnedBy
	m.idBySlug = newIdBySlug

	return nil
}

// GetIdBySlug 按 slug 反查厂商 id；未知 slug 返回 0（未知厂商）。
func (m *ModelOwnedBys) GetIdBySlug(slug string) int {
	if m == nil || slug == "" {
		return 0
	}
	m.RLock()
	defer m.RUnlock()

	return m.idBySlug[strings.ToLower(slug)]
}

func (m *ModelOwnedBys) Get(id int) *ModelOwnedBy {
	m.RLock()
	defer m.RUnlock()

	return m.ModelOwnedBy[id]
}

func (m *ModelOwnedBys) GetName(id int) string {
	modelOwnedBy := m.Get(id)
	if modelOwnedBy == nil {
		return UnknownOwnedBy
	}
	return modelOwnedBy.Name
}

// GetSlug 取厂商 slug；未登记或未设置 slug 的厂商返回空串。
func (m *ModelOwnedBys) GetSlug(id int) string {
	if m == nil || id == 0 {
		return ""
	}
	modelOwnedBy := m.Get(id)
	if modelOwnedBy == nil {
		return ""
	}
	return strings.ToLower(string(modelOwnedBy.Slug))
}

func (m *ModelOwnedBys) GetIcon(id int) string {
	modelOwnedBy := m.Get(id)
	if modelOwnedBy == nil {
		return DefaultModelIcon
	}
	return modelOwnedBy.Icon
}

func (m *ModelOwnedBys) GetAll() map[int]*ModelOwnedBy {
	m.RLock()
	defer m.RUnlock()

	return m.ModelOwnedBy
}

func (m *ModelOwnedBys) SyncModelOwnedBy(modelOwnedBies []*ModelOwnedBy) {
	var newModelOwnedBy []*ModelOwnedBy
	changed := false

	for _, modelOwnedBy := range modelOwnedBies {
		existing, ok := m.ModelOwnedBy[modelOwnedBy.Id]
		if !ok {
			newModelOwnedBy = append(newModelOwnedBy, modelOwnedBy)
			continue
		}
		// 存量行只回填缺失的 slug，管理员改过的名称 / 图标 / slug 一律不动。
		if existing.Slug == "" && modelOwnedBy.Slug != "" {
			if err := DB.Model(&ModelOwnedBy{}).Where("id = ?", existing.Id).
				Update("slug", modelOwnedBy.Slug).Error; err != nil {
				logger.SysError("Failed to backfill ModelOwnedBy slug:" + err.Error())
				continue
			}
			changed = true
		}
	}

	if len(newModelOwnedBy) > 0 {
		err := DB.CreateInBatches(newModelOwnedBy, 100).Error
		if err != nil {
			logger.SysError("Failed to sync ModelOwnedBy:" + err.Error())
		} else {
			changed = true
		}
	}

	if !changed {
		return
	}

	m.Load()
}

// defaultVendorSlugs 主流厂商的小写 slug，取值与 OpenRouter 模型名前缀 / models.dev 的
// provider id 对齐，便于同步时按元数据定位厂商。聚合型渠道（OpenRouter、Siliconflow 等）
// 与没有稳定公开标识的厂商不给 slug；Google PaLM 与 Gemini 同属 google，只给 Gemini。
var defaultVendorSlugs = map[int]string{
	config.ChannelTypeOpenAI:    "openai",
	config.ChannelTypeAnthropic: "anthropic",
	config.ChannelTypeGemini:    "google",
	config.ChannelTypeXAI:       "x-ai",
	config.ChannelTypeDeepseek:  "deepseek",
	config.ChannelTypeAli:       "qwen",
	config.ChannelTypeMoonshot:  "moonshotai",
	config.ChannelTypeZhipu:     "z-ai",
	config.ChannelTypeLLAMA:     "meta-llama",
	config.ChannelTypeMistral:   "mistralai",
	config.ChannelTypeBaidu:     "baidu",
	config.ChannelTypeCohere:    "cohere",
	config.ChannelTypeGroq:      "groq",
	config.ChannelTypeMiniMax:   "minimax",
	config.ChannelTypeBaichuan:  "baichuan",
	config.ChannelTypeHunyuan:   "tencent",
	config.ChannelTypeLingyi:    "01-ai",
}

func GetDefaultModelOwnedBy() []*ModelOwnedBy {
	owners := []*ModelOwnedBy{
		{Id: config.ChannelTypeOpenAI, Name: "OpenAI", Icon: "brand:openai"},
		{Id: config.ChannelTypePaLM, Name: "Google PaLM", Icon: "brand:gemini"},
		{Id: config.ChannelTypeAnthropic, Name: "Anthropic", Icon: "brand:claude"},
		{Id: config.ChannelTypeBaidu, Name: "Baidu", Icon: "brand:wenxin"},
		{Id: config.ChannelTypeZhipu, Name: "Zhipu", Icon: "brand:zhipu"},
		{Id: config.ChannelTypeAli, Name: "Qwen", Icon: "brand:qwen"},
		{Id: config.ChannelTypeXunfei, Name: "Spark", Icon: "brand:spark"},
		{Id: config.ChannelType360, Name: "360", Icon: "brand:ai360"},
		{Id: config.ChannelTypeTencent, Name: "Tencent", Icon: "brand:hunyuan"},
		{Id: config.ChannelTypeGemini, Name: "Google Gemini", Icon: "brand:gemini"},
		{Id: config.ChannelTypeBaichuan, Name: "Baichuan", Icon: "brand:baichuan"},
		{Id: config.ChannelTypeMiniMax, Name: "MiniMax", Icon: "brand:minimax"},
		{Id: config.ChannelTypeDeepseek, Name: "Deepseek", Icon: "brand:deepseek"},
		{Id: config.ChannelTypeMoonshot, Name: "Moonshot", Icon: "brand:moonshot"},
		{Id: config.ChannelTypeMistral, Name: "Mistral", Icon: "brand:mistral"},
		{Id: config.ChannelTypeGroq, Name: "Groq", Icon: "brand:groq"},
		{Id: config.ChannelTypeLingyi, Name: "Yi", Icon: "brand:yi"},
		{Id: config.ChannelTypeMidjourney, Name: "Midjourney", Icon: "brand:midjourney"},
		{Id: config.ChannelTypeCloudflareAI, Name: "Cloudflare AI", Icon: "brand:cloudflare"},
		{Id: config.ChannelTypeCohere, Name: "Cohere", Icon: "brand:cohere"},
		{Id: config.ChannelTypeStabilityAI, Name: "Stability AI", Icon: "brand:stability"},
		{Id: config.ChannelTypeCoze, Name: "Coze", Icon: "brand:coze"},
		{Id: config.ChannelTypeOllama, Name: "Ollama", Icon: "brand:ollama"},
		{Id: config.ChannelTypeHunyuan, Name: "Hunyuan", Icon: "brand:hunyuan"},
		{Id: config.ChannelTypeSuno, Name: "Suno", Icon: "brand:suno"},
		{Id: config.ChannelTypeLLAMA, Name: "Meta", Icon: "brand:meta"},
		{Id: config.ChannelTypeIdeogram, Name: "Ideogram", Icon: "brand:ideogram"},
		{Id: config.ChannelTypeSiliconflow, Name: "Siliconflow", Icon: "brand:siliconcloud"},
		{Id: config.ChannelTypeFlux, Name: "Flux", Icon: "brand:flux"},
		{Id: config.ChannelTypeJina, Name: "Jina", Icon: "brand:jina"},
		{Id: config.ChannelTypeRerank, Name: "Rerank", Icon: ""},
		{Id: config.ChannelTypeRecraft, Name: "RecraftAI", Icon: "brand:recraft"},
		{Id: config.ChannelTypeKling, Name: "Kling", Icon: "brand:kling"},
		{Id: config.ChannelTypeOpenRouter, Name: "OpenRouter", Icon: "brand:openrouter"},
		{Id: config.ChannelTypeXAI, Name: "xAI", Icon: "brand:xai"},
	}

	for _, owner := range owners {
		owner.Slug = NullableSlug(defaultVendorSlugs[owner.Id])
	}

	return owners
}
