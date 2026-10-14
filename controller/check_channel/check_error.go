package check_channel

import (
	"strings"

	"github.com/modeltaps/modeltaps/types"
)

type CheckErrorProcess struct {
	ModelName string
}

func CreateCheckErrorProcess(modelName string) *CheckErrorProcess {
	return &CheckErrorProcess{ModelName: modelName}
}

const errorProbeKey = "relay"

func (c *CheckErrorProcess) GetKey() string {
	return errorProbeKey
}

// GetRequest sends an invalid role on purpose to make the upstream fail.
func (c *CheckErrorProcess) GetRequest() *types.ChatCompletionRequest {
	return &types.ChatCompletionRequest{
		Model: c.ModelName,
		Messages: []types.ChatCompletionMessage{
			{Role: types.ChatMessageRoleUser, Content: "hi"},
			{Role: "user11", Content: "hi"},
		},
	}
}

// Check counts the "request id:" markers that gateways of this family append to error messages.
// It is only a clue: a relay can rewrite the error or clean the request before forwarding it, so
// it never fails and does not take part in the overall verdict. No marker passes; one or more is
// a warning, so a likely relay is flagged.
func (c *CheckErrorProcess) Check(ctx CheckContext, _ *types.ChatCompletionRequest, _ *types.ChatCompletionResponse, openaiErr *types.OpenAIError) []*CheckResult {
	if ctx.RewritesUnknownRole {
		// The Claude / Gemini conversion maps the invalid role to a valid one, so the upstream never
		// sees an invalid request.
		return []*CheckResult{{Name: resultRequestID, Status: CheckStatusSkipped, Code: "request_id_role_rewritten"}}
	}
	if openaiErr == nil {
		return []*CheckResult{{Name: resultRequestID, Status: CheckStatusWarning, Code: "request_id_no_error"}}
	}

	count := strings.Count(strings.ToLower(openaiErr.Message), "request id:")
	if count == 0 {
		return []*CheckResult{{Name: resultRequestID, Status: CheckStatusSuccess, Code: "request_id_none"}}
	}
	return []*CheckResult{{Name: resultRequestID, Status: CheckStatusWarning, Code: "request_id_found", Params: map[string]any{"count": count}}}
}
