package ahocorasick

import (
	"encoding/json"
	"math/rand"
	"os"
	"reflect"
	"slices"
	"testing"
)

type goldenMatch struct {
	Pos  int    `json:"pos"`
	Word string `json:"word"`
}

type goldenCase struct {
	Name     string        `json:"name"`
	Patterns []string      `json:"patterns"`
	Text     string        `json:"text"`
	Matches  []goldenMatch `json:"matches"`
}

func find(patterns []string, text string) []goldenMatch {
	out := []goldenMatch{}
	for _, m := range New(patterns).FindAll([]rune(text)) {
		out = append(out, goldenMatch{Pos: m.Pos, Word: string(m.Word)})
	}
	return out
}

// naiveFind is the reference: for every end offset, every distinct non-empty pattern that ends
// there, shortest first.
func naiveFind(patterns []string, text string) []goldenMatch {
	var words [][]rune
	seen := map[string]bool{}
	for _, p := range patterns {
		w := []rune(p)
		if len(w) == 0 || seen[string(w)] {
			continue
		}
		seen[string(w)] = true
		words = append(words, w)
	}
	slices.SortStableFunc(words, func(a, b []rune) int { return len(a) - len(b) })
	rt := []rune(text)
	out := []goldenMatch{}
	for end := 1; end <= len(rt); end++ {
		for _, w := range words {
			if len(w) <= end && slices.Equal(rt[end-len(w):end], w) {
				out = append(out, goldenMatch{Pos: end - len(w), Word: string(w)})
			}
		}
	}
	return out
}

// The golden file was recorded from the previous third-party matcher
// (github.com/anknown/ahocorasick) right before it was removed, after a 20,000-case random sweep
// showed both matchers returning identical results. It pins positions, overlap reporting,
// ordering, duplicate handling and case sensitivity across ASCII, CJK, emoji (ZWJ sequences,
// skin-tone modifiers), combining marks and U+FFFD.
func TestFindAllMatchesLegacyGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/legacy_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []goldenCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 200 {
		t.Fatalf("golden corpus has only %d cases", len(cases))
	}
	for i, c := range cases {
		if got := find(c.Patterns, c.Text); !reflect.DeepEqual(got, c.Matches) {
			t.Errorf("case %d (%s): patterns %q text %q\n got  %v\n want %v", i, c.Name, c.Patterns, c.Text, got, c.Matches)
		}
	}
}

var alphabets = [][]rune{
	[]rune("aAbB"),
	[]rune("敏感词汇中文"),
	[]rune("😀👍🏽‍❤️"),
	[]rune("aA敏😀‍�́"),
}

func randString(r *rand.Rand, alpha []rune, n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = alpha[r.Intn(len(alpha))]
	}
	return string(b)
}

func TestFindAllMatchesNaiveReference(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 5000; i++ {
		alpha := alphabets[i%len(alphabets)]
		pats := make([]string, 1+r.Intn(8))
		for j := range pats {
			pats[j] = randString(r, alpha, r.Intn(5))
		}
		text := randString(r, alpha, r.Intn(60))
		if got, want := find(pats, text), naiveFind(pats, text); !reflect.DeepEqual(got, want) {
			t.Fatalf("patterns %q text %q\n got  %v\n want %v", pats, text, got, want)
		}
	}
}

func TestContainsStringAgreesWithFindAll(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	for i := 0; i < 3000; i++ {
		alpha := alphabets[i%len(alphabets)]
		pats := make([]string, 1+r.Intn(4))
		for j := range pats {
			pats[j] = randString(r, alpha, 1+r.Intn(3))
		}
		text := randString(r, alpha, r.Intn(30))
		m := New(pats)
		if got, want := m.ContainsString(text), len(m.FindAll([]rune(text))) > 0; got != want {
			t.Fatalf("patterns %q text %q: ContainsString=%v FindAll=%v", pats, text, got, want)
		}
	}
	// Invalid UTF-8 decodes to U+FFFD byte by byte, exactly as []rune(s) does.
	m := New([]string{"��"})
	if !m.ContainsString("a\xff\xfeb") {
		t.Fatal("two invalid bytes must match two U+FFFD runes")
	}
	if m.ContainsString("a\xffb") {
		t.Fatal("one invalid byte is a single U+FFFD")
	}
}

func TestEmptyPatternsAreIgnored(t *testing.T) {
	// The previous matcher panicked (index out of range) when a pattern was empty, which a
	// whitespace-only keyword line produced.
	m := New([]string{"", "a", ""})
	if got := m.FindAll([]rune("bab")); len(got) != 1 || got[0].Pos != 1 {
		t.Fatalf("got %v", got)
	}
	for _, m := range []*Matcher{New(nil), New([]string{""})} {
		if m.ContainsString("anything") || len(m.FindAll([]rune("anything"))) != 0 {
			t.Fatal("a matcher without patterns must match nothing")
		}
	}
}

func TestMatchingIsCaseSensitive(t *testing.T) {
	m := New([]string{"API key not valid"})
	if m.ContainsString("api key not valid") || m.ContainsString("API KEY NOT VALID") {
		t.Fatal("matching must not fold case")
	}
	if !m.ContainsString(`{"message":"API key not valid. Please pass a valid API key."}`) {
		t.Fatal("exact substring must match")
	}
}

func FuzzFindAllMatchesNaive(f *testing.F) {
	f.Add("he\nshe\nhis\nhers", "ushers")
	f.Add("a\naa\naaa", "aaaa")
	f.Add("敏感\n感词", "敏感词")
	f.Add("👍🏽\n‍❤", "👍🏽‍❤️")
	f.Add("\xff\n�", "a\xffb")
	f.Fuzz(func(t *testing.T, patternLines, text string) {
		var pats []string
		start := 0
		for i := 0; i <= len(patternLines); i++ {
			if i == len(patternLines) || patternLines[i] == '\n' {
				pats = append(pats, patternLines[start:i])
				start = i + 1
			}
		}
		if got, want := find(pats, text), naiveFind(pats, text); !reflect.DeepEqual(got, want) {
			t.Fatalf("patterns %q text %q\n got  %v\n want %v", pats, text, got, want)
		}
	})
}
