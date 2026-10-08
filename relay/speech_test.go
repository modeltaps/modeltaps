package relay

import (
	"testing"

	"github.com/modeltaps/modeltaps/types"
)

func TestSpeechPromptTokensCountsCharacters(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"hello", 5},
		{"你好世界", 4},
		{"hello 你好", 8},
		{"こんにちは", 5},
		{"안녕", 2},
	}
	for _, tc := range cases {
		r := &relaySpeech{request: types.SpeechAudioRequest{Input: tc.input}}
		got, err := r.getPromptTokens()
		if err != nil {
			t.Fatalf("getPromptTokens(%q) error: %v", tc.input, err)
		}
		if got != tc.want {
			t.Errorf("getPromptTokens(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}
