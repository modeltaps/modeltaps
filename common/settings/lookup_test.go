package settings

import (
	"slices"
	"strings"
	"testing"
)

func keysOf(list []Setting) []string {
	out := make([]string, 0, len(list))
	for _, s := range list {
		out = append(out, s.Key)
	}
	return out
}

func TestSearchDropsWordsNoEntryHas(t *testing.T) {
	mixed, plain := keysOf(Search("retry 重试", Filter{})), keysOf(Search("retry", Filter{}))
	if len(plain) == 0 || !slices.Equal(mixed, plain) {
		t.Fatalf("a word in another language should be dropped: %v vs %v", mixed, plain)
	}
	if got := Search("重试", Filter{}); len(got) != 0 {
		t.Fatalf("a query of unknown words should find nothing, got %v", keysOf(got))
	}
}

func TestSearchFallsBackToTheMostMatchedWords(t *testing.T) {
	got := keysOf(Search("upstream keeps returning 429 too many retries", Filter{}))
	if len(got) == 0 || got[0] != "RetryCooldownSeconds" {
		t.Fatalf("a question-like query should find the rate limit cooldown first, got %v", got)
	}
	// Every word still has to match when some entry has them all.
	for _, s := range Search("retry 429", Filter{}) {
		text := SearchText(s)
		if !strings.Contains(text, "retry") || !strings.Contains(text, "429") {
			t.Fatalf("%s does not have every word", s.Key)
		}
	}
}
