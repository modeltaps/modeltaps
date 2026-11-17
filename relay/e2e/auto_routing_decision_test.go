package e2e

import (
	"net/http"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/relay"
)

const routingDecisionModel = "router-decision-1"

// newDecisionRoutingHarness is newRoutingHarness with a decision model (served on /v1/decisions
// only) as the policy's decision model.
func newDecisionRoutingHarness(t *testing.T, decisionURL, targetURL string) *Harness {
	t.Helper()
	h := NewHarness(t, HarnessOptions{
		Model: routingCheapModel,
		Channels: []ChannelSpec{
			{Name: "fake-decision", BaseURL: decisionURL, Models: routingDecisionModel},
			{Name: "fake-target", BaseURL: targetURL, Models: routingCheapModel + "," + routingBigModel},
		},
		Prices: []PriceSpec{
			{Model: routingDecisionModel, Input: 1, Output: 0},
			{Model: routingCheapModel, Input: 1, Output: 1},
			{Model: routingBigModel, Input: 10, Output: 10},
		},
	})
	info := model.ModelInfo{Model: routingDecisionModel, Status: model.ModelStatusPublished, Endpoints: `["decisions"]`}
	if err := model.DB.Create(&info).Error; err != nil {
		t.Fatal(err)
	}
	policy := &model.RoutingPolicy{
		Name: routingPolicyName, Enabled: true, DecisionModel: routingDecisionModel,
		Question: "How big is the change?",
		Routes: []model.RoutingRoute{
			{Label: "small", Description: "additive change", TargetModel: routingCheapModel},
			{Label: "large", Description: "cross-module change", TargetModel: routingBigModel},
		},
		FallbackModel: routingBigModel, Threshold: 0.5, PinTTLSeconds: 3600,
	}
	if err := policy.Insert(); err != nil {
		t.Fatalf("insert policy: %v", err)
	}
	if err := model.ReloadRoutingPolicyCache(); err != nil {
		t.Fatalf("reload policies: %v", err)
	}
	prevRedis := config.RedisEnabled
	config.RedisEnabled = false
	model.ClearStickyMemory()
	relay.ResetRoutingScoringCache()
	t.Cleanup(func() {
		config.RedisEnabled = prevRedis
		model.ResetRoutingPolicyCache()
		model.ClearStickyMemory()
	})
	return h
}

func decisionsAnswer(small, large float64) string {
	choice := "small"
	if large > small {
		choice = "large"
	}
	return `{"model":"router-decision-1","answers":[{"type":"choice","name":"route","choice":"` + choice +
		`","confidence":0.5,"probabilities":[{"value":"small","probability":` + ftoa(small) +
		`},{"value":"large","probability":` + ftoa(large) + `}]}],"usage":{"input_tokens":40,"output_tokens":0,"total_tokens":40}}`
}

// A decision model is asked one choice question on /v1/decisions: the policy's question, with the
// route labels and descriptions as the choices and the first user message as the input. The
// chosen label's probability is the confidence, and the call is billed by its input tokens.
func TestAutoRoutingDecisionModelAsksOneChoiceQuestion(t *testing.T) {
	decision := NewFakeProvider(t)
	decision.SetResponse(FakeResponse{Raw: decisionsAnswer(0.88, 0.12)})
	target := NewFakeProvider(t)
	target.SetResponse(FakeResponse{JSON: ChatCompletionJSON(routingCheapModel, "done", 100, 50)})
	h := newDecisionRoutingHarness(t, decision.Server.URL, target.Server.URL)
	quotaBefore := h.UserQuota(t)

	w := h.Post("/v1/chat/completions", chatBodyFor(routingPolicyName, "add a log line"))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}

	req := decision.LastRequest(t)
	if req.Path != "/v1/decisions" {
		t.Fatalf("decision upstream path = %s, want /v1/decisions", req.Path)
	}
	body := req.JSONBody(t)
	if body["model"] != routingDecisionModel || body["input"] != "add a log line" {
		t.Fatalf("decision body: %v", body)
	}
	questions := body["questions"].([]any)
	q := questions[0].(map[string]any)
	choices := q["choices"].([]any)
	if len(questions) != 1 || q["type"] != "choice" || q["instructions"] != "How big is the change?" || len(choices) != 2 {
		t.Fatalf("question: %v", q)
	}
	if c := choices[1].(map[string]any); c["value"] != "large" || c["description"] != "cross-module change" {
		t.Fatalf("choice: %v", c)
	}

	if got := target.LastRequest(t).JSONBody(t)["model"]; got != routingCheapModel {
		t.Fatalf("target model = %v", got)
	}
	if got := w.Header().Get("X-Modeltaps-Route"); got != "policy=auto/coder; label=small; model=cheap-model; confidence=0.88; scoring=decision; reason=classified" {
		t.Fatalf("X-Modeltaps-Route = %q", got)
	}
	// Target 150 tokens at 1/1, decision 40 input tokens at 1.
	if spent := quotaBefore - h.UserQuota(t); spent != 190 {
		t.Fatalf("spent %d, want 190", spent)
	}
	logs := consumeLogsByModel(t)
	if l, ok := logs[routingDecisionModel]; !ok || l.PromptTokens != 40 || l.CompletionTokens != 0 || l.Metadata.Data()["routing_decision"] != true {
		t.Fatalf("decision log: %+v (logs %v)", l, logKeys(logs))
	}
}

// The chosen label's probability below the threshold sends the request to the fallback.
func TestAutoRoutingDecisionModelBelowThreshold(t *testing.T) {
	decision := NewFakeProvider(t)
	decision.SetResponse(FakeResponse{Raw: decisionsAnswer(0.4, 0.35)})
	target := NewFakeProvider(t)
	target.SetResponse(FakeResponse{JSON: ChatCompletionJSON(routingBigModel, "done", 10, 5)})
	h := newDecisionRoutingHarness(t, decision.Server.URL, target.Server.URL)

	w := h.Post("/v1/chat/completions", chatBodyFor(routingPolicyName, "hmm"))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if got := target.LastRequest(t).JSONBody(t)["model"]; got != routingBigModel {
		t.Fatalf("target model = %v, want the fallback", got)
	}
}

// A refusal is a decision error: the request goes to the fallback, never fails.
func TestAutoRoutingDecisionModelRefusalFallsBack(t *testing.T) {
	decision := NewFakeProvider(t)
	decision.SetResponse(FakeResponse{Raw: `{"answers":[{"type":"refusal","name":"route"}],"usage":{"input_tokens":5}}`})
	target := NewFakeProvider(t)
	target.SetResponse(FakeResponse{JSON: ChatCompletionJSON(routingBigModel, "done", 10, 5)})
	h := newDecisionRoutingHarness(t, decision.Server.URL, target.Server.URL)

	w := h.Post("/v1/chat/completions", chatBodyFor(routingPolicyName, "hello"))
	if w.Code != http.StatusOK || target.LastRequest(t).JSONBody(t)["model"] != routingBigModel {
		t.Fatalf("status %d, want the fallback", w.Code)
	}
}
