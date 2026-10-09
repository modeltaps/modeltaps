package model_utils

import (
	"github.com/modeltaps/modeltaps/common/config"
	"strings"
)

// Gemini 原生生图模型：通过 generateContent + responseModalities:[Text,Image] 出图，
// 只支持 generateContent 端点，不支持 :predict。
// 名单集中于此，供 chat 注入 modalities 与渠道测速判类型共用：历史上 chat.go 用硬编码白名单、
// channel-test.go 用 imageRegex 的裸 image 关键词，两套口径不一致，导致 gemini-3.1-flash-image
// 等新模型测速被判成 image 走 predict 而 404 并被自动禁用。
//
// 用前缀匹配而非精确匹配，自动覆盖 -preview 等变体。
// Legacy models without variants are matched exactly so a broad prefix cannot misclassify
// future non-image models that share it.
var geminiNativeImageModelPrefixes = []string{
	"gemini-2.5-flash-image",      // 覆盖 -preview
	"gemini-3-pro-image",          // 覆盖 -preview
	"gemini-3.1-flash-image",      // 覆盖 -preview
	"gemini-3.1-flash-lite-image", // 前缀不被上一条覆盖，单列
	"gemini-nano-banana",          // nano-banana family (2.1 and later)
}

// geminiNativeImageModelExact lists legacy native image models matched by exact name.
var geminiNativeImageModelExact = []string{
	"gemini-2.0-flash-exp", // 旧的实验性生图，保留防回归
}

// IsGeminiNativeImageModel 判断是否为 Gemini 原生生图模型（走 generateContent）。
// 注意与 types.IsImageGenerationModel 区分：后者是走 predict/images 协议的模型
// （dall-e / gpt-image / imagen），两者对 Gemini 家族互斥，原生生图绝不能进那一类。
func IsGeminiNativeImageModel(modelName string) bool {
	for _, exact := range geminiNativeImageModelExact {
		if modelName == exact || (config.ModelNameCaseInsensitiveEnabled && strings.EqualFold(modelName, exact)) {
			return true
		}
	}
	for _, p := range geminiNativeImageModelPrefixes {
		if HasPrefixCaseInsensitive(modelName, p) {
			return true
		}
	}
	return false
}
