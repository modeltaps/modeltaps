package model

import (
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"

	"github.com/spf13/viper"
)

// setupOrgRoutingTestDB builds an in-memory database with organizations, users, policies and the
// catalog, turns organization policies on and gives the "default" group three models.
func setupOrgRoutingTestDB(t *testing.T) {
	t.Helper()
	setupOrgTestDB(t)
	if err := DB.AutoMigrate(&RoutingPolicy{}, &ModelInfo{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ResetRoutingPolicyCache()
	modelInfoCache.Lock()
	oldInfoLoaded := modelInfoCache.loaded
	modelInfoCache.loaded = false
	modelInfoCache.Unlock()
	prevRule, prevEnabled, prevMax := ChannelGroup.Rule, config.UserRoutingPolicyEnabled, config.OrganizationRoutingPolicyMaxCount
	ChannelGroup.Rule = map[string]map[string][][]int{
		"default": {"cheap-model": {{1}}, "big-model": {{1}}, "decider": {{1}}, "secret-model": {{1}}},
	}
	config.UserRoutingPolicyEnabled = true
	t.Cleanup(func() {
		ChannelGroup.Rule = prevRule
		config.UserRoutingPolicyEnabled, config.OrganizationRoutingPolicyMaxCount = prevEnabled, prevMax
		ResetRoutingPolicyCache()
		modelInfoCache.Lock()
		modelInfoCache.loaded = oldInfoLoaded
		modelInfoCache.Unlock()
	})
}

func createRoutingTestOrg(t *testing.T, name string) *Organization {
	t.Helper()
	owner := createOrgTestUser(t, name+"-owner")
	org, err := CreateOrganizationWithOwner(name, "", owner.Id)
	if err != nil {
		t.Fatalf("create organization: %v", err)
	}
	if err := DB.Model(&User{}).Where("id = ?", org.ShadowUserId).Update("group", "default").Error; err != nil {
		t.Fatalf("set shadow group: %v", err)
	}
	return org
}

func orgPolicy(organizationId int, name string) *RoutingPolicy {
	return &RoutingPolicy{
		OrganizationId: organizationId,
		Name:           name,
		Enabled:        true,
		DecisionModel:  "decider",
		Routes: []RoutingRoute{
			{Label: "small", TargetModel: "cheap-model"},
			{Label: "large", TargetModel: "big-model"},
		},
		FallbackModel: "big-model",
		Threshold:     0.5,
	}
}

// Organizations may reuse a name the platform or another organization uses; within one scope names
// stay unique, and organization policies get no global catalog row.
func TestOrganizationRoutingPolicyNames(t *testing.T) {
	setupOrgRoutingTestDB(t)
	acme, globex := createRoutingTestOrg(t, "acme"), createRoutingTestOrg(t, "globex")

	platform := orgPolicy(0, "auto/coder")
	if err := platform.Insert(); err != nil {
		t.Fatalf("platform insert: %v", err)
	}
	for _, org := range []*Organization{acme, globex} {
		if err := orgPolicy(org.Id, "auto/coder").Insert(); err != nil {
			t.Fatalf("org %d insert with the platform's name: %v", org.Id, err)
		}
	}
	if err := orgPolicy(acme.Id, "auto/coder").Insert(); err == nil || apiCode(t, err) != common.APICodeRoutingPolicyNameTaken {
		t.Fatalf("a second policy of the same name in one organization must be refused, got %v", err)
	}
	// Model names stay off limits.
	if err := orgPolicy(acme.Id, "cheap-model").Insert(); err == nil {
		t.Fatal("an organization policy must not take a catalog model's name")
	}
	if err := DB.Create(&ModelInfo{Model: "listed-model"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := orgPolicy(acme.Id, "listed-model").Insert(); err == nil || apiCode(t, err) != common.APICodeRoutingPolicyNameTaken {
		t.Fatalf("catalog model names must be refused, got %v", err)
	}

	var rows int64
	DB.Model(&ModelInfo{}).Where("kind = ?", ModelInfoKindRoutingPolicy).Count(&rows)
	if rows != 1 {
		t.Fatalf("only the platform policy owns a catalog row, got %d rows", rows)
	}
}

// Requests from an organization resolve to its own enabled policy first, then the platform's.
func TestResolveRoutingPolicyScopes(t *testing.T) {
	setupOrgRoutingTestDB(t)
	acme, globex := createRoutingTestOrg(t, "acme"), createRoutingTestOrg(t, "globex")
	platform := orgPolicy(0, "auto/coder")
	own := orgPolicy(acme.Id, "auto/coder")
	onlyAcme := orgPolicy(acme.Id, "auto/acme")
	for _, p := range []*RoutingPolicy{platform, own, onlyAcme} {
		if err := p.Insert(); err != nil {
			t.Fatalf("insert %s: %v", p.Name, err)
		}
	}

	for _, loaded := range []bool{false, true} {
		if loaded {
			if err := ReloadRoutingPolicyCache(); err != nil {
				t.Fatal(err)
			}
		}
		if got := ResolveRoutingPolicy(OrganizationRoutingScope(acme.Id), "auto/coder"); got == nil || got.Id != own.Id {
			t.Fatalf("loaded=%v: acme should get its own policy, got %+v", loaded, got)
		}
		if got := ResolveRoutingPolicy(OrganizationRoutingScope(globex.Id), "auto/coder"); got == nil || got.Id != platform.Id {
			t.Fatalf("loaded=%v: globex should get the platform policy, got %+v", loaded, got)
		}
		if got := ResolveRoutingPolicy(PlatformRoutingScope, "auto/coder"); got == nil || got.Id != platform.Id {
			t.Fatalf("loaded=%v: personal keys get the platform policy, got %+v", loaded, got)
		}
		if got := ResolveRoutingPolicy(OrganizationRoutingScope(globex.Id), "auto/acme"); got != nil {
			t.Fatalf("loaded=%v: another organization's policy must not resolve, got %+v", loaded, got)
		}
		if got := policyNames(EnabledRoutingPoliciesFor(OrganizationRoutingScope(acme.Id))); len(got) != 2 || got[0] != "auto/acme" || got[1] != "auto/coder" {
			t.Fatalf("loaded=%v: acme lists its own policies once each, got %v", loaded, got)
		}
		if got := policyNames(EnabledRoutingPoliciesFor(OrganizationRoutingScope(globex.Id))); len(got) != 1 || got[0] != "auto/coder" {
			t.Fatalf("loaded=%v: globex lists the platform policy only, got %v", loaded, got)
		}
	}

	// A disabled organization policy no longer hides the platform one.
	if _, err := SetRoutingPolicyEnabled(own.Id, false); err != nil {
		t.Fatal(err)
	}
	if got := ResolveRoutingPolicy(OrganizationRoutingScope(acme.Id), "auto/coder"); got == nil || got.Id != platform.Id {
		t.Fatalf("disabled own policy should fall through to the platform one, got %+v", got)
	}

	// With the site option off, organization policies are not resolved at all.
	config.UserRoutingPolicyEnabled = false
	if got := ResolveRoutingPolicy(OrganizationRoutingScope(acme.Id), "auto/acme"); got != nil {
		t.Fatalf("organization policies must not resolve while the option is off, got %+v", got)
	}
	if got := policyNames(EnabledRoutingPoliciesFor(OrganizationRoutingScope(acme.Id))); len(got) != 1 || got[0] != "auto/coder" {
		t.Fatalf("listing falls back to platform policies while the option is off, got %v", got)
	}
}

// Organization policies may only use models the organization can reach and the catalog shows.
func TestOrganizationRoutingPolicyModels(t *testing.T) {
	setupOrgRoutingTestDB(t)
	acme := createRoutingTestOrg(t, "acme")
	prevHidden, hadHidden := viper.Get("catalog.enforce_hidden"), viper.IsSet("catalog.enforce_hidden")
	viper.Set("catalog.enforce_hidden", true)
	t.Cleanup(func() {
		if hadHidden {
			viper.Set("catalog.enforce_hidden", prevHidden)
		} else {
			// Unset means enforced; viper cannot unset a key, so restore the default value.
			viper.Set("catalog.enforce_hidden", true)
		}
	})
	if err := DB.Create(&ModelInfo{Model: "secret-model", Hidden: true}).Error; err != nil {
		t.Fatal(err)
	}

	missing := orgPolicy(acme.Id, "auto/missing")
	missing.Routes[0].TargetModel = "elsewhere-model"
	if err := missing.Insert(); err == nil || apiCode(t, err) != common.APICodeRoutingPolicyModelUnavailable {
		t.Fatalf("a target outside the organization's group must be refused, got %v", err)
	}
	hidden := orgPolicy(acme.Id, "auto/hidden")
	hidden.FallbackModel = "secret-model"
	if err := hidden.Insert(); err == nil || apiCode(t, err) != common.APICodeRoutingPolicyModelUnavailable {
		t.Fatalf("a hidden fallback must be refused, got %v", err)
	}
	// The platform may still route to hidden models.
	platform := orgPolicy(0, "auto/platform")
	platform.FallbackModel = "secret-model"
	if err := platform.Insert(); err != nil {
		t.Fatalf("platform policies may target hidden models: %v", err)
	}
	// Organization policies cannot point at policies either.
	nested := orgPolicy(acme.Id, "auto/nested")
	nested.Routes[1].TargetModel = "auto/platform"
	if err := nested.Insert(); err == nil || apiCode(t, err) != common.APICodeRoutingPolicyModelIsPolicy {
		t.Fatalf("nesting must be refused, got %v", err)
	}
	if RoutingTargetUsable("secret-model") || !RoutingTargetUsable("cheap-model") {
		t.Fatal("usable check should follow the catalog's hidden flag")
	}
}

// Each organization is capped, and a policy never moves between scopes or leaks across organizations.
func TestOrganizationRoutingPolicyOwnership(t *testing.T) {
	setupOrgRoutingTestDB(t)
	acme, globex := createRoutingTestOrg(t, "acme"), createRoutingTestOrg(t, "globex")
	config.OrganizationRoutingPolicyMaxCount = 1
	first := orgPolicy(acme.Id, "auto/one")
	if err := first.Insert(); err != nil {
		t.Fatal(err)
	}
	if err := orgPolicy(acme.Id, "auto/two").Insert(); err == nil || apiCode(t, err) != common.APICodeRoutingPolicyCountExceeded {
		t.Fatalf("the per-organization cap must apply, got %v", err)
	}
	if err := orgPolicy(globex.Id, "auto/two").Insert(); err != nil {
		t.Fatalf("the cap is per organization: %v", err)
	}

	moved := orgPolicy(globex.Id, "auto/one")
	moved.Id = first.Id
	if err := moved.Update(); err != nil {
		t.Fatalf("update: %v", err)
	}
	stored, _ := GetRoutingPolicyById(first.Id)
	if stored.OrganizationId != acme.Id {
		t.Fatalf("update must keep the owning organization, got %d", stored.OrganizationId)
	}
	if _, err := GetOrganizationRoutingPolicyById(globex.Id, first.Id); err == nil {
		t.Fatal("another organization must not load the policy by id")
	}
	if _, err := GetOrganizationRoutingPolicyById(acme.Id, first.Id); err != nil {
		t.Fatalf("the owner loads its policy: %v", err)
	}
}

// Stats tell same-named policies apart by routing_policy_id; logs from before the id existed count
// for the platform policy.
func TestRoutingPolicyStatsByPolicyId(t *testing.T) {
	db := setupRoutingStatsTestDB(t)
	now := time.Now().Unix()
	platform := &RoutingPolicy{Id: 1, Name: "auto/t", FallbackModel: "big-model"}
	own := &RoutingPolicy{Id: 2, OrganizationId: 7, Name: "auto/t", FallbackModel: "big-model"}
	meta := func(id int, extra map[string]any) map[string]any {
		m := map[string]any{"routing_policy": "auto/t", "routing_target": "cheap-model", "routing_reason": "classified", "routing_label": "small"}
		if id > 0 {
			m["routing_policy_id"] = id
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	logs := []*Log{
		routedLog(now-10, "cheap-model", 10, 10, 30, meta(0, nil)),
		routedLog(now-9, "cheap-model", 10, 10, 30, meta(1, nil)),
		routedLog(now-8, "cheap-model", 10, 10, 30, meta(2, nil)),
		routedLog(now-7, "cheap-model", 10, 10, 30, meta(2, nil)),
		routedLog(now-6, "cheap-model", 10, 10, 30, meta(2, nil)),
		routedLog(now-5, "decider", 1, 1, 3, map[string]any{"routing_policy": "auto/t", "routing_policy_id": 2, "routing_decision": true}),
		// "Try it" in the organization console is billed but is not traffic.
		routedLog(now-4, "decider", 1, 1, 3, map[string]any{"routing_policy": "auto/t", "routing_policy_id": 2, "routing_decision": true, "routing_try": true}),
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatal(err)
	}
	platformStats, err := GetRoutingPolicyStats(platform, 7)
	if err != nil {
		t.Fatal(err)
	}
	if platformStats.Requests != 2 || platformStats.DecisionCalls != 0 {
		t.Fatalf("platform: want 2 requests and no decision calls, got %d / %d", platformStats.Requests, platformStats.DecisionCalls)
	}
	ownStats, err := GetRoutingPolicyStats(own, 7)
	if err != nil {
		t.Fatal(err)
	}
	if ownStats.Requests != 3 || ownStats.DecisionCalls != 1 {
		t.Fatalf("organization: want 3 requests and 1 decision call, got %d / %d", ownStats.Requests, ownStats.DecisionCalls)
	}
}

// Upgrading drops the old global unique index on name so two organizations can share a name.
func TestMigrateRoutingPolicyScopedNameIndex(t *testing.T) {
	testDB := setupRoutingPolicyTestDB(t)
	if err := testDB.Migrator().DropTable(&RoutingPolicy{}); err != nil {
		t.Fatal(err)
	}
	// The table as 0.3.0 created it.
	if err := testDB.Exec("CREATE TABLE routing_policies (id integer PRIMARY KEY AUTOINCREMENT, name varchar(100) NOT NULL, decision_model varchar(255) NOT NULL, fallback_model varchar(255) NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	if err := testDB.Exec("CREATE UNIQUE INDEX idx_routing_policies_name ON routing_policies(name)").Error; err != nil {
		t.Fatal(err)
	}
	if err := testDB.Exec("INSERT INTO routing_policies (name, decision_model, fallback_model) VALUES ('auto/coder', 'd', 'f')").Error; err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&RoutingPolicy{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateRoutingPolicyScopedNameIndex().Migrate(testDB); err != nil {
		t.Fatal(err)
	}
	if testDB.Migrator().HasIndex(&RoutingPolicy{}, "idx_routing_policies_name") {
		t.Fatal("the old index should be gone")
	}
	var existing RoutingPolicy
	if err := testDB.First(&existing).Error; err != nil || existing.OrganizationId != 0 {
		t.Fatalf("existing rows become platform policies, got %+v (%v)", existing, err)
	}
	if err := testDB.Exec("INSERT INTO routing_policies (organization_id, name, decision_model, fallback_model) VALUES (3, 'auto/coder', 'd', 'f')").Error; err != nil {
		t.Fatalf("an organization may reuse the name after the upgrade: %v", err)
	}
	if err := testDB.Exec("INSERT INTO routing_policies (organization_id, name, decision_model, fallback_model) VALUES (3, 'auto/coder', 'd', 'f')").Error; err == nil {
		t.Fatal("names stay unique within one organization")
	}
}

// The console's model pickers list the organization's routable, visible, non-policy models.
func TestOrganizationRoutingModels(t *testing.T) {
	setupOrgRoutingTestDB(t)
	acme := createRoutingTestOrg(t, "acme")
	prevHidden, hadHidden := viper.Get("catalog.enforce_hidden"), viper.IsSet("catalog.enforce_hidden")
	viper.Set("catalog.enforce_hidden", true)
	t.Cleanup(func() {
		if hadHidden {
			viper.Set("catalog.enforce_hidden", prevHidden)
		} else {
			viper.Set("catalog.enforce_hidden", true)
		}
	})
	if err := DB.Create(&ModelInfo{Model: "secret-model", Hidden: true}).Error; err != nil {
		t.Fatal(err)
	}
	got, err := ScopedRoutingModels(OrganizationRoutingScope(acme.Id))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"big-model", "cheap-model", "decider"}
	if len(got) != len(want) {
		t.Fatalf("want %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("want %v, got %v", want, got)
		}
	}
}
