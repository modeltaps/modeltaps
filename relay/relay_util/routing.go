package relay_util

import (
	"fmt"
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
)

// RoutingDecisionContextKey gin context 里本次请求的决策结果（*RoutingDecision）。
const RoutingDecisionContextKey = "routing_decision"

// RoutingDecisionCallContextKey 标记一个 gin context 是决策调用本身（而非用户请求），其消费日志据此打标。
const RoutingDecisionCallContextKey = "routing_decision_call"

// RoutingRouteHeader 响应头，告诉客户端本次请求被路由到了哪里。
const RoutingRouteHeader = "X-Modeltaps-Route"

// RoutingDecision 一次请求的路由决策。
type RoutingDecision struct {
	Policy      string
	Label       string
	TargetModel string
	Confidence  float64
	Reason      string
	DecisionMs  int64
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
	if d.Reason == RoutingReasonClassified || d.Reason == RoutingReasonBelowThreshold {
		value += "; confidence=" + strconv.FormatFloat(d.Confidence, 'f', 2, 64)
	}
	return value + "; reason=" + d.Reason
}

// LogMeta 写进消费日志 metadata 的字段。
func (d *RoutingDecision) LogMeta(meta map[string]any) {
	meta["routing_policy"] = d.Policy
	meta["routing_reason"] = d.Reason
	meta["routing_target"] = d.TargetModel
	if d.Label != "" {
		meta["routing_label"] = d.Label
	}
	if d.Reason == RoutingReasonClassified || d.Reason == RoutingReasonBelowThreshold {
		meta["routing_confidence"] = d.Confidence
	}
	if d.DecisionMs > 0 || d.Reason == RoutingReasonClassified || d.Reason == RoutingReasonBelowThreshold {
		meta["routing_decision_ms"] = d.DecisionMs
	}
}

func (d *RoutingDecision) String() string {
	return fmt.Sprintf("policy=%s label=%s target=%s confidence=%.2f reason=%s decision_ms=%d",
		d.Policy, d.Label, d.TargetModel, d.Confidence, d.Reason, d.DecisionMs)
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
