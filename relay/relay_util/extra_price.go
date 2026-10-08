package relay_util

import (
	"github.com/modeltaps/modeltaps/common/model_utils"
	"github.com/modeltaps/modeltaps/types"
	"strings"
)

// 暂时先放这里，简单处理
type ExtraServicePriceConfig struct {
	// Web Search 价格配置
	WebSearch map[string]float64 `json:"web_search"` // tier -> price
	// File Search 价格
	FileSearch float64 `json:"file_search"`
	// Code Interpreter 价格
	CodeInterpreter float64 `json:"code_interpreter"`

	ImageGeneration map[string]map[string]float64 `json:"image_generation"`
}

var defaultExtraServicePrices = ExtraServicePriceConfig{
	WebSearch: map[string]float64{
		"high_tier": 0.025,
		"standard":  0.01,
	},
	FileSearch:      0.0025,
	CodeInterpreter: 0.03,
	ImageGeneration: map[string]map[string]float64{
		"low": {
			"1024x1024": 0.011,
			"1024x1536": 0.016,
			"1536x1024": 0.016,
		},
		"medium": {
			"1024x1024": 0.042,
			"1024x1536": 0.063,
			"1536x1024": 0.063,
		},
		"high": {
			"1024x1024": 0.167,
			"1024x1536": 0.25,
			"1536x1024": 0.25,
		},
		// xhigh and max are new gpt-image-2.5 tiers, priced at $30/M output tokens times the
		// third-party token measurements (1024x1024: xhigh ~3122, max ~7024), with rectangular
		// sizes extrapolated at 1.5x. xhigh ($0.094) < high ($0.167) is expected: the
		// low/medium/high rows come from gpt-image-1's official values (output $40/M), and the
		// two generations differ in token scale and rate; the table is keyed by quality, not model.
		"xhigh": {
			"1024x1024": 0.094,
			"1024x1536": 0.141,
			"1536x1024": 0.141,
		},
		"max": {
			"1024x1024": 0.211,
			"1024x1536": 0.316,
			"1536x1024": 0.316,
		},
	},
}

func getModelTier(modelName string) string {
	// 高级模型：gpt-4.1, gpt-4o(包含 mini)
	if model_utils.HasPrefixCaseInsensitive(modelName, "gpt-4.1") || model_utils.HasPrefixCaseInsensitive(modelName, "gpt-4o") {
		return "high_tier"
	}
	return "standard"
}

// 获取默认的额外服务价格
func getDefaultExtraServicePrice(serviceType, modelName, extraType string) float64 {
	switch serviceType {
	case types.APITollTypeWebSearchPreview:
		tier := getModelTier(modelName)
		return defaultExtraServicePrices.WebSearch[tier]
	case types.APITollTypeFileSearch:
		return defaultExtraServicePrices.FileSearch
	case types.APITollTypeCodeInterpreter:
		return defaultExtraServicePrices.CodeInterpreter

	case types.APITollTypeImageGeneration:
		if extraType == "" {
			return 0
		}
		// imageType 需要是 quality + "-" + size 的格式
		parts := strings.Split(extraType, "-")
		if len(parts) != 2 {
			return 0
		}
		quality := strings.ToLower(parts[0])
		size := strings.ToLower(parts[1])

		if quality == "" || size == "" {
			return 0
		}

		if _, ok := defaultExtraServicePrices.ImageGeneration[quality]; !ok {
			// 如果 quality 不在预设的价格表中，返回 0
			return 0
		}
		if price, ok := defaultExtraServicePrices.ImageGeneration[quality][size]; ok {
			// 如果 size 在预设的价格表中，返回对应的价格
			return price
		}
		// 如果 size 不在预设的价格表中，返回 0
		return 0
	default:
		return 0
	}
}
