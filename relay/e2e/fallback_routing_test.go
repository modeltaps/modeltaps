package e2e

// Fallback routing: a fallback policy tries its models in order, the next one when every line of
// the one before has failed; an auto policy falls back from its target to its fallback model the
// same way.

import (
	"net/http"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

const (
	fallbackPolicyName = "auto/steady"
	fallbackPrimary    = "primary-model"
	fallbackSecond     = "second-model"
)

// newFallbackHarness: one fake upstream per model and a fallback policy primary → second.
func newFallbackHarness(t *testing.T, primaryURL, secondURL string) *Harness {
	t.Helper()
	h := NewHarness(t, HarnessOptions{
		Model: fallbackPrimary,
		Channels: []ChannelSpec{
			{Name: "fake-primary", BaseURL: primaryURL, Models: fallbackPrimary},
			{Name: "fake-second", BaseURL: secondURL, Models: fallbackSecond},
		},
		Prices: []PriceSpec{
			{Model: fallbackPrimary, Input: 10, Output: 10},
			{Model: fallbackSecond, Input: 1, Output: 1},
		},
	})
	policy := &model.RoutingPolicy{
		Name: fallbackPolicyName, Enabled: true, Type: model.RoutingPolicyTypeFallback,
		Models: []string{fallbackPrimary, fallbackSecond},
	}
	if err := policy.Insert(); err != nil {
		t.Fatalf("insert policy: %v", err)
	}
	if err := model.ReloadRoutingPolicyCache(); err != nil {
		t.Fatalf("reload policies: %v", err)
	}
	t.Cleanup(model.ResetRoutingPolicyCache)
	setAndRestore(t, &config.RetryTimes, 2)
	return h
}

// The primary model's only line fails: the request moves to the second model and is charged and
// logged as the second model.
func TestFallbackPolicyMovesToNextModel(t *testing.T) {
	primary := NewFakeProvider(t)
	primary.SetResponse(FakeResponse{Status: http.StatusInternalServerError, JSON: OpenAIErrorJSON("boom", "server_error", "internal_error")})
	second := NewFakeProvider(t)
	second.SetResponse(FakeResponse{JSON: ChatCompletionJSON(fallbackSecond, "ok", 100, 50)})
	h := newFallbackHarness(t, primary.Server.URL, second.Server.URL)
	quotaBefore := h.UserQuota(t)

	w := h.Post("/v1/chat/completions", chatBodyFor(fallbackPolicyName, "hello"))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if n := len(primary.Requests()); n != 1 {
		t.Fatalf("primary upstream got %d requests, want 1", n)
	}
	if got := second.LastRequest(t).JSONBody(t)["model"]; got != fallbackSecond {
		t.Fatalf("second upstream got model %v", got)
	}
	if got := w.Header().Get("X-Modeltaps-Route"); got != "policy=auto/steady; model=second-model; step=1; reason=primary" {
		t.Fatalf("X-Modeltaps-Route = %q", got)
	}
	if spent := quotaBefore - h.UserQuota(t); spent != 150 {
		t.Fatalf("charged %d, want 150 at the second model's price", spent)
	}
	logs := consumeLogsByModel(t)
	log, ok := logs[fallbackSecond]
	if !ok {
		t.Fatalf("no consume log for the second model: %v", logKeys(logs))
	}
	meta := log.Metadata.Data()
	if meta["routing_policy"] != fallbackPolicyName || meta["routing_target"] != fallbackSecond || meta["routing_step"] != float64(1) {
		t.Fatalf("routing metadata = %v", meta)
	}
}

// The primary model answers: the second model is never asked.
func TestFallbackPolicyUsesPrimaryFirst(t *testing.T) {
	primary := NewFakeProvider(t)
	primary.SetResponse(FakeResponse{JSON: ChatCompletionJSON(fallbackPrimary, "ok", 10, 5)})
	second := NewFakeProvider(t)
	second.SetResponse(FakeResponse{JSON: ChatCompletionJSON(fallbackSecond, "ok", 10, 5)})
	h := newFallbackHarness(t, primary.Server.URL, second.Server.URL)

	w := h.Post("/v1/chat/completions", chatBodyFor(fallbackPolicyName, "hello"))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if len(second.Requests()) != 0 {
		t.Fatalf("second upstream was asked although the primary answered")
	}
	if got := w.Header().Get("X-Modeltaps-Route"); got != "policy=auto/steady; model=primary-model; reason=primary" {
		t.Fatalf("X-Modeltaps-Route = %q", got)
	}
}

// Every model fails: the client gets the error once each model's lines are used up.
func TestFallbackPolicyAllModelsFail(t *testing.T) {
	primary := NewFakeProvider(t)
	primary.SetResponse(FakeResponse{Status: http.StatusInternalServerError, JSON: OpenAIErrorJSON("boom", "server_error", "internal_error")})
	second := NewFakeProvider(t)
	second.SetResponse(FakeResponse{Status: http.StatusBadGateway, JSON: OpenAIErrorJSON("bad gateway", "server_error", "bad_gateway")})
	h := newFallbackHarness(t, primary.Server.URL, second.Server.URL)
	quotaBefore := h.UserQuota(t)

	w := h.Post("/v1/chat/completions", chatBodyFor(fallbackPolicyName, "hello"))
	if w.Code == http.StatusOK {
		t.Fatalf("want an error, got 200")
	}
	if len(primary.Requests()) != 1 || len(second.Requests()) != 1 {
		t.Fatalf("each upstream should be asked once, got primary=%d second=%d", len(primary.Requests()), len(second.Requests()))
	}
	if spent := quotaBefore - h.UserQuota(t); spent != 0 {
		t.Fatalf("a failed request charged %d", spent)
	}
}

// An auto policy whose chosen target fails falls back to its fallback model.
func TestAutoRoutingFallsBackWhenTargetFails(t *testing.T) {
	decision := NewFakeProvider(t)
	decision.SetResponse(FakeResponse{JSON: decisionJSON("small", 0.9)})
	cheap := NewFakeProvider(t)
	cheap.SetResponse(FakeResponse{Status: http.StatusInternalServerError, JSON: OpenAIErrorJSON("boom", "server_error", "internal_error")})
	big := NewFakeProvider(t)
	big.SetResponse(FakeResponse{JSON: ChatCompletionJSON(routingBigModel, "ok", 10, 5)})

	h := NewHarness(t, HarnessOptions{
		Model: routingCheapModel,
		Channels: []ChannelSpec{
			{Name: "fake-decision", BaseURL: decision.Server.URL, Models: routingDecisionName},
			{Name: "fake-cheap", BaseURL: cheap.Server.URL, Models: routingCheapModel},
			{Name: "fake-big", BaseURL: big.Server.URL, Models: routingBigModel},
		},
		Prices: []PriceSpec{
			{Model: routingDecisionName, Input: 1, Output: 1},
			{Model: routingCheapModel, Input: 1, Output: 1},
			{Model: routingBigModel, Input: 10, Output: 10},
		},
	})
	policy := &model.RoutingPolicy{
		Name: routingPolicyName, Enabled: true, DecisionModel: routingDecisionName, Question: "How big?",
		Routes: []model.RoutingRoute{
			{Label: "small", TargetModel: routingCheapModel},
			{Label: "large", TargetModel: routingBigModel},
		},
		FallbackModel: routingBigModel, Threshold: 0.5, Scoring: model.RoutingScoringSelfReported,
	}
	if err := policy.Insert(); err != nil {
		t.Fatalf("insert policy: %v", err)
	}
	if err := model.ReloadRoutingPolicyCache(); err != nil {
		t.Fatalf("reload policies: %v", err)
	}
	t.Cleanup(model.ResetRoutingPolicyCache)
	setAndRestore(t, &config.RetryTimes, 2)

	w := h.Post("/v1/chat/completions", chatBodyFor(routingPolicyName, "add a log line"))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if len(cheap.Requests()) != 1 {
		t.Fatalf("the chosen target should be tried once, got %d", len(cheap.Requests()))
	}
	if got := big.LastRequest(t).JSONBody(t)["model"]; got != routingBigModel {
		t.Fatalf("fallback upstream got model %v", got)
	}
	if got := w.Header().Get("X-Modeltaps-Route"); got != "policy=auto/coder; label=small; model=big-model; confidence=0.90; scoring=self_reported; step=1; reason=classified" {
		t.Fatalf("X-Modeltaps-Route = %q", got)
	}
}
