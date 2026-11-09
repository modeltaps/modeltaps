package relay_util

import (
	"fmt"
	"math"
	"strconv"

	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
)

// 自动路由决策的类型放在这里而不是 relay 包：计费与日志（本包）要读它，relay 又依赖本包。

// 决策结果的原因，进日志 metadata 与响应头。
const (
	RoutingReasonClassified     = "classified"      // 分类成功且过阈值
	RoutingReasonPinned         = "pinned"          // 命中会话固定记录
	RoutingReasonBelowThreshold = "below_threshold" // 分类成功但最高概率低于阈值
	RoutingReasonDecisionError  = "decision_error"  // 决策调用失败 / 超时 / 答非所问
	RoutingReasonEmptyInput     = "empty_input"     // 首条用户消息没有文本
	RoutingReasonAuxiliary      = "auxiliary"       // 编码 Agent 的旁路请求（如 Claude Code 生成标题），会话还没固定时直接走兜底
	// RoutingReasonTargetUnavailable the chosen target of an organization policy is no longer usable
	// (hidden in the catalog), so the request went to the fallback model.
	RoutingReasonTargetUnavailable = "target_unavailable"
)

// RoutingPolicyIdContextKey gin context key holding the id of the policy a decision call belongs to.
const RoutingPolicyIdContextKey = "routing_policy_id"

// RoutingTryContextKey marks decision calls made by "Try it" in the organization console.
const RoutingTryContextKey = "routing_try"

// RoutingDecisionContextKey gin context 里本次请求的决策结果（*RoutingDecision）。
const RoutingDecisionContextKey = "routing_decision"

// RoutingDecisionCallContextKey 标记一个 gin context 是决策调用本身（而非用户请求），其消费日志据此打标。
const RoutingDecisionCallContextKey = "routing_decision_call"

// RoutingRouteHeader 响应头，告诉客户端本次请求被路由到了哪里。
const RoutingRouteHeader = "X-Modeltaps-Route"

// RoutingLabelProbability 一个标签的概率（logprobs 打分时才有）。
type RoutingLabelProbability struct {
	Label       string  `json:"label"`
	Probability float64 `json:"probability"`
}

// RoutingDecision 一次请求的路由决策。
type RoutingDecision struct {
	Policy string
	// PolicyId tells policies with the same name apart (an organization may override a platform policy).
	PolicyId    int
	Label       string
	TargetModel string
	Confidence  float64
	Reason      string
	DecisionMs  int64
	// Scoring 实际用的打分方式（logprobs / self_reported），没做分类时为空。
	Scoring string
	// Probabilities logprobs 打分时每个标签的概率（按概率降序、已归一），Coverage 是归一前
	// 所有选项字母的概率之和。自报打分时为空。
	Probabilities []RoutingLabelProbability
	Coverage      float64
	// DecisionUsage 决策调用的用量，命中固定记录或未调用时为 nil。
	DecisionUsage *types.Usage
	// Error 决策失败的原因文本（只进系统日志与后台「试一试」，不进消费日志）。
	Error string
}

// HeaderValue 组成 X-Modeltaps-Route 的值：policy=…; label=…; model=…; confidence=…; reason=…。
// 标签为空（兜底且没有标签）时省略 label。
func (d *RoutingDecision) HeaderValue() string {
	value := "policy=" + d.Policy
	if d.Label != "" {
		value += "; label=" + d.Label
	}
	value += "; model=" + d.TargetModel
	if d.scored() {
		value += "; confidence=" + strconv.FormatFloat(d.Confidence, 'f', 2, 64)
		if d.Scoring != "" {
			value += "; scoring=" + d.Scoring
		}
	}
	return value + "; reason=" + d.Reason
}

// LogMeta 写进消费日志 metadata 的字段。
func (d *RoutingDecision) LogMeta(meta map[string]any) {
	meta["routing_policy"] = d.Policy
	if d.PolicyId > 0 {
		meta["routing_policy_id"] = d.PolicyId
	}
	meta["routing_reason"] = d.Reason
	meta["routing_target"] = d.TargetModel
	if d.Label != "" {
		meta["routing_label"] = d.Label
	}
	if d.scored() {
		meta["routing_confidence"] = d.Confidence
		if d.Scoring != "" {
			meta["routing_scoring"] = d.Scoring
		}
		if len(d.Probabilities) > 0 {
			probabilities := make([]RoutingLabelProbability, len(d.Probabilities))
			for i, p := range d.Probabilities {
				probabilities[i] = RoutingLabelProbability{Label: p.Label, Probability: roundProbability(p.Probability)}
			}
			meta["routing_probabilities"] = probabilities
			meta["routing_coverage"] = roundProbability(d.Coverage)
		}
	}
	if d.DecisionMs > 0 || d.scored() {
		meta["routing_decision_ms"] = d.DecisionMs
	}
}

// scored 决策模型给出了一个有效标签（无论是否过阈值）。
func (d *RoutingDecision) scored() bool {
	return d.Reason == RoutingReasonClassified || d.Reason == RoutingReasonBelowThreshold
}

func roundProbability(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func (d *RoutingDecision) String() string {
	return fmt.Sprintf("policy=%s label=%s target=%s confidence=%.2f scoring=%s reason=%s decision_ms=%d",
		d.Policy, d.Label, d.TargetModel, d.Confidence, d.Scoring, d.Reason, d.DecisionMs)
}

// RoutingDecisionFromContext 取本次请求的决策，没有返回 nil。
func RoutingDecisionFromContext(c *gin.Context) *RoutingDecision {
	if c == nil {
		return nil
	}
	decision, ok := utils.GetGinValue[*RoutingDecision](c, RoutingDecisionContextKey)
	if !ok {
		return nil
	}
	return decision
}
