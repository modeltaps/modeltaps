package e2e

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/prometheus/client_golang/prometheus"
)

const chatBody = `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`

// A channel listing a wildcard sends the requested model upstream, not the literal pattern;
// its model_mapping may name the request or the pattern, in that order.
func TestWildcardChannelSendsRequestedModel(t *testing.T) {
	cases := []struct {
		name    string
		mapping string
		want    string
	}{
		{"no mapping", "", "gpt-4o-mini"},
		{"mapping names the request", `{"gpt-4o-mini":"gpt-4o-mini-2024-07-18"}`, "gpt-4o-mini-2024-07-18"},
		{"mapping names the pattern", `{"gpt-4o*":"gpt-4o"}`, "gpt-4o"},
		{"request entry wins over pattern entry", `{"gpt-4o*":"gpt-4o","gpt-4o-mini":"gpt-4o-mini-2024-07-18"}`, "gpt-4o-mini-2024-07-18"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := NewFakeProvider(t)
			upstream.SetResponse(FakeResponse{JSON: ChatCompletionJSON(tc.want, "ok", 11, 7)})
			h := NewHarness(t, HarnessOptions{
				Channels: []ChannelSpec{{Name: "wild", BaseURL: upstream.Server.URL, Models: "gpt-4o*", ModelMapping: tc.mapping}},
				Prices: []PriceSpec{
					{Model: "gpt-4o-mini", Input: 1, Output: 1},
					{Model: "gpt-4o-mini-2024-07-18", Input: 1, Output: 1},
					{Model: "gpt-4o", Input: 1, Output: 1},
				},
			})

			if w := h.Post("/v1/chat/completions", chatBody); w.Code != http.StatusOK {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if got := upstream.LastRequest(t).JSONBody(t)["model"]; got != tc.want {
				t.Fatalf("upstream model = %v, want %s", got, tc.want)
			}
		})
	}
}

func rateLimited() FakeResponse {
	return FakeResponse{Status: http.StatusTooManyRequests, JSON: OpenAIErrorJSON("rate limited", "rate_limit_error", "rate_limit_exceeded")}
}

// With retries turned off, a primary that answers 429 is cooled down, so the next request goes
// to the backup instead of hitting the primary again.
func TestFailedChannelCooledDownWithoutRetries(t *testing.T) {
	primary := NewFakeProvider(t)
	primary.SetResponse(rateLimited())
	backup := NewFakeProvider(t)
	backup.SetResponse(FakeResponse{JSON: ChatCompletionJSON("gpt-4o-mini", "ok", 11, 7)})

	h := NewHarness(t, HarnessOptions{Channels: []ChannelSpec{
		{Name: "primary", BaseURL: primary.Server.URL, Priority: 10},
		{Name: "backup", BaseURL: backup.Server.URL, Priority: 1},
	}})
	setAndRestore(t, &config.RetryTimes, 0)
	setAndRestore(t, &config.RetryCooldownSeconds, 60)

	if w := h.Post("/v1/chat/completions", chatBody); w.Code == http.StatusOK {
		t.Fatalf("first request succeeded, want the primary's 429 to surface: %s", w.Body.String())
	}
	if !model.ChannelGroup.IsInCooldown(h.Channels[0].Id, "gpt-4o-mini") {
		t.Fatal("primary not cooled down after a 429 with retries off")
	}
	if w := h.Post("/v1/chat/completions", chatBody); w.Code != http.StatusOK {
		t.Fatalf("second request status %d, want the backup to serve it: %s", w.Code, w.Body.String())
	}
	if n := len(primary.Requests()); n != 1 {
		t.Fatalf("primary got %d requests, want 1", n)
	}
}

// When the retry budget ends before the channels do, the channel tried last is cooled down too.
func TestLastTriedChannelCooledDown(t *testing.T) {
	var specs []ChannelSpec
	for i, prio := range []int64{30, 20, 10} {
		f := NewFakeProvider(t)
		f.SetResponse(rateLimited())
		specs = append(specs, ChannelSpec{Name: "c" + strconv.Itoa(i), BaseURL: f.Server.URL, Priority: prio})
	}
	h := NewHarness(t, HarnessOptions{Channels: specs})
	setAndRestore(t, &config.RetryTimes, 2)
	setAndRestore(t, &config.RetryCooldownSeconds, 60)

	if w := h.Post("/v1/chat/completions", chatBody); w.Code == http.StatusOK {
		t.Fatalf("request succeeded with every channel rate limited: %s", w.Body.String())
	}
	for _, ch := range h.Channels {
		if !model.ChannelGroup.IsInCooldown(ch.Id, "gpt-4o-mini") {
			t.Errorf("channel %s (#%d) not cooled down", ch.Name, ch.Id)
		}
	}
}

func providerCount(t *testing.T, channelID int, modelName, status string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != "provider_requests_total" {
			continue
		}
		for _, m := range family.GetMetric() {
			labels := map[string]string{}
			for _, l := range m.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			if labels["channel_id"] == strconv.Itoa(channelID) && labels["model"] == modelName && labels["type"] == status {
				return m.GetCounter().GetValue()
			}
		}
	}
	return 0
}

// A failure that is not retried is counted once in the provider metrics.
func TestNonRetryableFailureCountedOnce(t *testing.T) {
	const modelName = "metric-probe-model"
	upstream := NewFakeProvider(t)
	upstream.SetResponse(FakeResponse{Status: http.StatusBadRequest, JSON: OpenAIErrorJSON("messages: field required", "invalid_request_error", "")})
	h := NewHarness(t, HarnessOptions{Model: modelName, FakeBaseURL: upstream.Server.URL})
	setAndRestore(t, &config.RetryTimes, 2)

	before := providerCount(t, h.Channel.Id, modelName, "400")
	if w := h.Post("/v1/chat/completions", `{"model":"`+modelName+`","messages":[{"role":"user","content":"hi"}]}`); w.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	// The counter is incremented on a goroutine: wait for the first increment, then give a
	// second one time to land before asserting.
	deadline := time.Now().Add(2 * time.Second)
	for providerCount(t, h.Channel.Id, modelName, "400")-before < 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)
	if got := providerCount(t, h.Channel.Id, modelName, "400") - before; got != 1 {
		t.Fatalf("400 counted %v times, want 1", got)
	}
}
