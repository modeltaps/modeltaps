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

// The managed list shows the enabled platform policies the user can reach and marks the ones their
// own policy overrides.
func TestManagedRoutingPoliciesFor(t *testing.T) {
	setupOrgRoutingTestDB(t)
	alice := createRoutingTestUser(t, "alice")
	coder := orgPolicy(0, "auto/coder")
	off := orgPolicy(0, "auto/off")
	off.Enabled = false
	unreachable := orgPolicy(0, "auto/far")
	for _, p := range []*RoutingPolicy{coder, off, unreachable} {
		if err := p.Insert(); err != nil {
			t.Fatalf("insert %s: %v", p.Name, err)
		}
	}
	// The fallback leaves the group after the policy was saved.
	ChannelGroup.Rule["other"] = map[string][][]int{"far-model": {{1}}}
	DB.Model(&RoutingPolicy{}).Where("id = ?", unreachable.Id).Update("fallback_model", "far-model")
	if err := personalPolicy(alice.Id, "auto/coder").Insert(); err != nil {
		t.Fatal(err)
	}
	if err := ReloadRoutingPolicyCache(); err != nil {
		t.Fatal(err)
	}
	got, err := ManagedRoutingPoliciesFor(PersonalRoutingScope(alice.Id))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "auto/coder" || !got[0].Overridden {
		t.Fatalf("want auto/coder (overridden), got %+v", got)
	}
}

// Upgrading from organization-scoped names: the older unique indexes go so two users can share a
// name, and the old site option (off by default) is dropped so the new one starts on.
func TestMigrateRoutingPolicyUserScope(t *testing.T) {
	testDB := setupRoutingPolicyTestDB(t)
	if err := testDB.AutoMigrate(&Option{}); err != nil {
		t.Fatal(err)
	}
	if err := testDB.Migrator().DropTable(&RoutingPolicy{}); err != nil {
		t.Fatal(err)
	}
	// The table as the organization policies release created it, plus the name index an older
	// build recreates.
	for _, stmt := range []string{
		"CREATE TABLE routing_policies (id integer PRIMARY KEY AUTOINCREMENT, organization_id integer NOT NULL DEFAULT 0, name varchar(100) NOT NULL, decision_model varchar(255) NOT NULL, fallback_model varchar(255) NOT NULL)",
		"CREATE UNIQUE INDEX idx_routing_policies_org_name ON routing_policies(organization_id, name)",
		"CREATE UNIQUE INDEX idx_routing_policies_name ON routing_policies(name)",
		"INSERT INTO routing_policies (organization_id, name, decision_model, fallback_model) VALUES (0, 'auto/coder', 'd', 'f')",
		"INSERT INTO options (key, value) VALUES ('OrganizationRoutingPolicyEnabled', 'false')",
	} {
		if err := testDB.Exec(stmt).Error; err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := testDB.AutoMigrate(&RoutingPolicy{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateRoutingPolicyUserScope().Migrate(testDB); err != nil {
		t.Fatal(err)
	}
	for _, index := range []string{"idx_routing_policies_org_name", "idx_routing_policies_name"} {
		if testDB.Migrator().HasIndex(&RoutingPolicy{}, index) {
			t.Fatalf("%s should be gone", index)
		}
	}
	for _, user := range []int{4, 5} {
		if err := testDB.Exec("INSERT INTO routing_policies (organization_id, user_id, name, decision_model, fallback_model) VALUES (0, ?, 'auto/coder', 'd', 'f')", user).Error; err != nil {
			t.Fatalf("user %d may reuse the name after the upgrade: %v", user, err)
		}
	}
	if err := testDB.Exec("INSERT INTO routing_policies (organization_id, user_id, name, decision_model, fallback_model) VALUES (0, 4, 'auto/coder', 'd', 'f')").Error; err == nil {
		t.Fatal("names stay unique within one user")
	}
	var count int64
	testDB.Model(&Option{}).Where(&Option{Key: "OrganizationRoutingPolicyEnabled"}).Count(&count)
	if count != 0 {
		t.Fatal("the old option should be dropped")
	}
}
