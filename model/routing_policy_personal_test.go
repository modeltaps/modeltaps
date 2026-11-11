package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
)

func personalPolicy(userId int, name string) *RoutingPolicy {
	p := orgPolicy(0, name)
	p.UserId = userId
	return p
}

func createRoutingTestUser(t *testing.T, name string) *User {
	t.Helper()
	user := createOrgTestUser(t, name)
	if err := DB.Model(&User{}).Where("id = ?", user.Id).Update("group", "default").Error; err != nil {
		t.Fatalf("set user group: %v", err)
	}
	return user
}

// Every user has their own namespace: names may repeat across users and the platform, but not
// within one user, and personal policies get no catalog row.
func TestPersonalRoutingPolicyNames(t *testing.T) {
	setupOrgRoutingTestDB(t)
	alice, bob := createRoutingTestUser(t, "alice"), createRoutingTestUser(t, "bob")
	if err := orgPolicy(0, "auto/coder").Insert(); err != nil {
		t.Fatalf("platform insert: %v", err)
	}
	for _, user := range []*User{alice, bob} {
		if err := personalPolicy(user.Id, "auto/coder").Insert(); err != nil {
			t.Fatalf("user %d insert: %v", user.Id, err)
		}
	}
	if err := personalPolicy(alice.Id, "auto/coder").Insert(); err == nil || apiCode(t, err) != common.APICodeRoutingPolicyNameTaken {
		t.Fatalf("a second policy of the same name for one user must be refused, got %v", err)
	}
	var rows int64
	DB.Model(&ModelInfo{}).Where("kind = ?", ModelInfoKindRoutingPolicy).Count(&rows)
	if rows != 1 {
		t.Fatalf("only the platform policy owns a catalog row, got %d rows", rows)
	}

	// A policy belongs to one scope only.
	both := personalPolicy(alice.Id, "auto/both")
	both.OrganizationId = 1
	if err := both.Insert(); err == nil {
		t.Fatal("a policy with both an organization and a user must be refused")
	}
	// The scope never changes on update.
	own, err := GetRoutingPoliciesByScope(PersonalRoutingScope(alice.Id))
	if err != nil || len(own) != 1 {
		t.Fatalf("alice's policies: %v %v", own, err)
	}
	moved := *own[0]
	moved.UserId = bob.Id
	if err := moved.Update(); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got, _ := GetRoutingPolicyById(own[0].Id); got.UserId != alice.Id {
		t.Fatalf("update must keep the owner, got user %d", got.UserId)
	}
	if _, err := GetScopedRoutingPolicyById(PersonalRoutingScope(bob.Id), own[0].Id); err == nil {
		t.Fatal("bob must not load alice's policy by id")
	}
}

func TestRoutingPolicyNameRejectsCallPrefix(t *testing.T) {
	for _, name := range []string{"policy/coder", "Policy/coder", "POLICY/x"} {
		if err := ValidateRoutingPolicyName(name); err == nil {
			t.Fatalf("%q must be refused: policy/ is the call prefix", name)
		}
	}
	if err := ValidateRoutingPolicyName("auto/policy/coder"); err != nil {
		t.Fatalf("policy/ inside a name is fine: %v", err)
	}
}

func TestPersonalRoutingPolicyMaxCount(t *testing.T) {
	setupOrgRoutingTestDB(t)
	prev := config.PersonalRoutingPolicyMaxCount
	config.PersonalRoutingPolicyMaxCount = 1
	t.Cleanup(func() { config.PersonalRoutingPolicyMaxCount = prev })
	alice := createRoutingTestUser(t, "alice")
	if err := personalPolicy(alice.Id, "auto/one").Insert(); err != nil {
		t.Fatal(err)
	}
	if err := personalPolicy(alice.Id, "auto/two").Insert(); err == nil {
		t.Fatal("the per-user cap must apply")
	}
}

// policy/<name> reaches only the key's own policy; the bare name keeps reaching the own policy first,
// then the platform's.
func TestResolvePersonalRoutingPolicy(t *testing.T) {
	setupOrgRoutingTestDB(t)
	alice, bob := createRoutingTestUser(t, "alice"), createRoutingTestUser(t, "bob")
	platform := orgPolicy(0, "auto/coder")
	shared := orgPolicy(0, "auto/shared")
	own := personalPolicy(alice.Id, "auto/coder")
	for _, p := range []*RoutingPolicy{platform, shared, own} {
		if err := p.Insert(); err != nil {
			t.Fatalf("insert %s: %v", p.Name, err)
		}
	}
	aliceScope, bobScope := PersonalRoutingScope(alice.Id), PersonalRoutingScope(bob.Id)
	for _, loaded := range []bool{false, true} {
		if loaded {
			if err := ReloadRoutingPolicyCache(); err != nil {
				t.Fatal(err)
			}
		}
		cases := []struct {
			scope RoutingScope
			name  string
			want  *RoutingPolicy
		}{
			{aliceScope, "policy/auto/coder", own},
			{aliceScope, "auto/coder", own},
			{aliceScope, "auto/shared", shared},
			{aliceScope, "policy/auto/shared", nil},
			{bobScope, "policy/auto/coder", nil},
			{bobScope, "auto/coder", platform},
			{PlatformRoutingScope, "policy/auto/coder", nil},
		}
		for _, tc := range cases {
			got := ResolveRoutingPolicy(tc.scope, tc.name)
			if (got == nil) != (tc.want == nil) || (got != nil && got.Id != tc.want.Id) {
				t.Fatalf("loaded=%v scope=%+v %s: want %+v, got %+v", loaded, tc.scope, tc.name, tc.want, got)
			}
		}
		listed := RoutingPolicyModelsFor(aliceScope, nil, nil)
		if len(listed) != 2 || listed[0] != "auto/shared" || listed[1] != "policy/auto/coder" {
			t.Fatalf("loaded=%v: alice lists %v", loaded, listed)
		}
	}

	config.UserRoutingPolicyEnabled = false
	if got := ResolveRoutingPolicy(aliceScope, "auto/coder"); got == nil || got.Id != platform.Id {
		t.Fatalf("switched off: own policies are not resolved, got %+v", got)
	}
	if got := ResolveRoutingPolicy(aliceScope, "policy/auto/coder"); got != nil {
		t.Fatalf("switched off: policy/ resolves nothing, got %+v", got)
	}
}

// Own policies must not point at policies, including through the policy/ prefix.
func TestPersonalRoutingPolicyNoNesting(t *testing.T) {
	setupOrgRoutingTestDB(t)
	alice := createRoutingTestUser(t, "alice")
	if err := personalPolicy(alice.Id, "auto/inner").Insert(); err != nil {
		t.Fatal(err)
	}
	outer := personalPolicy(alice.Id, "auto/outer")
	outer.FallbackModel = "policy/auto/inner"
	if err := outer.Insert(); err == nil {
		t.Fatal("a personal policy must not fall back to another policy")
	}
}

// Personal policies may only use models the user's group can route and the catalog shows.
func TestPersonalRoutingPolicyModels(t *testing.T) {
	setupOrgRoutingTestDB(t)
	alice := createRoutingTestUser(t, "alice")
	p := personalPolicy(alice.Id, "auto/x")
	p.FallbackModel = "not-in-group"
	if err := p.Insert(); err == nil || apiCode(t, err) != common.APICodeRoutingPolicyModelUnavailable {
		t.Fatalf("a model outside the user's group must be refused, got %v", err)
	}
	models, err := ScopedRoutingModels(PersonalRoutingScope(alice.Id))
	if err != nil || len(models) != 4 {
		t.Fatalf("models for alice: %v %v", models, err)
	}
}

func TestModelAllowlistPermitsPolicyCall(t *testing.T) {
	if !ModelAllowlistPermits([]string{"auto/coder"}, "policy/auto/coder") {
		t.Fatal("the bare name allows policy/<name>")
	}
	if !ModelAllowlistPermits([]string{"policy/*"}, "policy/auto/coder") {
		t.Fatal("policy/* allows every own policy")
	}
	if ModelAllowlistPermits([]string{"policy/auto/coder"}, "auto/coder") {
		t.Fatal("policy/<name> does not allow a bare platform name")
	}
}
