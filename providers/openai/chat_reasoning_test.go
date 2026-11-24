package openai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/types"
)

// Gateways that stream reasoning as `reasoning` get it renamed to reasoning_content, the field
// chat clients show as thinking; other providers pass the chunk through as it came.
func TestHandlerChatStreamRenamesReasoning(t *testing.T) {
	line := `data: {"id":"1","object":"chat.completion.chunk","model":"m","choices":[{"index":0,"delta":{"reasoning":"Let me think","reasoning_details":[{"type":"reasoning.text","text":"Let me think"}]}}]}`
	for _, on := range []bool{true, false} {
		h := &OpenAIStreamHandler{Usage: &types.Usage{}, ModelName: "m", ReasoningHandler: on}
		dataChan := make(chan string, 1)
		errChan := make(chan error, 1)
		raw := []byte(line)
		h.HandlerChatStream(&raw, dataChan, errChan)
		out := <-dataChan
		var chunk struct {
			Choices []struct {
				Delta map[string]json.RawMessage `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(out, "data: ")), &chunk); err != nil {
			t.Fatalf("on=%v: bad chunk %q: %v", on, out, err)
		}
		delta := chunk.Choices[0].Delta
		_, hasContent := delta["reasoning_content"]
		if hasContent != on {
			t.Errorf("on=%v: reasoning_content present = %v in %s", on, hasContent, out)
		}
		if on && string(delta["reasoning_content"]) != `"Let me think"` {
			t.Errorf("reasoning_content = %s", delta["reasoning_content"])
		}
	}
}

func TestRenameReasoning(t *testing.T) {
	choices := []types.ChatCompletionChoice{
		{Message: types.ChatCompletionMessage{Reasoning: "a"}},
		{Message: types.ChatCompletionMessage{Reasoning: "b", ReasoningContent: "kept"}},
		{Message: types.ChatCompletionMessage{}},
	}
	renameReasoning(choices)
	if choices[0].Message.ReasoningContent != "a" || choices[0].Message.Reasoning != "" {
		t.Errorf("first: %#v", choices[0].Message)
	}
	if choices[1].Message.ReasoningContent != "kept" || choices[1].Message.Reasoning != "b" {
		t.Errorf("second: %#v", choices[1].Message)
	}
	if choices[2].Message.ReasoningContent != "" {
		t.Errorf("third: %#v", choices[2].Message)
	}
}
