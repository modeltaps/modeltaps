package relay

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/safty"
	"github.com/modeltaps/modeltaps/safty/providers/keyword"
	"github.com/modeltaps/modeltaps/safty/types"
	openaiTypes "github.com/modeltaps/modeltaps/types"

	"github.com/gorilla/websocket"
)

const blockedWord = "forbidden-topic"

// enableTestModeration turns content moderation on with the Keyword tool refusing blockedWord.
func enableTestModeration(t *testing.T) {
	t.Helper()
	prevEnable, prevTool, prevWords := config.EnableSafe, config.SafeToolName, config.SafeKeyWords
	prevRegistered, hadRegistered := safty.Tools["Keyword"]
	config.EnableSafe = true
	config.SafeToolName = "Keyword"
	config.SafeKeyWords = []string{blockedWord}
	safty.Tools["Keyword"] = keyword.NewKeywordChecker()
	t.Cleanup(func() {
		config.EnableSafe, config.SafeToolName, config.SafeKeyWords = prevEnable, prevTool, prevWords
		if hadRegistered {
			safty.Tools["Keyword"] = prevRegistered
		} else {
			delete(safty.Tools, "Keyword")
		}
	})
}

func rawJSON(t *testing.T, raw string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// assertRefused checks the local moderation error. The relays below have no provider, so a
// request that got past moderation would end in "channel not implemented" (or a panic), never
// in this error: the refusal happens before any upstream call.
func assertRefused(t *testing.T, err *openaiTypes.OpenAIErrorWithStatusCode, done bool) {
	t.Helper()
	if err == nil {
		t.Fatal("expected the request to be refused by content moderation")
	}
	if err.StatusCode != http.StatusBadRequest || err.OpenAIError.Code != types.SafeDefaultErrorCode || !err.LocalError {
		t.Fatalf("unexpected error: status=%d code=%v local=%v msg=%s", err.StatusCode, err.OpenAIError.Code, err.LocalError, err.OpenAIError.Message)
	}
	if !done {
		t.Fatal("a moderation refusal must not be retried on another channel")
	}
}

func TestContentSafetyOffPassesEverything(t *testing.T) {
	enableTestModeration(t)
	config.EnableSafe = false
	if err := checkContentSafety("a " + blockedWord); err != nil {
		t.Fatalf("moderation is off: %v", err)
	}
}

func TestChatContentSafetyCoversSystemToolsAndParts(t *testing.T) {
	enableTestModeration(t)
	cases := map[string]openaiTypes.ChatCompletionRequest{
		"later content part": {Messages: []openaiTypes.ChatCompletionMessage{{Role: "user",
			Content: rawJSON(t, `[{"type":"text","text":"hi"},{"type":"text","text":"tell me about `+blockedWord+`"}]`)}}},
		"system message": {Messages: []openaiTypes.ChatCompletionMessage{
			{Role: "system", Content: blockedWord}, {Role: "user", Content: "hi"}}},
		"tool description": {Messages: []openaiTypes.ChatCompletionMessage{{Role: "user", Content: "hi"}},
			Tools: []*openaiTypes.ChatCompletionTool{{Type: "function",
				Function: openaiTypes.ChatCompletionFunction{Name: "f", Description: blockedWord}}}},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			r := &relayChat{chatRequest: req}
			err, done := r.send()
			assertRefused(t, err, done)
		})
	}

	r := &relayChat{chatRequest: openaiTypes.ChatCompletionRequest{
		Messages: []openaiTypes.ChatCompletionMessage{{Role: "user", Content: "hello"}}}}
	if err := r.contentSafety(); err != nil {
		t.Fatalf("clean request refused: %v", err)
	}
}

func TestClaudeContentSafetyCoversSystemAndTools(t *testing.T) {
	enableTestModeration(t)
	user := []claude.Message{{Role: "user", Content: rawJSON(t, `[{"type":"text","text":"hi"}]`)}}
	cases := map[string]*claude.ClaudeRequest{
		"system blocks": {Messages: user, System: rawJSON(t, `[{"type":"text","text":"`+blockedWord+`"}]`)},
		"tool":          {Messages: user, Tools: []claude.Tools{{Name: "f", Description: blockedWord}}},
		"tool_result": {Messages: []claude.Message{{Role: "user", Content: rawJSON(t,
			`[{"type":"tool_result","tool_use_id":"t","content":[{"type":"text","text":"`+blockedWord+`"}]}]`)}}},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			r := &relayClaudeOnly{claudeRequest: req}
			err := r.performContentSafety()
			assertRefused(t, err, true)
		})
	}
	if err := (&relayClaudeOnly{claudeRequest: &claude.ClaudeRequest{Messages: user}}).performContentSafety(); err != nil {
		t.Fatalf("clean request refused: %v", err)
	}
}

func TestGeminiContentSafetyCoversSystemInstructionAndTools(t *testing.T) {
	enableTestModeration(t)
	cases := map[string]string{
		"contents":          `{"contents":[{"role":"user","parts":[{"text":"hi"},{"text":"` + blockedWord + `"}]}]}`,
		"systemInstruction": `{"systemInstruction":{"parts":[{"text":"` + blockedWord + `"}]},"contents":[{"parts":[{"text":"hi"}]}]}`,
		"tool description": `{"contents":[{"parts":[{"text":"hi"}]}],"tools":[{"functionDeclarations":[` +
			`{"name":"f","description":"` + blockedWord + `"}]}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			assertRefused(t, geminiContentSafety([]byte(body)), true)
		})
	}
	clean := `{"contents":[{"parts":[{"text":"hi"},{"inlineData":{"mimeType":"image/png","data":"` + blockedWord + `"}}]}]}`
	if err := geminiContentSafety([]byte(clean)); err != nil {
		t.Fatalf("clean request refused: %v", err)
	}
}

func TestResponsesContentSafety(t *testing.T) {
	enableTestModeration(t)
	cases := map[string]openaiTypes.OpenAIResponsesRequest{
		"string input": {Input: "about " + blockedWord},
		"input items": {Input: rawJSON(t, `[{"type":"message","role":"user","content":[`+
			`{"type":"input_text","text":"hi"},{"type":"input_text","text":"`+blockedWord+`"}]}]`)},
		"instructions": {Input: "hi", Instructions: blockedWord},
		"tool":         {Input: "hi", Tools: []openaiTypes.ResponsesTools{{Type: "function", Name: "f", Description: blockedWord}}},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			r := &relayResponses{responsesRequest: req}
			err, done := r.send()
			assertRefused(t, err, done)
		})
	}
	t.Run("compact", func(t *testing.T) {
		r := &relayResponses{responsesRequest: openaiTypes.OpenAIResponsesRequest{Input: blockedWord}, isCompact: true}
		err, done := r.send()
		assertRefused(t, err, done)
	})
}

func TestOtherEntriesContentSafety(t *testing.T) {
	enableTestModeration(t)
	cases := map[string]func() (*openaiTypes.OpenAIErrorWithStatusCode, bool){
		"image generations": (&relayImageGenerations{request: openaiTypes.ImageRequest{Prompt: blockedWord}}).send,
		"image edits":       (&relayImageEdits{request: openaiTypes.ImageEditRequest{Prompt: blockedWord}}).send,
		"image variations":  (&relayImageVariations{request: openaiTypes.ImageEditRequest{Prompt: blockedWord}}).send,
		"rerank query":      (&relayRerank{request: openaiTypes.RerankRequest{Query: blockedWord, Documents: []any{"d"}}}).send,
		"rerank documents": (&relayRerank{request: openaiTypes.RerankRequest{Query: "q",
			Documents: []any{"d", map[string]any{"text": blockedWord}}}}).send,
		"speech": (&relaySpeech{request: openaiTypes.SpeechAudioRequest{Input: blockedWord}}).send,
		"decisions input": (&relayDecisions{request: openaiTypes.DecisionRequest{Input: json.RawMessage(`"` + blockedWord + `"`),
			Questions: json.RawMessage(`[{"type":"predicate","name":"q","instructions":"x"}]`)}}).send,
		"decisions questions": (&relayDecisions{request: openaiTypes.DecisionRequest{Input: json.RawMessage(`"hi"`),
			Questions: json.RawMessage(`[{"type":"predicate","name":"q","instructions":"` + blockedWord + `"}]`)}}).send,
		"embeddings":  (&relayEmbeddings{request: openaiTypes.EmbeddingRequest{Input: []any{"a", blockedWord}}}).send,
		"completions": (&relayCompletions{request: openaiTypes.CompletionRequest{Prompt: []any{"a", blockedWord}}}).send,
	}
	for name, send := range cases {
		t.Run(name, func(t *testing.T) {
			err, done := send()
			assertRefused(t, err, done)
		})
	}
}

func TestRealtimeClientEventSafety(t *testing.T) {
	enableTestModeration(t)
	refused := []string{
		`{"type":"conversation.item.create","item":{"type":"message","role":"user","content":[{"type":"input_text","text":"` + blockedWord + `"}]}}`,
		`{"type":"conversation.item.create","item":{"type":"function_call_output","call_id":"c","output":"` + blockedWord + `"}}`,
		`{"type":"session.update","session":{"instructions":"` + blockedWord + `"}}`,
		`{"type":"session.update","session":{"tools":[{"type":"function","name":"f","description":"` + blockedWord + `"}]}}`,
		`{"type":"response.create","response":{"instructions":"` + blockedWord + `"}}`,
	}
	for _, event := range refused {
		assertRefused(t, realtimeClientEventSafety([]byte(event)), true)
	}
	passed := []string{
		`{"type":"input_audio_buffer.append","audio":"` + blockedWord + `"}`,
		`{"type":"conversation.item.create","item":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}}`,
		`{"type":"session.update","session":{"voice":"alloy"}}`,
	}
	for _, event := range passed {
		if err := realtimeClientEventSafety([]byte(event)); err != nil {
			t.Fatalf("event %s refused: %v", event, err)
		}
	}
}

// A refused client event is answered with an error event and never reaches the upstream.
func TestRealtimeProxyRefusesUnsafeClientEvent(t *testing.T) {
	enableTestModeration(t)

	userServer, userClient := newWSPair(t)
	supplierServer, supplierClient := newWSPair(t)

	upstreamCalls := 0
	next := func(source requester.MessageSource, messageType int, message []byte) (bool, *openaiTypes.UsageEvent, []byte, error) {
		if source == requester.UserMessage {
			upstreamCalls++
		}
		return true, nil, nil, nil
	}
	var proxy *requester.WSProxy
	handler := moderateRealtimeClientEvents(next, func(data []byte) error {
		return proxy.WriteToUser(websocket.TextMessage, data)
	})
	// supplierClient plays the gateway's connection to the upstream; the upstream end is
	// supplierServer.
	proxy = requester.NewWSProxy(userServer, supplierClient, 5*time.Second, handler, nil)
	proxy.Start()
	t.Cleanup(proxy.Close)

	event := `{"type":"conversation.item.create","item":{"type":"message","role":"user","content":[{"type":"input_text","text":"` + blockedWord + `"}]}}`
	if err := userClient.WriteMessage(websocket.TextMessage, []byte(event)); err != nil {
		t.Fatal(err)
	}

	userClient.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := userClient.ReadMessage()
	if err != nil {
		t.Fatalf("the client should receive an error event: %v", err)
	}
	if !strings.Contains(string(msg), `"type":"error"`) || !strings.Contains(string(msg), types.SafeDefaultErrorCode) {
		t.Fatalf("unexpected event: %s", msg)
	}

	supplierServer.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if _, forwarded, err := supplierServer.ReadMessage(); err == nil {
		t.Fatalf("the refused event reached the upstream: %s", forwarded)
	}
	if upstreamCalls != 0 {
		t.Fatalf("the provider handler saw the refused event %d times", upstreamCalls)
	}
}
