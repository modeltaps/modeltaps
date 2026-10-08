package check_channel

import (
	"github.com/modeltaps/modeltaps/types"
	"encoding/json"
	"fmt"
)

type CheckToolProcess struct {
	ModelName string
}

func CreateCheckToolProcess(modelName string) *CheckToolProcess {
	return &CheckToolProcess{
		ModelName: modelName,
	}
}

func (c *CheckToolProcess) GetName() string {
	return "Function calling check"
}

func (c *CheckToolProcess) GetRequest() *types.ChatCompletionRequest {
	addTool := map[string]interface{}{}
	multiplyTool := map[string]interface{}{}

	json.Unmarshal([]byte(`{"properties":{"a":{"type":"integer"},"b":{"type":"integer"}},"required":["a","b"],"type":"object"}`), &addTool)
	json.Unmarshal([]byte(`{"properties":{"a":{"type":"integer"},"b":{"type":"integer"}},"required":["a","b"],"type":"object"}`), &multiplyTool)

	return &types.ChatCompletionRequest{
		Model: c.ModelName,
		Messages: []types.ChatCompletionMessage{
			{
				Role:    types.ChatMessageRoleUser,
				Content: "What is 3 * 12? And what is 11 + 49? Please use function calls.",
			},
		},
		Tools: []*types.ChatCompletionTool{
			{
				Type: "function",
				Function: types.ChatCompletionFunction{
					Name:        "add",
					Description: "Adds a and b.\n\n    Args:\n        a: first int\n        b: second int",
					Parameters:  addTool,
				},
			},
			{
				Type: "function",
				Function: types.ChatCompletionFunction{
					Name:        "multiply",
					Description: "Multiplies a and b.\n\n    Args:\n        a: first int\n        b: second int",
					Parameters:  multiplyTool,
				},
			},
		},
	}
}

func (c *CheckToolProcess) Check(req *types.ChatCompletionRequest, resp *types.ChatCompletionResponse, openaiErr *types.OpenAIError) []*CheckResult {
	checkResults := make([]*CheckResult, 0)

	if openaiErr != nil {
		checkResults = append(checkResults, &CheckResult{
			Name:   "Response",
			Status: CheckStatusFailed,
			Remark: openaiErr.Message,
		})
		return checkResults
	}

	if len(resp.Choices) == 0 {
		checkResults = append(checkResults, &CheckResult{
			Name:   "Function call check",
			Status: CheckStatusFailed,
			Remark: "Failed to get response data",
		})
		return checkResults
	}

	result := &CheckResult{
		Name:   "Function call check",
		Status: CheckStatusFailed,
		Remark: "",
	}

	firstChoice := resp.Choices[0]

	if len(firstChoice.Message.ToolCalls) == 0 {
		result.Remark = "No function calls were used"
		checkResults = append(checkResults, result)
		return checkResults
	}

	result.Remark = fmt.Sprintf("Used %d function call(s) (2 indicates strong model comprehension)", len(firstChoice.Message.ToolCalls))
	result.Status = CheckStatusSuccess
	checkResults = append(checkResults, result)

	return checkResults
}
