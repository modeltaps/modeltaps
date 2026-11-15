package protocol

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/types"
)

// This file serves an OpenAI Responses client from an Anthropic Messages upstream: the request
// goes Responses → Messages, the reply comes back Messages → Responses.

// reasoningBudgets maps a Responses reasoning effort to an Anthropic thinking budget.
var reasoningBudgets = map[string]int{
	"minimal": 1024,
	"low":     2048,
	"medium":  8192,
	"high":    16384,
	"xhigh":   32768,
}

// ResponsesToMessagesRequest converts a Responses request to an Anthropic Messages request.
// defaultMaxTokens fills max_tokens, which Anthropic requires, when the client sets no limit.
func ResponsesToMessagesRequest(request *types.OpenAIResponsesRequest, defaultMaxTokens int) (*claude.ClaudeRequest, error) {
	out := &claude.ClaudeRequest{
		Model:       request.Model,
		MaxTokens:   request.MaxOutputTokens,
		Temperature: request.Temperature,
		TopP:        request.TopP,
		Stream:      request.Stream,
	}
	if out.MaxTokens <= 0 {
		out.MaxTokens = defaultMaxTokens
	}
	if out.MaxTokens <= 0 {
		out.MaxTokens = 8192
	}
	// Recent Claude models reject temperature and top_p together.
	if out.Temperature != nil && out.TopP != nil {
		out.TopP = nil
	}

	var system []string
	if strings.TrimSpace(request.Instructions) != "" {
		system = append(system, request.Instructions)
	}

	items, err := request.ParseInput()
	if err != nil {
		return nil, err
	}
	builder := &claudeMessageBuilder{}
	for _, item := range items {
		if text, ok := responsesSystemText(item); ok {
			if text != "" {
				system = append(system, text)
			}
			continue
		}
		builder.add(item)
	}
	out.Messages = builder.messages
	if len(out.Messages) == 0 {
		return nil, errors.New("input has no messages")
	}
	if len(system) > 0 {
		out.System = strings.Join(system, "\n\n")
	}

	for _, tool := range request.Tools {
		if converted, ok := responsesToolToClaude(tool); ok {
			out.Tools = append(out.Tools, converted)
		}
	}
	if len(out.Tools) > 0 {
		out.ToolChoice = responsesToolChoiceToClaude(request.ToolChoice)
	}

	if request.Reasoning != nil && request.Reasoning.Effort != nil {
		if budget, ok := reasoningBudgets[strings.ToLower(*request.Reasoning.Effort)]; ok {
			forcedTool := out.ToolChoice != nil && (out.ToolChoice.Type == "any" || out.ToolChoice.Type == "tool")
			if !forcedTool {
				out.Thinking = &claude.Thinking{Type: "enabled", BudgetTokens: budget}
				if out.MaxTokens <= budget {
					out.MaxTokens = budget + out.MaxTokens
				}
				// Thinking requires the default sampling.
				out.Temperature = nil
				out.TopP = nil
			}
		}
	}

	return out, nil
}

// responsesSystemText reports whether an input item is a system or developer message, and
// its text.
func responsesSystemText(item types.InputResponses) (string, bool) {
	if item.Type != "" && item.Type != types.InputTypeMessage {
		return "", false
	}
	if item.Role != types.ChatMessageRoleSystem && item.Role != types.ChatMessageRoleDeveloper {
		return "", false
	}
	var texts []string
	for _, part := range responsesContentParts(item.Content) {
		if part.Text != "" {
			texts = append(texts, part.Text)
		}
	}
	return strings.Join(texts, "\n\n"), true
}

func responsesContentParts(content any) []types.ContentResponses {
	if text, ok := content.(string); ok {
		return []types.ContentResponses{{Type: types.ContentTypeInputText, Text: text}}
	}
	raw, err := json.Marshal(content)
	if err != nil {
		return nil
	}
	var parts []types.ContentResponses
	if json.Unmarshal(raw, &parts) != nil {
		return nil
	}
	return parts
}

// claudeMessageBuilder collects Responses input items into alternating Messages turns:
// function calls join the assistant turn, function call outputs the next user turn.
type claudeMessageBuilder struct {
	messages []claude.Message
	// pendingThinking holds a reasoning item until the assistant turn it belongs to starts.
	pendingThinking []map[string]any
}

func (b *claudeMessageBuilder) append(role string, blocks ...map[string]any) {
	if len(blocks) == 0 {
		return
	}
	if role == types.ChatMessageRoleAssistant && len(b.pendingThinking) > 0 {
		blocks = append(b.pendingThinking, blocks...)
		b.pendingThinking = nil
	}
	if n := len(b.messages); n > 0 && b.messages[n-1].Role == role {
		last := b.messages[n-1].Content.([]map[string]any)
		b.messages[n-1].Content = append(last, blocks...)
		return
	}
	b.messages = append(b.messages, claude.Message{Role: role, Content: blocks})
}

func (b *claudeMessageBuilder) add(item types.InputResponses) {
	switch item.Type {
	case "", types.InputTypeMessage:
		role := item.Role
		if role != types.ChatMessageRoleAssistant {
			role = types.ChatMessageRoleUser
		}
		var blocks []map[string]any
		for _, part := range responsesContentParts(item.Content) {
			if block, ok := responsesPartToClaude(part); ok {
				blocks = append(blocks, block)
			}
		}
		b.append(role, blocks...)
	case types.InputTypeFunctionCall:
		b.append(types.ChatMessageRoleAssistant, map[string]any{
			"type":  "tool_use",
			"id":    item.CallID,
			"name":  item.Name,
			"input": argumentsObject(item.ArgumentsString()),
		})
	case types.InputTypeFunctionCallOutput:
		b.append(types.ChatMessageRoleUser, map[string]any{
			"type":        "tool_result",
			"tool_use_id": item.CallID,
			"content":     functionOutputToClaude(item.Output),
		})
	case types.InputTypeReasoning:
		// Only reasoning a Claude upstream issued carries a signature it accepts back.
		var summary []string
		for _, part := range item.Summary {
			summary = append(summary, part.Text)
		}
		block, ok := reasoningToThinkingBlock(strings.Join(summary, "\n\n"), item.EncryptedContent)
		if ok && (block["type"] == "redacted_thinking" || block["signature"] != "") {
			b.pendingThinking = append(b.pendingThinking, block)
		}
	}
}

func responsesPartToClaude(part types.ContentResponses) (map[string]any, bool) {
	switch part.Type {
	case types.ContentTypeInputText, types.ContentTypeOutputText:
		if part.Text == "" {
			return nil, false
		}
		return map[string]any{"type": "text", "text": part.Text}, true
	case types.ContentTypeRefusal:
		if part.Refusal != nil && part.Refusal.Refusal != "" {
			return map[string]any{"type": "text", "text": part.Refusal.Refusal}, true
		}
	case types.ContentTypeInputImage:
		if mediaType, data, ok := splitDataURL(part.ImageUrl); ok {
			return map[string]any{"type": "image", "source": map[string]any{"type": "base64", "media_type": mediaType, "data": data}}, true
		}
		if part.ImageUrl != "" {
			return map[string]any{"type": "image", "source": map[string]any{"type": "url", "url": part.ImageUrl}}, true
		}
		if part.FileId != "" {
			return map[string]any{"type": "image", "source": map[string]any{"type": "file", "file_id": part.FileId}}, true
		}
	case types.ContentTypeInputFile:
		if mediaType, data, ok := splitDataURL(part.FileData); ok {
			return map[string]any{"type": "document", "source": map[string]any{"type": "base64", "media_type": mediaType, "data": data}}, true
		}
		if part.FileUrl != "" {
			return map[string]any{"type": "document", "source": map[string]any{"type": "url", "url": part.FileUrl}}, true
		}
		if part.FileId != "" {
			return map[string]any{"type": "document", "source": map[string]any{"type": "file", "file_id": part.FileId}}, true
		}
	}
	return nil, false
}

// functionOutputToClaude converts a function_call_output output (string or input parts) to
// tool_result content.
func functionOutputToClaude(output any) any {
	if text, ok := output.(string); ok {
		return text
	}
	var blocks []map[string]any
	for _, part := range responsesContentParts(output) {
		if block, ok := responsesPartToClaude(part); ok {
			blocks = append(blocks, block)
		}
	}
	if len(blocks) == 0 {
		raw, _ := json.Marshal(output)
		return string(raw)
	}
	return blocks
}

func responsesToolToClaude(tool types.ResponsesTools) (claude.Tools, bool) {
	switch tool.Type {
	case "function":
		schema := tool.Parameters
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		return claude.Tools{Name: tool.Name, Description: tool.Description, InputSchema: schema}, true
	case "web_search", "web_search_preview":
		return claude.Tools{Type: "web_search_20250305", Name: "web_search"}, true
	}
	return claude.Tools{}, false
}

func responsesToolChoiceToClaude(choice any) *claude.ToolChoice {
	switch v := choice.(type) {
	case string:
		switch v {
		case "required":
			return &claude.ToolChoice{Type: "any"}
		case "none":
			return &claude.ToolChoice{Type: "none"}
		}
	case map[string]any:
		if name, _ := v["name"].(string); name != "" {
			return &claude.ToolChoice{Type: "tool", Name: name}
		}
	}
	return &claude.ToolChoice{Type: "auto"}
}

// MessagesToResponsesResponse converts an Anthropic Messages reply to a Responses reply. The
// usage is the billed usage the provider recorded.
func MessagesToResponsesResponse(response *claude.ClaudeResponse, request *types.OpenAIResponsesRequest, model string, usage *types.Usage) *types.OpenAIResponsesResponses {
	if model == "" {
		model = response.Model
	}
	out := newResponsesEnvelope(responseIDFromMessageID(response.Id), model, request)
	out.Status = types.ResponseStatusCompleted

	var message *types.ResponsesOutput
	var texts []map[string]any
	closeMessage := func() {
		if message != nil {
			message.Content = texts
			out.Output = append(out.Output, *message)
			message, texts = nil, nil
		}
	}
	for _, block := range response.Content {
		switch block.Type {
		case "text":
			if message == nil {
				message = &types.ResponsesOutput{Type: types.InputTypeMessage, ID: "msg_" + utils.GetUUID(),
					Status: types.ResponseStatusCompleted, Role: types.ChatMessageRoleAssistant}
			}
			texts = append(texts, outputTextPart(block.Text))
		case "thinking", "redacted_thinking":
			closeMessage()
			out.Output = append(out.Output, reasoningItem(block.Type, block.Thinking, block.Signature, block.Data))
		case "tool_use":
			closeMessage()
			out.Output = append(out.Output, types.ResponsesOutput{
				Type: types.InputTypeFunctionCall, ID: "fc_" + block.Id, Status: types.ResponseStatusCompleted,
				CallID: block.Id, Name: block.Name, Arguments: types.ArgumentsFromString(argumentsString(block.Input)),
			})
		}
	}
	closeMessage()

	applyClaudeStopReason(out, response.StopReason)
	out.Usage = responsesUsageFromBilling(usage)
	return out
}

func newResponsesEnvelope(id, model string, request *types.OpenAIResponsesRequest) *types.OpenAIResponsesResponses {
	out := &types.OpenAIResponsesResponses{
		ID:        id,
		Object:    "response",
		CreatedAt: nowUnix(),
		Model:     model,
		Output:    []types.ResponsesOutput{},
	}
	if request != nil {
		out.Instructions = request.Instructions
		out.MaxOutputTokens = request.MaxOutputTokens
		out.ParallelToolCalls = request.ParallelToolCalls
		out.PreviousResponseID = request.PreviousResponseID
		out.Reasoning = request.Reasoning
		out.Temperature = request.Temperature
		out.TopP = request.TopP
		out.ToolChoice = request.ToolChoice
		out.Tools = request.Tools
		out.Truncation = request.Truncation
		if request.Text != nil {
			out.Text = request.Text
		}
	}
	return out
}

func outputTextPart(text string) map[string]any {
	return map[string]any{"type": types.ContentTypeOutputText, "text": text, "annotations": []any{}}
}

// reasoningItem carries a Claude thinking block as a reasoning item: the thinking text as its
// summary and the signature (or redacted data) as encrypted_content, so a client that sends
// reasoning items back returns a block Claude accepts.
func reasoningItem(blockType, thinking, signature, data string) types.ResponsesOutput {
	encrypted := signature
	if blockType == "redacted_thinking" {
		encrypted = redactedThinkingPrefix + data
	}
	item := types.ResponsesOutput{
		Type:    types.InputTypeReasoning,
		ID:      "rs_" + utils.GetUUID(),
		Status:  types.ResponseStatusCompleted,
		Summary: []types.SummaryResponses{},
	}
	if thinking != "" {
		item.Summary = append(item.Summary, types.SummaryResponses{Type: types.ContentTypeSummaryText, Text: thinking})
	}
	if encrypted != "" {
		item.EncryptedContent = &encrypted
	}
	return item
}

func applyClaudeStopReason(out *types.OpenAIResponsesResponses, stopReason string) {
	switch stopReason {
	case "max_tokens", "model_context_window_exceeded":
		out.Status = types.ResponseStatusIncomplete
		out.IncompleteDetail = &types.IncompleteDetail{Reason: "max_output_tokens"}
	case "refusal":
		out.Status = types.ResponseStatusIncomplete
		out.IncompleteDetail = &types.IncompleteDetail{Reason: "content_filter"}
	default:
		out.Status = types.ResponseStatusCompleted
	}
}

func responseIDFromMessageID(id string) string {
	if id == "" {
		return "resp_" + utils.GetUUID()
	}
	return "resp_" + strings.TrimPrefix(id, "msg_")
}

// MessagesToResponsesStream converts an Anthropic Messages event stream to a Responses event
// stream. Feed it the upstream chunks in order; it returns the SSE text to write.
type MessagesToResponsesStream struct {
	request *types.OpenAIResponsesRequest
	model   string
	usage   *types.Usage

	lines    sseDataLines
	response *types.OpenAIResponsesResponses
	sequence int
	finished bool

	blocks     map[int]*openResponsesItem // by Claude content block index
	stopReason string
}

type openResponsesItem struct {
	outputIndex int
	item        types.ResponsesOutput
	text        strings.Builder
	signature   string
}

// NewMessagesToResponsesStream starts a stream conversion. usage is the provider's billed usage,
// read when the stream ends.
func NewMessagesToResponsesStream(request *types.OpenAIResponsesRequest, model string, usage *types.Usage) *MessagesToResponsesStream {
	return &MessagesToResponsesStream{request: request, model: model, usage: usage, blocks: map[int]*openResponsesItem{}}
}

// claudeStreamEvent is the part of an Anthropic stream event the conversion reads.
type claudeStreamEvent struct {
	Type    string `json:"type"`
	Index   int    `json:"index"`
	Message *struct {
		Id    string `json:"id"`
		Model string `json:"model"`
	} `json:"message"`
	ContentBlock *claudeBlock `json:"content_block"`
	Delta        *struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		Thinking    string `json:"thinking"`
		Signature   string `json:"signature"`
		PartialJson string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Process converts one upstream chunk.
func (s *MessagesToResponsesStream) Process(chunk string) string {
	var out strings.Builder
	for _, payload := range s.lines.feed(chunk) {
		out.WriteString(s.processEvent(payload))
	}
	return out.String()
}

// Finish completes the response if the upstream ended without message_stop.
func (s *MessagesToResponsesStream) Finish() string {
	var out strings.Builder
	for _, payload := range s.lines.flush() {
		out.WriteString(s.processEvent(payload))
	}
	out.WriteString(s.complete())
	return out.String()
}

// Error reports an upstream error as a Responses error event.
func (s *MessagesToResponsesStream) Error(message string) string {
	s.finished = true
	return s.emit("error", map[string]any{"type": "error", "code": "upstream_error", "message": message})
}

func (s *MessagesToResponsesStream) emit(eventType string, event map[string]any) string {
	event["sequence_number"] = s.sequence
	s.sequence++
	return sseEvent(eventType, event)
}

func (s *MessagesToResponsesStream) processEvent(payload string) string {
	if s.finished {
		return ""
	}
	var event claudeStreamEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return ""
	}

	var out strings.Builder
	if s.response == nil && event.Type != "error" {
		out.WriteString(s.start(event))
	}
	switch event.Type {
	case "content_block_start":
		if event.ContentBlock != nil {
			out.WriteString(s.openItem(event.Index, *event.ContentBlock))
		}
	case "content_block_delta":
		if event.Delta != nil {
			out.WriteString(s.delta(event.Index, event.Delta.Type, event.Delta.Text, event.Delta.Thinking,
				event.Delta.Signature, event.Delta.PartialJson))
		}
	case "content_block_stop":
		out.WriteString(s.closeItem(event.Index))
	case "message_delta":
		if event.Delta != nil && event.Delta.StopReason != "" {
			s.stopReason = event.Delta.StopReason
		}
	case "message_stop":
		out.WriteString(s.complete())
	case "error":
		message := "upstream error"
		if event.Error != nil && event.Error.Message != "" {
			message = event.Error.Message
		}
		out.WriteString(s.Error(message))
	}
	return out.String()
}

func (s *MessagesToResponsesStream) start(event claudeStreamEvent) string {
	id, model := "", s.model
	if event.Message != nil {
		id = event.Message.Id
		if model == "" {
			model = event.Message.Model
		}
	}
	s.response = newResponsesEnvelope(responseIDFromMessageID(id), model, s.request)
	s.response.Status = types.ResponseStatusInProgress
	return s.emit("response.created", map[string]any{"type": "response.created", "response": s.response}) +
		s.emit("response.in_progress", map[string]any{"type": "response.in_progress", "response": s.response})
}

func (s *MessagesToResponsesStream) openItem(index int, block claudeBlock) string {
	if s.blocks[index] != nil {
		return ""
	}
	open := &openResponsesItem{outputIndex: len(s.response.Output) + len(s.blocks)}
	switch block.Type {
	case "text":
		open.item = types.ResponsesOutput{Type: types.InputTypeMessage, ID: "msg_" + utils.GetUUID(),
			Status: types.ResponseStatusInProgress, Role: types.ChatMessageRoleAssistant, Content: []any{}}
	case "thinking":
		open.item = types.ResponsesOutput{Type: types.InputTypeReasoning, ID: "rs_" + utils.GetUUID(),
			Status: types.ResponseStatusInProgress, Summary: []types.SummaryResponses{}}
	case "redacted_thinking":
		open.item = reasoningItem(block.Type, "", "", block.Data)
		open.item.Status = types.ResponseStatusInProgress
	case "tool_use":
		open.item = types.ResponsesOutput{Type: types.InputTypeFunctionCall, ID: "fc_" + block.Id,
			Status: types.ResponseStatusInProgress, CallID: block.Id, Name: block.Name, Arguments: types.ArgumentsFromString("")}
	default:
		// Server tool calls and their results have no Responses item here.
		return ""
	}
	s.blocks[index] = open

	var out strings.Builder
	out.WriteString(s.emit("response.output_item.added", map[string]any{
		"type": "response.output_item.added", "output_index": open.outputIndex, "item": open.item,
	}))
	switch block.Type {
	case "text":
		out.WriteString(s.emit("response.content_part.added", map[string]any{
			"type": "response.content_part.added", "item_id": open.item.ID, "output_index": open.outputIndex,
			"content_index": 0, "part": outputTextPart(""),
		}))
	case "thinking":
		out.WriteString(s.emit("response.reasoning_summary_part.added", map[string]any{
			"type": "response.reasoning_summary_part.added", "item_id": open.item.ID, "output_index": open.outputIndex,
			"summary_index": 0, "part": map[string]any{"type": types.ContentTypeSummaryText, "text": ""},
		}))
	}
	return out.String()
}

func (s *MessagesToResponsesStream) delta(index int, deltaType, text, thinking, signature, partialJSON string) string {
	open := s.blocks[index]
	if open == nil {
		return ""
	}
	switch deltaType {
	case "text_delta":
		if text == "" {
			return ""
		}
		open.text.WriteString(text)
		return s.emit("response.output_text.delta", map[string]any{
			"type": "response.output_text.delta", "item_id": open.item.ID, "output_index": open.outputIndex,
			"content_index": 0, "delta": text,
		})
	case "thinking_delta":
		if thinking == "" {
			return ""
		}
		open.text.WriteString(thinking)
		return s.emit("response.reasoning_summary_text.delta", map[string]any{
			"type": "response.reasoning_summary_text.delta", "item_id": open.item.ID, "output_index": open.outputIndex,
			"summary_index": 0, "delta": thinking,
		})
	case "signature_delta":
		open.signature += signature
	case "input_json_delta":
		if partialJSON == "" {
			return ""
		}
		open.text.WriteString(partialJSON)
		return s.emit("response.function_call_arguments.delta", map[string]any{
			"type": "response.function_call_arguments.delta", "item_id": open.item.ID, "output_index": open.outputIndex,
			"delta": partialJSON,
		})
	}
	return ""
}

func (s *MessagesToResponsesStream) closeItem(index int) string {
	open := s.blocks[index]
	if open == nil {
		return ""
	}
	delete(s.blocks, index)

	var out strings.Builder
	item := open.item
	item.Status = types.ResponseStatusCompleted
	text := open.text.String()
	switch item.Type {
	case types.InputTypeMessage:
		item.Content = []map[string]any{outputTextPart(text)}
		out.WriteString(s.emit("response.output_text.done", map[string]any{
			"type": "response.output_text.done", "item_id": item.ID, "output_index": open.outputIndex,
			"content_index": 0, "text": text,
		}))
		out.WriteString(s.emit("response.content_part.done", map[string]any{
			"type": "response.content_part.done", "item_id": item.ID, "output_index": open.outputIndex,
			"content_index": 0, "part": outputTextPart(text),
		}))
	case types.InputTypeReasoning:
		if item.EncryptedContent == nil {
			item = reasoningItem("thinking", text, open.signature, "")
			item.ID = open.item.ID
			out.WriteString(s.emit("response.reasoning_summary_text.done", map[string]any{
				"type": "response.reasoning_summary_text.done", "item_id": item.ID, "output_index": open.outputIndex,
				"summary_index": 0, "text": text,
			}))
			out.WriteString(s.emit("response.reasoning_summary_part.done", map[string]any{
				"type": "response.reasoning_summary_part.done", "item_id": item.ID, "output_index": open.outputIndex,
				"summary_index": 0, "part": map[string]any{"type": types.ContentTypeSummaryText, "text": text},
			}))
		}
	case types.InputTypeFunctionCall:
		if strings.TrimSpace(text) == "" {
			text = "{}"
		}
		item.Arguments = types.ArgumentsFromString(text)
		out.WriteString(s.emit("response.function_call_arguments.done", map[string]any{
			"type": "response.function_call_arguments.done", "item_id": item.ID, "output_index": open.outputIndex,
			"arguments": text,
		}))
	}
	out.WriteString(s.emit("response.output_item.done", map[string]any{
		"type": "response.output_item.done", "output_index": open.outputIndex, "item": item,
	}))
	s.response.Output = append(s.response.Output, item)
	return out.String()
}

func (s *MessagesToResponsesStream) complete() string {
	if s.finished {
		return ""
	}
	var out strings.Builder
	if s.response == nil {
		out.WriteString(s.start(claudeStreamEvent{}))
	}
	for len(s.blocks) > 0 {
		lowest := -1
		for index := range s.blocks {
			if lowest == -1 || index < lowest {
				lowest = index
			}
		}
		out.WriteString(s.closeItem(lowest))
	}
	s.finished = true
	applyClaudeStopReason(s.response, s.stopReason)
	s.response.Usage = responsesUsageFromBilling(s.usage)
	eventType := "response.completed"
	if s.response.Status == types.ResponseStatusIncomplete {
		eventType = "response.incomplete"
	}
	out.WriteString(s.emit(eventType, map[string]any{"type": eventType, "response": s.response}))
	return out.String()
}
