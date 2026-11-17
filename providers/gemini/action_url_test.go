package gemini

import "testing"

// TestGetActionURLNeverForwardsUnknownMethods: an unknown method never reaches the upstream URL
// (SEC-66); it falls back to the billed generateContent methods.
func TestGetActionURLNeverForwardsUnknownMethods(t *testing.T) {
	p := &GeminiProvider{}
	tests := []struct {
		action string
		stream bool
		want   string
	}{
		{"", false, "generateContent"},
		{"generateContent", false, "generateContent"},
		{"generateContent", true, "streamGenerateContent?alt=sse"},
		{"streamGenerateContent", true, "streamGenerateContent?alt=sse"},
		{"countTokens", false, "countTokens"},
		{"batchGenerateContent", false, "generateContent"},
		{"embedContent", true, "streamGenerateContent?alt=sse"},
		{"generateContent?key=x", false, "generateContent"},
	}
	for _, tt := range tests {
		got := p.getActionURL(&GeminiChatRequest{Action: tt.action, Stream: tt.stream})
		if got != tt.want {
			t.Errorf("action %q stream %v: got %q want %q", tt.action, tt.stream, got, tt.want)
		}
	}
}
