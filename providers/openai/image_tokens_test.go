package openai

import "testing"

func TestGPTImageOutputTokens(t *testing.T) {
	cases := []struct {
		quality, size string
		want          int
	}{
		{"", "", 1056},
		{"auto", "auto", 1056},
		{"high", "1024x1536", 6240},
		{"xhigh", "1024x1024", 3122},
		{"XHigh", "1536x1024", 4683},
		{"max", "1024x1536", 10536},
		{"max", "2048x2048", 7024},
		{"unknown", "1024x1024", 1056},
	}
	for _, c := range cases {
		if got := GPTImageOutputTokens(c.quality, c.size); got != c.want {
			t.Errorf("GPTImageOutputTokens(%q, %q) = %d, want %d", c.quality, c.size, got, c.want)
		}
	}
}
