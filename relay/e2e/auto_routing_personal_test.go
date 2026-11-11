package e2e

// Personal routing policies (ROUTE-13 / ROUTE-14): a user's own keys call their policy as
// policy/<name> (the bare name keeps working and overrides the platform policy of that name), the
// policy is listed as policy/<name>, and "Try it" in the personal console is billed to the user.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/relay"

	"github.com/gin-gonic/gin"
)

// newPersonalRoutingFixture gives the harness user a policy named like the platform one but with the
// targets swapped (small → big), so the upstream model name shows which policy ran.
func newPersonalRoutingFixture(t *testing.T, h *Harness) *model.RoutingPolicy {
	t.Helper()
	setAndRestore(t, &config.UserRoutingPolicyEnabled, true)
	policy := &model.RoutingPolicy{
		UserId: h.User.Id,
		Name:   routingPolicyName, Enabled: true, DecisionModel: routingDecisionName,
		Question: "How big is the change?",
		Routes: []model.RoutingRoute{
			{Label: "small", Description: "additive change", TargetModel: routingBigModel},
			{Label: "large", Description: "cross-module change", TargetModel: routingCheapModel},
		},
		FallbackModel: routingCheapModel, Threshold: 0.5, PinTTLSeconds: 3600, Scoring: model.RoutingScoringSelfReported,
	}
	if err := policy.Insert(); err != nil {
		t.Fatalf("create personal policy: %v", err)
	}
	return policy
}

func TestAutoRoutingPersonalPolicyCall(t *testing.T) {
	decision := NewFakeProvider(t)
	decision.SetResponse(FakeResponse{JSON: decisionJSON("small", 0.9)})
	target := NewFakeProvider(t)
	target.SetResponse(FakeResponse{JSON: ChatCompletionJSON(routingBigModel, "done", 100, 50)})
	h := newRoutingHarness(t, decision.Server.URL, target.Server.URL, 0)
	policy := newPersonalRoutingFixture(t, h)

	callName := model.RoutingPolicyCallPrefix + routingPolicyName
	w := h.Post("/v1/chat/completions", chatBodyFor(callName, "add a log line"))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := target.LastRequest(t).JSONBody(t)["model"]; got != routingBigModel {
		t.Fatalf("the personal policy sends small to big-model, upstream got %v", got)
	}
	if len(decision.Requests()) != 1 {
		t.Fatalf("one decision call per request, got %d", len(decision.Requests()))
	}
	logs := consumeLogsByModel(t)
	meta := logs[routingBigModel].Metadata.Data()
	if id, _ := meta["routing_policy_id"].(float64); int(id) != policy.Id {
		t.Fatalf("target log should carry the personal policy id %d: %v", policy.Id, meta)
	}

	// The bare name is the user's own policy too (it overrides the platform's).
	w = h.Post("/v1/chat/completions", chatBodyFor(routingPolicyName, "add another log line"))
	if w.Code != http.StatusOK {
		t.Fatalf("bare name: want 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := target.LastRequest(t).JSONBody(t)["model"]; got != routingBigModel {
		t.Fatalf("the bare name uses the own policy, upstream got %v", got)
	}

	listed := listedModels(t, h)
	if !listed[callName] || listed[routingPolicyName] {
		t.Fatalf("the list shows the own policy as %s and hides the overridden platform name: %v", callName, listed)
	}

	// Switched off: policy/<name> is an unknown model, the bare name is the platform policy again.
	config.UserRoutingPolicyEnabled = false
	if w := h.Post("/v1/chat/completions", chatBodyFor(callName, "add a third log line")); w.Code != http.StatusNotFound {
		t.Fatalf("switched off: want 404, got %d: %s", w.Code, w.Body.String())
	}
}

// policy/<name> never reaches a platform policy.
func TestAutoRoutingPolicyPrefixSkipsPlatform(t *testing.T) {
	decision := NewFakeProvider(t)
	decision.SetResponse(FakeResponse{JSON: decisionJSON("small", 0.9)})
	target := NewFakeProvider(t)
	h := newRoutingHarness(t, decision.Server.URL, target.Server.URL, 0)
	setAndRestore(t, &config.UserRoutingPolicyEnabled, true)

	w := h.Post("/v1/chat/completions", chatBodyFor(model.RoutingPolicyCallPrefix+routingPolicyName, "add a log line"))
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 for policy/<platform policy>, got %d: %s", w.Code, w.Body.String())
	}
}

// "Try it" in the personal console is charged to the user and logged as a try.
func TestPersonalRoutingTryIsBilled(t *testing.T) {
	decision := NewFakeProvider(t)
	decision.SetResponse(FakeResponse{JSON: decisionJSON("small", 0.9)})
	target := NewFakeProvider(t)
	h := newRoutingHarness(t, decision.Server.URL, target.Server.URL, 0)
	policy := newPersonalRoutingFixture(t, h)
	before := h.UserQuota(t)

	body, _ := json.Marshal(map[string]any{"policy": policy, "text": "add a log line"})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/routing_policy/try", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", h.User.Id)
	relay.UserTryRoutingPolicyHandler(c)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	if spent := before - h.UserQuota(t); spent != 28 {
		t.Fatalf("the try should charge the decision call (28) to the user, spent %d", spent)
	}
	log, ok := consumeLogsByModel(t)[routingDecisionName]
	if !ok || log.Metadata.Data()["routing_try"] != true || log.UserId != h.User.Id {
		t.Fatalf("try log should be marked and charged to the user: %+v", log)
	}
}
