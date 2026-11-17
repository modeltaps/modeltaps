package vertexai

import "testing"

// TestGetVertexAIGeminiURLNeverForwardsUnknownMethods: an unknown method never reaches the
// upstream URL (SEC-66); it falls back to the billed generateContent methods.
func TestGetVertexAIGeminiURLNeverForwardsUnknownMethods(t *testing.T) {
	tests := []struct {
		action string
		stream bool
		want   string
	}{
		{"generateContent", false, "generateContent"},
		{"generateContent", true, "streamGenerateContent?alt=sse"},
		{"streamGenerateContent", true, "streamGenerateContent?alt=sse"},
		{"countTokens", false, "countTokens"},
		{"batchGenerateContent", false, "generateContent"},
		{"embedContent", true, "streamGenerateContent?alt=sse"},
		{"", false, "generateContent"},
	}
	for _, tt := range tests {
		if got := getVertexAIGeminiURL(tt.action, tt.stream); got != tt.want {
			t.Errorf("action %q stream %v: got %q want %q", tt.action, tt.stream, got, tt.want)
		}
	}
}
