package openai

import (
	"testing"

	"github.com/modeltaps/modeltaps/types"
)

// feedChunk 驱动一次 HandlerChatStream(用带缓冲 channel 避免阻塞)。
func feedChunk(h *OpenAIStreamHandler, line string) {
	raw := []byte(line)
	dataChan := make(chan string, 8)
	errChan := make(chan error, 8)
	h.HandlerChatStream(&raw, dataChan, errChan)
}

// 流式累积:finish 分片先到、尾部 usage 分片(choices 为空)后到,
// 整体覆盖 Usage 不得清空已捕获的 finish_reason。
func TestHandlerChatStreamFinishReasonAcrossUsageChunk(t *testing.T) {
	usage := &types.Usage{}
	h := &OpenAIStreamHandler{Usage: usage, ModelName: "gpt-4o"}

	feedChunk(h, `data: {"choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":null}]}`)
	if usage.FinishReason != "" {
		t.Fatalf("content chunk should not set finish_reason, got %q", usage.FinishReason)
	}

	feedChunk(h, `data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)
	if usage.FinishReason != "stop" {
		t.Fatalf("finish chunk should set finish_reason=stop, got %q", usage.FinishReason)
	}

	// 尾部 usage-only 分片:整体覆盖 Usage 后必须保留 finish_reason
	feedChunk(h, `data: {"choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`)
	if usage.FinishReason != "stop" {
		t.Fatalf("finish_reason must survive usage overwrite, got %q", usage.FinishReason)
	}
	if usage.CompletionTokens != 2 {
		t.Fatalf("usage should be applied, completion_tokens=%d", usage.CompletionTokens)
	}
}

// 取最后一个非空 finish_reason:多分片携带 finish_reason 时以最后一个为准。
func TestHandlerChatStreamFinishReasonLastNonEmpty(t *testing.T) {
	usage := &types.Usage{}
	h := &OpenAIStreamHandler{Usage: usage, ModelName: "gpt-4o"}

	feedChunk(h, `data: {"choices":[{"index":0,"delta":{},"finish_reason":"length"}]}`)
	feedChunk(h, `data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`)
	if usage.FinishReason != "tool_calls" {
		t.Fatalf("expected last non-empty finish_reason=tool_calls, got %q", usage.FinishReason)
	}
}
