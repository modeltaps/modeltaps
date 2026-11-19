package relay

import (
	"net/http"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// TestShouldRetry_StrictSessionAffinity: in strict mode a failure on the session's bound
// channel is returned instead of moving the session; new bindings and prefer mode retry.
func TestShouldRetry_StrictSessionAffinity(t *testing.T) {
	old := config.SessionAffinityStrict
	t.Cleanup(func() { config.SessionAffinityStrict = old })

	cases := []struct {
		strict   bool
		affinity string
		want     bool
	}{
		{true, "hit", false},
		{true, "bound", true},
		{true, "", true},
		{false, "hit", true},
	}
	for _, tc := range cases {
		config.SessionAffinityStrict = tc.strict
		c := newTestContext()
		c.Set(model.SessionAffinityContextKey, tc.affinity)
		if got := shouldRetry(c, apiErr(http.StatusServiceUnavailable, false), 0); got != tc.want {
			t.Errorf("strict=%v affinity=%q: shouldRetry = %v, want %v", tc.strict, tc.affinity, got, tc.want)
		}
	}
}

// Strict affinity keeps a session on its bound channel through ordinary failures, but not
// when the channel itself is unusable: staying on it cannot keep the session's cache.
func TestShouldRetry_StrictSessionAffinityUnusableChannel(t *testing.T) {
	old := config.SessionAffinityStrict
	config.SessionAffinityStrict = true
	t.Cleanup(func() { config.SessionAffinityStrict = old })

	c := newTestContext()
	c.Set(model.SessionAffinityContextKey, "hit")
	e := apiErr(http.StatusTooManyRequests, false)
	e.OpenAIError.Type = "insufficient_quota"
	if !shouldRetry(c, e, config.ChannelTypeOpenAI) {
		t.Fatal("strict affinity kept the session on a channel with no quota")
	}
}
