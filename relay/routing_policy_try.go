package relay

// 「试一试」的 HTTP 处理放在 relay 包：它要调决策链路，而 relay 又依赖 controller，不能反过来。

import (
	"net/http"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// tryRoutingPolicyRequest 「试一试」入参：一份策略（可以是未保存的草稿）、一段首条消息，
// 以及可选的多个候选决策模型（为空时用策略自己的）。
type tryRoutingPolicyRequest struct {
	Policy         model.RoutingPolicy `json:"policy"`
	Text           string              `json:"text"`
	DecisionModels []string            `json:"decision_models"`
}

// tryRoutingPolicyResult 单个决策模型的结果；Error 非空表示决策调用失败（结果为兜底）。
type tryRoutingPolicyResult struct {
	DecisionModel string  `json:"decision_model"`
	Label         string  `json:"label"`
	Confidence    float64 `json:"confidence"`
	TargetModel   string  `json:"target_model"`
	Reason        string  `json:"reason"`
	DecisionMs    int64   `json:"decision_ms"`
	PromptTokens  int     `json:"prompt_tokens"`
	OutputTokens  int     `json:"completion_tokens"`
	Error         string  `json:"error,omitempty"`
}

// routingPolicyTryMaxModels 一次最多并列比较的决策模型数。
const routingPolicyTryMaxModels = 4

// TryRoutingPolicy 只跑决策，不转发、不计费、不写会话固定记录；多个候选决策模型逐个跑。
func TryRoutingPolicyHandler(c *gin.Context) {
	req := tryRoutingPolicyRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeInvalidParams, err.Error(), nil)
		return
	}
	policy := req.Policy
	if policy.Name == "" {
		// 草稿可能还没起名；校验要名字，用一个合法占位，不落库。
		policy.Name = "draft"
	}
	policy.Normalize()
	if err := policy.Validate(model.IsRoutingPolicyModel); err != nil {
		common.APIRespondWithError(c, http.StatusBadRequest, err)
		return
	}
	models := req.DecisionModels
	if len(models) == 0 {
		models = []string{policy.DecisionModel}
	}
	if len(models) > routingPolicyTryMaxModels {
		models = models[:routingPolicyTryMaxModels]
	}

	results := make([]tryRoutingPolicyResult, 0, len(models))
	for _, decisionModel := range models {
		candidate := policy
		candidate.DecisionModel = decisionModel
		decision := tryRoutingPolicy(c, &candidate, req.Text)
		result := tryRoutingPolicyResult{
			DecisionModel: decisionModel,
			Label:         decision.Label,
			Confidence:    decision.Confidence,
			TargetModel:   decision.TargetModel,
			Reason:        decision.Reason,
			DecisionMs:    decision.DecisionMs,
			Error:         decision.Error,
		}
		if decision.DecisionUsage != nil {
			result.PromptTokens = decision.DecisionUsage.PromptTokens
			result.OutputTokens = decision.DecisionUsage.CompletionTokens
		}
		results = append(results, result)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    results,
	})
}
