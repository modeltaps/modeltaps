package codex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// functionCallEvents is an upstream Responses stream that answers with a single function call.
var functionCallEvents = []string{
	`{"type":"response.created","response":{"id":"resp_1","status":"in_progress","output":[]}}`,
	`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_1","type":"function_call","status":"in_progress","call_id":"call_1","name":"get_weather","arguments":""}}`,
	`{"type":"response.function_call_arguments.delta","output_index":0,"item_id":"fc_1","delta":"{\"city\":"}`,
	`{"type":"response.function_call_arguments.delta","output_index":0,"item_id":"fc_1","delta":"\"Paris\"}"}`,
	`{"type":"response.function_call_arguments.done","output_index":0,"item_id":"fc_1","arguments":"{\"city\":\"Paris\"}"}`,
	`{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_1","type":"function_call","status":"completed","call_id":"call_1","name":"get_weather","arguments":"{\"city\":\"Paris\"}"}}`,
	`{"type":"response.completed","response":{"id":"resp_1","status":"completed","output":[{"id":"fc_1","type":"function_call","status":"completed","call_id":"call_1","name":"get_weather","arguments":"{\"city\":\"Paris\"}"}],"usage":{"input_tokens":40,"output_tokens":12,"total_tokens":52}}}`,
}

// newTestProvider points a Codex provider at a fake upstream that replays events and
// stores the last request body in *body.
func newTestProvider(t *testing.T, events []string, body *map[string]any) *CodexProvider {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body != nil {
			if err := json.NewDecoder(r.Body).Decode(body); err != nil {
				t.Errorf("decode upstream request: %v", err)
			}
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, event := range events {
			var head struct {
				Type string `json:"type"`
			}
			_ = json.Unmarshal([]byte(event), &head)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", head.Type, event)
		}
	}))
	t.Cleanup(upstream.Close)

	// InitHttpClient logs.
	oldLogger := logger.Logger
	logger.Logger = zap.NewNop()
	t.Cleanup(func() { logger.Logger = oldLogger })
	requester.InitHttpClient()

	baseURL := upstream.URL
	channel := &model.Channel{Type: 59, Key: "token", BaseURL: &baseURL}
	provider := CodexProviderFactory{}.Create(channel).(*CodexProvider)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	provider.SetContext(ctx)
	provider.SetUsage(&types.Usage{})
	return provider
}

func weatherChatRequest(stream bool) *types.ChatCompletionRequest {
	return &types.ChatCompletionRequest{
		Model:  "gpt-test",
		Stream: stream,
		Messages: []types.ChatCompletionMessage{
			{Role: types.ChatMessageRoleUser, Content: "What is the weather in Paris?"},
		},
		Tools: []*types.ChatCompletionTool{{
			Type: "function",
			Function: types.ChatCompletionFunction{
				Name:       "get_weather",
				Parameters: map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}},
			},
		}},
	}
}

func TestCreateChatCompletionReturnsToolCalls(t *testing.T) {
	var body map[string]any
	provider := newTestProvider(t, functionCallEvents, &body)

	response, errWithCode := provider.CreateChatCompletion(weatherChatRequest(false))
	if errWithCode != nil {
		t.Fatal(errWithCode.Message)
	}

	choice := response.Choices[0]
	if choice.FinishReason != types.FinishReasonToolCalls {
		t.Fatalf("finish_reason = %v, want tool_calls", choice.FinishReason)
	}
	if len(choice.Message.ToolCalls) != 1 {
		t.Fatalf("tool_calls = %d, want 1", len(choice.Message.ToolCalls))
	}
	call := choice.Message.ToolCalls[0]
	if call.Id != "call_1" || call.Function.Name != "get_weather" || call.Function.Arguments != `{"city":"Paris"}` {
		t.Fatalf("unexpected tool call %+v %+v", call, call.Function)
	}
	if response.Usage == nil || response.Usage.PromptTokens != 40 || response.Usage.CompletionTokens != 12 {
		t.Fatalf("unexpected usage %+v", response.Usage)
	}
	if provider.Usage.PromptTokens != 40 || provider.Usage.CompletionTokens != 12 {
		t.Fatalf("unexpected billed usage %+v", provider.Usage)
	}
	if body["instructions"] != DefaultCodexInstructions {
		t.Fatalf("instructions = %q, want default", body["instructions"])
	}
	if _, ok := body["tools"].([]any); !ok {
		t.Fatalf("tools not sent upstream: %v", body["tools"])
	}
}

func TestCreateChatCompletionStreamReturnsToolCalls(t *testing.T) {
	provider := newTestProvider(t, functionCallEvents, nil)

	stream, errWithCode := provider.CreateChatCompletionStream(weatherChatRequest(true))
	if errWithCode != nil {
		t.Fatal(errWithCode.Message)
	}
	defer stream.Close()

	var name, arguments, callID string
	var finishReason any
	dataChan, errChan := stream.Recv()
	for done := false; !done; {
		select {
		case data, ok := <-dataChan:
			if !ok {
				done = true
				break
			}
			var chunk types.ChatCompletionStreamResponse
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				t.Fatalf("expected chat chunk, got %q", data)
			}
			for _, choice := range chunk.Choices {
				for _, call := range choice.Delta.ToolCalls {
					if call.Id != "" {
						callID = call.Id
					}
					if call.Function != nil {
						name += call.Function.Name
						arguments += call.Function.Arguments
					}
				}
				if choice.FinishReason != nil {
					finishReason = choice.FinishReason
				}
			}
		case <-errChan:
			done = true
		}
	}

	if callID != "call_1" || name != "get_weather" || arguments != `{"city":"Paris"}` {
		t.Fatalf("tool call = %q %q %q", callID, name, arguments)
	}
	if finishReason != types.FinishReasonToolCalls {
		t.Fatalf("finish_reason = %v, want tool_calls", finishReason)
	}
	if provider.Usage.PromptTokens != 40 || provider.Usage.CompletionTokens != 12 {
		t.Fatalf("unexpected billed usage %+v", provider.Usage)
	}
}

func TestPrepareCodexRequest(t *testing.T) {
	provider := &CodexProvider{}

	t.Run("string input becomes a user message", func(t *testing.T) {
		request := &types.OpenAIResponsesRequest{Model: "gpt-test", Input: "hello"}
		provider.prepareCodexRequest(request)

		raw, err := json.Marshal(request.Input)
		if err != nil {
			t.Fatal(err)
		}
		want := `[{"content":[{"text":"hello","type":"input_text"}],"role":"user"}]`
		if string(raw) != want {
			t.Fatalf("input = %s, want %s", raw, want)
		}
	})

	t.Run("list input is kept", func(t *testing.T) {
		input := []any{map[string]any{"role": "user", "content": "hi"}}
		request := &types.OpenAIResponsesRequest{Model: "gpt-test", Input: input}
		provider.prepareCodexRequest(request)

		if _, ok := request.Input.([]any); !ok {
			t.Fatalf("input changed to %T", request.Input)
		}
	})

	t.Run("missing instructions get the short default", func(t *testing.T) {
		request := &types.OpenAIResponsesRequest{Model: "gpt-test", Instructions: "  "}
		provider.prepareCodexRequest(request)

		if request.Instructions != DefaultCodexInstructions || len(request.Instructions) > 200 {
			t.Fatalf("instructions = %q", request.Instructions)
		}
	})

	t.Run("client instructions are kept", func(t *testing.T) {
		request := &types.OpenAIResponsesRequest{Model: "gpt-test", Instructions: "Answer in French."}
		provider.prepareCodexRequest(request)

		if request.Instructions != "Answer in French." {
			t.Fatalf("instructions = %q", request.Instructions)
		}
	})
}
