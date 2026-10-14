package check_channel

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
)

// A random arithmetic challenge, asked through a few-shot template. The question changes every
// time, so a relay that fakes the response cannot pass; only a real call to a working model can.
const challengePromptTemplate = `Calculate and respond with ONLY the number, nothing else.

Q: 3 + 5 = ?
A: 8

Q: 12 - 7 = ?
A: 5

Q: %d %s %d = ?
A:`

const (
	challengeMin = 1
	challengeMax = 50
)

var challengeNumberRegex = regexp.MustCompile(`-?\d+`)

type challenge struct {
	Prompt   string
	Expected string
}

func generateChallenge() challenge {
	a := challengeMin + rand.IntN(challengeMax-challengeMin+1)
	b := challengeMin + rand.IntN(challengeMax-challengeMin+1)
	if rand.IntN(2) == 0 {
		return challenge{Prompt: fmt.Sprintf(challengePromptTemplate, a, "+", b), Expected: strconv.Itoa(a + b)}
	}
	// Subtract the smaller number so the answer is never negative.
	if b > a {
		a, b = b, a
	}
	return challenge{Prompt: fmt.Sprintf(challengePromptTemplate, a, "-", b), Expected: strconv.Itoa(a - b)}
}

// validateChallenge compares the last integer of the answer, tolerating a few extra words.
// A relay that echoes the prompt carries the few-shot answers and the operands, which may happen
// to equal the expected value: an answer containing the question mark of the prompt fails, and a
// match anywhere else in the answer does not count.
func validateChallenge(answer, expected string) bool {
	if expected == "" || strings.Contains(answer, "= ?") {
		return false
	}
	matches := challengeNumberRegex.FindAllString(answer, -1)
	return len(matches) > 0 && matches[len(matches)-1] == expected
}
