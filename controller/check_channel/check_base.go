package check_channel

import (
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/types"
	"regexp"
	"strings"
)

type CheckBaseProcess struct {
	ModelName string
}

func CreateCheckBaseProcess(modelName string) *CheckBaseProcess {
	return &CheckBaseProcess{
		ModelName: modelName,
	}
}

func (c *CheckBaseProcess) GetName() string {
	return "Basic check"
}

func (c *CheckBaseProcess) GetRequest() *types.ChatCompletionRequest {
	req := &types.ChatCompletionRequest{
		Model: c.ModelName,
		Messages: []types.ChatCompletionMessage{
			{
				Role:    types.ChatMessageRoleUser,
				Content: "hi",
			},
		},
	}

	if strings.Contains(c.ModelName, "o1") {
		req.MaxCompletionTokens = 1
	} else {
		req.MaxTokens = 1
	}
	return req
}

func (c *CheckBaseProcess) Check(req *types.ChatCompletionRequest, resp *types.ChatCompletionResponse, openaiErr *types.OpenAIError) []*CheckResult {
	checkResults := make([]*CheckResult, 0)
	if openaiErr != nil {
		checkResults = append(checkResults, &CheckResult{
			Name:   "Response",
			Status: CheckStatusFailed,
			Remark: openaiErr.Message,
		})

		return checkResults
	}

	if resp.Usage == nil || resp.Usage.CompletionTokens <= 0 || resp.Usage.PromptTokens <= 0 {
		checkResults = append(checkResults, &CheckResult{
			Name:   "Response",
			Status: CheckStatusFailed,
			Remark: "Usage data is empty",
		})
	}

	channelType := getChannelTypeByModelName(c.ModelName)

	modelCheckResult := &CheckResult{
		Name:   "Model check",
		Status: CheckStatusFailed,
		Remark: "Model check failed",
	}

	if resp.Model == "" {
		modelCheckResult.Remark = "Returned model is empty"
	} else {
		switch channelType {
		case config.ChannelTypeOpenAI:
			// 正则 检查是否是4个数字结尾的格式 (如 gpt-4-0613)
			numberEndingPattern := regexp.MustCompile(`\d{4}$`)
			// 正则 检查是否是年月日结尾的格式 (如 gpt-4o-mini-2024-07-18)
			dateEndingPattern := regexp.MustCompile(`\d{4}-\d{2}-\d{2}$`)

			isVersioned := numberEndingPattern.MatchString(c.ModelName) || dateEndingPattern.MatchString(c.ModelName)

			// 如果是带版本的模型，那么请求模型应该等于响应模型
			if isVersioned {
				if req.Model != resp.Model {
					modelCheckResult.Remark = "Request model and response model should match (except azure)"
				} else {
					modelCheckResult.Status = CheckStatusSuccess
					modelCheckResult.Remark = "SUCCESS"
				}
			} else {
				// 否则，请求模型一定不等于响应模型
				if req.Model == resp.Model {
					modelCheckResult.Remark = "Request model and response model should not match (except azure)"
				} else {
					// 响应模型的前缀应该等于请求模型的前缀
					if strings.HasPrefix(resp.Model, req.Model) {
						modelCheckResult.Status = CheckStatusSuccess
						modelCheckResult.Remark = "SUCCESS"
					} else {
						modelCheckResult.Remark = "Response model prefix should equal the request model (except azure)"
					}
				}
			}
		default:
			if resp.Model != req.Model {
				modelCheckResult.Remark = "Request model and response model should match (not absolute, use your judgment)"
			} else {
				modelCheckResult.Status = CheckStatusSuccess
				modelCheckResult.Remark = "SUCCESS"
			}
		}
	}

	checkResults = append(checkResults, modelCheckResult)

	IDCheckResult := &CheckResult{
		Name:   "ID check",
		Status: CheckStatusFailed,
		Remark: "ID check failed",
	}

	if resp.ID == "" {
		IDCheckResult.Remark = "ID is empty (not absolute, e.g. gemini has no ID)"
	} else {
		switch channelType {
		case config.ChannelTypeOpenAI:
			if strings.HasPrefix(resp.ID, "chatcmpl-") {
				IDCheckResult.Status = CheckStatusSuccess
				IDCheckResult.Remark = "SUCCESS"
			} else {
				IDCheckResult.Remark = "ID should start with chatcmpl"
			}

		case config.ChannelTypeAnthropic:
			if strings.HasPrefix(resp.ID, "msg_") {
				IDCheckResult.Status = CheckStatusSuccess
				IDCheckResult.Remark = "SUCCESS"
			} else {
				IDCheckResult.Remark = "ID should start with msg_"
			}
		default:
			IDCheckResult.Status = CheckStatusSuccess
			IDCheckResult.Remark = "SUCCESS"
		}
	}

	checkResults = append(checkResults, IDCheckResult)

	return checkResults
}
