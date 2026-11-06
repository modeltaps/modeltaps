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
	RoutingReasonAuxiliary      = relay_util.RoutingReasonAuxiliary
	RoutingDecisionContextKey   = relay_util.RoutingDecisionContextKey
)

// ClassifyResult 决策后端的输出：选中的标签与它的置信度（0~1）。logprobs 打分时
// Confidence 是该标签的归一概率，Probabilities 列出全部标签。
type ClassifyResult struct {
	Label         string
	Confidence    float64
	Scoring       string
	Probabilities []relay_util.RoutingLabelProbability
	Coverage      float64
	Usage         *types.Usage
}

// classifyFunc 把一段文本按策略分类。默认实现调决策模型，测试替换成假的。
type classifyFunc func(c *gin.Context, policy *model.RoutingPolicy, text string) (*ClassifyResult, error)

// classifyRoute 为 nil 时用 classifyWithDecisionModelOpts；不能在包级直接赋值，那会和 getProvider 形成初始化环。
var classifyRoute classifyFunc

func classify(c *gin.Context, policy *model.RoutingPolicy, text string, bill bool) (*ClassifyResult, error) {
	if classifyRoute != nil {
		return classifyRoute(c, policy, text)
	}
	return classifyWithDecisionModelOpts(c, policy, text, bill)
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

	// 旁路请求（Claude Code 生成标题等）与会话首轮并发、常常先到；它的首条消息是 Agent 的模板，
	// 拿去分类会把整个会话固定错。会话已固定时上面已跟随固定结果，这里不分类、不写固定记录。
	if isAgentAuxiliaryRequest(requestBody(c)) {
		return &RoutingDecision{Policy: policy.Name, TargetModel: policy.FallbackModel, Reason: RoutingReasonAuxiliary}
	}

	decision := classifyRequest(c, policy)

	// 只固定真正分类出来的结果。决策失败或没有文本时的兜底不固定：决策模型短暂故障时，
	// 新会话不该在整个固定时长里都停在兜底上，下一轮重新分类。
	if pinTTL > 0 && sessionHash != "" && decisionIsPinnable(decision) {
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

// decisionIsPinnable 分类成功（含低于阈值走兜底）的决策才写固定记录。
func decisionIsPinnable(decision *RoutingDecision) bool {
	return decision.Reason == RoutingReasonClassified || decision.Reason == RoutingReasonBelowThreshold
}

func requestBody(c *gin.Context) []byte {
	if raw, ok := c.Get(config.GinRequestBodyKey); ok {
		body, _ := raw.([]byte)
		return body
	}
	return nil
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

	text := firstUserMessageText(requestBody(c), policy.MaxInputChars)
	if text == "" {
		decision.Reason = RoutingReasonEmptyInput
		return decision
	}

	started := routingPolicyNow()
	result, err := classify(c, policy, text, config.RoutingDecisionBillingEnabled)
	decision.DecisionMs = routingPolicyNow().Sub(started).Milliseconds()
	applyClassifyResult(decision, policy, result, err)
	if decision.Reason == RoutingReasonDecisionError {
		logger.LogWarn(c.Request.Context(), "routing_decision_failed policy="+policy.Name+" error="+decision.Error)
	}
	return decision
}

// ---- 自报打分：普通对话模型 + JSON 自报置信度（logprobs 打分见 routing_scoring.go） ----

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
// tryRoutingPolicy 后台「试一试」：只跑决策这一步，不计费、不写固定记录，返回假定按该决策会得到的结果。
// 调用方是管理员会话（无令牌分组），决策模型按管理员自己的用户分组选渠道。
func tryRoutingPolicy(c *gin.Context, policy *model.RoutingPolicy, text string) *RoutingDecision {
	ensureAdminGroup(c)
	decision := &RoutingDecision{Policy: policy.Name, TargetModel: policy.FallbackModel}
	text = firstUserMessageTextFromPlain(text, policy.MaxInputChars)
	if text == "" {
		decision.Reason = RoutingReasonEmptyInput
		return decision
	}
	started := routingPolicyNow()
	result, err := classify(c, policy, text, false)
	decision.DecisionMs = routingPolicyNow().Sub(started).Milliseconds()
	applyClassifyResult(decision, policy, result, err)
	return decision
}

// ensureAdminGroup 管理员会话没有令牌分组，决策模型按管理员自己的用户分组选渠道。
func ensureAdminGroup(c *gin.Context) {
	if c.GetString("group") == "" {
		if group, err := model.CacheGetUserGroup(c.GetInt("id")); err == nil {
			c.Set("group", group)
		}
	}
}

// firstUserMessageTextFromPlain 对已经是纯文本的输入做与请求路径相同的清理与截断。
func firstUserMessageTextFromPlain(text string, maxChars int) string {
	return truncateRunes(stripAgentContext(text), maxChars)
}

// applyClassifyResult 把分类结果按阈值落到决策上：出错 / 未知标签 → decision_error，低于阈值 → below_threshold。
func applyClassifyResult(decision *RoutingDecision, policy *model.RoutingPolicy, result *ClassifyResult, err error) {
	if err != nil {
		decision.Reason = RoutingReasonDecisionError
		decision.Error = err.Error()
		return
	}
	decision.DecisionUsage = result.Usage
	decision.Confidence = result.Confidence
	decision.Scoring = result.Scoring
	decision.Probabilities = result.Probabilities
	decision.Coverage = result.Coverage
	route := policy.RouteFor(result.Label)
	if route == nil {
		decision.Reason = RoutingReasonDecisionError
		decision.Error = "decision model answered with an unknown label: " + result.Label
		return
	}
	decision.Label = route.Label
	if result.Confidence < policy.Threshold {
		decision.Reason = RoutingReasonBelowThreshold
		return
	}
	decision.TargetModel = route.TargetModel
	decision.Reason = RoutingReasonClassified
}

func classifyWithDecisionModelOpts(c *gin.Context, policy *model.RoutingPolicy, text string, bill bool) (*ClassifyResult, error) {
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
	call := &decisionCall{ctx: ctx, decisionCtx: decisionCtx, provider: provider, upstreamModel: upstreamModel,
		decisionModel: policy.DecisionModel, timeout: timeout, bill: bill}

	scoring := policy.Scoring
	if scoring == "" {
		scoring = model.RoutingScoringAuto
	}
	channelID := 0
	if channel := provider.GetChannel(); channel != nil {
		channelID = channel.Id
	}
	tryLogprobs := scoring == model.RoutingScoringLogprobs
	if scoring == model.RoutingScoringAuto {
		supported, known := lookupLogprobsSupport(channelID, upstreamModel)
		tryLogprobs = !known || supported
	}
	// Responses-only 的模型拿不到逐 token 的 logprobs。
	if tryLogprobs && call.usesResponses() {
		rememberLogprobsSupport(channelID, upstreamModel, false)
		if scoring == model.RoutingScoringLogprobs {
			return nil, fmt.Errorf("decision model %s: %w", policy.DecisionModel, errNoLogprobs)
		}
		tryLogprobs = false
	}

	usage := &types.Usage{}
	if tryLogprobs {
		choice, err := call.send(buildLetterRequest(policy, upstreamModel, text), usage)
		switch {
		case err == nil:
			result, parseErr := parseLetterLogprobs(policy, choice.LogProbs)
			if !errors.Is(parseErr, errNoLogprobs) {
				rememberLogprobsSupport(channelID, upstreamModel, true)
				if parseErr != nil {
					return nil, parseErr
				}
				result.Usage = usage
				return result, nil
			}
			err = parseErr
			fallthrough
		case isLogprobsRejection(err):
			rememberLogprobsSupport(channelID, upstreamModel, false)
			if scoring == model.RoutingScoringLogprobs {
				return nil, fmt.Errorf("decision model %s: %w", policy.DecisionModel, errNoLogprobs)
			}
			logger.LogInfo(c.Request.Context(), "routing_scoring_fallback policy="+policy.Name+" model="+policy.DecisionModel+" channel="+strconv.Itoa(channelID)+" scoring=self_reported")
		default:
			return nil, err
		}
	}

	temperature := 0.0
	request := &types.ChatCompletionRequest{
		Model:       upstreamModel,
		Messages:    buildDecisionMessages(policy, text),
		MaxTokens:   routingDecisionMaxTokens,
		Temperature: &temperature,
	}
	choice, err := call.send(request, usage)
	if err != nil {
		return nil, err
	}
	result, err := parseDecisionAnswer(choice.Message.StringContent())
	if err != nil {
		return nil, err
	}
	result.Scoring = model.RoutingScoringSelfReported
	result.Usage = usage
	return result, nil
}

// decisionCall 一次决策里对决策模型的调用；auto 打分时可能调两次（logprobs、再自报），每次单独计费。
type decisionCall struct {
	ctx           context.Context
	decisionCtx   *gin.Context
	provider      providersBase.ProviderInterface
	upstreamModel string
	decisionModel string
	timeout       time.Duration
	bill          bool
}

func (d *decisionCall) usesResponses() bool {
	channel := d.provider.GetChannel()
	if channel == nil || !channel.ShouldUseResponsesForModel(d.upstreamModel) {
		return false
	}
	_, ok := d.provider.(providersBase.ResponsesInterface)
	return ok
}

// send 发一次请求，用量累加进 total。决策调用按决策模型的价格单独计一笔（站点开关可关）。
// 预扣失败（额度不足 / 预算超限）时不发起决策，直接兜底：用户的主请求随后也会被同一道闸拦下。
func (d *decisionCall) send(request *types.ChatCompletionRequest, total *types.Usage) (*types.ChatCompletionChoice, error) {
	usage := &types.Usage{}
	d.provider.SetUsage(usage)
	var quota *relay_util.Quota
	if d.bill {
		quota = relay_util.NewQuota(d.decisionCtx, d.decisionModel, 0)
		if errWithCode := quota.PreQuotaConsumption(); errWithCode != nil {
			return nil, fmt.Errorf("decision model %s: %s", d.decisionModel, errWithCode.Message)
		}
	}

	type outcome struct {
		choice *types.ChatCompletionChoice
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		choice, err := sendDecisionRequest(d.provider, d.upstreamModel, request)
		done <- outcome{choice: choice, err: err}
	}()
	select {
	case <-d.ctx.Done():
		if quota != nil {
			quota.Undo(d.decisionCtx)
		}
		return nil, fmt.Errorf("decision model %s: timed out after %s", d.decisionModel, d.timeout)
	case out := <-done:
		if out.err != nil {
			if quota != nil {
				quota.Undo(d.decisionCtx)
			}
			return nil, out.err
		}
		if quota != nil {
			// 答案解析失败也要计费：上游已经按用量收了钱。
			quota.Consume(d.decisionCtx, usage, false)
		}
		total.PromptTokens += usage.PromptTokens
		total.CompletionTokens += usage.CompletionTokens
		total.TotalTokens += usage.TotalTokens
		return out.choice, nil
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
func sendDecisionRequest(provider providersBase.ProviderInterface, upstreamModel string, request *types.ChatCompletionRequest) (*types.ChatCompletionChoice, error) {
	channel := provider.GetChannel()
	if channel != nil && channel.ShouldUseResponsesForModel(upstreamModel) {
		if resProvider, ok := provider.(providersBase.ResponsesInterface); ok {
			resReq := request.ToResponsesRequest()
			resReq.Model = upstreamModel
			resReq.ConvertChat = true
			resp, errWithCode := resProvider.CreateResponses(resReq)
			if errWithCode != nil {
				return nil, errors.New(errWithCode.Message)
			}
			return firstChatChoice(resp.ToChat())
		}
	}
	chat, ok := provider.(providersBase.ChatInterface)
	if !ok {
		return nil, errors.New("decision model channel does not support chat")
	}
	resp, errWithCode := chat.CreateChatCompletion(request)
	if errWithCode != nil {
		return nil, errors.New(errWithCode.Message)
	}
	return firstChatChoice(resp)
}

func firstChatChoice(resp *types.ChatCompletionResponse) (*types.ChatCompletionChoice, error) {
	if resp == nil || len(resp.Choices) == 0 {
		return nil, errors.New("decision model returned no choices")
	}
	return &resp.Choices[0], nil
}
