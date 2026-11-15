package protocol

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/types"
)

// This file serves an Anthropic Messages client from an OpenAI Responses upstream: the request
// goes Messages → Responses, the reply comes back Responses → Messages.

// MessagesToResponsesRequest converts an Anthropic Messages request to a Responses request.
// Thinking blocks in the history are dropped: a Responses upstream only accepts reasoning items
// it issued itself. cache_control is dropped too, since OpenAI caches prefixes on its own.
func MessagesToResponsesRequest(request *claude.ClaudeRequest) *types.OpenAIResponsesRequest {
	out := &types.OpenAIResponsesRequest{
		Model:           request.Model,
		Instructions:    claudeSystemText(request.System),
		MaxOutputTokens: request.MaxTokens,
		Temperature:     request.Temperature,
		TopP:            request.TopP,
		Stream:          request.Stream,
	}

	input := make([]map[string]any, 0, len(request.Messages))
	for _, message := range request.Messages {
		input = append(input, messageToResponsesItems(message)...)
	}
	out.Input = input

	for _, tool := range request.Tools {
		if converted, ok := claudeToolToResponses(tool); ok {
			out.Tools = append(out.Tools, converted)
		}
	}
	if len(out.Tools) > 0 {
		out.ParallelToolCalls = true
	}
	if request.ToolChoice != nil {
		out.ToolChoice = claudeToolChoiceToResponses(request.ToolChoice)
		if request.ToolChoice.DisableParallelToolUse {
			out.ParallelToolCalls = false
		}
	}

	if effort := claudeThinkingEffort(request.Thinking); effort != "" {
		summary := "auto"
		out.Reasoning = &types.ReasoningEffort{Effort: &effort, Summary: &summary}
	}

	return out
}

// messageToResponsesItems converts one Messages turn into Responses input items, keeping the
// order of text, tool calls and tool results.
func messageToResponsesItems(message claude.Message) []map[string]any {
	assistant := message.Role == types.ChatMessageRoleAssistant
	textType := types.ContentTypeInputText
	if assistant {
		textType = types.ContentTypeOutputText
	}

	var items []map[string]any
	var parts []map[string]any
	flush := func() {
		if len(parts) == 0 {
			return
		}
		items = append(items, map[string]any{
			"type":    types.InputTypeMessage,
			"role":    message.Role,
			"content": parts,
		})
		parts = nil
	}

	for _, block := range parseClaudeBlocks(message.Content) {
		switch block.Type {
		case "text":
			if block.Text != "" {
				parts = append(parts, map[string]any{"type": textType, "text": block.Text})
			}
		case "image":
			if part, ok := claudeImageToResponses(block); ok && !assistant {
				parts = append(parts, part)
			}
		case "document":
			if part, ok := claudeDocumentToResponses(block); ok && !assistant {
				parts = append(parts, part)
			}
		case "tool_use", "server_tool_use":
			flush()
			items = append(items, map[string]any{
				"type":      types.InputTypeFunctionCall,
				"call_id":   block.Id,
				"name":      block.Name,
				"arguments": argumentsString(block.Input),
			})
		case "tool_result":
			flush()
			items = append(items, map[string]any{
				"type":    types.InputTypeFunctionCallOutput,
				"call_id": block.ToolUseId,
				"output":  toolResultToResponsesOutput(block),
			})
		}
	}
	flush()
	return items
}

func claudeImageToResponses(block claudeBlock) (map[string]any, bool) {
	if block.Source == nil {
		return nil, false
	}
	switch block.Source.Type {
	case "base64":
		mediaType := block.Source.MediaType
		if mediaType == "" {
			mediaType = "image/png"
		}
		return map[string]any{"type": types.ContentTypeInputImage, "image_url": dataURL(mediaType, block.Source.Data), "detail": "auto"}, true
	case "url":
		return map[string]any{"type": types.ContentTypeInputImage, "image_url": block.Source.Url, "detail": "auto"}, true
	case "file":
		return map[string]any{"type": types.ContentTypeInputImage, "file_id": block.Source.FileId, "detail": "auto"}, true
	}
	return nil, false
}

func claudeDocumentToResponses(block claudeBlock) (map[string]any, bool) {
	if block.Source == nil {
		return nil, false
	}
	name := block.Title
	if name == "" {
		name = "document.pdf"
	}
	switch block.Source.Type {
	case "base64":
		mediaType := block.Source.MediaType
		if mediaType == "" {
			mediaType = "application/pdf"
		}
		return map[string]any{"type": types.ContentTypeInputFile, "filename": name, "file_data": dataURL(mediaType, block.Source.Data)}, true
	case "url":
		return map[string]any{"type": types.ContentTypeInputFile, "file_url": block.Source.Url}, true
	case "file":
		return map[string]any{"type": types.ContentTypeInputFile, "file_id": block.Source.FileId}, true
	case "text":
		return map[string]any{"type": types.ContentTypeInputText, "text": block.Source.Data}, true
	}
	return nil, false
}

// toolResultToResponsesOutput gives a function_call_output its output: plain text when the
// result is only text, otherwise a list of input parts so images reach the model.
func toolResultToResponsesOutput(block claudeBlock) any {
	if text, ok := block.Content.(string); ok {
		return text
	}
	blocks := parseClaudeBlocks(block.Content)
	var texts []string
	var parts []map[string]any
	hasImage := false
	for _, inner := range blocks {
		switch inner.Type {
		case "text":
			texts = append(texts, inner.Text)
			parts = append(parts, map[string]any{"type": types.ContentTypeInputText, "text": inner.Text})
		case "image":
			if part, ok := claudeImageToResponses(inner); ok {
				hasImage = true
				parts = append(parts, part)
			}
		}
	}
	if hasImage {
		return parts
	}
	return strings.Join(texts, "\n")
}

func claudeToolToResponses(tool claude.Tools) (types.ResponsesTools, bool) {
	switch {
	case tool.Type == "" || tool.Type == "custom":
		parameters := tool.InputSchema
		if parameters == nil {
			parameters = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		return types.ResponsesTools{
			Type:        "function",
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  parameters,
		}, true
	case strings.HasPrefix(tool.Type, "web_search"):
		return types.ResponsesTools{Type: "web_search"}, true
	}
	// Anthropic-defined client tools (bash, text_editor, computer) have no Responses counterpart.
	return types.ResponsesTools{}, false
}

func claudeToolChoiceToResponses(choice *claude.ToolChoice) any {
	switch choice.Type {
	case "any":
		return "required"
	case "none":
		return "none"
	case "tool":
		return map[string]any{"type": "function", "name": choice.Name}
	}
	return "auto"
}

// claudeThinkingEffort maps an Anthropic thinking budget to a Responses reasoning effort.
func claudeThinkingEffort(thinking *claude.Thinking) string {
	if thinking == nil {
		return ""
	}
	switch thinking.Type {
	case "enabled":
		switch {
		case thinking.BudgetTokens >= 10000:
			return "high"
		case thinking.BudgetTokens >= 5000:
			return "medium"
		default:
			return "low"
		}
	case "adaptive":
		return "medium"
	}
	return ""
}

// ResponsesToMessagesResponse converts a Responses reply to an Anthropic Messages reply. The
// usage is the billed usage the provider recorded, so the client sees what is charged.
func ResponsesToMessagesResponse(response *types.OpenAIResponsesResponses, model string, usage *types.Usage) map[string]any {
	content := make([]map[string]any, 0, len(response.Output))
	hasToolUse := false
	for _, item := range response.Output {
		switch item.Type {
		case types.InputTypeReasoning:
			if block, ok := reasoningToThinkingBlock(item.GetSummaryString(), item.EncryptedContent); ok {
				content = append(content, block)
			}
		case types.InputTypeMessage:
			for _, text := range responsesMessageTexts(item.Content) {
				content = append(content, map[string]any{"type": "text", "text": text})
			}
		case types.InputTypeFunctionCall:
			hasToolUse = true
			content = append(content, map[string]any{
				"type":  "tool_use",
				"id":    item.CallID,
				"name":  item.Name,
				"input": argumentsObject(item.ArgumentsString()),
			})
		}
	}

	if model == "" {
		model = response.Model
	}
	return map[string]any{
		"id":            messageIDFromResponseID(response.ID),
		"type":          "message",
		"role":          "assistant",
		"model":         model,
		"content":       content,
		"stop_reason":   responsesStopReason(response.Status, response.IncompleteDetail, hasToolUse),
		"stop_sequence": nil,
		"usage":         claudeUsageFromBilling(usage),
	}
}

// reasoningToThinkingBlock turns a reasoning item back into the Claude block it came from when
// it carries a Claude signature, or into a thinking block with the summary text otherwise.
func reasoningToThinkingBlock(summary string, encrypted *string) (map[string]any, bool) {
	signature := ""
	if encrypted != nil {
		signature = *encrypted
	}
	if data, ok := strings.CutPrefix(signature, redactedThinkingPrefix); ok {
		return map[string]any{"type": "redacted_thinking", "data": data}, true
	}
	if summary == "" && signature == "" {
		return nil, false
	}
	return map[string]any{"type": "thinking", "thinking": summary, "signature": signature}, true
}

// responsesMessageTexts returns the text of each output_text and refusal part of a message.
func responsesMessageTexts(content any) []string {
	if text, ok := content.(string); ok {
		if text == "" {
			return nil
		}
		return []string{text}
	}
	raw, err := json.Marshal(content)
	if err != nil {
		return nil
	}
	var parts []struct {
		Type    string `json:"type"`
		Text    string `json:"text"`
		Refusal any    `json:"refusal"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return nil
	}
	var texts []string
	for _, part := range parts {
		switch part.Type {
		case types.ContentTypeOutputText, types.ContentTypeInputText:
			if part.Text != "" {
				texts = append(texts, part.Text)
			}
		case types.ContentTypeRefusal:
			if refusal, ok := part.Refusal.(string); ok && refusal != "" {
				texts = append(texts, refusal)
			}
		}
	}
	return texts
}

func responsesStopReason(status string, incomplete *types.IncompleteDetail, hasToolUse bool) string {
	if status == types.ResponseStatusIncomplete && incomplete != nil {
		switch incomplete.Reason {
		case "max_output_tokens":
			return "max_tokens"
		case "content_filter":
			return "refusal"
		}
	}
	if hasToolUse {
		return "tool_use"
	}
	return "end_turn"
}

func messageIDFromResponseID(id string) string {
	if id == "" {
		return "msg_" + utils.GetUUID()
	}
	return "msg_" + strings.TrimPrefix(id, "resp_")
}

// ResponsesToMessagesStream converts a Responses event stream to an Anthropic Messages event
// stream. Feed it the upstream chunks in order; it returns the SSE text to write.
type ResponsesToMessagesStream struct {
	model string
	usage *types.Usage

	lines        sseDataLines
	started      bool
	finished     bool
	nextIndex    int
	open         map[int]*openClaudeBlock // by Responses output_index
	hasToolUse   bool
	status       string
	incomplete   *types.IncompleteDetail
	responseID   string
	summaryParts map[int]int // reasoning output_index → summary parts seen
}

type openClaudeBlock struct {
	index     int
	kind      string // text / thinking / tool_use
	argsSent  bool
	signature string
}

// NewResponsesToMessagesStream starts a stream conversion. usage is the provider's billed usage,
// read when the stream ends.
func NewResponsesToMessagesStream(model string, usage *types.Usage) *ResponsesToMessagesStream {
	return &ResponsesToMessagesStream{
		model:        model,
		usage:        usage,
		open:         map[int]*openClaudeBlock{},
		summaryParts: map[int]int{},
	}
}

// Process converts one upstream chunk.
func (s *ResponsesToMessagesStream) Process(chunk string) string {
	var out strings.Builder
	for _, payload := range s.lines.feed(chunk) {
		out.WriteString(s.processEvent(payload))
	}
	return out.String()
}

// Finish closes the message if the upstream ended without a terminal event.
func (s *ResponsesToMessagesStream) Finish() string {
	var out strings.Builder
	for _, payload := range s.lines.flush() {
		out.WriteString(s.processEvent(payload))
	}
	out.WriteString(s.finish())
	return out.String()
}

// Error reports an upstream error as an Anthropic error event.
func (s *ResponsesToMessagesStream) Error(message string) string {
	s.finished = true
	return sseEvent("error", map[string]any{
		"type":  "error",
		"error": map[string]any{"type": "api_error", "message": message},
	})
}

func (s *ResponsesToMessagesStream) processEvent(payload string) string {
	if s.finished {
		return ""
	}
	var event types.OpenAIResponsesStreamResponses
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return ""
	}

	var out strings.Builder
	if !s.started && event.Type != "error" {
		out.WriteString(s.start(event.Response))
	}

	switch event.Type {
	case "response.output_item.added":
		if event.Item != nil && event.OutputIndex != nil {
			out.WriteString(s.openBlock(*event.OutputIndex, event.Item))
		}
	case "response.output_text.delta", "response.refusal.delta":
		if delta, ok := event.Delta.(string); ok && delta != "" && event.OutputIndex != nil {
			block := s.ensureBlock(&out, *event.OutputIndex, "text")
			out.WriteString(sseEvent("content_block_delta", map[string]any{
				"type": "content_block_delta", "index": block.index,
				"delta": map[string]any{"type": "text_delta", "text": delta},
			}))
		}
	case "response.reasoning_summary_part.added":
		if event.OutputIndex != nil {
			block := s.ensureBlock(&out, *event.OutputIndex, "thinking")
			s.summaryParts[*event.OutputIndex]++
			if s.summaryParts[*event.OutputIndex] > 1 {
				out.WriteString(thinkingDelta(block.index, "\n\n"))
			}
		}
	case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
		if delta, ok := event.Delta.(string); ok && delta != "" && event.OutputIndex != nil {
			block := s.ensureBlock(&out, *event.OutputIndex, "thinking")
			out.WriteString(thinkingDelta(block.index, delta))
		}
	case "response.function_call_arguments.delta":
		if delta, ok := event.Delta.(string); ok && delta != "" && event.OutputIndex != nil {
			if block := s.open[*event.OutputIndex]; block != nil && block.kind == "tool_use" {
				block.argsSent = true
				out.WriteString(inputJSONDelta(block.index, delta))
			}
		}
	case "response.output_item.done":
		if event.OutputIndex != nil {
			out.WriteString(s.closeBlock(*event.OutputIndex, event.Item))
		}
	case "response.completed", "response.incomplete", "response.done":
		if event.Response != nil {
			s.status = event.Response.Status
			s.incomplete = event.Response.IncompleteDetail
		}
		out.WriteString(s.finish())
	case "response.failed":
		message := "upstream response failed"
		if event.Response != nil && event.Response.Error != nil && event.Response.Error.Message != "" {
			message = event.Response.Error.Message
		}
		out.WriteString(s.Error(message))
	case "error":
		message := "upstream error"
		if event.Message != nil {
			message = *event.Message
		}
		out.WriteString(s.Error(message))
	}
	return out.String()
}

func (s *ResponsesToMessagesStream) start(response *types.OpenAIResponsesResponses) string {
	s.started = true
	if response != nil {
		s.responseID = response.ID
	}
	model := s.model
	if model == "" && response != nil {
		model = response.Model
	}
	inputTokens := 0
	if s.usage != nil {
		inputTokens = s.usage.PromptTokens
	}
	return sseEvent("message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id": messageIDFromResponseID(s.responseID), "type": "message", "role": "assistant",
			"model": model, "content": []any{}, "stop_reason": nil, "stop_sequence": nil,
			"usage": map[string]any{"input_tokens": inputTokens, "output_tokens": 0},
		},
	})
}

func (s *ResponsesToMessagesStream) openBlock(outputIndex int, item *types.ResponsesOutput) string {
	switch item.Type {
	case types.InputTypeMessage:
		return s.startBlock(outputIndex, "text", map[string]any{"type": "text", "text": ""})
	case types.InputTypeReasoning:
		return s.startBlock(outputIndex, "thinking", map[string]any{"type": "thinking", "thinking": ""})
	case types.InputTypeFunctionCall:
		s.hasToolUse = true
		return s.startBlock(outputIndex, "tool_use", map[string]any{
			"type": "tool_use", "id": item.CallID, "name": item.Name, "input": map[string]any{},
		})
	}
	return ""
}

func (s *ResponsesToMessagesStream) startBlock(outputIndex int, kind string, contentBlock map[string]any) string {
	if block := s.open[outputIndex]; block != nil {
		return ""
	}
	block := &openClaudeBlock{index: s.nextIndex, kind: kind}
	s.nextIndex++
	s.open[outputIndex] = block
	return sseEvent("content_block_start", map[string]any{
		"type": "content_block_start", "index": block.index, "content_block": contentBlock,
	})
}

// ensureBlock opens a block for a delta whose output_item.added was not seen.
func (s *ResponsesToMessagesStream) ensureBlock(out *strings.Builder, outputIndex int, kind string) *openClaudeBlock {
	if block := s.open[outputIndex]; block != nil {
		return block
	}
	if kind == "thinking" {
		out.WriteString(s.startBlock(outputIndex, kind, map[string]any{"type": "thinking", "thinking": ""}))
	} else {
		out.WriteString(s.startBlock(outputIndex, kind, map[string]any{"type": "text", "text": ""}))
	}
	return s.open[outputIndex]
}

func (s *ResponsesToMessagesStream) closeBlock(outputIndex int, item *types.ResponsesOutput) string {
	block := s.open[outputIndex]
	if block == nil {
		return ""
	}
	var out strings.Builder
	if item != nil {
		switch block.kind {
		case "tool_use":
			if !block.argsSent {
				if args := item.ArgumentsString(); args != "" {
					out.WriteString(inputJSONDelta(block.index, args))
				}
			}
		case "thinking":
			if item.EncryptedContent != nil && *item.EncryptedContent != "" {
				out.WriteString(sseEvent("content_block_delta", map[string]any{
					"type": "content_block_delta", "index": block.index,
					"delta": map[string]any{"type": "signature_delta", "signature": *item.EncryptedContent},
				}))
			}
		}
	}
	out.WriteString(sseEvent("content_block_stop", map[string]any{"type": "content_block_stop", "index": block.index}))
	delete(s.open, outputIndex)
	return out.String()
}

func (s *ResponsesToMessagesStream) finish() string {
	if s.finished {
		return ""
	}
	var out strings.Builder
	if !s.started {
		out.WriteString(s.start(nil))
	}
	// Close any block still open, in index order.
	for len(s.open) > 0 {
		lowest, lowestIndex := -1, 0
		for outputIndex, block := range s.open {
			if lowest == -1 || block.index < lowestIndex {
				lowest, lowestIndex = outputIndex, block.index
			}
		}
		out.WriteString(s.closeBlock(lowest, nil))
	}
	s.finished = true
	out.WriteString(sseEvent("message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": responsesStopReason(s.status, s.incomplete, s.hasToolUse), "stop_sequence": nil},
		"usage": claudeUsageFromBilling(s.usage),
	}))
	out.WriteString(sseEvent("message_stop", map[string]any{"type": "message_stop"}))
	return out.String()
}

func thinkingDelta(index int, text string) string {
	return sseEvent("content_block_delta", map[string]any{
		"type": "content_block_delta", "index": index,
		"delta": map[string]any{"type": "thinking_delta", "thinking": text},
	})
}

func inputJSONDelta(index int, partial string) string {
	return sseEvent("content_block_delta", map[string]any{
		"type": "content_block_delta", "index": index,
		"delta": map[string]any{"type": "input_json_delta", "partial_json": partial},
	})
}

// nowUnix is the created_at of synthesized Responses replies; tests may replace it.
var nowUnix = func() int64 { return time.Now().Unix() }
