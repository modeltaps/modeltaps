package relay

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/model"
)

func scoringPolicy(routes int) *model.RoutingPolicy {
	policy := &model.RoutingPolicy{Name: "p"}
	for i := 0; i < routes; i++ {
		policy.Routes = append(policy.Routes, model.RoutingRoute{Label: string(rune('a' + i))})
	}
	return policy
}

func TestOptionLetterIndex(t *testing.T) {
	cases := map[string]int{"A": 0, " B": 1, "b": 1, "C.": 2, "C)": 2, "\nA": 0, "D": -1, "AB": -1, "": -1, "Sure": -1}
	for token, want := range cases {
		if got := optionLetterIndex(token, 3); got != want {
			t.Errorf("optionLetterIndex(%q) = %d, want %d", token, got, want)
		}
	}
}

func logprobsPayload(tokens ...map[string]any) any {
	content := make([]any, 0, len(tokens))
	for _, tok := range tokens {
		content = append(content, tok)
	}
	return map[string]any{"content": content}
}

func top(token string, p float64) map[string]any {
	return map[string]any{"token": token, "logprob": math.Log(p)}
}

func TestParseLetterLogprobsMergesVariantsAndNormalizes(t *testing.T) {
	payload := logprobsPayload(map[string]any{
		"token": "B", "logprob": math.Log(0.6),
		"top_logprobs": []any{top("B", 0.6), top(" B", 0.1), top("A", 0.2), top("x", 0.1)},
	})
	result, err := parseLetterLogprobs(scoringPolicy(2), payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Label != "b" || math.Abs(result.Confidence-0.7/0.9) > 1e-9 {
		t.Fatalf("got label=%s confidence=%v", result.Label, result.Confidence)
	}
	if math.Abs(result.Coverage-0.9) > 1e-9 || result.Scoring != model.RoutingScoringLogprobs {
		t.Fatalf("coverage=%v scoring=%s", result.Coverage, result.Scoring)
	}
	if len(result.Probabilities) != 2 || result.Probabilities[1].Label != "a" {
		t.Fatalf("probabilities not sorted: %+v", result.Probabilities)
	}
}

func TestParseLetterLogprobsSkipsLeadingWhitespaceToken(t *testing.T) {
	payload := logprobsPayload(
		map[string]any{"token": "\n", "logprob": -0.01, "top_logprobs": []any{top("\n", 0.99)}},
		map[string]any{"token": "A", "logprob": math.Log(0.9), "top_logprobs": []any{top("A", 0.9), top("B", 0.1)}},
	)
	result, err := parseLetterLogprobs(scoringPolicy(2), payload)
	if err != nil || result.Label != "a" {
		t.Fatalf("got %+v, %v", result, err)
	}
}

func TestParseLetterLogprobsSampledTokenOnly(t *testing.T) {
	payload := logprobsPayload(map[string]any{"token": "A", "logprob": math.Log(0.7)})
	result, err := parseLetterLogprobs(scoringPolicy(2), payload)
	if err != nil || result.Label != "a" || result.Confidence != 1 || math.Abs(result.Coverage-0.7) > 1e-9 {
		t.Fatalf("got %+v, %v", result, err)
	}
}

func TestParseLetterLogprobsErrors(t *testing.T) {
	if _, err := parseLetterLogprobs(scoringPolicy(2), nil); !errors.Is(err, errNoLogprobs) {
		t.Fatalf("nil logprobs should be errNoLogprobs, got %v", err)
	}
	if _, err := parseLetterLogprobs(scoringPolicy(2), map[string]any{"content": []any{}}); !errors.Is(err, errNoLogprobs) {
		t.Fatalf("empty content should be errNoLogprobs, got %v", err)
	}
	low := logprobsPayload(map[string]any{"token": "Sure", "logprob": math.Log(0.8), "top_logprobs": []any{top("Sure", 0.8), top("A", 0.2)}})
	if _, err := parseLetterLogprobs(scoringPolicy(2), low); err == nil || errors.Is(err, errNoLogprobs) {
		t.Fatalf("low coverage should be a plain error, got %v", err)
	}
	// 超出路由数的字母不算。
	outOfRange := logprobsPayload(map[string]any{"token": "C", "logprob": math.Log(0.9), "top_logprobs": []any{top("C", 0.9), top("A", 0.1)}})
	if _, err := parseLetterLogprobs(scoringPolicy(2), outOfRange); err == nil {
		t.Fatalf("letter beyond the routes should not count")
	}
}

func TestBuildLetterMessagesLettersRoutes(t *testing.T) {
	policy := &model.RoutingPolicy{Question: "Q?", Routes: []model.RoutingRoute{{Label: "x", Description: "dx"}, {Label: "y"}}}
	msgs := buildLetterMessages(policy, "hello")
	user := msgs[1].StringContent()
	for _, want := range []string{"Q?", "A. x: dx", "B. y\n", "hello", "Answer with one letter."} {
		if !strings.Contains(user, want) {
			t.Fatalf("prompt missing %q:\n%s", want, user)
		}
	}
}

func TestLogprobsSupportCacheExpires(t *testing.T) {
	ResetRoutingScoringCache()
	t.Cleanup(ResetRoutingScoringCache)
	now := time.Unix(1_000_000, 0)
	prev := routingPolicyNow
	routingPolicyNow = func() time.Time { return now }
	t.Cleanup(func() { routingPolicyNow = prev })

	if _, known := lookupLogprobsSupport(1, "m"); known {
		t.Fatalf("empty cache should be unknown")
	}
	rememberLogprobsSupport(1, "m", false)
	if supported, known := lookupLogprobsSupport(1, "m"); !known || supported {
		t.Fatalf("want known unsupported")
	}
	if _, known := lookupLogprobsSupport(2, "m"); known {
		t.Fatalf("other channel should be unknown")
	}
	now = now.Add(routingLogprobsCacheTTL + time.Second)
	if _, known := lookupLogprobsSupport(1, "m"); known {
		t.Fatalf("entry should expire")
	}
}
