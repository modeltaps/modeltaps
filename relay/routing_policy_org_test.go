package relay

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// setupOrgRoutingResolveTest stores a platform policy "auto/t" plus an organization policy of the
// same name for organization 7, and makes member 5 of shadow account 100 belong to organization 7.
func setupOrgRoutingResolveTest(t *testing.T) (platform, own *model.RoutingPolicy, setResult func(label string, conf float64, err error)) {
	t.Helper()
	platform = testPolicy("auto/t", 0)
	_, setResult = setupRoutingResolveTest(t, platform)
	own = testPolicy("auto/t", 0)
	own.OrganizationId = 7
	own.Routes[0].TargetModel = "org-cheap-model"
	own.Normalize()
	// Stored directly: saving through Insert would also check the organization's group and catalog.
	if err := model.DB.Create(own).Error; err != nil {
		t.Fatalf("store organization policy: %v", err)
	}
	if err := model.ReloadRoutingPolicyCache(); err != nil {
		t.Fatal(err)
	}
	if err := model.ReloadModelInfoCache(); err != nil {
		t.Fatal(err)
	}
	stubOrgGuardrail(t, func(shadowUserId, memberUserId int) (*model.OrgMemberGuardrail, error) {
		return &model.OrgMemberGuardrail{OrganizationId: 7, MemberUserId: memberUserId}, nil
	})
	prev := config.UserRoutingPolicyEnabled
	config.UserRoutingPolicyEnabled = true
	t.Cleanup(func() {
		config.UserRoutingPolicyEnabled = prev
		_ = model.ReloadModelInfoCache()
	})
	return platform, own, setResult
}

func newOrgRoutingRequest() *gin.Context {
	c := newRoutingRequest("", routingBody)
	c.Set("id", 100)
	c.Set("token_created_by", 5)
	return c
}

// Organization keys get their organization's policy; personal keys and other organizations get the
// platform's. Decisions carry the policy id so logs and stats can tell them apart.
func TestRouteRoutingPolicyPrefersOrganizationPolicy(t *testing.T) {
	platform, own, setResult := setupOrgRoutingResolveTest(t)
	setResult("small", 0.9, nil)

	orgReq := newOrgRoutingRequest()
	if got, err := routeRoutingPolicy(orgReq, "auto/t"); err != nil || got != "org-cheap-model" {
		t.Fatalf("organization key: want org-cheap-model, got %q (%v)", got, err)
	}
	if d := decisionOf(t, orgReq); d.PolicyId != own.Id {
		t.Fatalf("decision should name the organization policy id %d, got %d", own.Id, d.PolicyId)
	}

	personal := newRoutingRequest("", routingBody)
	if got := resolveRoutingPolicy(personal, "auto/t"); got != "cheap-model" {
		t.Fatalf("personal key: want the platform target, got %q", got)
	}
	if d := decisionOf(t, personal); d.PolicyId != platform.Id {
		t.Fatalf("decision should name the platform policy id %d, got %d", platform.Id, d.PolicyId)
	}

	// Option off: organization keys fall back to the platform policy.
	config.UserRoutingPolicyEnabled = false
	off := newOrgRoutingRequest()
	if got := resolveRoutingPolicy(off, "auto/t"); got != "cheap-model" {
		t.Fatalf("option off: want the platform target, got %q", got)
	}
}

// A target the platform hid after the policy was saved sends the request to the fallback; when the
// fallback is hidden too the request fails as model not found instead of reaching a hidden model.
func TestRouteRoutingPolicyOrganizationHiddenTarget(t *testing.T) {
	_, _, setResult := setupOrgRoutingResolveTest(t)
	setResult("small", 0.9, nil)
	setEnforceHidden(t, true)
	if err := model.DB.Create(&model.ModelInfo{Model: "org-cheap-model", Hidden: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := model.ReloadModelInfoCache(); err != nil {
		t.Fatal(err)
	}

	c := newOrgRoutingRequest()
	if got, err := routeRoutingPolicy(c, "auto/t"); err != nil || got != "fallback-model" {
		t.Fatalf("hidden target: want the fallback, got %q (%v)", got, err)
	}
	if d := decisionOf(t, c); d.Reason != RoutingReasonTargetUnavailable || d.Label != "" {
		t.Fatalf("want reason target_unavailable without a label, got %+v", d)
	}

	if err := model.DB.Create(&model.ModelInfo{Model: "fallback-model", Hidden: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := model.ReloadModelInfoCache(); err != nil {
		t.Fatal(err)
	}
	blocked := newOrgRoutingRequest()
	if _, err := routeRoutingPolicy(blocked, "auto/t"); err == nil || !IsModelNotFound(err) {
		t.Fatalf("hidden fallback: want model not found, got %v", err)
	}
	// The retry loop asks again within the same request and must get the same answer.
	if _, err := routeRoutingPolicy(blocked, "auto/t"); err == nil || !IsModelNotFound(err) {
		t.Fatalf("cached decision: want model not found, got %v", err)
	}
}

// Pins are keyed by policy id, so a platform policy and an organization policy of the same name
// never share a pin.
func TestRoutingPinScopeUsesPolicyId(t *testing.T) {
	a := &model.RoutingPolicy{Id: 1, Name: "auto/t"}
	b := &model.RoutingPolicy{Id: 2, OrganizationId: 7, Name: "auto/t"}
	if routingPinScope(a) == routingPinScope(b) {
		t.Fatal("pin scopes of different policies must differ")
	}
}

// Model lists of an organization key add its own policies and do not repeat an overridden name.
func TestAppendRoutingPolicyModelsForOrganization(t *testing.T) {
	_, _, _ = setupOrgRoutingResolveTest(t)
	extra := testPolicy("auto/org-only", 0)
	extra.OrganizationId = 7
	extra.Normalize()
	if err := model.DB.Create(extra).Error; err != nil {
		t.Fatal(err)
	}
	if err := model.ReloadRoutingPolicyCache(); err != nil {
		t.Fatal(err)
	}
	base := []string{"fallback-model"}
	got := appendRoutingPolicyModels(model.OrganizationRoutingScope(7), "default", append([]string(nil), base...), nil)
	if len(got) != 3 || got[1] != "policy/auto/org-only" || got[2] != "policy/auto/t" {
		t.Fatalf("organization list: want fallback-model, policy/auto/org-only, policy/auto/t (own overrides platform); got %v", got)
	}
	got = appendRoutingPolicyModels(model.PersonalRoutingScope(1), "default", append([]string(nil), base...), nil)
	if len(got) != 2 || got[1] != "auto/t" {
		t.Fatalf("personal list: want fallback-model, auto/t; got %v", got)
	}
}
