package check_channel

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/modeltaps/modeltaps/types"
)

type CheckJsonFormatProcess struct {
	ModelName string
}

func CreateCheckJsonFormatProcess(modelName string) *CheckJsonFormatProcess {
	return &CheckJsonFormatProcess{ModelName: modelName}
}

const jsonProbeKey = "json"

func (c *CheckJsonFormatProcess) GetKey() string {
	return jsonProbeKey
}

const jsonExpectedAnswer = -3.75

func (c *CheckJsonFormatProcess) GetRequest() *types.ChatCompletionRequest {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"steps": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"explanation": map[string]any{"type": "string"},
						"output":      map[string]any{"type": "string"},
					},
					"required":             []string{"explanation", "output"},
					"additionalProperties": false,
				},
			},
			"final_answer": map[string]any{"type": "string"},
		},
		"required":             []string{"steps", "final_answer"},
		"additionalProperties": false,
	}

	return &types.ChatCompletionRequest{
		Model: c.ModelName,
		Messages: []types.ChatCompletionMessage{
			{Role: types.ChatMessageRoleSystem, Content: "You are a helpful math tutor. Guide the user through the solution step by step."},
			{Role: types.ChatMessageRoleUser, Content: "Solve 8x + 7 = -23. Put only the value of x in final_answer."},
		},
		ResponseFormat: &types.ChatCompletionResponseFormat{
			Type: "json_schema",
			JsonSchema: &types.FormatJsonSchema{
				Name:   "math_reasoning",
				Schema: schema,
				Strict: true,
			},
		},
	}
}

var jsonCodeFence = regexp.MustCompile("(?s)^```(?:json)?\\s*(.*?)\\s*```$")
var decimalPattern = regexp.MustCompile(`-?\d+(?:\.\d+)?(?:/\d+)?`)

type mathReasoning struct {
	Steps []struct {
		Explanation string `json:"explanation"`
		Output      string `json:"output"`
	} `json:"steps"`
	FinalAnswer *string `json:"final_answer"`
}

func (c *CheckJsonFormatProcess) Check(ctx CheckContext, _ *types.ChatCompletionRequest, resp *types.ChatCompletionResponse, openaiErr *types.OpenAIError) []*CheckResult {
	if openaiErr != nil {
		return failedResponse(openaiErr)
	}

	content, ok := firstChoiceContent(resp)
	if !ok {
		return []*CheckResult{noContent(resultJSONSchema)}
	}

	content = strings.TrimSpace(content)
	fenced := false
	if m := jsonCodeFence.FindStringSubmatch(content); m != nil {
		content, fenced = m[1], true
	}

	var parsed mathReasoning
	err := json.Unmarshal([]byte(content), &parsed)
	shapeOK := err == nil && parsed.FinalAnswer != nil && len(parsed.Steps) > 0
	if ctx.ClaudeConversion && !shapeOK {
		// The Claude conversion drops response_format (as Anthropic's own OpenAI compatibility layer
		// does), so the upstream never saw the schema; not following it is expected, not a failure.
		return []*CheckResult{{Name: resultJSONSchema, Status: CheckStatusSkipped, Code: "json_skipped_claude"}}
	}
	if err != nil {
		return []*CheckResult{{Name: resultJSONSchema, Status: CheckStatusFailed, Code: "json_invalid", Params: map[string]any{"content": truncate(content, 120)}}}
	}
	if !shapeOK {
		return []*CheckResult{{Name: resultJSONSchema, Status: CheckStatusFailed, Code: "json_missing_fields"}}
	}
	if !answerEquals(*parsed.FinalAnswer, jsonExpectedAnswer) {
		return []*CheckResult{{
			Name:   resultJSONSchema,
			Status: CheckStatusWarning,
			Code:   "json_answer_mismatch",
			Params: map[string]any{"answer": truncate(*parsed.FinalAnswer, 60), "expected": jsonExpectedAnswer},
		}}
	}

	params := map[string]any{"steps": len(parsed.Steps)}
	switch {
	case ctx.ClaudeConversion:
		// The model chose to answer in JSON, but without the schema reaching it this says nothing
		// about structured output support.
		return []*CheckResult{{Name: resultJSONSchema, Status: CheckStatusWarning, Code: "json_passed_unconstrained", Params: params}}
	case fenced:
		// Native json_schema output is never wrapped in a code fence; a fence means the upstream emulates it with prompting.
		return []*CheckResult{{Name: resultJSONSchema, Status: CheckStatusWarning, Code: "json_passed_fenced", Params: params}}
	}
	return []*CheckResult{{Name: resultJSONSchema, Status: CheckStatusSuccess, Code: "json_passed", Params: params}}
}

// answerEquals accepts -3.75, -15/4, x = -3.75 and similar spellings.
func answerEquals(answer string, want float64) bool {
	for _, m := range decimalPattern.FindAllString(answer, -1) {
		var v float64
		if num, den, isFrac := strings.Cut(m, "/"); isFrac {
			n, err1 := strconv.ParseFloat(num, 64)
			d, err2 := strconv.ParseFloat(den, 64)
			if err1 != nil || err2 != nil || d == 0 {
				continue
			}
			v = n / d
		} else {
			f, err := strconv.ParseFloat(m, 64)
			if err != nil {
				continue
			}
			v = f
		}
		if math.Abs(v-want) < 1e-9 {
			return true
		}
	}
	return false
}
