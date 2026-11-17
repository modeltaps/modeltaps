package gemini

import (
	"testing"

	"github.com/modeltaps/modeltaps/types"
)

// An image the gateway returned earlier is echoed back as "![modeltaps-gemini-image](url)" and
// must become an inline image between the surrounding text. Positions are rune offsets, so
// multi-byte text before the symbol must not shift the split.
func TestOpenAIToGeminiChatContentSplitsImageSymbol(t *testing.T) {
	text := "前文😀" + GeminiImageSymbol + "(data:image/png;base64,iVBORw0KGgo=)后文👍🏽"
	contents, _, errWithCode := OpenAIToGeminiChatContent([]types.ChatCompletionMessage{
		{Role: types.ChatMessageRoleAssistant, Content: text},
	})
	if errWithCode != nil {
		t.Fatal(errWithCode)
	}
	if len(contents) != 1 {
		t.Fatalf("got %d contents", len(contents))
	}
	parts := contents[0].Parts
	if len(parts) != 3 {
		t.Fatalf("got %d parts: %+v", len(parts), parts)
	}
	if parts[0].Text != "前文😀" || parts[2].Text != "后文👍🏽" {
		t.Errorf("text parts = %q, %q", parts[0].Text, parts[2].Text)
	}
	if got := parts[1].InlineData; got == nil || got.MimeType != "image/png" || got.Data != "iVBORw0KGgo=" {
		t.Errorf("inline data = %+v", got)
	}
}

// Two image symbols: the text between and after them is sent once each, and the second symbol
// is not sent as raw text.
func TestOpenAIToGeminiChatContentSplitsTwoImageSymbols(t *testing.T) {
	img := GeminiImageSymbol + "(data:image/png;base64,iVBORw0KGgo=)"
	text := "A" + img + "B" + img + "C"
	contents, _, errWithCode := OpenAIToGeminiChatContent([]types.ChatCompletionMessage{
		{Role: types.ChatMessageRoleAssistant, Content: text},
	})
	if errWithCode != nil {
		t.Fatal(errWithCode)
	}
	parts := contents[0].Parts
	if len(parts) != 5 {
		t.Fatalf("got %d parts: %+v", len(parts), parts)
	}
	for i, want := range map[int]string{0: "A", 2: "B", 4: "C"} {
		if parts[i].Text != want {
			t.Errorf("part %d text = %q, want %q", i, parts[i].Text, want)
		}
	}
	for _, i := range []int{1, 3} {
		if parts[i].InlineData == nil {
			t.Errorf("part %d is not an image: %+v", i, parts[i])
		}
	}
}
