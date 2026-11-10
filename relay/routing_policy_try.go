package relay

// 「试一试」的 HTTP 处理放在 relay 包：它要调决策链路，而 relay 又依赖 controller，不能反过来。

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/middleware"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/relay/relay_util"

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
	// Scoring 实际用的打分方式；Probabilities / Coverage 只在 logprobs 打分时有。
	Scoring       string                               `json:"scoring,omitempty"`
	Probabilities []relay_util.RoutingLabelProbability `json:"probabilities,omitempty"`
	Coverage      float64                              `json:"coverage,omitempty"`
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

	respondTryRoutingPolicy(c, &policy, models, req.Text, false)
}

// respondTryRoutingPolicy runs the draft once per decision model and writes the results.
func respondTryRoutingPolicy(c *gin.Context, policy *model.RoutingPolicy, models []string, text string, bill bool) {
	results := make([]tryRoutingPolicyResult, 0, len(models))
	for _, decisionModel := range models {
		candidate := *policy
		candidate.DecisionModel = decisionModel
		decision := tryRoutingPolicy(c, &candidate, text, bill)
		result := tryRoutingPolicyResult{
			DecisionModel: decisionModel,
			Label:         decision.Label,
			Confidence:    decision.Confidence,
			TargetModel:   decision.TargetModel,
			Reason:        decision.Reason,
			DecisionMs:    decision.DecisionMs,
			Error:         decision.Error,
			Scoring:       decision.Scoring,
			Probabilities: decision.Probabilities,
			Coverage:      decision.Coverage,
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

// routingPolicySample 「试一试」可回放的真实样本：从留存的请求明细里取出首条用户消息。
type routingPolicySample struct {
	LogId       int     `json:"log_id"`
	CreatedAt   int64   `json:"created_at"`
	RequestId   string  `json:"request_id"`
	Label       string  `json:"label"`
	Reason      string  `json:"reason"`
	TargetModel string  `json:"target_model"`
	Confidence  float64 `json:"confidence"`
	Text        string  `json:"text"`
}

// routingPolicySamplePreviewChars 回放样本的首条消息长度上限；再长对调参没有帮助。
const routingPolicySamplePreviewChars = 4000

// RoutingPolicySamplesHandler 返回最近被该策略路由、且留存了请求明细的日志，附首条用户消息文本。
// 没开请求明细留存时为空列表，前端据此提示。
func RoutingPolicySamplesHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeInvalidParams, "invalid routing policy id", nil)
		return
	}
	policy, err := model.GetRoutingPolicyById(id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusNotFound, err)
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	rows, err := model.GetRoutingPolicyLogSamples(policy, limit)
	if err != nil {
		common.APIRespondWithError(c, http.StatusInternalServerError, err)
		return
	}
	samples := make([]routingPolicySample, 0, len(rows))
	for _, row := range rows {
		text := firstUserMessageText([]byte(row.RequestBody), routingPolicySamplePreviewChars)
		if text == "" {
			continue
		}
		samples = append(samples, routingPolicySample{
			LogId: row.LogId, CreatedAt: row.CreatedAt, RequestId: row.RequestId,
			Label: row.Label, Reason: row.Reason, TargetModel: row.TargetModel, Confidence: row.Confidence,
			Text: text,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    samples,
	})
}

// probeRoutingScoringRequest 探测一个决策模型能否做 logprobs 打分。
type probeRoutingScoringRequest struct {
	DecisionModel string `json:"decision_model"`
}

// probeRoutingScoringResult Supported 为 nil 表示探测本身失败（Error 说明原因）。
type probeRoutingScoringResult struct {
	DecisionModel string `json:"decision_model"`
	Supported     *bool  `json:"supported"`
	Error         string `json:"error,omitempty"`
}

// probeRoutingPolicy 用的固定小题：两个选项，答案明确。
var probeRoutingPolicy = model.RoutingPolicy{
	Name:     "probe",
	Question: "What is this request about?",
	Routes: []model.RoutingRoute{
		{Label: "code", Description: "writing or changing source code"},
		{Label: "other", Description: "anything else"},
	},
	Scoring:           model.RoutingScoringLogprobs,
	MaxInputChars:     model.RoutingPolicyDefaultInputChars,
	DecisionTimeoutMs: 10000,
}

// ProbeRoutingScoringHandler 发一次 logprobs 打分请求（不计费），结论写进能力缓存，后台据此提示
// 这个决策模型「支持概率打分 / 仅自报」。结论针对本次选中的渠道。
func ProbeRoutingScoringHandler(c *gin.Context) {
	req := probeRoutingScoringRequest{}
	if err := c.ShouldBindJSON(&req); err != nil || req.DecisionModel == "" {
		common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeInvalidParams, "decision_model is required", nil)
		return
	}
	if model.IsRoutingPolicyModel(req.DecisionModel) {
		common.APIRespondWithError(c, http.StatusBadRequest, model.ErrRoutingPolicyModelIsPolicy("decision_model", req.DecisionModel))
		return
	}
	ensureAdminGroup(c)
	respondProbeRoutingScoring(c, req.DecisionModel, false)
}

func respondProbeRoutingScoring(c *gin.Context, decisionModel string, bill bool) {
	policy := probeRoutingPolicy
	policy.DecisionModel = decisionModel
	result := probeRoutingScoringResult{DecisionModel: decisionModel}
	_, err := classify(c, &policy, "Fix the off-by-one error in the pagination helper.", bill)
	switch {
	case err == nil:
		supported := true
		result.Supported = &supported
	case errors.Is(err, errNoLogprobs):
		supported := false
		result.Supported = &supported
	default:
		result.Error = err.Error()
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

// ---- Organization console ----
//
// "Try it" and the probe call the decision model for real. On the platform console that is the
// admin's own site and goes unbilled; in an organization it would be free model access, so these
// calls are charged to the organization like its API keys are (shadow account, the operator's member
// budget and the team budget) and logged with routing_try=true, which keeps them out of the stats.

// organizationFromContext returns the organization OrgAuth put on the request.
func organizationFromContext(c *gin.Context) *model.Organization {
	org, _ := c.MustGet(middleware.OrgContextKey).(*model.Organization)
	return org
}

// useOrganizationBilling makes the console request look like a call with an organization API key
// owned by the operator, so the decision call is billed and guarded like one. It has no API key, so
// it runs with unlimited key quota and is bounded by the account, member and team budgets.
func useOrganizationBilling(c *gin.Context, org *model.Organization) error {
	group, err := model.CacheGetUserGroup(org.ShadowUserId)
	if err != nil {
		return err
	}
	operator := c.GetInt("id")
	c.Set("id", org.ShadowUserId)
	c.Set("token_created_by", operator)
	c.Set("token_id", 0)
	c.Set("token_unlimited_quota", true)
	c.Set("group", group)
	c.Set("token_group", group)
	c.Set(relay_util.RoutingTryContextKey, true)
	return nil
}

// OrgTryRoutingPolicyHandler is TryRoutingPolicyHandler for the organization console.
func OrgTryRoutingPolicyHandler(c *gin.Context) {
	org := organizationFromContext(c)
	req := tryRoutingPolicyRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeInvalidParams, err.Error(), nil)
		return
	}
	policy := req.Policy
	if policy.Name == "" {
		policy.Name = "draft"
	}
	// Only a saved policy of this organization may lend its id to the billing log.
	if policy.Id > 0 {
		if _, err := model.GetOrganizationRoutingPolicyById(org.Id, policy.Id); err != nil {
			policy.Id = 0
		}
	}
	policy.OrganizationId = org.Id
	policy.Normalize()
	if err := policy.Validate(model.IsRoutingPolicyModelFor(org.Id)); err != nil {
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
	for _, decisionModel := range models {
		candidate := policy
		candidate.DecisionModel = decisionModel
		if err := candidate.ValidateOrganizationModels(org.Id); err != nil {
			common.APIRespondWithError(c, http.StatusBadRequest, err)
			return
		}
	}
	if err := useOrganizationBilling(c, org); err != nil {
		common.APIRespondWithError(c, http.StatusInternalServerError, err)
		return
	}
	respondTryRoutingPolicy(c, &policy, models, req.Text, config.RoutingDecisionBillingEnabled)
}

// OrgProbeRoutingScoringHandler is ProbeRoutingScoringHandler for the organization console.
func OrgProbeRoutingScoringHandler(c *gin.Context) {
	org := organizationFromContext(c)
	req := probeRoutingScoringRequest{}
	if err := c.ShouldBindJSON(&req); err != nil || req.DecisionModel == "" {
		common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeInvalidParams, "decision_model is required", nil)
		return
	}
	if model.IsRoutingPolicyModelFor(org.Id)(req.DecisionModel) {
		common.APIRespondWithError(c, http.StatusBadRequest, model.ErrRoutingPolicyModelIsPolicy("decision_model", req.DecisionModel))
		return
	}
	if err := model.CheckOrganizationRoutingModel(org.Id, "decision_model", req.DecisionModel); err != nil {
		common.APIRespondWithError(c, http.StatusBadRequest, err)
		return
	}
	if err := useOrganizationBilling(c, org); err != nil {
		common.APIRespondWithError(c, http.StatusInternalServerError, err)
		return
	}
	respondProbeRoutingScoring(c, req.DecisionModel, config.RoutingDecisionBillingEnabled)
}
