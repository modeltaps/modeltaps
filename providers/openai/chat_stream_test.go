package openai

import (
	"github.com/modeltaps/modeltaps/types"
	"testing"
)

// feedStream 把若干 SSE data 行逐个喂给 HandlerChatStream，drain 掉 data/err 通道。
func feedStream(h *OpenAIStreamHandler, lines []string) {
	dataChan := make(chan string, len(lines))
	errChan := make(chan error, len(lines))
	for _, line := range lines {
		raw := []byte(line)
		h.HandlerChatStream(&raw, dataChan, errChan)
	}
}

// 回归：含尾部 usage-only 分片(include_usage,真实 OpenAI 默认行为)时，
// 整体覆盖 Usage 不得清空已累积的 TextBuilder(BUG-1)。
func TestHandlerChatStream_TrailingUsageKeepsText(t *testing.T) {
	h := &OpenAIStreamHandler{
		Usage:     &types.Usage{},
		ModelName: "gpt-4o",
	}

	feedStream(h, []string{
		`data: {"id":"1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"}}]}`,
		`data: {"id":"1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"content":" world"}}]}`,
		// 尾部 usage-only 分片：choices 为空，仅携带 usage。
		`data: {"id":"1","object":"chat.completion.chunk","model":"gpt-4o","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
	})

	if got := h.Usage.TextBuilder.String(); got != "Hello world" {
		t.Fatalf("尾部 usage 分片清空了已累积文本: got %q, want %q", got, "Hello world")
	}
	if h.Usage.CompletionTokens != 5 || h.Usage.PromptTokens != 10 || h.Usage.TotalTokens != 15 {
		t.Fatalf("usage 未正确合并: prompt=%d completion=%d total=%d",
			h.Usage.PromptTokens, h.Usage.CompletionTokens, h.Usage.TotalTokens)
	}
}

// 回归：usage 携带在 choices[0].Usage 的变体，同样不得清空已累积文本。
func TestHandlerChatStream_ChoiceUsageKeepsText(t *testing.T) {
	h := &OpenAIStreamHandler{
		Usage:     &types.Usage{},
		ModelName: "gpt-4o",
	}

	feedStream(h, []string{
		`data: {"id":"1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Foo"}}]}`,
		`data: {"id":"1","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop","usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}]}`,
	})

	if got := h.Usage.TextBuilder.String(); got != "Foo" {
		t.Fatalf("choice usage 分片清空了已累积文本: got %q, want %q", got, "Foo")
	}
	if h.Usage.CompletionTokens != 1 || h.Usage.PromptTokens != 3 || h.Usage.TotalTokens != 4 {
		t.Fatalf("usage 未正确合并: prompt=%d completion=%d total=%d",
			h.Usage.PromptTokens, h.Usage.CompletionTokens, h.Usage.TotalTokens)
	}
}
