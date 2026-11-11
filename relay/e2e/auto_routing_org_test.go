package e2e

// Organization routing policies (ROUTE-11): an organization's API keys use its own policy of a name
// the platform also uses, logs carry the policy id, and "Try it" in the organization console is billed
// to the organization.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/middleware"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/relay"

	"github.com/gin-gonic/gin"
)

// newOrgRoutingFixture adds an organization policy named like the platform one but with the targets
// swapped (small → big), so the upstream model name shows which policy ran.
func newOrgRoutingFixture(t *testing.T, h *Harness) (tokenKey string, org *model.Organization, policy *model.RoutingPolicy) {
	t.Helper()
	prev := config.UserRoutingPolicyEnabled
	config.UserRoutingPolicyEnabled = true
	t.Cleanup(func() { config.UserRoutingPolicyEnabled = prev })

	tokenKey, shadowUserId, _ := orgTokenFixture(t, h, "default", nil, 0)
	org = &model.Organization{}
	if err := model.DB.Where("shadow_user_id = ?", shadowUserId).First(org).Error; err != nil {
		t.Fatalf("load organization: %v", err)
	}
	policy = &model.RoutingPolicy{
		OrganizationId: org.Id,
		Name:           routingPolicyName, Enabled: true, DecisionModel: routingDecisionName,
		Question: "How big is the change?",
		Routes: []model.RoutingRoute{
			{Label: "small", Description: "additive change", TargetModel: routingBigModel},
			{Label: "large", Description: "cross-module change", TargetModel: routingCheapModel},
		},
		FallbackModel: routingCheapModel, Threshold: 0.5, PinTTLSeconds: 3600, Scoring: model.RoutingScoringSelfReported,
	}
	if err := policy.Insert(); err != nil {
		t.Fatalf("create organization policy: %v", err)
	}
	return tokenKey, org, policy
}

func TestAutoRoutingOrganizationPolicyOverridesPlatform(t *testing.T) {
	decision := NewFakeProvider(t)
	decision.SetResponse(FakeResponse{JSON: decisionJSON("small", 0.9)})
	target := NewFakeProvider(t)
	target.SetResponse(FakeResponse{JSON: ChatCompletionJSON(routingBigModel, "done", 100, 50)})
	h := newRoutingHarness(t, decision.Server.URL, target.Server.URL, 0)
	tokenKey, org, policy := newOrgRoutingFixture(t, h)

	w := h.PostWithToken(tokenKey, "/v1/chat/completions", chatBodyFor(routingPolicyName, "add a log line"))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := target.LastRequest(t).JSONBody(t)["model"]; got != routingBigModel {
		t.Fatalf("the organization policy sends small to big-model, upstream got %v", got)
	}
	logs := consumeLogsByModel(t)
	meta := logs[routingBigModel].Metadata.Data()
	if id, _ := meta["routing_policy_id"].(float64); int(id) != policy.Id {
		t.Fatalf("target log should carry the organization policy id %d: %v", policy.Id, meta)
	}
	dmeta := logs[routingDecisionName].Metadata.Data()
	if id, _ := dmeta["routing_policy_id"].(float64); int(id) != policy.Id || dmeta["routing_decision"] != true {
		t.Fatalf("decision log should carry the organization policy id %d: %v", policy.Id, dmeta)
	}
	if logs[routingDecisionName].UserId != org.ShadowUserId {
		t.Fatalf("the decision call is charged to the organization, got user %d", logs[routingDecisionName].UserId)
	}

	// The same name from a personal key still means the platform policy (small → cheap).
	target.SetResponse(FakeResponse{JSON: ChatCompletionJSON(routingCheapModel, "done", 100, 50)})
	personal := h.Post("/v1/chat/completions", chatBodyFor(routingPolicyName, "add another log line"))
	if personal.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", personal.Code, personal.Body.String())
	}
	if got := target.LastRequest(t).JSONBody(t)["model"]; got != routingCheapModel {
		t.Fatalf("personal keys use the platform policy, upstream got %v", got)
	}
}

// "Try it" in the organization console calls the decision model for real, so it is charged to the
// organization's shadow account and logged as a try (kept out of the policy stats).
func TestOrganizationRoutingTryIsBilled(t *testing.T) {
	decision := NewFakeProvider(t)
	decision.SetResponse(FakeResponse{JSON: decisionJSON("small", 0.9)})
	target := NewFakeProvider(t)
	h := newRoutingHarness(t, decision.Server.URL, target.Server.URL, 0)
	_, org, policy := newOrgRoutingFixture(t, h)
	before, _ := model.GetUserQuota(org.ShadowUserId)

	body, _ := json.Marshal(map[string]any{"policy": policy, "text": "add a log line"})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/org/1/routing_policy/try", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", h.User.Id)
	c.Set(middleware.OrgContextKey, org)
	relay.OrgTryRoutingPolicyHandler(c)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data []struct {
			Label       string `json:"label"`
			TargetModel string `json:"target_model"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || len(resp.Data) != 1 {
		t.Fatalf("decode try response: %v (%s)", err, w.Body.String())
	}
	if resp.Data[0].Label != "small" || resp.Data[0].TargetModel != routingBigModel {
		t.Fatalf("want small → big-model, got %+v", resp.Data[0])
	}

	after, _ := model.GetUserQuota(org.ShadowUserId)
	if spent := before - after; spent != 28 {
		t.Fatalf("the try should charge the decision call (28) to the organization, spent %d", spent)
	}
	log, ok := consumeLogsByModel(t)[routingDecisionName]
	if !ok {
		t.Fatal("the try should write a consume log")
	}
	meta := log.Metadata.Data()
	if meta["routing_try"] != true || log.UserId != org.ShadowUserId {
		t.Fatalf("try log should be marked and charged to the organization: user=%d meta=%v", log.UserId, meta)
	}
	// The harness runs on SQLite; the stats queries pick their JSON functions from this flag.
	prevSQLite := common.UsingSQLite
	common.UsingSQLite = true
	defer func() { common.UsingSQLite = prevSQLite }()
	stats, err := model.GetRoutingPolicyStats(policy, 7)
	if err != nil {
		t.Fatal(err)
	}
	if stats.DecisionCalls != 0 {
		t.Fatalf("tries must not count as decision calls, got %d", stats.DecisionCalls)
	}
}
