// Package protocol converts between the Anthropic Messages protocol and the OpenAI Responses
// protocol in one hop, without going through Chat Completions. Field coverage follows the
// LiteLLM Messages-to-Responses mapping; anything with no counterpart on the other side is
// dropped rather than approximated.
package protocol

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modeltaps/modeltaps/types"
)

// redactedThinkingPrefix marks a Responses reasoning item that carries a Claude
// redacted_thinking block instead of a thinking signature, so the block survives a round trip
// through a Responses client that sends reasoning items back.
const redactedThinkingPrefix = "anthropic-redacted:"

// claudeBlock is every field of an Anthropic content block the conversions read.
type claudeBlock struct {
	Type      string             `json:"type"`
	Text      string             `json:"text,omitempty"`
	Source    *claudeBlockSource `json:"source,omitempty"`
	Id        string             `json:"id,omitempty"`
	Name      string             `json:"name,omitempty"`
	Input     any                `json:"input,omitempty"`
	Content   any                `json:"content,omitempty"`
	ToolUseId string             `json:"tool_use_id,omitempty"`
	IsError   *bool              `json:"is_error,omitempty"`
	Thinking  string             `json:"thinking,omitempty"`
	Signature string             `json:"signature,omitempty"`
	Data      string             `json:"data,omitempty"`
	Title     string             `json:"title,omitempty"`
}

type claudeBlockSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	Url       string `json:"url,omitempty"`
	FileId    string `json:"file_id,omitempty"`
}

// parseClaudeBlocks reads a Messages content value: a string becomes one text block.
func parseClaudeBlocks(content any) []claudeBlock {
	switch v := content.(type) {
	case nil:
		return nil
	case string:
		return []claudeBlock{{Type: "text", Text: v}}
	}
	raw, err := json.Marshal(content)
	if err != nil {
		return nil
	}
	var blocks []claudeBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		var single claudeBlock
		if json.Unmarshal(raw, &single) == nil && single.Type != "" {
			return []claudeBlock{single}
		}
		return nil
	}
	return blocks
}

// claudeSystemText joins the text of a Messages system value (string or text blocks).
func claudeSystemText(system any) string {
	var parts []string
	for _, block := range parseClaudeBlocks(system) {
		if block.Type == "text" && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

// dataURL builds a data: URL from a media type and base64 data.
func dataURL(mediaType, data string) string {
	return fmt.Sprintf("data:%s;base64,%s", mediaType, data)
}

// splitDataURL splits a data: URL into its media type and base64 payload.
func splitDataURL(url string) (mediaType, data string, ok bool) {
	if !strings.HasPrefix(url, "data:") {
		return "", "", false
	}
	meta, payload, found := strings.Cut(strings.TrimPrefix(url, "data:"), ",")
	if !found || !strings.HasSuffix(meta, ";base64") {
		return "", "", false
	}
	return strings.TrimSuffix(meta, ";base64"), payload, true
}

// argumentsObject decodes tool-call arguments into the object an Anthropic tool_use input
// expects; malformed or empty arguments become an empty object.
func argumentsObject(arguments string) any {
	arguments = strings.TrimSpace(arguments)
	if arguments == "" {
		return map[string]any{}
	}
	var input any
	if err := json.Unmarshal([]byte(arguments), &input); err != nil {
		return map[string]any{}
	}
	if _, ok := input.(map[string]any); !ok {
		return map[string]any{}
	}
	return input
}

// argumentsString encodes an Anthropic tool_use input as Responses arguments.
func argumentsString(input any) string {
	if input == nil {
		return "{}"
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// sseEvent formats one server-sent event.
func sseEvent(event string, data any) string {
	raw, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return "event: " + event + "\ndata: " + string(raw) + "\n\n"
}

// sseDataLines splits stream chunks into the JSON payloads of their data: lines. Chunks may
// carry a whole event block or a single line, so partial lines are buffered until complete.
type sseDataLines struct {
	pending string
}

func (s *sseDataLines) feed(chunk string) []string {
	s.pending += chunk
	var payloads []string
	for {
		line, rest, found := strings.Cut(s.pending, "\n")
		if !found {
			break
		}
		s.pending = rest
		if payload, ok := dataPayload(line); ok {
			payloads = append(payloads, payload)
		}
	}
	return payloads
}

// flush returns the payload of a final line that had no newline.
func (s *sseDataLines) flush() []string {
	line := s.pending
	s.pending = ""
	if payload, ok := dataPayload(line); ok {
		return []string{payload}
	}
	return nil
}

func dataPayload(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "data:") {
		// Some upstreams send bare JSON lines.
		if strings.HasPrefix(line, "{") {
			return line, true
		}
		return "", false
	}
	payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if payload == "" || payload == "[DONE]" {
		return "", false
	}
	return payload, true
}

// claudeUsageFromBilling reports the billed usage in Anthropic form. OpenAI input counts include
// cached and cache-written tokens; Anthropic reports them apart from input_tokens.
func claudeUsageFromBilling(usage *types.Usage) map[string]any {
	if usage == nil {
		return map[string]any{"input_tokens": 0, "output_tokens": 0}
	}
	cacheRead := usage.PromptTokensDetails.CachedTokens
	if cacheRead == 0 {
		cacheRead = usage.PromptTokensDetails.CachedReadTokens
	}
	cacheWrite := usage.PromptTokensDetails.OpenAICacheWriteTokens
	input := usage.PromptTokens - cacheRead - cacheWrite
	if input < 0 {
		input = 0
	}
	out := map[string]any{
		"input_tokens":  input,
		"output_tokens": usage.CompletionTokens,
	}
	if cacheRead > 0 {
		out["cache_read_input_tokens"] = cacheRead
	}
	if cacheWrite > 0 {
		out["cache_creation_input_tokens"] = cacheWrite
	}
	return out
}

// responsesUsageFromBilling reports the billed usage in Responses form: input_tokens includes
// cache reads and writes, and cached_tokens is the cache read part.
func responsesUsageFromBilling(usage *types.Usage) *types.ResponsesUsage {
	if usage == nil {
		return &types.ResponsesUsage{InputTokensDetails: &types.ResponsesUsageInputTokensDetails{}}
	}
	cached := usage.PromptTokensDetails.CachedReadTokens
	if cached == 0 {
		cached = usage.PromptTokensDetails.CachedTokens
	}
	out := &types.ResponsesUsage{
		InputTokens:        usage.PromptTokens,
		OutputTokens:       usage.CompletionTokens,
		TotalTokens:        usage.PromptTokens + usage.CompletionTokens,
		InputTokensDetails: &types.ResponsesUsageInputTokensDetails{CachedTokens: cached},
	}
	if usage.CompletionTokensDetails.ReasoningTokens > 0 {
		out.OutputTokensDetails = &types.ResponsesUsageOutputTokensDetails{
			ReasoningTokens: usage.CompletionTokensDetails.ReasoningTokens,
		}
	}
	return out
}
