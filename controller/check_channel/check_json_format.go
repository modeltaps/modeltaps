package check_channel

import (
	"github.com/modeltaps/modeltaps/types"
	"encoding/json"
)

type CheckJsonFormatProcess struct {
	ModelName string
}

func CreateCheckJsonFormatProcess(modelName string) *CheckJsonFormatProcess {
	return &CheckJsonFormatProcess{
		ModelName: modelName,
	}
}

func (c *CheckJsonFormatProcess) GetName() string {
	return "JSON format check"
}

func (c *CheckJsonFormatProcess) GetRequest() *types.ChatCompletionRequest {
	jsonSchema := map[string]interface{}{}
	json.Unmarshal([]byte(`{"type":"object","properties":{"steps":{"type":"array","items":{"type":"object","properties":{"explanation":{"type":"string"},"output":{"type":"string"}},"required":["explanation","output"],"additionalProperties":false}},"final_answer":{"type":"string"}},"required":["steps","final_answer"],"additionalProperties":false}`), &jsonSchema)

	return &types.ChatCompletionRequest{
		Model: c.ModelName,
		Messages: []types.ChatCompletionMessage{
			{
				Role:    types.ChatMessageRoleSystem,
				Content: "You are a helpful math tutor. Guide the user through the solution step by step.",
			},
			{
				Role:    types.ChatMessageRoleUser,
				Content: "how can I solve 8x + 7 = -23",
			},
		},
		ResponseFormat: &types.ChatCompletionResponseFormat{
			Type: "json_schema",
			JsonSchema: &types.FormatJsonSchema{
				Name:   "math_reasoning",
				Schema: jsonSchema,
				Strict: true,
			},
		},
	}
}

func (c *CheckJsonFormatProcess) Check(req *types.ChatCompletionRequest, resp *types.ChatCompletionResponse, openaiErr *types.OpenAIError) []*CheckResult {
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
			Name:   "Structured output check",
			Status: CheckStatusFailed,
			Remark: "Failed to get response data",
		})
		return checkResults
	}

	result := &CheckResult{
		Name:   "Structured output check",
		Status: CheckStatusFailed,
		Remark: "",
	}

	firstChoice := resp.Choices[0]

	content := firstChoice.Message.StringContent()

	var jsonSchema map[string]interface{}
	// 判断content是否是json
	err := json.Unmarshal([]byte(content), &jsonSchema)
	if err != nil {
		result.Remark = "Response is not JSON"
	} else {
		result.Remark = "Response is JSON"
		result.Status = CheckStatusSuccess
	}

	checkResults = append(checkResults, result)

	return checkResults
}
