package model

import (
	"time"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	TokensPriceType = "tokens"
	TimesPriceType  = "times"
	// MinutesPriceType bills by audio length (speech-to-text). Input is the rate per minute,
	// in the same unit as a times price per call (1 = $0.002); usage is pro-rated by the second.
	MinutesPriceType   = "minutes"
	DefaultPrice       = 30.0
	DollarRate         = 0.002
	RMBRate            = 0.014
	DefaultCacheRatios = 0.5
	DefaultAudioRatio  = 40

	DefaultCachedWriteRatio = 1.25
	DefaultCachedReadRatio  = 0.1
)

func GetIncreaseTokens(tokens int, ratio float64) int {
	return int(float64(tokens) * (ratio - 1))
}

var ExtraKeyIsPrompt = map[string]bool{
	config.UsageExtraCache:             true,
	config.UsageExtraCachedWrite:       true,
	config.UsageExtraCachedWrite1h:     true,
	config.UsageExtraCachedRead:        true,
	config.UsageExtraInputAudio:        true,
	config.UsageExtraOutputAudio:       false,
	config.UsageExtraReasoning:         false,
	config.UsageExtraInputTextTokens:   true,
	config.UsageExtraOutputTextTokens:  false,
	config.UsageExtraInputImageTokens:  true,
	config.UsageExtraOutputImageTokens: false,
	config.UsageExtraOpenAICacheWrite:  true,
}

func GetExtraPriceIsPrompt(key string) bool {
	return ExtraKeyIsPrompt[key]
}

var defaultExtraPrice = map[string]float64{
	config.UsageExtraCache:            1,
	config.UsageExtraCachedWrite:      1.25,
	config.UsageExtraCachedWrite1h:    2,
	config.UsageExtraCachedRead:       0.1,
	config.UsageExtraInputAudio:       1,
	config.UsageExtraOutputAudio:      1,
	config.UsageExtraReasoning:        1,
	config.UsageExtraInputTextTokens:  1,
	config.UsageExtraOutputTextTokens: 1,
	config.UsageExtraOpenAICacheWrite: 1.25,
}

type Price struct {
	Model       string  `json:"model" gorm:"type:varchar(100)" binding:"required"`
	Type        string  `json:"type"  gorm:"default:'tokens'" binding:"required"`
	ChannelType int     `json:"channel_type" gorm:"default:0" binding:"gte=0"`
	Input       float64 `json:"input" gorm:"default:0" binding:"gte=0"`
	Output      float64 `json:"output" gorm:"default:0" binding:"gte=0"`
	Locked      bool    `json:"locked" gorm:"default:false"` // 如果模型为locked 则覆盖模式不会更新locked的模型价格

	ExtraRatios *datatypes.JSONType[map[string]float64] `json:"extra_ratios,omitempty" gorm:"type:json"`
	LongContext *datatypes.JSONType[LongContextTier]    `json:"long_context,omitempty" gorm:"type:json"`
	ModelInfo   *ModelInfoResponse                      `json:"model_info,omitempty" gorm:"-"`

	// Unconfigured 标记「价格表里查无此模型」的兜底 Price(非持久化)。
	// 计费侧据 unpriced_model_policy 处理,避免静默按某个数字计费(参见 ApplyUnpricedPolicy)。
	Unconfigured bool `json:"unconfigured,omitempty" gorm:"-"`
}

// LongContextTier 长上下文分档计费：输入 token 超过 Threshold 时，整次请求输入/输出侧套用对应倍率。
type LongContextTier struct {
	Threshold   int     `json:"threshold"`    // 输入 token 阈值，如 272000
	InputRatio  float64 `json:"input_ratio"`  // 超阈值时输入倍率，如 2
	OutputRatio float64 `json:"output_ratio"` // 超阈值时输出倍率，如 1.5
}

// ApplyUnpricedPolicy 对「未配置价格」的兜底 Price 按策略处理:
// block(默认/未知)→reject=true(拒绝请求,零费用); zero→保持 0(放行不计费); default→用 defaultRatio。
// 已配置价格(Unconfigured=false)永不受影响。
func (p *Price) ApplyUnpricedPolicy(policy string, defaultRatio float64) (reject bool) {
	if !p.Unconfigured {
		return false
	}
	switch policy {
	case "zero":
		return false
	case "default":
		p.Input = defaultRatio
		p.Output = defaultRatio
		return false
	default:
		return true
	}
}

func GetAllPrices() ([]*Price, error) {
	var prices []*Price
	if err := DB.Find(&prices).Error; err != nil {
		return nil, err
	}
	// if config.ExtraTokenPriceJson == "" {
	// 	return prices, nil
	// }

	// extraRatios := make(map[string]map[string]float64)
	// err := json.Unmarshal([]byte(config.ExtraTokenPriceJson), &extraRatios)
	// if err != nil {
	// 	return nil, err
	// }

	// for _, price := range prices {
	// 	if ratio, ok := extraRatios[price.Model]; ok {
	// 		price.ExtraRatios = ratio
	// 	}
	// }

	return prices, nil
}

func (price *Price) Update(modelName string) error {
	if err := DB.Model(price).Select("*").Where("model = ?", modelName).Updates(price).Error; err != nil {
		return err
	}

	return nil
}

func (price *Price) Insert() error {
	if err := DB.Create(price).Error; err != nil {
		return err
	}

	return nil
}

func (price *Price) GetInput() float64 {
	if price.Input <= 0 {
		return 0
	}
	return price.Input
}

func (price *Price) GetOutput() float64 {
	if price.Output <= 0 || price.Type == TimesPriceType || price.Type == MinutesPriceType {
		return 0
	}

	return price.Output
}

func (price *Price) GetExtraRatio(key string) float64 {
	if price.ExtraRatios != nil {
		extraRatios := price.ExtraRatios.Data()
		if ratio, ok := extraRatios[key]; ok {
			return ratio
		}
	}

	ratio, ok := defaultExtraPrice[key]
	if !ok {
		return 1
	}

	return ratio
}

// GetLongContextMultiplier 返回本次请求输入侧/输出侧应额外乘的倍率。
// 按次计费、未配置、阈值 <= 0 或输入 token 未超阈值时返回 (1, 1)。
func (price *Price) GetLongContextMultiplier(inputTokens int) (float64, float64) {
	if price.LongContext == nil || price.Type == TimesPriceType || price.Type == MinutesPriceType {
		return 1, 1
	}

	tier := price.LongContext.Data()
	if tier.Threshold <= 0 || inputTokens <= tier.Threshold {
		return 1, 1
	}

	in, out := tier.InputRatio, tier.OutputRatio
	if in <= 0 {
		in = 1
	}
	if out <= 0 {
		out = 1
	}

	return in, out
}

func (price *Price) FetchInputCurrencyPrice(rate float64) string {
	r := decimal.NewFromFloat(price.GetInput()).Mul(decimal.NewFromFloat(rate))
	return r.String()
}

func (price *Price) FetchOutputCurrencyPrice(rate float64) string {
	r := decimal.NewFromFloat(price.GetOutput()).Mul(decimal.NewFromFloat(rate))
	return r.String()
}

func UpdatePrices(tx *gorm.DB, models []string, prices *Price) error {
	err := tx.Model(Price{}).Where("model IN (?)", models).Select("*").Omit("model").Updates(
		Price{
			Type:        prices.Type,
			ChannelType: prices.ChannelType,
			Input:       prices.Input,
			Output:      prices.Output,
			Locked:      prices.Locked,
			ExtraRatios: prices.ExtraRatios,
			LongContext: prices.LongContext,
		}).Error

	return err
}

func DeletePrices(tx *gorm.DB, models []string) error {
	err := tx.Where("model IN (?)", models).Delete(&Price{}).Error

	return err
}

func InsertPrices(tx *gorm.DB, prices []*Price) error {
	err := tx.CreateInBatches(prices, 100).Error
	return err
}

func DeleteAllPrices(tx *gorm.DB) error {
	err := tx.Where("1=1").Delete(&Price{}).Error
	return err
}

// 只删除未lock的价格
func DeleteAllPricesNotLock(tx *gorm.DB) error {
	err := tx.Where("locked = ?", false).Delete(&Price{}).Error
	return err
}

// 只删除指定的未lock的数据
func DeletePricesByModelNameAndNotLock(tx *gorm.DB, models []string) error {
	err := tx.Where("locked = ? and model IN (?)", false, models).Delete(&Price{}).Error
	return err
}

func (price *Price) Delete() error {
	return DB.Where("model = ?", price.Model).Delete(&Price{}).Error
}

// GetDefaultPrice returns the prices a new deployment starts with: the built-in official list prices
// in effect now (price_official.go and price_schedule.go), each citing the vendor's pricing page, plus
// the gateway default prices (price_gateway.go) for supported models without a convertible official
// price. Retired models are not preset (instances that already stored them keep their rows, since the
// startup sync only adds missing models).
func GetDefaultPrice() []*Price {
	prices := GetOfficialPrices()
	for model, g := range gatewayDefaultPrices() {
		prices = append(prices, g.toPrice(model))
	}
	return prices
}

// GetOfficialPrices returns the built-in official list prices in effect now: per-token prices
// (price_official.go, with announced changes from price_schedule.go applied) and per-request prices.
// It is the source to restore prices that a sync replaced, and never includes gateway default prices.
func GetOfficialPrices() []*Price {
	tokens := officialTokenPricesAt(time.Now())
	requests := officialRequestPrices()
	prices := make([]*Price, 0, len(tokens)+len(requests))
	for model, o := range tokens {
		prices = append(prices, o.toPrice(model))
	}
	for model, o := range requests {
		prices = append(prices, o.toPrice(model))
	}
	return prices
}

func GetDefaultExtraRatio() string {
	return `{"gpt-4o-audio-preview":{"input_audio_tokens":40,"output_audio_tokens":20},"gpt-4o-audio-preview-2024-10-01":{"input_audio_tokens":40,"output_audio_tokens":20},"gpt-4o-audio-preview-2024-12-17":{"input_audio_tokens":16,"output_audio_tokens":8},"gpt-4o-mini-audio-preview":{"input_audio_tokens":67,"output_audio_tokens":34},"gpt-4o-mini-audio-preview-2024-12-17":{"input_audio_tokens":67,"output_audio_tokens":34},"gpt-4o-realtime-preview":{"input_audio_tokens":20,"output_audio_tokens":10},"gpt-4o-realtime-preview-2024-10-01":{"input_audio_tokens":20,"output_audio_tokens":10},"gpt-4o-realtime-preview-2024-12-17":{"input_audio_tokens":8,"output_audio_tokens":4},"gpt-4o-mini-realtime-preview":{"input_audio_tokens":17,"output_audio_tokens":8.4},"gpt-4o-mini-realtime-preview-2024-12-17":{"input_audio_tokens":17,"output_audio_tokens":8.4},"gemini-2.5-flash-preview-04-17":{"reasoning_tokens":5.833},"gpt-image-1":{"input_text_tokens": 0.5},"gpt-image-2":{"input_text_tokens": 0.625},"gpt-image-2.5-flare":{"input_text_tokens": 0.625},"gpt-image-2.5-flare-2026-09-08":{"input_text_tokens": 0.625},"gpt-image-2.5-sunburst":{"input_text_tokens": 0.625},"gpt-image-2.5-sunburst-2026-09-08":{"input_text_tokens": 0.625},"gpt-image-2.6-flare":{"input_text_tokens": 0.625},"gpt-image-2.6-sunburst":{"input_text_tokens": 0.625}}`

}
