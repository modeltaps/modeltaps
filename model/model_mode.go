package model

import (
	"github.com/modeltaps/modeltaps/common/model_utils"
	"github.com/modeltaps/modeltaps/types"

	"github.com/spf13/viper"
)

// 模型任务类型：决定请求走哪种协议，而不是模型能出什么模态。
// 空串表示未设置，按硬编码名单兜底。
const (
	ModelModeChat      = "chat"
	ModelModeImage     = "image"
	ModelModeChatImage = "chat_image"
)

// IsValidModelMode 校验后台写入的任务类型，空串（未设置）视为合法。
func IsValidModelMode(mode string) bool {
	switch mode {
	case "", ModelModeChat, ModelModeImage, ModelModeChatImage:
		return true
	}
	return false
}

// fallbackModelMode 硬编码名单兜底判定，等价于本次改动前四个调用点的行为。
func fallbackModelMode(modelName string) string {
	if types.IsImageGenerationModel(modelName) {
		return ModelModeImage
	}
	if model_utils.IsGeminiNativeImageModel(modelName) {
		return ModelModeChatImage
	}
	return ModelModeChat
}

// ModelModeFromInfo 返回 model_info 中显式设置的任务类型；
// routing.model_info_mode 关闭、无 model_info 行或 mode 未设置时返回空串。
func ModelModeFromInfo(modelName string) string {
	if !viper.GetBool("routing.model_info_mode") || PricingInstance == nil {
		return ""
	}
	price := PricingInstance.GetPrice(modelName)
	if price == nil || price.ModelInfo == nil {
		return ""
	}
	return price.ModelInfo.Mode
}

// ResolveModelMode 判定模型任务类型的统一入口。
// routing.model_info_mode 关闭（默认）时只走硬编码名单、忽略 DB；打开时 model_info.mode
// 非空以其为准（显式 chat 可否决名单），为空或无 model_info 行则回落名单。
func ResolveModelMode(modelName string) string {
	if mode := ModelModeFromInfo(modelName); mode != "" {
		return mode
	}
	return fallbackModelMode(modelName)
}
