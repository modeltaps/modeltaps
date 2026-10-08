package check_channel

import (
	"github.com/modeltaps/modeltaps/types"
	"fmt"
	"strings"
)

type CheckErrorProcess struct {
	ModelName string
}

func CreateCheckErrorProcess(modelName string) *CheckErrorProcess {
	return &CheckErrorProcess{
		ModelName: modelName,
	}
}

func (c *CheckErrorProcess) GetName() string {
	return "Error check"
}

func (c *CheckErrorProcess) GetRequest() *types.ChatCompletionRequest {
	return &types.ChatCompletionRequest{
		Model: c.ModelName,
		Messages: []types.ChatCompletionMessage{
			{
				Role:    types.ChatMessageRoleUser,
				Content: "hi",
			},
			{
				Role:    "user11",
				Content: "hi",
			},
		},
	}
}

func (c *CheckErrorProcess) Check(req *types.ChatCompletionRequest, resp *types.ChatCompletionResponse, openaiErr *types.OpenAIError) []*CheckResult {
	checkResults := make([]*CheckResult, 0)
	if openaiErr == nil {
		checkResults = append(checkResults, &CheckResult{
			Name:   "Response",
			Status: CheckStatusFailed,
			Remark: "An error was expected but none was returned (not absolute, use your judgment)",
		})
		return checkResults
	}

	// 判断错误中存在多少个 request id:
	requestIDCount := strings.Count(openaiErr.Message, "request id:")

	checkResults = append(checkResults, &CheckResult{
		Name:   "Error",
		Status: CheckStatusSuccess,
		Remark: fmt.Sprintf("Error contains %d request id(s) (indicates how many relay proxies the request passed through)", requestIDCount),
	})
	return checkResults
}
