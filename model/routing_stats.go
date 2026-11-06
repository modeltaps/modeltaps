package model

import (
	"math"
	"sort"
	"time"

	"github.com/modeltaps/modeltaps/common"

	"gorm.io/gorm"
)

// 自动路由策略的效果统计，全部从消费日志的 metadata 聚合（见 relay_util.RoutingDecision.LogMeta）。
// 目标模型那条日志带 routing_policy / routing_label / routing_reason / routing_target / routing_decision_ms；
// 决策调用那条带 routing_decision=true + routing_policy。

// metadataTextExpr 返回按方言提取 metadata 文本字段的 SQL 表达式。
func metadataTextExpr(key string) string {
	switch {
	case common.UsingPostgreSQL:
		return "(metadata ->> '" + key + "')"
	case common.UsingSQLite:
		return "JSON_EXTRACT(metadata, '$." + key + "')"
	default:
		return "JSON_UNQUOTE(JSON_EXTRACT(metadata, '$." + key + "'))"
	}
}

// metadataNumberExpr 返回按方言提取 metadata 数值字段的 SQL 表达式（缺失为 NULL）。
func metadataNumberExpr(key string) string {
	if common.UsingPostgreSQL {
		return "(metadata ->> '" + key + "')::numeric"
	}
	return "JSON_EXTRACT(metadata, '$." + key + "')"
}

// RoutingLabelStat 一个标签（或兜底）的命中情况。
type RoutingLabelStat struct {
	Label       string `json:"label"`
	TargetModel string `json:"target_model"`
	Requests    int64  `json:"requests"`
	Quota       int64  `json:"quota"`
}

// RoutingPolicyStats 一条策略在统计窗口内的效果。
type RoutingPolicyStats struct {
	Days     int   `json:"days"`
	Requests int64 `json:"requests"`
	// ByReason 按决策原因计数（classified / pinned / below_threshold / decision_error / empty_input / auxiliary）。
	ByReason map[string]int64 `json:"by_reason"`
	// FallbackRequests 实际由兜底应答的请求：没有标签的那些（低于阈值、决策失败、空输入，以及固定住的兜底）。
	FallbackRequests int64              `json:"fallback_requests"`
	Labels           []RoutingLabelStat `json:"labels"`
	// AvgDecisionMs 做过分类的请求的平均决策耗时（固定命中的不算）。
	AvgDecisionMs float64 `json:"avg_decision_ms"`
	DecisionCalls int64   `json:"decision_calls"`
	DecisionQuota int64   `json:"decision_quota"`
	// ActualQuota 被路由请求实际扣费；FallbackQuota 是按「全部走兜底模型」估算的费用；
	// SavedQuota = FallbackQuota - ActualQuota - DecisionQuota，可以为负。估算只按 token 与兜底模型
	// 的基础价、沿用各请求的分组倍率，不计缓存折扣与档位倍率。
	ActualQuota   int64 `json:"actual_quota"`
	FallbackQuota int64 `json:"fallback_quota"`
	SavedQuota    int64 `json:"saved_quota"`
	// FallbackPriceKnown 兜底模型有配置价格时为 true；没有时 FallbackQuota / SavedQuota 无意义，前端不显示。
	FallbackPriceKnown bool `json:"fallback_price_known"`
	// ByScoring 做过分类的请求按打分方式计数（logprobs / self_reported；更早的日志没有这个字段，不计入）。
	ByScoring map[string]int64 `json:"by_scoring"`
	// AvgTopProbability logprobs 打分的请求里最高标签概率的平均值，越低说明标签越难区分；没有这类请求时为 0。
	AvgTopProbability float64 `json:"avg_top_probability"`
}

// routedLogsQuery 统计窗口内走了该策略的目标模型日志（不含决策调用那条）。
func routedLogsQuery(policyName string, since int64) *gorm.DB {
	return DB.Table("logs").
		Where("type = ? AND created_at >= ?", LogTypeConsume, since).
		Where(metadataTextExpr("routing_policy")+" = ?", policyName).
		Where(metadataTextExpr("routing_decision") + " IS NULL")
}

// GetRoutingPolicyStats 聚合最近 days 天的策略效果。
func GetRoutingPolicyStats(policy *RoutingPolicy, days int) (*RoutingPolicyStats, error) {
	if days <= 0 {
		days = 7
	}
	since := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
	stats := &RoutingPolicyStats{Days: days, ByReason: map[string]int64{}, Labels: []RoutingLabelStat{}, ByScoring: map[string]int64{}}

	// 原因分布
	var reasons []struct {
		Reason string
		Count  int64
	}
	if err := routedLogsQuery(policy.Name, since).
		Select(metadataTextExpr("routing_reason") + " as reason, count(*) as count").
		Group(metadataTextExpr("routing_reason")).Scan(&reasons).Error; err != nil {
		return nil, err
	}
	for _, r := range reasons {
		stats.ByReason[r.Reason] += r.Count
		stats.Requests += r.Count
	}

	// 标签分布：没有标签的行归为兜底
	var labels []struct {
		Label  string
		Target string
		Count  int64
		Quota  int64
	}
	if err := routedLogsQuery(policy.Name, since).
		Select(metadataTextExpr("routing_label") + " as label, " + metadataTextExpr("routing_target") + " as target, count(*) as count, COALESCE(sum(quota), 0) as quota").
		Group(metadataTextExpr("routing_label") + ", " + metadataTextExpr("routing_target")).Scan(&labels).Error; err != nil {
		return nil, err
	}
	for _, l := range labels {
		if l.Label == "" {
			stats.FallbackRequests += l.Count
		}
		stats.Labels = append(stats.Labels, RoutingLabelStat{Label: l.Label, TargetModel: l.Target, Requests: l.Count, Quota: l.Quota})
	}
	sort.Slice(stats.Labels, func(i, j int) bool {
		if stats.Labels[i].Requests != stats.Labels[j].Requests {
			return stats.Labels[i].Requests > stats.Labels[j].Requests
		}
		return stats.Labels[i].Label < stats.Labels[j].Label
	})

	// 平均决策耗时：只算做过分类的行
	var avg struct{ Avg *float64 }
	if err := routedLogsQuery(policy.Name, since).
		Where(metadataNumberExpr("routing_decision_ms") + " IS NOT NULL").
		Select("AVG(" + metadataNumberExpr("routing_decision_ms") + ") as avg").Scan(&avg).Error; err != nil {
		return nil, err
	}
	if avg.Avg != nil {
		stats.AvgDecisionMs = math.Round(*avg.Avg*10) / 10
	}

	// 打分方式分布与 logprobs 打分的平均最高概率
	var scorings []struct {
		Scoring string
		Count   int64
		AvgTop  *float64
	}
	if err := routedLogsQuery(policy.Name, since).
		Where(metadataTextExpr("routing_scoring") + " IS NOT NULL").
		Select(metadataTextExpr("routing_scoring") + " as scoring, count(*) as count, AVG(" + metadataNumberExpr("routing_confidence") + ") as avg_top").
		Group(metadataTextExpr("routing_scoring")).Scan(&scorings).Error; err != nil {
		return nil, err
	}
	for _, sc := range scorings {
		if sc.Scoring == "" {
			continue
		}
		stats.ByScoring[sc.Scoring] += sc.Count
		if sc.Scoring == RoutingScoringLogprobs && sc.AvgTop != nil {
			stats.AvgTopProbability = math.Round(*sc.AvgTop*1000) / 1000
		}
	}

	// 决策调用
	var decision struct {
		Count int64
		Quota int64
	}
	if err := DB.Table("logs").
		Where("type = ? AND created_at >= ?", LogTypeConsume, since).
		Where(metadataTextExpr("routing_policy")+" = ?", policy.Name).
		Where(metadataTextExpr("routing_decision") + " IS NOT NULL").
		Select("count(*) as count, COALESCE(sum(quota), 0) as quota").Scan(&decision).Error; err != nil {
		return nil, err
	}
	stats.DecisionCalls, stats.DecisionQuota = decision.Count, decision.Quota

	// 估算节省：按（实际模型, 分组倍率）分桶求 token 和，用兜底模型价格重算
	var buckets []struct {
		ModelName        string
		GroupRatio       *float64
		PromptTokens     int64
		CompletionTokens int64
		Quota            int64
	}
	if err := routedLogsQuery(policy.Name, since).
		Select("model_name, " + metadataNumberExpr("group_ratio") + " as group_ratio, COALESCE(sum(prompt_tokens), 0) as prompt_tokens, COALESCE(sum(completion_tokens), 0) as completion_tokens, COALESCE(sum(quota), 0) as quota").
		Group("model_name, " + metadataNumberExpr("group_ratio")).Scan(&buckets).Error; err != nil {
		return nil, err
	}
	fallbackPrice := fallbackPriceFor(policy.FallbackModel)
	stats.FallbackPriceKnown = fallbackPrice != nil
	for _, b := range buckets {
		stats.ActualQuota += b.Quota
		if fallbackPrice == nil {
			continue
		}
		ratio := 1.0
		if b.GroupRatio != nil && *b.GroupRatio > 0 {
			ratio = *b.GroupRatio
		}
		if b.ModelName == policy.FallbackModel {
			stats.FallbackQuota += b.Quota
			continue
		}
		stats.FallbackQuota += estimateQuota(fallbackPrice, b.PromptTokens, b.CompletionTokens, ratio)
	}
	if fallbackPrice != nil {
		stats.SavedQuota = stats.FallbackQuota - stats.ActualQuota - stats.DecisionQuota
	}
	return stats, nil
}

// fallbackPriceFor 取兜底模型的价格；价格实例未初始化或未配置时返回 nil（不估算）。
func fallbackPriceFor(modelName string) *Price {
	if PricingInstance == nil {
		return nil
	}
	price := PricingInstance.GetPrice(modelName)
	if price == nil || price.Unconfigured {
		return nil
	}
	return price
}

// estimateQuota 按 token 价估算一桶请求若由该模型应答的费用（与 calcQuota 的 tokens 分支同式）。
func estimateQuota(price *Price, promptTokens, completionTokens int64, groupRatio float64) int64 {
	if price.Type == TimesPriceType {
		return int64(common.QuotaFromFloat(1000 * price.GetInput() * groupRatio))
	}
	return int64(common.QuotaFromFloat(math.Ceil(float64(promptTokens)*price.GetInput()*groupRatio + float64(completionTokens)*price.GetOutput()*groupRatio)))
}

// RoutingPolicyLogSample 一条被路由请求的样本，供后台「试一试」回放；RequestBody 来自 log_details，
// 只有开了请求明细留存的日志才有。
type RoutingPolicyLogSample struct {
	LogId       int     `json:"log_id"`
	CreatedAt   int64   `json:"created_at"`
	RequestId   string  `json:"request_id"`
	Label       string  `json:"label"`
	Reason      string  `json:"reason"`
	TargetModel string  `json:"target_model"`
	Confidence  float64 `json:"confidence"`
	RequestBody string  `json:"-"`
}

// GetRoutingPolicyLogSamples 返回最近 limit 条带请求明细的被路由日志（新的在前）。
func GetRoutingPolicyLogSamples(policyName string, limit int) ([]*RoutingPolicyLogSample, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var rows []struct {
		LogId       int
		CreatedAt   int64
		RequestId   string
		Label       string
		Reason      string
		Target      string
		Confidence  *float64
		RequestBody string
	}
	err := DB.Table("logs").
		Joins("JOIN log_details ON log_details.log_id = logs.id").
		Where("logs.type = ?", LogTypeConsume).
		Where(metadataTextExpr("routing_policy")+" = ?", policyName).
		Where(metadataTextExpr("routing_decision") + " IS NULL").
		Select("logs.id as log_id, logs.created_at as created_at, logs.request_id as request_id, " +
			metadataTextExpr("routing_label") + " as label, " + metadataTextExpr("routing_reason") + " as reason, " +
			metadataTextExpr("routing_target") + " as target, " + metadataNumberExpr("routing_confidence") + " as confidence, " +
			"log_details.request_body as request_body").
		Order("logs.id desc").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	samples := make([]*RoutingPolicyLogSample, 0, len(rows))
	for _, r := range rows {
		sample := &RoutingPolicyLogSample{LogId: r.LogId, CreatedAt: r.CreatedAt, RequestId: r.RequestId, Label: r.Label, Reason: r.Reason, TargetModel: r.Target, RequestBody: r.RequestBody}
		if r.Confidence != nil {
			sample.Confidence = *r.Confidence
		}
		samples = append(samples, sample)
	}
	return samples, nil
}
