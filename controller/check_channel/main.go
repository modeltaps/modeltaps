package check_channel

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers"
	providers_base "github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
)

const (
	CheckStatusFailed  = 0
	CheckStatusSuccess = 1
	// CheckStatusWarning marks a result that is only a hint: it neither proves nor rules out a problem.
	CheckStatusWarning = 2
	// CheckStatusSkipped marks a check that does not apply to this channel (for example the request
	// conversion drops the parameter under test). Skipped items are left out of the overall status.
	CheckStatusSkipped = 3
)

// A successful base request slower than this is reported as slow.
const slowThreshold = 6 * time.Second

// ErrModelRequired is returned when the check request names no model.
var ErrModelRequired = errors.New("model is required")

type ModelResult struct {
	Model  string                `json:"model"`
	Status int                   `json:"status"`
	Items  []*CheckProcessResult `json:"items"`
}

type CheckProcessResult struct {
	Key       string `json:"key"`
	Status    int    `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
	// Category is a stable key for the kind of upstream error (see classifyError); empty when unknown.
	Category string                        `json:"category,omitempty"`
	Results  []*CheckResult                `json:"results"`
	Response *types.ChatCompletionResponse `json:"response,omitempty"`
}

// CheckResult is one verdict of a check. Name and Code are stable keys that the web UI
// translates; Params carries the values the message is built from (numbers, model names,
// upstream text), which are shown as they are and never translated.
type CheckResult struct {
	Name   string         `json:"name"`
	Status int            `json:"status"`
	Code   string         `json:"code"`
	Params map[string]any `json:"params,omitempty"`
}

// CheckContext tells a check how the request reaches the upstream, so that "the upstream does not
// support it" can be told apart from "the gateway's own conversion dropped it".
type CheckContext struct {
	// ClaudeConversion: the request is converted to a Claude Messages request, which ignores response_format.
	ClaudeConversion bool
	// RewritesUnknownRole: the request goes through the Claude or Gemini conversion, which turns unknown roles into valid ones.
	RewritesUnknownRole bool
}

type CheckProcess interface {
	GetKey() string
	GetRequest() *types.ChatCompletionRequest
	Check(ctx CheckContext, req *types.ChatCompletionRequest, resp *types.ChatCompletionResponse, openaiErr *types.OpenAIError) []*CheckResult
}

// CheckModel runs every check against one model of a channel. The base check runs first and the
// others then run in parallel. Running the base check alone first matters for OAuth channels
// (Codex, Claude Code, ...): with an expired token every provider would refresh it with the same
// refresh token, and after the base check the new token is already cached. It also keeps the
// base check's latency free of contention from the other checks.
// Each check builds its own provider: a provider holds its usage and gin.Context and must not be
// shared across goroutines.
func CheckModel(channelId int, modelName string) (*ModelResult, error) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return nil, ErrModelRequired
	}

	channel, err := model.GetChannelById(channelId)
	if err != nil {
		return nil, err
	}
	channel.SetProxy()

	return runChecks(modelName, getProcess(modelName), func(p CheckProcess) *CheckProcessResult {
		return runProcess(channel, modelName, p)
	}), nil
}

// runChecks runs processes[0] (the base check) first and the rest in parallel.
func runChecks(modelName string, processes []CheckProcess, run func(CheckProcess) *CheckProcessResult) *ModelResult {
	result := &ModelResult{
		Model: modelName,
		Items: make([]*CheckProcessResult, len(processes)),
	}

	result.Items[0] = run(processes[0])

	var wg sync.WaitGroup
	for i := 1; i < len(processes); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result.Items[i] = run(processes[i])
		}(i)
	}
	wg.Wait()

	result.Status = aggregateStatus(result.Items)
	return result
}

func runProcess(channel *model.Channel, modelName string, p CheckProcess) (item *CheckProcessResult) {
	// The checks run on their own goroutines, outside gin's Recovery: a provider panic would
	// otherwise take down the whole process.
	defer func() {
		if r := recover(); r != nil {
			logger.SysError(fmt.Sprintf("check channel panic: %v\n%s", r, debug.Stack()))
			item = &CheckProcessResult{
				Key:     p.GetKey(),
				Status:  CheckStatusFailed,
				Results: []*CheckResult{{Name: resultResponse, Status: CheckStatusFailed, Code: "check_panicked"}},
			}
		}
	}()

	item = &CheckProcessResult{Key: p.GetKey()}

	target, err := newCheckTarget(channel, modelName)
	if err != nil {
		item.Results = []*CheckResult{{
			Name:   resultResponse,
			Status: CheckStatusFailed,
			Code:   "provider_error",
			Params: map[string]any{"message": utils.MaskSensitiveInfo(err.Error())},
		}}
		item.Status = CheckStatusFailed
		return item
	}

	req := p.GetRequest()
	req.Model = target.upstreamModel
	ctx := target.context

	if ctx.RewritesUnknownRole && p.GetKey() == errorProbeKey {
		// The conversion turns the invalid role into a valid one, so the upstream always accepts the
		// request and the result is "not applicable"; sending it would only spend quota.
		item.Results = p.Check(ctx, req, nil, nil)
		item.Status = itemStatus(item.Results)
		return item
	}

	start := time.Now()
	resp, apiErr := target.send(req)
	latency := time.Since(start)
	item.LatencyMs = latency.Milliseconds()

	var openaiErr *types.OpenAIError
	if apiErr != nil {
		openaiErr = &apiErr.OpenAIError
		resp = nil
		// The relay check expects an error; classifying it would be noise.
		if p.GetKey() != errorProbeKey {
			item.Category = classifyError(apiErr.StatusCode, openaiErr.Type, openaiErr.Message)
		}
	}

	item.Results = p.Check(ctx, req, resp, openaiErr)
	if p.GetKey() == baseProbeKey && openaiErr == nil {
		item.Results = append(item.Results, checkLatency(latency))
	}
	item.Response = resp
	item.Status = itemStatus(item.Results)
	return item
}

type checkTarget struct {
	upstreamModel string
	context       CheckContext
	send          func(*types.ChatCompletionRequest) (*types.ChatCompletionResponse, *types.OpenAIErrorWithStatusCode)
}

// newCheckTarget builds a provider the way the manual channel test does: it parses the # suffix
// and applies the channel's model mapping, so a mapped alias is not sent to the upstream as is.
// Like the relay, chat requests for Responses-only models go through the Responses API.
func newCheckTarget(channel *model.Channel, modelName string) (*checkTarget, error) {
	var otherArg string
	if name, arg, ok := strings.Cut(modelName, "#"); ok {
		modelName, otherArg = name, arg
	}

	req, err := http.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	provider := providers.GetProvider(channel, c)
	if provider == nil {
		return nil, errors.New("channel not implemented")
	}
	if otherArg != "" {
		provider.SetOtherArg(otherArg)
	}
	upstreamModel, err := provider.ModelMappingHandler(modelName)
	if err != nil {
		return nil, err
	}
	upstreamModel = strings.TrimPrefix(upstreamModel, "+")
	provider.SetUsage(&types.Usage{})

	target := &checkTarget{
		upstreamModel: upstreamModel,
		context: CheckContext{
			ClaudeConversion:    usesClaudeConversion(channel.Type, upstreamModel),
			RewritesUnknownRole: rewritesUnknownRole(channel.Type, upstreamModel, usesGeminiOpenAIAPI(provider)),
		},
	}

	if channel.ShouldUseResponsesForModel(upstreamModel) {
		if resProvider, ok := provider.(providers_base.ResponsesInterface); ok {
			target.send = func(chatReq *types.ChatCompletionRequest) (*types.ChatCompletionResponse, *types.OpenAIErrorWithStatusCode) {
				resReq := chatReq.ToResponsesRequest()
				resReq.Model = upstreamModel
				resReq.ConvertChat = true
				resp, errWithCode := resProvider.CreateResponses(resReq)
				if errWithCode != nil {
					return nil, errWithCode
				}
				return resp.ToChat(), nil
			}
			return target, nil
		}
	}

	chat, ok := provider.(providers_base.ChatInterface)
	if !ok {
		return nil, errors.New("channel not implemented")
	}
	target.send = chat.CreateChatCompletion
	return target, nil
}

// itemStatus: any failure fails the item; otherwise any warning makes it a warning; an item whose
// results are all skipped is skipped; anything else passes.
func itemStatus(results []*CheckResult) int {
	status := CheckStatusSkipped
	for _, r := range results {
		switch r.Status {
		case CheckStatusFailed:
			return CheckStatusFailed
		case CheckStatusWarning:
			status = CheckStatusWarning
		case CheckStatusSuccess:
			if status == CheckStatusSkipped {
				status = CheckStatusSuccess
			}
		}
	}
	return status
}

// aggregateStatus is the overall verdict for a model. Only a failed base check fails the model:
// another failed check only means the capability is missing, so it counts as a warning. The relay
// check is only a clue (its signal can be rewritten) and skipped checks say nothing about the
// channel, so neither takes part.
func aggregateStatus(items []*CheckProcessResult) int {
	status := CheckStatusSuccess
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.Key == baseProbeKey && item.Status == CheckStatusFailed {
			return CheckStatusFailed
		}
		if item.Key == errorProbeKey || item.Status == CheckStatusSkipped {
			continue
		}
		if item.Status != CheckStatusSuccess {
			status = CheckStatusWarning
		}
	}
	return status
}

// getProcess returns the checks of one model; the base check must stay first (see runChecks).
func getProcess(modelName string) []CheckProcess {
	return []CheckProcess{
		CreateCheckBaseProcess(modelName),
		CreateCheckImgProcess(modelName),
		CreateCheckJsonFormatProcess(modelName),
		CreateCheckToolProcess(modelName),
		CreateCheckErrorProcess(modelName),
	}
}

// Names of the individual results; the web UI translates them.
const (
	resultResponse      = "response"
	resultChallenge     = "challenge"
	resultUsage         = "usage"
	resultResponseModel = "response_model"
	resultResponseID    = "response_id"
	resultLatency       = "latency"
	resultImage         = "image"
	resultJSONSchema    = "json_schema"
	resultToolCall      = "tool_call"
	resultRequestID     = "request_id"
)

// failedResponse masks the upstream error like the manual channel test does. Other params (model
// names, answers) are not masked: masking would mangle model names such as us.anthropic.xxx.
func failedResponse(openaiErr *types.OpenAIError) []*CheckResult {
	return []*CheckResult{{
		Name:   resultResponse,
		Status: CheckStatusFailed,
		Code:   "upstream_error",
		Params: map[string]any{"message": utils.MaskSensitiveInfo(openaiErr.Message)},
	}}
}

func noContent(name string) *CheckResult {
	return &CheckResult{Name: name, Status: CheckStatusFailed, Code: "no_content"}
}

func firstChoiceContent(resp *types.ChatCompletionResponse) (string, bool) {
	if resp == nil || len(resp.Choices) == 0 {
		return "", false
	}
	return resp.Choices[0].Message.StringContent(), true
}

func truncate(s string, limit int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "…"
}
