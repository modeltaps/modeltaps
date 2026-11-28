package claude

import (
	"testing"

	"github.com/modeltaps/modeltaps/types"
)

// An error event of the upstream stream reaches a Messages client as a data line that carries
// the upstream's error type and message.
func TestClaudeRelayStreamErrorEvent(t *testing.T) {
	const want = `data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}` + "\n\n"

	h := &ClaudeRelayStreamHandler{Usage: &types.Usage{}, Prefix: `data: {`, SkipModelUnify: true}
	dataChan, errChan := make(chan string, 8), make(chan error, 8)
	raw := []byte(`data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`)
	h.HandlerStream(&raw, dataChan, errChan)
	if len(errChan) != 1 {
		t.Fatalf("errors = %d, want 1", len(errChan))
	}
	if got := (<-errChan).Error(); got != want {
		t.Errorf("error event = %q, want %q", got, want)
	}
	if len(dataChan) != 0 {
		t.Errorf("the upstream sends its own event line, got an extra %q", <-dataChan)
	}

	// Bedrock sends bare JSON, so the handler adds the event line.
	h = &ClaudeRelayStreamHandler{Usage: &types.Usage{}, Prefix: `{"type"`, AddEvent: true, SkipModelUnify: true}
	raw = []byte(`{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`)
	h.HandlerStream(&raw, dataChan, errChan)
	if got := <-dataChan; got != "event: error\n" {
		t.Errorf("event line = %q", got)
	}
	if got := (<-errChan).Error(); got != want {
		t.Errorf("error event = %q, want %q", got, want)
	}
}

// The chat stream reports the same event with its message.
func TestStreamErrorKeepsTheMessage(t *testing.T) {
	r := &ClaudeStreamResponse{Type: "error", Error: &ClaudeErrorInfo{Type: "overloaded_error", Message: "Overloaded"}}
	got := errorHandle(r.streamError())
	if got == nil || got.Message != "Overloaded" || got.Type != "overloaded_error" {
		t.Errorf("error = %+v", got)
	}
	if (&ClaudeStreamResponse{Type: "ping"}).streamError() != nil {
		t.Error("an event without an error must not report one")
	}
}
