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
	return q.calcQuotaWith(&q.price, promptTokens, completionTokens, inputRatio, outputRatio, ratio)
}

// calcQuotaWith is calcQuota for a price list other than the catalog price (the channel cost).
func (q *Quota) calcQuotaWith(price *model.Price, promptTokens, completionTokens int, inputRatio, outputRatio, ratio float64) (quota int) {
	minutes := price.Type == model.MinutesPriceType
	if price.Type == model.TimesPriceType {
		quota = common.QuotaFromFloat(1000 * inputRatio)
	} else if minutes {
		quota = minutesQuota(q.audioSeconds, inputRatio)
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
	if minutes {
		// Minutes prices bill the audio length; no audio means nothing was transcribed.
		if q.audioSeconds <= 0 {
			quota = 0
		}
	} else if totalTokens == 0 {
		// in this case, must be some error happened
		// we cannot just return, because we may have to return the pre-consumed quota
		quota = 0
	}

	// 空回复计费闸对按次计费（times）属于误伤：按次语义是"调用成功即全额收"，
	// 与 completion token 无关（Lyria 等音乐模型成功返回音频时 completionTokens 常为 0）。
	// 仅对 token 计费类型生效；闸1（totalTokens==0，上游未成功返回）对两类都保留。
	if price.Type != model.TimesPriceType && !minutes && !config.EmptyResponseBillingEnabled && completionTokens == 0 {
		quota = 0
	}

	return quota
}

// minutesQuota prices audio of the given length at a per-minute rate, rounding the length up
// to whole seconds: 61.2 s bills as 62 s.
func minutesQuota(seconds, perMinuteRatio float64) int {
	if seconds <= 0 || perMinuteRatio <= 0 {
		return 0
	}
	return common.QuotaFromFloat(math.Ceil(1000 * perMinuteRatio * math.Ceil(seconds) / 60))
}

// minutesPreConsumeQuota reserves the probed length of the uploaded audio, or the default
// length when the file could not be probed.
func (q *Quota) minutesPreConsumeQuota() int {
	return q.minutesPreConsumeQuotaAt(q.inputRatio)
}

func (q *Quota) minutesPreConsumeQuotaAt(perMinuteRatio float64) int {
	seconds := q.audioSecondsEstimate
	if seconds <= 0 {
		seconds = config.DefaultAudioSeconds
	}
	return minutesQuota(seconds, perMinuteRatio)
}

// preConsumeQuotaWith estimates the quota to reserve before the request at the given price list
// and per-unit ratios (group ratio included). The estimate covers the declared output:
// estimating from the input alone left requests with pricey output costing a hundred times the
// reserve.
func (q *Quota) preConsumeQuotaWith(price *model.Price, inputRatio, outputRatio float64) int {
	switch {
	case price.Type == model.TimesPriceType:
		return common.QuotaFromFloat(1000 * inputRatio)
	case price.Type == model.MinutesPriceType:
		return q.minutesPreConsumeQuotaAt(inputRatio)
	case price.Input != 0 || price.Output != 0:
		return common.QuotaFromFloat(float64(q.promptTokens)*inputRatio+
			float64(q.expectedOutputTokens())*outputRatio) + config.PreConsumedQuota
	}
	return 0
}

// 获取计算的 token 数
func (q *Quota) getComputeTokensByUsage(usage *types.Usage) (promptTokens, completionTokens int) {
	return getComputeTokensWith(&q.price, usage)
}

// getComputeTokensWith converts the usage to billed prompt and completion tokens with the extra
// ratios (cache, audio, reasoning) of the given price list.
func getComputeTokensWith(price *model.Price, usage *types.Usage) (promptTokens, completionTokens int) {
	promptTokens = usage.PromptTokens
	completionTokens = usage.CompletionTokens

	extraTokens := usage.GetExtraTokens()

	for key, value := range extraTokens {
		extraRatio := price.GetExtraRatio(key)
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
	inRatio, outRatio := q.price.RequestMultipliers(usage.PromptTokens, q.startedAt)
	// 上游按处理档位（OpenAI fast / flex、Claude fast、US 区推理）整体调价时，token 部分同比例计费。
	tier := usage.TierMultiplier()
	q.GetExtraBillingData(usage.ExtraBilling)
	q.audioSeconds = usage.AudioSeconds
	quota = q.calcQuota(promptTokens, completionTokens, q.inputRatio*inRatio*tier, q.outputRatio*outRatio*tier, q.groupRatio)
	q.baseQuota = q.calcQuota(promptTokens, completionTokens, q.price.GetInput()*inRatio*tier, q.price.GetOutput()*outRatio*tier, 1)
	return quota
}

// channelCostQuota prices the usage at the channel cost (fee included, no group ratio).
func (q *Quota) channelCostQuota(usage *types.Usage) int {
	price := q.costPrice
	promptTokens, completionTokens := getComputeTokensWith(price, usage)
	inRatio, outRatio := price.RequestMultipliers(usage.PromptTokens, q.startedAt)
	tier := usage.TierMultiplier()
	q.GetExtraBillingData(usage.ExtraBilling)
	q.audioSeconds = usage.AudioSeconds
	return q.calcQuotaWith(price, promptTokens, completionTokens, price.GetInput()*inRatio*tier, price.GetOutput()*outRatio*tier, 1+q.feeRate)
}

// requestCost is the request's cost for the cost and margin figures, and where it comes from:
// what the upstream says it charged, with the channel's fee, when it is one whose charge is known
// to be USD (upstreamCostTypes); else the channel's list price with its fee (channelCostQuota).
// It is never charged. The upstream's own figure follows where the request actually went (a
// gateway's fallback host, a long-context tier, a cache hit) where the list price is an estimate.
//
// A model without a price of its own (its unpriced policy charges a default) has no channel
// price to compare with and falls back to the catalog price at the channel's cost ratio.
//
// An empty source means the cost is not known: no price list, no cost ratio and not a vendor
// channel. The cost is then 0, and the log keeps the empty source so the request is told apart
// from one that cost nothing and stays out of the profit figures (model.GetRpmTpmStatistics).
func (q *Quota) requestCost(usage *types.Usage) (costQuota int, source string) {
	if usage != nil && usage.UpstreamCost > 0 && upstreamCostTypes[q.channelType] {
		return int(math.Round(usage.UpstreamCost * (1 + q.feeRate) * config.QuotaPerUnit)), model.CostSourceUpstream
	}
	if q.costPrice != nil {
		source = q.costSource
		if source == "" {
			source = model.CostSourceChannelPrice
		}
		return q.channelCostQuota(usage), source
	}
	if q.costRatio > 0 {
		return q.costQuotaAt(usage, q.costRatio), model.CostSourceCostRatio
	}
	return 0, ""
}

// GetCostQuotaByUsage is the cost of requestCost: 0 when the cost is not known.
func (q *Quota) GetCostQuotaByUsage(usage *types.Usage) int {
	costQuota, _ := q.requestCost(usage)
	return costQuota
}

// costQuotaAt prices the usage from the price table at the given ratio, without the group ratio.
func (q *Quota) costQuotaAt(usage *types.Usage, ratio float64) int {
	promptTokens, completionTokens := q.getComputeTokensByUsage(usage)
	inRatio, outRatio := q.price.RequestMultipliers(usage.PromptTokens, q.startedAt)
	tier := usage.TierMultiplier()
	q.GetExtraBillingData(usage.ExtraBilling)
	q.audioSeconds = usage.AudioSeconds
	return q.calcQuota(promptTokens, completionTokens, q.price.GetInput()*ratio*inRatio*tier, q.price.GetOutput()*ratio*outRatio*tier, ratio)
}

// ExpectedUpstreamCost is the gateway's estimate, in USD, of what the upstream charges for the
// usage: the price table at the channel's cost ratio, or at 1 (the listed price) when the
// channel has none. It is logged next to the cost the upstream reports, to show the difference.
//
// It is the channel's list price, without the fee: upstreams deduct the list price from the
// account's credits (the fee was paid when the credits were bought), so this is what their own
// per-request and daily figures are compared with. Without a known list price, the catalog price.
//
// Unlike the request's cost (requestCost), it is always a figure: a guess at the catalog price is
// still worth comparing with an upstream's bill, while a guessed cost would pass for a known one
// in the profit figures.
func (q *Quota) ExpectedUpstreamCost(usage *types.Usage) float64 {
	if q.listPrice != nil {
		price := q.listPrice
		promptTokens, completionTokens := getComputeTokensWith(price, usage)
		inRatio, outRatio := price.RequestMultipliers(usage.PromptTokens, q.startedAt)
		tier := usage.TierMultiplier()
		q.GetExtraBillingData(usage.ExtraBilling)
		q.audioSeconds = usage.AudioSeconds
		listQuota := q.calcQuotaWith(price, promptTokens, completionTokens, price.GetInput()*inRatio*tier, price.GetOutput()*outRatio*tier, 1)
		return float64(listQuota) / float64(config.QuotaPerUnit)
	}
	ratio := q.costRatio
	if ratio <= 0 {
		ratio = 1
	}
	return float64(q.costQuotaAt(usage, ratio)) / float64(config.QuotaPerUnit)
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
