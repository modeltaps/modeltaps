package e2e

import (
	"net/http"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/relay"
)

// chatInSession is a chat request of one client session: requests that carry the same
// prompt_cache_key are bound to the channel that served the first one.
func chatInSession(modelName, session string) string {
	return `{"model":"` + modelName + `","prompt_cache_key":"` + session + `","messages":[{"role":"user","content":"hi"}]}`
}

func twoChannels(t *testing.T, modelName string) (h *Harness, primary, backup *FakeProvider) {
	t.Helper()
	primary = NewFakeProvider(t)
	primary.SetResponse(FakeResponse{JSON: ChatCompletionJSON(modelName, "primary", 11, 7)})
	backup = NewFakeProvider(t)
	backup.SetResponse(FakeResponse{JSON: ChatCompletionJSON(modelName, "backup", 11, 7)})
	h = NewHarness(t, HarnessOptions{Model: modelName, Channels: []ChannelSpec{
		{Name: "primary", BaseURL: primary.Server.URL, Priority: 10},
		{Name: "backup", BaseURL: backup.Server.URL, Priority: 0},
	}})
	setAndRestore(t, &config.RetryTimes, 2)
	setAndRestore(t, &config.SessionAffinityEnabled, true)
	return h, primary, backup
}

func mustPost(t *testing.T, h *Harness, body string) {
	t.Helper()
	if w := h.Post("/v1/chat/completions", body); w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
}

// A session bound to a channel that fails once: the request is sent to the same channel again
// and passes there, so the session keeps the channel that holds its prompt cache. The backup is
// never asked.
func TestBoundSessionRetriesItsChannelOnce(t *testing.T) {
	const modelName = "same-channel-retry-model"
	h, primary, backup := twoChannels(t, modelName)
	session := "same-channel-retry"

	mustPost(t, h, chatInSession(modelName, session))
	healthy := FakeResponse{JSON: ChatCompletionJSON(modelName, "primary", 11, 7)}
	primary.SetResponse(FakeResponse{
		Status: http.StatusInternalServerError, JSON: OpenAIErrorJSON("boom", "server_error", "internal_error"),
		Hook: func() { primary.SetResponse(healthy) },
	})
	mustPost(t, h, chatInSession(modelName, session))

	if len(primary.Requests()) != 3 || len(backup.Requests()) != 0 {
		t.Fatalf("requests: primary %d, backup %d; want the failed request sent to the primary again", len(primary.Requests()), len(backup.Requests()))
	}
	logs := consumeLogs(t)
	attempts := logAttempts(t, logs[len(logs)-1])
	if len(attempts) != 1 || attempts[0].ChannelID != h.Channels[0].Id || attempts[0].Status != http.StatusInternalServerError {
		t.Fatalf("retry_attempts = %+v, want the one failure on the primary", attempts)
	}
}

// A request whose session is not on the failing channel has no cache to keep there: it moves to
// the next channel at once.
func TestUnboundRequestMovesOnAtOnce(t *testing.T) {
	const modelName = "unbound-moves-on-model"
	h, primary, backup := twoChannels(t, modelName)
	primary.SetResponse(FakeResponse{Status: http.StatusInternalServerError, JSON: OpenAIErrorJSON("boom", "server_error", "internal_error")})

	mustPost(t, h, chatInSession(modelName, "unbound-moves-on"))
	if len(primary.Requests()) != 1 || len(backup.Requests()) != 1 {
		t.Fatalf("requests: primary %d, backup %d; want one each", len(primary.Requests()), len(backup.Requests()))
	}
}

// A session whose channel keeps failing moves to the backup after the second try, and stays
// there once the primary serves again; a new session starts on the primary.
func TestSessionStaysWhereItMoved(t *testing.T) {
	const modelName = "session-stays-model"
	h, primary, backup := twoChannels(t, modelName)
	session := "session-stays"

	mustPost(t, h, chatInSession(modelName, session))
	primary.SetResponse(FakeResponse{Status: http.StatusBadGateway, JSON: OpenAIErrorJSON("bad gateway", "server_error", "bad_gateway")})
	mustPost(t, h, chatInSession(modelName, session))
	if len(primary.Requests()) != 3 || len(backup.Requests()) != 1 {
		t.Fatalf("requests: primary %d, backup %d; want the primary tried twice, then the backup", len(primary.Requests()), len(backup.Requests()))
	}

	primary.SetResponse(FakeResponse{JSON: ChatCompletionJSON(modelName, "primary", 11, 7)})
	mustPost(t, h, chatInSession(modelName, session))
	mustPost(t, h, chatInSession(modelName, session))
	if len(primary.Requests()) != 3 || len(backup.Requests()) != 3 {
		t.Fatalf("requests: primary %d, backup %d; want the session to stay on the backup", len(primary.Requests()), len(backup.Requests()))
	}
	mustPost(t, h, chatInSession(modelName, "session-stays-other"))
	if len(primary.Requests()) != 4 {
		t.Fatalf("a new session did not start on the primary (primary requests: %d)", len(primary.Requests()))
	}
}

// A rate limit that says when to come back: the model is kept off that channel until then, not
// for the fixed cooldown, and the request is served by the backup.
func TestRateLimitPausesUntilRetryAfter(t *testing.T) {
	const modelName = "retry-after-model"
	h, primary, backup := twoChannels(t, modelName)
	primary.SetResponse(FakeResponse{
		Status: http.StatusTooManyRequests, Headers: map[string]string{"Retry-After": "120"},
		JSON: OpenAIErrorJSON("Rate limit reached", "rate_limit_error", "rate_limit_exceeded"),
	})

	mustPost(t, h, chatFor(modelName))
	mustPost(t, h, chatFor(modelName))
	if len(primary.Requests()) != 1 || len(backup.Requests()) != 2 {
		t.Fatalf("requests: primary %d, backup %d; want the primary tried once", len(primary.Requests()), len(backup.Requests()))
	}
	pauses := storedPauses(t)
	if len(pauses) != 1 || pauses[0].ChannelId != h.Channels[0].Id || pauses[0].Model != modelName || pauses[0].Rule != "rate_limit_reset" {
		t.Fatalf("pauses = %+v, want the model paused on the primary by rate_limit_reset", pauses)
	}
	if left := pauses[0].Until - time.Now().Unix(); left < 110 || left > 122 {
		t.Fatalf("paused for %ds more, want about the 120s the upstream asked for", left)
	}
}

// shortHeaderTimeout makes the gateway give up on an upstream that stays silent after this
// long, so a test does not wait the two minutes of a deployment.
func shortHeaderTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	transport := requester.HTTPClient.Transport.(*http.Transport)
	old := transport.ResponseHeaderTimeout
	transport.ResponseHeaderTimeout = d
	t.Cleanup(func() {
		// The test request of the stall check runs in the background and uses the transport.
		relay.WaitRelayErrorHandling()
		transport.ResponseHeaderTimeout = old
	})
}

// An upstream that accepts requests and never answers: the request moves to the backup, a test
// request confirms the upstream is stalled, and only then is the model paused on it, with a
// test before it resumes. Later requests go straight to the backup.
func TestStalledUpstreamIsPausedAfterTheTestFails(t *testing.T) {
	const modelName = "stalled-upstream-model"
	h, primary, backup := twoChannels(t, modelName)
	shortHeaderTimeout(t, 200*time.Millisecond)
	primary.SetResponse(FakeResponse{Delay: 3 * time.Second, JSON: ChatCompletionJSON(modelName, "late", 11, 7)})

	mustPost(t, h, chatFor(modelName))
	pauses := storedPauses(t)
	if len(pauses) != 1 || pauses[0].ChannelId != h.Channels[0].Id || pauses[0].Model != modelName ||
		pauses[0].Rule != "upstream_failure" || !pauses[0].Probe {
		t.Fatalf("pauses = %+v, want the model paused on the primary with a recovery test", pauses)
	}
	if got := len(primary.Requests()); got != 2 {
		t.Fatalf("primary requests = %d, want the caller's request and the test request", got)
	}

	mustPost(t, h, chatFor(modelName))
	if len(primary.Requests()) != 2 || len(backup.Requests()) != 2 {
		t.Fatalf("requests: primary %d, backup %d; want the paused primary skipped", len(primary.Requests()), len(backup.Requests()))
	}
}

// A request that got no answer in time on an upstream that then answers the test request was
// only a long request: nothing is paused and the next request uses the channel again.
func TestSlowRequestDoesNotPauseAHealthyUpstream(t *testing.T) {
	const modelName = "slow-request-model"
	h, primary, backup := twoChannels(t, modelName)
	shortHeaderTimeout(t, 200*time.Millisecond)
	healthy := FakeResponse{JSON: ChatCompletionJSON(modelName, "primary", 11, 7)}
	primary.SetResponse(FakeResponse{
		Delay: 3 * time.Second, JSON: ChatCompletionJSON(modelName, "late", 11, 7),
		Hook: func() { primary.SetResponse(healthy) },
	})

	mustPost(t, h, chatFor(modelName))
	if pauses := storedPauses(t); len(pauses) != 0 {
		t.Fatalf("a slow request paused a channel that answers its test: %+v", pauses)
	}
	if model.ChannelGroup.IsInCooldown(h.Channels[0].Id, modelName) {
		t.Fatal("a slow request cooled down a channel that answers its test")
	}
	if len(primary.Requests()) != 2 || len(backup.Requests()) != 1 {
		t.Fatalf("requests: primary %d, backup %d; want the request, its test, and one on the backup", len(primary.Requests()), len(backup.Requests()))
	}
}
