package check_channel

import (
	"encoding/json"
	"fmt"

	"github.com/modeltaps/modeltaps/types"
)

type CheckToolProcess struct {
	ModelName string
}

func CreateCheckToolProcess(modelName string) *CheckToolProcess {
	return &CheckToolProcess{ModelName: modelName}
}

const toolProbeKey = "tool"

func (c *CheckToolProcess) GetKey() string {
	return toolProbeKey
}

func intPairSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"a": map[string]any{"type": "integer"},
			"b": map[string]any{"type": "integer"},
		},
		"required": []string{"a", "b"},
	}
}

func (c *CheckToolProcess) GetRequest() *types.ChatCompletionRequest {
	return &types.ChatCompletionRequest{
		Model: c.ModelName,
		Messages: []types.ChatCompletionMessage{
			{Role: types.ChatMessageRoleUser, Content: "What is 3 * 12, and what is 11 + 49? You must use the provided tools; do not compute the answers yourself."},
		},
		Tools: []*types.ChatCompletionTool{
			{Type: "function", Function: types.ChatCompletionFunction{Name: "add", Description: "Adds a and b.", Parameters: intPairSchema()}},
			{Type: "function", Function: types.ChatCompletionFunction{Name: "multiply", Description: "Multiplies a and b.", Parameters: intPairSchema()}},
		},
	}
}

type expectedCall struct {
	name string
	a, b int
}

func (e expectedCall) String() string {
	return fmt.Sprintf("%s(%d, %d)", e.name, e.a, e.b)
}

var expectedToolCalls = []expectedCall{{"multiply", 3, 12}, {"add", 11, 49}}

func (c *CheckToolProcess) Check(_ CheckContext, _ *types.ChatCompletionRequest, resp *types.ChatCompletionResponse, openaiErr *types.OpenAIError) []*CheckResult {
	if openaiErr != nil {
		return failedResponse(openaiErr)
	}
	if resp == nil || len(resp.Choices) == 0 {
		return []*CheckResult{noContent(resultToolCall)}
	}
	return []*CheckResult{matchToolCalls(resp.Choices[0].Message.ToolCalls)}
}

// matchToolCalls compares the calls with expectedToolCalls. Params.calls lists what the model
// called, in order, as name(a, b); calls whose arguments cannot be parsed are listed as name(?).
func matchToolCalls(calls []*types.ChatCompletionToolCalls) *CheckResult {
	if len(calls) == 0 {
		return &CheckResult{Name: resultToolCall, Status: CheckStatusFailed, Code: "tool_none"}
	}

	got := make(map[expectedCall]bool, len(calls))
	gotNames := make([]string, 0, len(calls))
	for _, call := range calls {
		if call == nil || call.Function == nil {
			continue
		}
		// float64 because some upstreams (e.g. through the Gemini conversion) emit integers as 3.0.
		var args struct {
			A float64 `json:"a"`
			B float64 `json:"b"`
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			gotNames = append(gotNames, call.Function.Name+"(?)")
			continue
		}
		ec := expectedCall{name: call.Function.Name, a: int(args.A), b: int(args.B)}
		gotNames = append(gotNames, ec.String())
		// Addition and multiplication commute, so add(49, 11) is correct too.
		if ec.a > ec.b {
			ec.a, ec.b = ec.b, ec.a
		}
		got[ec] = true
	}

	matched := 0
	for _, want := range expectedToolCalls {
		if got[want] {
			matched++
		}
	}
	params := map[string]any{"calls": gotNames}
	switch matched {
	case len(expectedToolCalls):
		return &CheckResult{Name: resultToolCall, Status: CheckStatusSuccess, Code: "tool_passed", Params: params}
	case 0:
		return &CheckResult{Name: resultToolCall, Status: CheckStatusFailed, Code: "tool_wrong", Params: params}
	}
	// One correct call still means tool calling works, just not in parallel.
	params["matched"] = matched
	params["total"] = len(expectedToolCalls)
	return &CheckResult{Name: resultToolCall, Status: CheckStatusWarning, Code: "tool_partial", Params: params}
}
