package claude

import (
	"testing"

	"github.com/modeltaps/modeltaps/types"
)

func convertForTest(t *testing.T, request *types.ChatCompletionRequest) *ClaudeRequest {
	t.Helper()
	if request.Model == "" {
		request.Model = "claude-sonnet-5-5"
	}
	if request.MaxTokens == 0 {
		request.MaxTokens = 1024
	}
	claudeRequest, errWithCode := ConvertFromChatOpenai(request)
	if errWithCode != nil {
		t.Fatalf("ConvertFromChatOpenai returned error: %v", errWithCode)
	}
	return claudeRequest
}

func TestConvertFromChatOpenaiLiftsDeveloperToSystem(t *testing.T) {
	claudeRequest := convertForTest(t, &types.ChatCompletionRequest{
		Messages: []types.ChatCompletionMessage{
			{Role: types.ChatMessageRoleDeveloper, Content: "Answer in English."},
			{Role: types.ChatMessageRoleUser, Content: "hi"},
		},
	})

	if claudeRequest.System != "Answer in English." {
		t.Fatalf("System = %#v, want the developer message", claudeRequest.System)
	}
	if len(claudeRequest.Messages) != 1 {
		t.Fatalf("got %d messages, want 1: %#v", len(claudeRequest.Messages), claudeRequest.Messages)
	}
	if claudeRequest.Messages[0].Role != types.ChatMessageRoleUser {
		t.Fatalf("Messages[0].Role = %q, want user", claudeRequest.Messages[0].Role)
	}
}

func TestConvertFromChatOpenaiJoinsSystemMessagesWithNewline(t *testing.T) {
	claudeRequest := convertForTest(t, &types.ChatCompletionRequest{
		Messages: []types.ChatCompletionMessage{
			{Role: types.ChatMessageRoleSystem, Content: "You are a helpful assistant."},
			{Role: types.ChatMessageRoleDeveloper, Content: "Answer in English."},
			{Role: types.ChatMessageRoleUser, Content: "hi"},
		},
	})

	want := "You are a helpful assistant.\nAnswer in English."
	if claudeRequest.System != want {
		t.Fatalf("System = %#v, want %q", claudeRequest.System, want)
	}
	if len(claudeRequest.Messages) != 1 {
		t.Fatalf("got %d messages, want 1: %#v", len(claudeRequest.Messages), claudeRequest.Messages)
	}
}

func TestConvertFromChatOpenaiKeepsPresetSystem(t *testing.T) {
	claudeRequest := convertForTest(t, &types.ChatCompletionRequest{
		System: "preset",
		Messages: []types.ChatCompletionMessage{
			{Role: types.ChatMessageRoleDeveloper, Content: "Answer in English."},
			{Role: types.ChatMessageRoleUser, Content: "hi"},
		},
	})

	if claudeRequest.System != "preset" {
		t.Fatalf("System = %#v, want the preset value", claudeRequest.System)
	}
	if len(claudeRequest.Messages) != 1 || claudeRequest.Messages[0].Role != types.ChatMessageRoleUser {
		t.Fatalf("developer message must not stay in messages: %#v", claudeRequest.Messages)
	}
}

func TestConvertFromChatOpenaiTrailingDeveloperWithThinking(t *testing.T) {
	// With thinking enabled the last message must not be a prefill, so a trailing
	// developer message is sent as user, the same as a trailing system message.
	claudeRequest := convertForTest(t, &types.ChatCompletionRequest{
		MaxTokens: 4096,
		Reasoning: &types.ChatReasoning{MaxTokens: 1024},
		Messages: []types.ChatCompletionMessage{
			{Role: types.ChatMessageRoleUser, Content: "hi"},
			{Role: types.ChatMessageRoleAssistant, Content: "hello"},
			{Role: types.ChatMessageRoleDeveloper, Content: "Continue."},
		},
	})

	if claudeRequest.System != nil {
		t.Fatalf("System = %#v, want nil", claudeRequest.System)
	}
	if len(claudeRequest.Messages) != 3 {
		t.Fatalf("got %d messages, want 3: %#v", len(claudeRequest.Messages), claudeRequest.Messages)
	}
	if last := claudeRequest.Messages[2]; last.Role != types.ChatMessageRoleUser {
		t.Fatalf("last message role = %q, want user", last.Role)
	}
}

func TestConvertFromChatOpenaiToolAndFunctionRoles(t *testing.T) {
	claudeRequest := convertForTest(t, &types.ChatCompletionRequest{
		Messages: []types.ChatCompletionMessage{
			{Role: types.ChatMessageRoleUser, Content: "weather?"},
			{
				Role: types.ChatMessageRoleAssistant,
				ToolCalls: []*types.ChatCompletionToolCalls{{
					Id:       "call_1",
					Type:     "function",
					Function: &types.ChatCompletionToolCallsFunction{Name: "get_weather", Arguments: `{"city":"Paris"}`},
				}},
			},
			{Role: types.ChatMessageRoleTool, ToolCallID: "call_1", Content: "sunny"},
			{Role: types.ChatMessageRoleFunction, Content: "legacy result"},
		},
	})

	if len(claudeRequest.Messages) != 4 {
		t.Fatalf("got %d messages, want 4: %#v", len(claudeRequest.Messages), claudeRequest.Messages)
	}

	toolUse := claudeRequest.Messages[1]
	if toolUse.Role != types.ChatMessageRoleAssistant {
		t.Fatalf("tool call message role = %q, want assistant", toolUse.Role)
	}
	toolUseContent, ok := toolUse.Content.([]MessageContent)
	if !ok || len(toolUseContent) != 1 || toolUseContent[0].Type != ContentTypeToolUes || toolUseContent[0].Id != "call_1" {
		t.Fatalf("tool call content = %#v, want one tool_use block", toolUse.Content)
	}

	toolResult := claudeRequest.Messages[2]
	if toolResult.Role != types.ChatMessageRoleUser {
		t.Fatalf("tool message role = %q, want user", toolResult.Role)
	}
	toolResultContent, ok := toolResult.Content.([]MessageContent)
	if !ok || len(toolResultContent) != 1 || toolResultContent[0].Type != ContentTypeToolResult || toolResultContent[0].ToolUseId != "call_1" {
		t.Fatalf("tool message content = %#v, want one tool_result block", toolResult.Content)
	}

	if role := claudeRequest.Messages[3].Role; role != types.ChatMessageRoleUser {
		t.Fatalf("function message role = %q, want user", role)
	}
}

func TestConvertRole(t *testing.T) {
	cases := map[string]string{
		types.ChatMessageRoleAssistant: types.ChatMessageRoleAssistant,
		types.ChatMessageRoleUser:      types.ChatMessageRoleUser,
		types.ChatMessageRoleTool:      types.ChatMessageRoleUser,
		types.ChatMessageRoleFunction:  types.ChatMessageRoleUser,
		"critic":                       types.ChatMessageRoleUser,
		"":                             types.ChatMessageRoleUser,
	}
	for role, want := range cases {
		if got := convertRole(role); got != want {
			t.Errorf("convertRole(%q) = %q, want %q", role, got, want)
		}
	}
}
