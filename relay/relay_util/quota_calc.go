package relay_util

// 计价计算域:token 折算、配额计算(销售额/成本共用)、额外计费项载体。
// 由 quota.go 的 UpdateUserRealtimeQuota / completedQuotaConsumption 调用。

import (
	"math"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"
)

func (q *Quota) GetInputRatio() float64 {
	return q.inputRatio
}

func (q *Quota) getRequestTime() int {
	return int(time.Since(q.startTime).Milliseconds())
}

// calcQuota 按给定倍率计算配额。inputRatio/outputRatio 已含对应倍率（分组倍率或成本倍率），
// ratio 用于额外计费项。销售额与成本共用同一计算逻辑，仅倍率不同。
// 依赖调用方已通过 GetExtraBillingData 设置好 extraBillingData。
func (q *Quota) calcQuota(promptTokens, completionTokens int, inputRatio, outputRatio, ratio float64) (quota int) {
	if q.price.Type == model.TimesPriceType {
		quota = common.QuotaFromFloat(1000 * inputRatio)
	} else {
		quota = common.QuotaFromFloat(math.Ceil((float64(promptTokens) * inputRatio) + (float64(completionTokens) * outputRatio)))
	}

	extraBillingQuota := 0
	if q.extraBillingData != nil {
		for _, value := range q.extraBillingData {
			extraBillingQuota += common.QuotaFromFloat(
				math.Ceil(float64(value.Price)*float64(config.QuotaPerUnit)) * float64(value.CallCount),
			)
		}
	}

	if extraBillingQuota > 0 {
		quota += common.QuotaFromFloat(math.Ceil(
			float64(extraBillingQuota) * ratio,
		))
	}

	if inputRatio != 0 && quota <= 0 {
		quota = 1
	}
	totalTokens := promptTokens + completionTokens
	if totalTokens == 0 {
		// in this case, must be some error happened
		// we cannot just return, because we may have to return the pre-consumed quota
		quota = 0
	}

	// 空回复计费闸对按次计费（times）属于误伤：按次语义是"调用成功即全额收"，
	// 与 completion token 无关（Lyria 等音乐模型成功返回音频时 completionTokens 常为 0）。
	// 仅对 token 计费类型生效；闸1（totalTokens==0，上游未成功返回）对两类都保留。
	if q.price.Type != model.TimesPriceType && !config.EmptyResponseBillingEnabled && completionTokens == 0 {
		quota = 0
	}

	return quota
}

// 获取计算的 token 数
func (q *Quota) getComputeTokensByUsage(usage *types.Usage) (promptTokens, completionTokens int) {
	promptTokens = usage.PromptTokens
	completionTokens = usage.CompletionTokens

	extraTokens := usage.GetExtraTokens()

	for key, value := range extraTokens {
		extraRatio := q.price.GetExtraRatio(key)
		if model.GetExtraPriceIsPrompt(key) {
			promptTokens += model.GetIncreaseTokens(value, extraRatio)
		} else {
			completionTokens += model.GetIncreaseTokens(value, extraRatio)
		}
	}

	return
}

func (q *Quota) getComputeTokensByUsageEvent(usage *types.UsageEvent) (promptTokens, completionTokens int) {
	promptTokens = usage.InputTokens
	completionTokens = usage.OutputTokens
	extraTokens := usage.GetExtraTokens()

	for key, value := range extraTokens {
		extraRatio := q.price.GetExtraRatio(key)
		if model.GetExtraPriceIsPrompt(key) {
			promptTokens += model.GetIncreaseTokens(value, extraRatio)
		} else {
			completionTokens += model.GetIncreaseTokens(value, extraRatio)
		}
	}

	return
}

// 通过 usage 获取消费配额
func (q *Quota) GetTotalQuotaByUsage(usage *types.Usage) (quota int) {
	promptTokens, completionTokens := q.getComputeTokensByUsage(usage)
	// 长上下文分档：按原始输入 token（未经缓存折算）判断档位，超阈值时整次请求套用分档倍率。
	inRatio, outRatio := q.price.GetLongContextMultiplier(usage.PromptTokens)
	q.GetExtraBillingData(usage.ExtraBilling)
	return q.calcQuota(promptTokens, completionTokens, q.inputRatio*inRatio, q.outputRatio*outRatio, q.groupRatio)
}

// GetCostQuotaByUsage 按渠道成本倍率计算本次请求的上游成本配额，仅用于成本/利润统计，不参与扣费。
func (q *Quota) GetCostQuotaByUsage(usage *types.Usage) (costQuota int) {
	// 未配置成本倍率时不计成本，省去无谓计算。
	if q.costRatio <= 0 {
		return 0
	}
	promptTokens, completionTokens := q.getComputeTokensByUsage(usage)
	inRatio, outRatio := q.price.GetLongContextMultiplier(usage.PromptTokens)
	q.GetExtraBillingData(usage.ExtraBilling)
	return q.calcQuota(promptTokens, completionTokens, q.price.GetInput()*q.costRatio*inRatio, q.price.GetOutput()*q.costRatio*outRatio, q.costRatio)
}

type ExtraBillingData struct {
	Type      string  `json:"type"`
	CallCount int     `json:"call_count"`
	Price     float64 `json:"price"`
}

func (q *Quota) GetExtraBillingData(extraBilling map[string]types.ExtraBilling) {
	if extraBilling == nil {
		return
	}

	extraBillingData := make(map[string]ExtraBillingData)
	for serviceType, value := range extraBilling {
		extraBillingData[serviceType] = ExtraBillingData{
			Type:      value.Type,
			CallCount: value.CallCount,
			Price:     getDefaultExtraServicePrice(serviceType, q.modelName, value.Type),
		}

	}

	if len(extraBillingData) == 0 {
		return
	}

	q.extraBillingData = extraBillingData
}
