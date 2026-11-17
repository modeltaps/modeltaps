package types

import "encoding/json"

// DecisionRequest is the part of an OpenAI Decisions API request (POST /v1/decisions) the
// gateway reads to route and bill it. The body goes upstream as the client sent it, with only
// the model renamed, so fields added to the API later are not lost.
// https://developers.openai.com/api/reference/resources/decisions/methods/create
type DecisionRequest struct {
	Model     string          `json:"model" binding:"required"`
	Input     json.RawMessage `json:"input"`
	Questions json.RawMessage `json:"questions"`
}

// DecisionUsage is the usage a Decisions API response reports. Decision models bill input only.
type DecisionUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}
