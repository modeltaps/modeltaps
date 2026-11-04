package relay

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/session"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	providersBase "github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/relay/relay_util"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// 自动路由的请求路径：getProvider 解析出目录主名之后、选渠道之前，把策略名换成一个目标模型。
// 一次请求只决策一次（结果放进 gin context），重试循环再次 setProvider 时直接复用，
// 既不多扣决策费，也不会换目标。决策永不拒绝请求：任何失败都走兜底模型。

// RoutingDecision 与原因常量定义在 relay_util（计费与日志也要用），这里取别名便于阅读。
type RoutingDecision = relay_util.RoutingDecision

const (
	RoutingReasonClassified     = relay_util.RoutingReasonClassified
	RoutingReasonPinned         = relay_util.RoutingReasonPinned
	RoutingReasonBelowThreshold = relay_util.RoutingReasonBelowThreshold
	RoutingReasonDecisionError  = relay_util.RoutingReasonDecisionError
	RoutingReasonEmptyInput     = relay_util.RoutingReasonEmptyInput
	RoutingDecisionContextKey   = relay_util.RoutingDecisionContextKey
)

// ClassifyResult 决策后端的输出：选中的标签与它的置信度（0~1）。
type ClassifyResult struct {
	Label      string
	Confidence float64
	Usage      *types.Usage
}

// classifyFunc 把一段文本按策略分类。默认实现调决策模型，测试替换成假的。
type classifyFunc func(c *gin.Context, policy *model.RoutingPolicy, text string) (*ClassifyResult, error)

// classifyRoute 为 nil 时用 classifyWithDecisionModel；不能在包级直接赋值，那会和 getProvider 形成初始化环。
var classifyRoute classifyFunc

func classify(c *gin.Context, policy *model.RoutingPolicy, text string) (*ClassifyResult, error) {
	if classifyRoute != nil {
		return classifyRoute(c, policy, text)
	}
	return classifyWithDecisionModel(c, policy, text)
}

// routingPolicyNow 便于测试控制耗时。
var routingPolicyNow = time.Now

// resolveRoutingPolicy 把策略名解析为目标模型；不是策略（或策略已停用）时原样返回。
// 同一请求内第二次调用直接返回缓存的决策。
func resolveRoutingPolicy(c *gin.Context, modelName string) string {
	if cached, ok := utils.GetGinValue[*RoutingDecision](c, RoutingDecisionContextKey); ok && cached != nil && cached.Policy == modelName {
		return cached.TargetModel
	}
	policy := model.LookupRoutingPolicy(modelName)
	if policy == nil || !policy.Enabled {
		return modelName
	}

	decision := decideRoute(c, policy)
	c.Set(RoutingDecisionContextKey, decision)
	c.Set("routing_policy", policy.Name)
	// 用户请求的是策略名；别名改写已经填过 requested_model 时保留用户原文。
	if c.GetString("requested_model") == "" {
		c.Set("requested_model", policy.Name)
	}
	// 响应头在选渠道前写，流式与非流式都能带上。
	c.Header(relay_util.RoutingRouteHeader, decision.HeaderValue())
	logger.LogInfo(c.Request.Context(), "routing_decision "+decision.String())
	return decision.TargetModel
}

// decideRoute 先查会话固定记录，没有再分类；分类结果写回固定记录。
func decideRoute(c *gin.Context, policy *model.RoutingPolicy) *RoutingDecision {
	pinTTL := time.Duration(policy.PinTTLSeconds) * time.Second
	sessionHash := ""
	if pinTTL > 0 {
		sessionHash = session.GenerateGenericSessionHash(c, policy.Name)
		if pin, ok := model.LookupRoutingPin(sessionHash); ok {
			if policyHasTarget(policy, pin.Label, pin.TargetModel) {
				model.RenewRoutingPin(sessionHash, pin, pinTTL)
				return &RoutingDecision{Policy: policy.Name, Label: pin.Label, TargetModel: pin.TargetModel, Reason: RoutingReasonPinned}
			}
			// 策略改过，固定的目标已不在路由表里：丢弃并重新分类。
			model.ForgetRoutingPin(sessionHash)
		}
	}

	decision := classifyRequest(c, policy)

	if pinTTL > 0 && sessionHash != "" {
		pin := model.RoutingPin{Label: decision.Label, TargetModel: decision.TargetModel}
		bound, err := model.BindRoutingPin(sessionHash, pin, pinTTL)
		if err != nil {
			logger.LogWarn(c.Request.Context(), "routing_pin_bind_failed policy="+policy.Name+" error="+err.Error())
		} else if bound != pin {
			// 并发首批请求里别人先写入了：跟随它，同一会话只有一个目标。
			decision.Label, decision.TargetModel, decision.Reason = bound.Label, bound.TargetModel, RoutingReasonPinned
		}
	}
	return decision
}

// policyHasTarget 判断固定记录是否仍对应策略里的一条路由（或兜底）。
func policyHasTarget(policy *model.RoutingPolicy, label, target string) bool {
	if label == "" {
		return target == policy.FallbackModel
	}
	route := policy.RouteFor(label)
	if route == nil {
		return false
	}
	return route.TargetModel == target
}

// classifyRequest 取首条用户消息、调分类、按阈值定目标。失败一律兜底。
func classifyRequest(c *gin.Context, policy *model.RoutingPolicy) *RoutingDecision {
	decision := &RoutingDecision{Policy: policy.Name, TargetModel: policy.FallbackModel}

	var body []byte
	if raw, ok := c.Get(config.GinRequestBodyKey); ok {
		body, _ = raw.([]byte)
	}
	text := firstUserMessageText(body, policy.MaxInputChars)
	if text == "" {
		decision.Reason = RoutingReasonEmptyInput
		return decision
	}

	started := routingPolicyNow()
	result, err := classify(c, policy, text)
	decision.DecisionMs = routingPolicyNow().Sub(started).Milliseconds()
	if err != nil {
		logger.LogWarn(c.Request.Context(), "routing_decision_failed policy="+policy.Name+" error="+err.Error())
		decision.Reason = RoutingReasonDecisionError
		return decision
	}
	decision.DecisionUsage = result.Usage
	decision.Confidence = result.Confidence
	route := policy.RouteFor(result.Label)
	if route == nil {
		logger.LogWarn(c.Request.Context(), "routing_decision_unknown_label policy="+policy.Name+" label="+result.Label)
		decision.Reason = RoutingReasonDecisionError
		return decision
	}
	decision.Label = route.Label
	if result.Confidence < policy.Threshold {
		decision.Reason = RoutingReasonBelowThreshold
		return decision
	}
	decision.TargetModel = route.TargetModel
	decision.Reason = RoutingReasonClassified
	return decision
}

// ---- 默认决策后端：普通对话模型 + JSON 自报置信度 ----

const routingDecisionSystemPrompt = `You are a request router. You will be given a question, a list of labels with descriptions, and the first message of a user's request. Pick the single label that best answers the question for that request. Reply with a JSON object only, no prose, in the form {"label": "<label>", "confidence": <number between 0 and 1>}. The confidence is how sure you are that the chosen label is right.`

// routingDecisionMaxTokens 给 JSON 答案留的输出上限；推理模型会把思考也算进去，所以不能太小。
const routingDecisionMaxTokens = 256

// buildDecisionMessages 组装发给决策模型的两条消息。
func buildDecisionMessages(policy *model.RoutingPolicy, text string) []types.ChatCompletionMessage {
	var sb strings.Builder
	if policy.Question != "" {
		sb.WriteString(policy.Question)
		sb.WriteString("\n\n")
	}
	sb.WriteString("Labels:\n")
	for _, route := range policy.Routes {
		sb.WriteString("- ")
		sb.WriteString(route.Label)
		if route.Description != "" {
			sb.WriteString(": ")
			sb.WriteString(route.Description)
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\nUser request:\n<<<\n")
	sb.WriteString(text)
	sb.WriteString("\n>>>")
	return []types.ChatCompletionMessage{
		{Role: types.ChatMessageRoleSystem, Content: routingDecisionSystemPrompt},
		{Role: types.ChatMessageRoleUser, Content: sb.String()},
	}
}

// parseDecisionAnswer 从模型回复里取出 {"label","confidence"}；允许回复带多余文字或代码围栏。
func parseDecisionAnswer(content string) (*ClassifyResult, error) {
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return nil, errors.New("decision model did not return a JSON object")
	}
	payload := content[start : end+1]
	if !gjson.Valid(payload) {
		return nil, errors.New("decision model returned invalid JSON")
	}
	label := strings.TrimSpace(gjson.Get(payload, "label").String())
	if label == "" {
		return nil, errors.New("decision model returned no label")
	}
	confidence := gjson.Get(payload, "confidence")
	value := confidence.Float()
	if confidence.Type == gjson.String {
		parsed, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(confidence.String()), "%"), 64)
		if err == nil {
			value = parsed
		}
	}
	// 有的模型给百分数。
	if value > 1 && value <= 100 {
		value = value / 100
	}
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	return &ClassifyResult{Label: label, Confidence: value}, nil
}

// classifyWithDecisionModel 用用户的分组链给决策模型选渠道，发一条无状态对话请求。
// 用全新的 gin context：provider 会从 context 读客户端原始请求体做透传，不能让它看到用户的 body；
// 也不带令牌的模型白名单，决策模型是管理员配的，不受用户 key 限制。
func classifyWithDecisionModel(c *gin.Context, policy *model.RoutingPolicy, text string) (*ClassifyResult, error) {
	timeout := time.Duration(policy.DecisionTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = model.RoutingPolicyDefaultTimeoutMs * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()

	decisionCtx, err := newDecisionContext(c, ctx, policy.Name)
	if err != nil {
		return nil, err
	}
	provider, upstreamModel, err := GetProvider(decisionCtx, policy.DecisionModel)
	if err != nil {
		return nil, fmt.Errorf("decision model %s: %w", policy.DecisionModel, err)
	}
	provider.GetRequester().Context = ctx
	usage := &types.Usage{}
	provider.SetUsage(usage)

	// 决策调用按决策模型的价格单独计一笔（站点开关可关）。预扣失败（额度不足 / 预算超限）
	// 时不发起决策，直接兜底：用户的主请求随后也会被同一道闸拦下。
	var quota *relay_util.Quota
	if config.RoutingDecisionBillingEnabled {
		quota = relay_util.NewQuota(decisionCtx, policy.DecisionModel, 0)
		if errWithCode := quota.PreQuotaConsumption(); errWithCode != nil {
			return nil, fmt.Errorf("decision model %s: %s", policy.DecisionModel, errWithCode.Message)
		}
	}

	temperature := 0.0
	request := &types.ChatCompletionRequest{
		Model:       upstreamModel,
		Messages:    buildDecisionMessages(policy, text),
		MaxTokens:   routingDecisionMaxTokens,
		Temperature: &temperature,
	}

	type outcome struct {
		content string
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		content, err := sendDecisionRequest(provider, upstreamModel, request)
		done <- outcome{content: content, err: err}
	}()
	select {
	case <-ctx.Done():
		if quota != nil {
			quota.Undo(decisionCtx)
		}
		return nil, fmt.Errorf("decision model %s: timed out after %s", policy.DecisionModel, timeout)
	case out := <-done:
		if out.err != nil {
			if quota != nil {
				quota.Undo(decisionCtx)
			}
			return nil, out.err
		}
		if quota != nil {
			// 答案解析失败也要计费：上游已经按用量收了钱。
			quota.Consume(decisionCtx, usage, false)
		}
		result, err := parseDecisionAnswer(out.content)
		if err != nil {
			return nil, err
		}
		result.Usage = usage
		return result, nil
	}
}

// newDecisionContext 复制决策调用需要的身份与分组信息，其余（请求体、令牌白名单）不带。
func newDecisionContext(c *gin.Context, ctx context.Context, policyName string) (*gin.Context, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/v1/chat/completions", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Request != nil {
		req.RemoteAddr = c.Request.RemoteAddr
		for _, header := range []string{"X-Forwarded-For", "X-Real-Ip"} {
			if v := c.Request.Header.Get(header); v != "" {
				req.Header.Set(header, v)
			}
		}
	}
	decisionCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	decisionCtx.Request = req
	// 身份、分组与额度相关的键照搬，计费才能落到同一个用户 / 令牌 / 组织成员上；
	// token_setting 不带（它承载令牌模型白名单）。
	for _, key := range []string{"id", "token_id", "token_name", "token_created_by", "token_unlimited_quota",
		"group", "token_group", "token_backup_group", "group_ratio", logger.RequestIdKey} {
		if value, ok := c.Get(key); ok {
			decisionCtx.Set(key, value)
		}
	}
	decisionCtx.Set("routing_policy", policyName)
	decisionCtx.Set(relay_util.RoutingDecisionCallContextKey, true)
	decisionCtx.Set("requestStartTime", routingPolicyNow())
	return decisionCtx, nil
}

// sendDecisionRequest 按渠道能力发送：Responses-only 的模型走 Responses，其余走 Chat。
func sendDecisionRequest(provider providersBase.ProviderInterface, upstreamModel string, request *types.ChatCompletionRequest) (string, error) {
	channel := provider.GetChannel()
	if channel != nil && channel.ShouldUseResponsesForModel(upstreamModel) {
		if resProvider, ok := provider.(providersBase.ResponsesInterface); ok {
			resReq := request.ToResponsesRequest()
			resReq.Model = upstreamModel
			resReq.ConvertChat = true
			resp, errWithCode := resProvider.CreateResponses(resReq)
			if errWithCode != nil {
				return "", errors.New(errWithCode.Message)
			}
			return firstChatContent(resp.ToChat())
		}
	}
	chat, ok := provider.(providersBase.ChatInterface)
	if !ok {
		return "", errors.New("decision model channel does not support chat")
	}
	resp, errWithCode := chat.CreateChatCompletion(request)
	if errWithCode != nil {
		return "", errors.New(errWithCode.Message)
	}
	return firstChatContent(resp)
}

func firstChatContent(resp *types.ChatCompletionResponse) (string, error) {
	if resp == nil || len(resp.Choices) == 0 {
		return "", errors.New("decision model returned no choices")
	}
	return resp.Choices[0].Message.StringContent(), nil
}
