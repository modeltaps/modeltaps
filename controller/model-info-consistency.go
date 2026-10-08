package controller

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/modeltaps/modeltaps/common/model_utils"
	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
)

// ModelModalities 是一致性校验的输入单元：模型名 + model_info 里的输出模态列表。
// model_info 缺失时 OutputModalities 为 nil，等同「未标注 image」。
// DBMode 为 model_info.mode 的原始值（空 = 未设置），只作输入用、不参与序列化。
type ModelModalities struct {
	Model            string   `json:"model"`
	OutputModalities []string `json:"output_modalities"`
	DBMode           string   `json:"-"`
}

// ModelModeConflict 记录 model_info 显式任务类型与硬编码名单判定不一致的模型。
type ModelModeConflict struct {
	Model         string `json:"model"`
	DBMode        string `json:"db_mode"`
	HardcodedMode string `json:"hardcoded_mode"`
}

// ModelInfoConsistencySummary 汇总一次一致性校验。
// MissingImage 是强制方向：硬编码生图名单判真但 output_modalities 不含 image，非空即告警。
// UnexpectedImage 是反向差异（model_info 标了 image 但两份硬编码名单都不认识），
// ModeConflicts 是 DB 显式 mode 与硬编码名单判定的差异；两者均仅作为次要字段返回，
// 不参与是否发通知的判断。
type ModelInfoConsistencySummary struct {
	Checked         int                 `json:"checked"`
	MissingImage    []ModelModalities   `json:"missing_image"`
	UnexpectedImage []ModelModalities   `json:"unexpected_image,omitempty"`
	ModeConflicts   []ModelModeConflict `json:"mode_conflicts,omitempty"`
}

// isHardcodedImageModel 汇总两份硬编码生图名单：predict/images 协议一类与 Gemini 原生生图一类。
func isHardcodedImageModel(modelName string) bool {
	return types.IsImageGenerationModel(modelName) || model_utils.IsGeminiNativeImageModel(modelName)
}

// hardcodedModelMode 只按两份硬编码名单给出任务类型结论，不受 routing.model_info_mode 开关影响。
func hardcodedModelMode(modelName string) string {
	switch {
	case types.IsImageGenerationModel(modelName):
		return model.ModelModeImage
	case model_utils.IsGeminiNativeImageModel(modelName):
		return model.ModelModeChatImage
	}
	return model.ModelModeChat
}

// hasImageOutputModality 判断输出模态列表是否标注了 image（大小写与空白不敏感）。
func hasImageOutputModality(modalities []string) bool {
	for _, m := range modalities {
		if strings.EqualFold(strings.TrimSpace(m), "image") {
			return true
		}
	}
	return false
}

// computeModelInfoConsistency 是一致性判定的纯函数：给定模型名与其输出模态列表，
// 返回强制方向（名单判真但未标 image）、反向差异（标了 image 但名单不认识），
// 以及 DB 显式 mode 与名单判定不一致的清单。
func computeModelInfoConsistency(models []ModelModalities) *ModelInfoConsistencySummary {
	summary := &ModelInfoConsistencySummary{Checked: len(models)}
	for _, m := range models {
		hardcoded := isHardcodedImageModel(m.Model)
		tagged := hasImageOutputModality(m.OutputModalities)
		switch {
		case hardcoded && !tagged:
			summary.MissingImage = append(summary.MissingImage, m)
		case !hardcoded && tagged:
			summary.UnexpectedImage = append(summary.UnexpectedImage, m)
		}
		if hardcodedMode := hardcodedModelMode(m.Model); m.DBMode != "" && m.DBMode != hardcodedMode {
			summary.ModeConflicts = append(summary.ModeConflicts, ModelModeConflict{
				Model:         m.Model,
				DBMode:        m.DBMode,
				HardcodedMode: hardcodedMode,
			})
		}
	}
	return summary
}

// CheckAllModelInfoConsistency 对价格表内全部模型执行一致性校验。
// 数据源为 model.PricingInstance.GetAllPrices()，每条 Price 已带 .ModelInfo（缺失时为 nil）。
// 模型名排序后再判定，保证结果与通知文案稳定可比。
func CheckAllModelInfoConsistency() *ModelInfoConsistencySummary {
	prices := model.PricingInstance.GetAllPrices()
	names := make([]string, 0, len(prices))
	for name := range prices {
		names = append(names, name)
	}
	sort.Strings(names)

	models := make([]ModelModalities, 0, len(names))
	for _, name := range names {
		item := ModelModalities{Model: name}
		if info := prices[name].ModelInfo; info != nil {
			item.OutputModalities = info.OutputModalities
			item.DBMode = info.Mode
		}
		models = append(models, item)
	}
	return computeModelInfoConsistency(models)
}

// CheckAndNotifyModelInfoConsistency 供 cron 调用：执行一致性校验，
// 仅当强制方向（MissingImage）非空时发通知，反向差异不触发通知。
func CheckAndNotifyModelInfoConsistency() *ModelInfoConsistencySummary {
	summary := CheckAllModelInfoConsistency()
	if len(summary.MissingImage) > 0 {
		notifyModelInfoConsistency(summary)
	}
	return summary
}

// notifyModelInfoConsistency 组装一致性告警：标题含待修模型数，正文逐条列出模型名与当前输出模态。
func notifyModelInfoConsistency(summary *ModelInfoConsistencySummary) {
	subject := fmt.Sprintf("Detected %d image generation model(s) without the image output modality", len(summary.MissingImage))

	var b strings.Builder
	for _, m := range summary.MissingImage {
		current := strings.Join(m.OutputModalities, ", ")
		if current == "" {
			current = "(missing model_info)"
		}
		b.WriteString(fmt.Sprintf("Model \"%s\" current output modalities: %s\n", m.Model, current))
	}
	notify.Send(subject, b.String())
}

// CheckModelInfoConsistency 手动触发一致性校验（AdminAuth）：只返回校验结果，不发通知。
func CheckModelInfoConsistency(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": CheckAllModelInfoConsistency()})
}
