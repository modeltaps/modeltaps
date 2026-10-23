package claude

import (
	"net/http"
	"testing"

	"github.com/modeltaps/modeltaps/types"
)

func TestConvertFromChatOpenaiRejectsInvalidToolChoice(t *testing.T) {
	req := &types.ChatCompletionRequest{
		Model:      "claude-sonnet-4-5",
		Messages:   []types.ChatCompletionMessage{{Role: "user", Content: "hi"}},
		ToolChoice: map[string]any{"type": "function"},
	}
	_, errWithCode := ConvertFromChatOpenai(req)
	if errWithCode == nil {
		t.Fatal("expected an error for tool_choice without a function object")
	}
	if errWithCode.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", errWithCode.StatusCode)
	}
}
