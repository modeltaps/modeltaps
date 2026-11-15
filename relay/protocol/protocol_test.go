package protocol

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/types"

	"github.com/tidwall/gjson"
)

// sseEvents parses SSE text into (event, data) pairs.
func sseEvents(t *testing.T, text string) []struct {
	name string
	data gjson.Result
} {
	t.Helper()
	var events []struct {
		name string
		data gjson.Result
	}
	for _, block := range strings.Split(strings.TrimSpace(text), "\n\n") {
		var name, data string
		for _, line := range strings.Split(block, "\n") {
			if v, ok := strings.CutPrefix(line, "event: "); ok {
				name = v
			}
			if v, ok := strings.CutPrefix(line, "data: "); ok {
				data = v
			}
		}
		if name == "" && data == "" {
			continue
		}
		if !gjson.Valid(data) {
			t.Fatalf("event %q has invalid JSON: %s", name, data)
		}
		events = append(events, struct {
			name string
			data gjson.Result
		}{name, gjson.Parse(data)})
	}
	return events
}

func eventNames(events []struct {
	name string
	data gjson.Result
}) []string {
	names := make([]string, len(events))
	for i, e := range events {
		names[i] = e.name
	}
	return names
}

func TestMessagesToResponsesRequest(t *testing.T) {
	body := `{"model":"gpt-5","max_tokens":1024,"stream":true,
		"system":[{"type":"text","text":"be brief","cache_control":{"type":"ephemeral"}}],
		"thinking":{"type":"enabled","budget_tokens":6000},
		"tools":[{"name":"get_weather","description":"weather","input_schema":{"type":"object","properties":{"city":{"type":"string"}}}},
		         {"type":"bash_20250124","name":"bash"}],
		"tool_choice":{"type":"any","disable_parallel_tool_use":true},
		"messages":[
		  {"role":"user","content":[{"type":"text","text":"look"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AAA"}}]},
		  {"role":"assistant","content":[{"type":"thinking","thinking":"hmm","signature":"sig"},{"type":"text","text":"calling"},
		     {"type":"tool_use","id":"toolu_1","name":"get_weather","input":{"city":"Paris"}}]},
		  {"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"sunny"}]},{"type":"text","text":"thanks"}]}
		]}`
	var request claude.ClaudeRequest
	if err := json.Unmarshal([]byte(body), &request); err != nil {
		t.Fatal(err)
	}
	out := MessagesToResponsesRequest(&request)
	raw, _ := json.Marshal(out)
	got := gjson.ParseBytes(raw)

	checks := map[string]string{
		"model":                       "gpt-5",
		"instructions":                "be brief",
		"max_output_tokens":           "1024",
		"stream":                      "true",
		"reasoning.effort":            "medium",
		"reasoning.summary":           "auto",
		"tool_choice":                 "required",
		"tools.#":                     "1",
		"tools.0.type":                "function",
		"tools.0.name":                "get_weather",
		"tools.0.parameters.type":     "object",
		"input.#":                     "5",
		"input.0.role":                "user",
		"input.0.content.0.type":      "input_text",
		"input.0.content.1.type":      "input_image",
		"input.0.content.1.image_url": "data:image/png;base64,AAA",
		"input.1.role":                "assistant",
		"input.1.content.0.type":      "output_text",
		"input.1.content.0.text":      "calling",
		"input.2.type":                "function_call",
		"input.2.call_id":             "toolu_1",
		"input.2.arguments":           `{"city":"Paris"}`,
		"input.3.type":                "function_call_output",
		"input.3.call_id":             "toolu_1",
		"input.3.output":              "sunny",
		"input.4.content.0.text":      "thanks",
	}
	for path, want := range checks {
		if got.Get(path).String() != want {
			t.Errorf("%s = %q, want %q\n%s", path, got.Get(path).String(), want, raw)
		}
	}
	if got.Get("parallel_tool_calls").Bool() {
		t.Error("disable_parallel_tool_use must turn parallel tool calls off")
	}
	if strings.Contains(string(raw), "cache_control") || strings.Contains(string(raw), `"messages"`) {
		t.Errorf("Anthropic-only fields leaked: %s", raw)
	}
}

func TestResponsesToMessagesResponse(t *testing.T) {
	raw := `{"id":"resp_abc","object":"response","model":"gpt-5","status":"completed","output":[
		{"type":"reasoning","id":"rs_1","summary":[{"type":"summary_text","text":"thinking it over"}],"encrypted_content":"enc"},
		{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hi"}]},
		{"type":"function_call","id":"fc_1","call_id":"call_1","name":"get_weather","arguments":"{\"city\":\"Paris\"}","status":"completed"}]}`
	var response types.OpenAIResponsesResponses
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		t.Fatal(err)
	}
	usage := &types.Usage{PromptTokens: 5000, CompletionTokens: 20}
	usage.PromptTokensDetails.CachedTokens = 4000
	out := ResponsesToMessagesResponse(&response, "gpt-5", usage)
	b, _ := json.Marshal(out)
	got := gjson.ParseBytes(b)
	checks := map[string]string{
		"id":                            "msg_abc",
		"type":                          "message",
		"stop_reason":                   "tool_use",
		"content.0.type":                "thinking",
		"content.0.thinking":            "thinking it over",
		"content.0.signature":           "enc",
		"content.1.text":                "hi",
		"content.2.type":                "tool_use",
		"content.2.id":                  "call_1",
		"content.2.input.city":          "Paris",
		"usage.input_tokens":            "1000",
		"usage.cache_read_input_tokens": "4000",
		"usage.output_tokens":           "20",
	}
	for path, want := range checks {
		if got.Get(path).String() != want {
			t.Errorf("%s = %q, want %q\n%s", path, got.Get(path).String(), want, b)
		}
	}
}

// Codex-style chunks: whole event blocks with event: lines.
func TestResponsesToMessagesStream(t *testing.T) {
	usage := &types.Usage{PromptTokens: 3000, CompletionTokens: 12}
	usage.PromptTokensDetails.CachedTokens = 2048
	s := NewResponsesToMessagesStream("gpt-5", usage)

	chunks := []string{
		"event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_9\",\"model\":\"gpt-5\",\"status\":\"in_progress\"}}\n\n",
		"event: response.output_item.added\ndata: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"type\":\"reasoning\",\"id\":\"rs_1\",\"status\":\"in_progress\"}}\n\n",
		"data: {\"type\":\"response.reasoning_summary_part.added\",\"output_index\":0,\"summary_index\":0}\n\n",
		"data: {\"type\":\"response.reasoning_summary_text.delta\",\"output_index\":0,\"summary_index\":0,\"delta\":\"plan\"}\n\n",
		"data: {\"type\":\"response.output_item.done\",\"output_index\":0,\"item\":{\"type\":\"reasoning\",\"id\":\"rs_1\",\"status\":\"completed\",\"encrypted_content\":\"enc\"}}\n\n",
		"data: {\"type\":\"response.output_item.added\",\"output_index\":1,\"item\":{\"type\":\"message\",\"id\":\"m1\",\"status\":\"in_progress\"}}\n\n",
		// a delta split across two chunks
		"data: {\"type\":\"response.output_text.delta\",\"output_index\":1,",
		"\"content_index\":0,\"delta\":\"Hello\"}\n\n",
		"data: {\"type\":\"response.output_item.done\",\"output_index\":1,\"item\":{\"type\":\"message\",\"id\":\"m1\",\"status\":\"completed\"}}\n\n",
		"data: {\"type\":\"response.output_item.added\",\"output_index\":2,\"item\":{\"type\":\"function_call\",\"id\":\"fc_1\",\"call_id\":\"call_1\",\"name\":\"get_weather\",\"arguments\":\"\"}}\n\n",
		"data: {\"type\":\"response.function_call_arguments.delta\",\"output_index\":2,\"delta\":\"{\\\"city\\\":\"}\n\n",
		"data: {\"type\":\"response.function_call_arguments.delta\",\"output_index\":2,\"delta\":\"\\\"Paris\\\"}\"}\n\n",
		"data: {\"type\":\"response.output_item.done\",\"output_index\":2,\"item\":{\"type\":\"function_call\",\"id\":\"fc_1\",\"call_id\":\"call_1\",\"name\":\"get_weather\",\"arguments\":\"{\\\"city\\\":\\\"Paris\\\"}\"}}\n\n",
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_9\",\"model\":\"gpt-5\",\"status\":\"completed\"}}\n\n",
	}
	var out strings.Builder
	for _, chunk := range chunks {
		out.WriteString(s.Process(chunk))
	}
	out.WriteString(s.Finish())
	events := sseEvents(t, out.String())

	want := []string{
		"message_start",
		"content_block_start", "content_block_delta", "content_block_delta", "content_block_stop",
		"content_block_start", "content_block_delta", "content_block_stop",
		"content_block_start", "content_block_delta", "content_block_delta", "content_block_stop",
		"message_delta", "message_stop",
	}
	if strings.Join(eventNames(events), ",") != strings.Join(want, ",") {
		t.Fatalf("events = %v\nwant %v\n%s", eventNames(events), want, out.String())
	}
	if events[1].data.Get("content_block.type").String() != "thinking" ||
		events[2].data.Get("delta.thinking").String() != "plan" ||
		events[3].data.Get("delta.signature").String() != "enc" {
		t.Errorf("thinking block wrong: %v %v %v", events[1].data, events[2].data, events[3].data)
	}
	if events[6].data.Get("delta.text").String() != "Hello" || events[6].data.Get("index").Int() != 1 {
		t.Errorf("text delta wrong: %v", events[6].data)
	}
	if events[8].data.Get("content_block.id").String() != "call_1" ||
		events[9].data.Get("delta.partial_json").String()+events[10].data.Get("delta.partial_json").String() != `{"city":"Paris"}` {
		t.Errorf("tool_use wrong: %v %v %v", events[8].data, events[9].data, events[10].data)
	}
	delta := events[12].data
	if delta.Get("delta.stop_reason").String() != "tool_use" ||
		delta.Get("usage.cache_read_input_tokens").Int() != 2048 ||
		delta.Get("usage.input_tokens").Int() != 952 ||
		delta.Get("usage.output_tokens").Int() != 12 {
		t.Errorf("message_delta = %v", delta)
	}
	if events[0].data.Get("message.id").String() != "msg_9" {
		t.Errorf("message_start = %v", events[0].data)
	}
}

func TestResponsesToMessagesStream_EndsWithoutTerminalEvent(t *testing.T) {
	s := NewResponsesToMessagesStream("gpt-5", &types.Usage{})
	out := s.Process("data: {\"type\":\"response.output_text.delta\",\"output_index\":0,\"delta\":\"x\"}\n\n") + s.Finish()
	names := eventNames(sseEvents(t, out))
	want := "message_start,content_block_start,content_block_delta,content_block_stop,message_delta,message_stop"
	if strings.Join(names, ",") != want {
		t.Fatalf("events = %v", names)
	}
}

func TestResponsesToMessagesRequest(t *testing.T) {
	effort := "high"
	request := &types.OpenAIResponsesRequest{
		Model:        "claude-sonnet-4-5",
		Instructions: "be brief",
		Stream:       true,
		Reasoning:    &types.ReasoningEffort{Effort: &effort},
		Tools: []types.ResponsesTools{
			{Type: "function", Name: "get_weather", Parameters: map[string]any{"type": "object"}},
			{Type: "file_search"},
		},
	}
	if err := json.Unmarshal([]byte(`[
		{"role":"developer","content":"use tools"},
		{"role":"user","content":[{"type":"input_text","text":"weather?"},{"type":"input_image","image_url":"data:image/jpeg;base64,BBB"}]},
		{"type":"reasoning","id":"rs_1","summary":[{"type":"summary_text","text":"need weather"}],"encrypted_content":"claude-sig"},
		{"type":"function_call","call_id":"toolu_1","name":"get_weather","arguments":"{\"city\":\"Paris\"}"},
		{"type":"function_call","call_id":"toolu_2","name":"get_weather","arguments":"{\"city\":\"Rome\"}"},
		{"type":"function_call_output","call_id":"toolu_1","output":"sunny"},
		{"type":"function_call_output","call_id":"toolu_2","output":"rain"}
	]`), &request.Input); err != nil {
		t.Fatal(err)
	}
	out, err := ResponsesToMessagesRequest(request, 4096)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(out)
	got := gjson.ParseBytes(raw)
	checks := map[string]string{
		"model":                                  "claude-sonnet-4-5",
		"system":                                 "be brief\n\nuse tools",
		"thinking.type":                          "enabled",
		"thinking.budget_tokens":                 "16384",
		"max_tokens":                             "20480",
		"stream":                                 "true",
		"tools.#":                                "1",
		"tools.0.name":                           "get_weather",
		"tools.0.input_schema.type":              "object",
		"tool_choice.type":                       "auto",
		"messages.#":                             "3",
		"messages.0.role":                        "user",
		"messages.0.content.1.type":              "image",
		"messages.0.content.1.source.data":       "BBB",
		"messages.0.content.1.source.media_type": "image/jpeg",
		"messages.1.role":                        "assistant",
		"messages.1.content.0.type":              "thinking",
		"messages.1.content.0.signature":         "claude-sig",
		"messages.1.content.0.thinking":          "need weather",
		"messages.1.content.1.type":              "tool_use",
		"messages.1.content.1.input.city":        "Paris",
		"messages.1.content.2.id":                "toolu_2",
		"messages.2.role":                        "user",
		"messages.2.content.#":                   "2",
		"messages.2.content.0.type":              "tool_result",
		"messages.2.content.1.tool_use_id":       "toolu_2",
		"messages.2.content.1.content":           "rain",
	}
	for path, want := range checks {
		if got.Get(path).String() != want {
			t.Errorf("%s = %q, want %q\n%s", path, got.Get(path).String(), want, raw)
		}
	}
}

func TestMessagesToResponsesResponse(t *testing.T) {
	raw := `{"id":"msg_01","type":"message","role":"assistant","model":"claude-sonnet-4-5","stop_reason":"max_tokens",
		"content":[{"type":"thinking","thinking":"hm","signature":"sig"},{"type":"redacted_thinking","data":"xyz"},
		{"type":"text","text":"a"},{"type":"text","text":"b"},{"type":"tool_use","id":"toolu_1","name":"f","input":{"k":1}}],
		"usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":4000,"cache_creation_input_tokens":900}}`
	var response claude.ClaudeResponse
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		t.Fatal(err)
	}
	usage := &types.Usage{}
	claude.ClaudeUsageToOpenaiUsage(&response.Usage, usage)
	out := MessagesToResponsesResponse(&response, &types.OpenAIResponsesRequest{Model: "claude-sonnet-4-5"}, "claude-sonnet-4-5", usage)
	b, _ := json.Marshal(out)
	got := gjson.ParseBytes(b)
	checks := map[string]string{
		"id":                         "resp_01",
		"object":                     "response",
		"status":                     "incomplete",
		"incomplete_details.reason":  "max_output_tokens",
		"output.#":                   "4",
		"output.0.type":              "reasoning",
		"output.0.summary.0.text":    "hm",
		"output.0.encrypted_content": "sig",
		"output.1.encrypted_content": redactedThinkingPrefix + "xyz",
		"output.2.type":              "message",
		"output.2.content.#":         "2",
		"output.3.type":              "function_call",
		"output.3.call_id":           "toolu_1",
		"output.3.arguments":         `{"k":1}`,
		"usage.input_tokens":         "4910",
		"usage.input_tokens_details.cached_tokens": "4000",
		"usage.output_tokens":                      "5",
	}
	for path, want := range checks {
		if got.Get(path).String() != want {
			t.Errorf("%s = %q, want %q\n%s", path, got.Get(path).String(), want, b)
		}
	}
}

// Claude-style chunks: one SSE line per chunk, as ClaudeRelayStreamHandler forwards them.
func TestMessagesToResponsesStream(t *testing.T) {
	usage := &types.Usage{}
	s := NewMessagesToResponsesStream(&types.OpenAIResponsesRequest{Model: "claude-sonnet-4-5"}, "claude-sonnet-4-5", usage)
	lines := []string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_7","model":"claude-sonnet-4-5","usage":{"input_tokens":10,"cache_read_input_tokens":4000,"cache_creation_input_tokens":900,"output_tokens":1}}}`,
		``,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"plan"}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig"}}`,
		`data: {"type":"content_block_stop","index":0}`,
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Hi"}}`,
		`data: {"type":"content_block_stop","index":1}`,
		`data: {"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"toolu_1","name":"f","input":{}}}`,
		`data: {"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"k\":"}}`,
		`data: {"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"1}"}}`,
		`data: {"type":"content_block_stop","index":2}`,
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":30}}`,
		`data: {"type":"message_stop"}`,
	}
	// The provider records usage as it forwards; mimic the final state.
	claude.ClaudeUsageToOpenaiUsage(&claude.Usage{InputTokens: 10, OutputTokens: 30, CacheReadInputTokens: 4000, CacheCreationInputTokens: 900}, usage)

	var out strings.Builder
	for _, line := range lines {
		out.WriteString(s.Process(line + "\n"))
	}
	out.WriteString(s.Finish())
	events := sseEvents(t, out.String())
	want := []string{
		"response.created", "response.in_progress",
		"response.output_item.added", "response.reasoning_summary_part.added", "response.reasoning_summary_text.delta",
		"response.reasoning_summary_text.done", "response.reasoning_summary_part.done", "response.output_item.done",
		"response.output_item.added", "response.content_part.added", "response.output_text.delta",
		"response.output_text.done", "response.content_part.done", "response.output_item.done",
		"response.output_item.added", "response.function_call_arguments.delta", "response.function_call_arguments.delta",
		"response.function_call_arguments.done", "response.output_item.done",
		"response.completed",
	}
	if strings.Join(eventNames(events), ",") != strings.Join(want, ",") {
		t.Fatalf("events = %v\nwant %v", eventNames(events), want)
	}
	for i, e := range events {
		if e.data.Get("sequence_number").Int() != int64(i) {
			t.Fatalf("event %d sequence_number = %d", i, e.data.Get("sequence_number").Int())
		}
	}
	if events[7].data.Get("item.encrypted_content").String() != "sig" || events[7].data.Get("item.summary.0.text").String() != "plan" {
		t.Errorf("reasoning item = %v", events[7].data)
	}
	if events[14].data.Get("output_index").Int() != 2 || events[18].data.Get("item.arguments").String() != `{"k":1}` {
		t.Errorf("function call = %v / %v", events[14].data, events[18].data)
	}
	final := events[19].data.Get("response")
	if final.Get("id").String() != "resp_7" || final.Get("status").String() != "completed" ||
		final.Get("output.#").Int() != 3 ||
		final.Get("usage.input_tokens").Int() != 4910 ||
		final.Get("usage.input_tokens_details.cached_tokens").Int() != 4000 ||
		final.Get("usage.output_tokens").Int() != 30 {
		t.Errorf("completed response = %s", final.Raw)
	}
}

func TestMessagesToResponsesStream_UpstreamError(t *testing.T) {
	s := NewMessagesToResponsesStream(nil, "claude", &types.Usage{})
	out := s.Error("overloaded") + s.Finish()
	events := sseEvents(t, out)
	if len(events) != 1 || events[0].name != "error" || events[0].data.Get("message").String() != "overloaded" {
		t.Fatalf("events = %s", out)
	}
}
