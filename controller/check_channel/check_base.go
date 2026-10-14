package check_channel

import (
	"math"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/types"
)

// Snapshot suffix: a date such as 2024-08-06 or 0613.
var snapshotSuffixPattern = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}|\d{4})$`)

type CheckBaseProcess struct {
	ModelName string
	challenge challenge
}

func CreateCheckBaseProcess(modelName string) *CheckBaseProcess {
	return &CheckBaseProcess{ModelName: modelName, challenge: generateChallenge()}
}

const baseProbeKey = "base"

func (c *CheckBaseProcess) GetKey() string {
	return baseProbeKey
}

func (c *CheckBaseProcess) GetRequest() *types.ChatCompletionRequest {
	return &types.ChatCompletionRequest{
		Model: c.ModelName,
		Messages: []types.ChatCompletionMessage{
			{Role: types.ChatMessageRoleUser, Content: c.challenge.Prompt},
		},
	}
}

func (c *CheckBaseProcess) Check(_ CheckContext, req *types.ChatCompletionRequest, resp *types.ChatCompletionResponse, openaiErr *types.OpenAIError) []*CheckResult {
	if openaiErr != nil {
		return failedResponse(openaiErr)
	}
	if resp == nil {
		return []*CheckResult{noContent(resultResponse)}
	}

	results := []*CheckResult{c.checkChallenge(resp), checkUsage(resp)}
	// Infer the official format from the mapped upstream model: a channel alias says nothing about the upstream.
	channelType := getChannelTypeByModelName(req.Model)
	return append(results, checkResponseModel(channelType, req.Model, resp.Model), checkResponseID(channelType, resp.ID))
}

// checkChallenge is the only result that proves a working model was really called: the question
// is random, so a faked response cannot answer it.
func (c *CheckBaseProcess) checkChallenge(resp *types.ChatCompletionResponse) *CheckResult {
	answer, ok := firstChoiceContent(resp)
	if !ok {
		return noContent(resultChallenge)
	}
	params := map[string]any{"expected": c.challenge.Expected}
	if validateChallenge(answer, c.challenge.Expected) {
		return &CheckResult{Name: resultChallenge, Status: CheckStatusSuccess, Code: "challenge_passed", Params: params}
	}
	params["answer"] = truncate(answer, 200)
	return &CheckResult{Name: resultChallenge, Status: CheckStatusFailed, Code: "challenge_mismatch", Params: params}
}

func checkUsage(resp *types.ChatCompletionResponse) *CheckResult {
	if resp.Usage == nil || resp.Usage.PromptTokens <= 0 || resp.Usage.CompletionTokens <= 0 {
		return &CheckResult{Name: resultUsage, Status: CheckStatusWarning, Code: "usage_missing"}
	}
	return &CheckResult{
		Name:   resultUsage,
		Status: CheckStatusSuccess,
		Code:   "usage_ok",
		Params: map[string]any{"prompt_tokens": resp.Usage.PromptTokens, "completion_tokens": resp.Usage.CompletionTokens},
	}
}

// The response model and ID can be rewritten freely by a relay, so they are hints and never fail.
func checkResponseModel(channelType int, requestModel, responseModel string) *CheckResult {
	if responseModel == "" {
		return &CheckResult{Name: resultResponseModel, Status: CheckStatusWarning, Code: "response_model_missing"}
	}

	consistent := responseModel == requestModel
	if !consistent && channelType == config.ChannelTypeOpenAI {
		// OpenAI answers an alias without a version with a dated snapshot, e.g. gpt-4o -> gpt-4o-2024-08-06.
		// Accept only "alias-date" so gpt-4o-mini cannot pass for gpt-4o.
		if suffix, ok := strings.CutPrefix(responseModel, requestModel+"-"); ok {
			consistent = snapshotSuffixPattern.MatchString(suffix)
		}
	}

	if consistent {
		return &CheckResult{Name: resultResponseModel, Status: CheckStatusSuccess, Code: "response_model_match", Params: map[string]any{"model": responseModel}}
	}
	return &CheckResult{
		Name:   resultResponseModel,
		Status: CheckStatusWarning,
		Code:   "response_model_mismatch",
		Params: map[string]any{"request": requestModel, "response": responseModel},
	}
}

// Codex and OpenAI channels that route chat to the Responses API return a Responses resp_ ID.
var officialIDPrefixes = map[int][]string{
	config.ChannelTypeOpenAI:    {"chatcmpl-", "resp_"},
	config.ChannelTypeAnthropic: {"msg_"},
}

func checkResponseID(channelType int, id string) *CheckResult {
	prefixes, ok := officialIDPrefixes[channelType]
	switch {
	case id == "":
		return &CheckResult{Name: resultResponseID, Status: CheckStatusWarning, Code: "response_id_missing"}
	case !ok || slices.ContainsFunc(prefixes, func(p string) bool { return strings.HasPrefix(id, p) }):
		return &CheckResult{Name: resultResponseID, Status: CheckStatusSuccess, Code: "response_id_ok", Params: map[string]any{"id": id}}
	default:
		return &CheckResult{
			Name:   resultResponseID,
			Status: CheckStatusWarning,
			Code:   "response_id_unofficial",
			Params: map[string]any{"id": id, "prefixes": strings.Join(prefixes, ", ")},
		}
	}
}

// checkLatency reports a successful base request that took longer than slowThreshold as slow.
func checkLatency(latency time.Duration) *CheckResult {
	params := map[string]any{"seconds": math.Round(latency.Seconds()*100) / 100}
	if latency >= slowThreshold {
		return &CheckResult{Name: resultLatency, Status: CheckStatusWarning, Code: "latency_slow", Params: params}
	}
	return &CheckResult{Name: resultLatency, Status: CheckStatusSuccess, Code: "latency_ok", Params: params}
}

// getChannelTypeByModelName infers the official upstream of a model to judge the official format
// of the response fields.
func getChannelTypeByModelName(modelName string) int {
	switch {
	case strings.HasPrefix(modelName, "gpt-"), strings.HasPrefix(modelName, "chatgpt-"),
		strings.HasPrefix(modelName, "o1"), strings.HasPrefix(modelName, "o3"), strings.HasPrefix(modelName, "o4"):
		// -all / -realtime / -instruct are third-party or non-chat variants without the official format.
		if strings.Contains(modelName, "-all") || strings.Contains(modelName, "-realtime") || strings.Contains(modelName, "-instruct") {
			return config.ChannelTypeUnknown
		}
		return config.ChannelTypeOpenAI
	case strings.HasPrefix(modelName, "claude-"):
		return config.ChannelTypeAnthropic
	case strings.HasPrefix(modelName, "gemini-"):
		return config.ChannelTypeGemini
	}
	return config.ChannelTypeUnknown
}
