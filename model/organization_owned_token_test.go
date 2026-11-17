package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// ORG-16: an organization API key the organization owns (created_by is the shadow account) keeps
// working when members leave, gets only the organization layers of model access and no member
// budget, and a member's keys can be made owned by the organization instead of being disabled.
func TestOrganizationOwnedTokens(t *testing.T) {
	setupOrgTestDB(t)
	owner := createOrgTestUser(t, "oo-owner")
	bob := createOrgTestUser(t, "oo-bob")
	carol := createOrgTestUser(t, "oo-carol")
	org, err := CreateOrganizationWithOwner("Owned Org", "", owner.Id)
	if err != nil {
		t.Fatalf("create organization: %v", err)
	}
	for _, uid := range []int{bob.Id, carol.Id} {
		if err := (&OrganizationMember{OrganizationId: org.Id, UserId: uid, Role: OrgRoleMember}).Insert(); err != nil {
			t.Fatalf("insert member: %v", err)
		}
	}
	// Organization default allows gpt-*; bob's own layer narrows it to gpt-4o.
	setting := org.Setting.Data()
	setting.ModelWhitelist = []string{"gpt-*"}
	org.Setting.Set(setting)
	if err := DB.Model(org).Select("setting").Updates(org).Error; err != nil {
		t.Fatalf("save organization default: %v", err)
	}
	if _, err := UpdateOrgMemberLimits(org, bob.Id, &QuotaResetSetting{Limit: 100, Period: TokenQuotaResetPeriodDaily}, []string{"gpt-4o"}, 0); err != nil {
		t.Fatalf("set bob limits: %v", err)
	}

	owned := &Token{UserId: org.ShadowUserId, CreatedBy: org.ShadowUserId, Key: "oo-owned-key", Name: "owned", Status: config.TokenStatusEnabled}
	bobKey := &Token{UserId: org.ShadowUserId, CreatedBy: bob.Id, Key: "oo-bob-key", Name: "bob", Status: config.TokenStatusEnabled}
	carolKey := &Token{UserId: org.ShadowUserId, CreatedBy: carol.Id, Key: "oo-carol-key", Name: "carol", Status: config.TokenStatusEnabled}
	personal := &Token{UserId: bob.Id, Key: "oo-personal-key", Name: "personal", Status: config.TokenStatusEnabled}
	for _, tk := range []*Token{owned, bobKey, carolKey, personal} {
		if err := DB.Create(tk).Error; err != nil {
			t.Fatalf("create token: %v", err)
		}
	}

	// Who a key is attributed to.
	if !TokenIsOrgOwned(owned) || TokenIsOrgOwned(bobKey) || TokenIsOrgOwned(personal) {
		t.Fatal("only the key whose created_by is the shadow account is owned by the organization")
	}
	if TokenOrgMemberId(owned) != 0 || TokenOrgMemberId(bobKey) != bob.Id || TokenOrgMemberId(personal) != 0 {
		t.Fatal("TokenOrgMemberId must name the member only for member keys")
	}
	if !tokenIsOrganizationToken(owned) || !tokenIsOrganizationToken(bobKey) || tokenIsOrganizationToken(personal) {
		t.Fatal("organization-owned and member keys both charge the organization; personal keys do not")
	}

	// The guardrail of an organization-owned key has the organization layers only.
	guardrail, err := getOrgMemberGuardrailFromDB(org.ShadowUserId, org.ShadowUserId)
	if err != nil {
		t.Fatalf("load guardrail of an organization-owned key: %v", err)
	}
	if guardrail.OrganizationId != org.Id || guardrail.MemberId != 0 || guardrail.HasBudget() {
		t.Fatalf("organization-owned key must have no member budget: %+v", guardrail)
	}
	if !guardrail.ModelAllowed("gpt-4.1") || guardrail.ModelAllowed("claude-sonnet") {
		t.Fatalf("organization-owned key must follow the organization default: %v", guardrail.ModelWhitelist)
	}

	// Key-level model access is checked against the organization layers, not a member's.
	if err := ValidateOrgTokenModelAccess(org, org.ShadowUserId, OrgRoleAdmin, &LimitModelSetting{Enabled: true, Models: []string{"gpt-4.1"}}); err != nil {
		t.Fatalf("organization-owned key may use what the organization default allows: %v", err)
	}
	if err := ValidateOrgTokenModelAccess(org, bob.Id, OrgRoleMember, &LimitModelSetting{Enabled: true, Models: []string{"gpt-4.1"}}); err == nil {
		t.Fatal("bob's key stays inside bob's member layer")
	}
	scope, err := GetOrgModelAccessScope(org, org.ShadowUserId)
	if err != nil || len(scope.EffectiveMember) != 1 || scope.EffectiveMember[0] != "gpt-*" {
		t.Fatalf("model access scope of an organization-owned key is the organization default: %+v %v", scope, err)
	}

	// Removing bob with keep_keys makes his key owned by the organization; carol leaving disables hers.
	if err := RemoveOrganizationMemberWithKeys(org, bob.Id, true); err != nil {
		t.Fatalf("remove bob keeping keys: %v", err)
	}
	if err := RemoveOrganizationMember(org, carol.Id); err != nil {
		t.Fatalf("remove carol: %v", err)
	}
	var gotOwned, gotBob, gotCarol, gotPersonal Token
	DB.First(&gotOwned, owned.Id)
	DB.First(&gotBob, bobKey.Id)
	DB.First(&gotCarol, carolKey.Id)
	DB.First(&gotPersonal, personal.Id)
	if gotOwned.Status != config.TokenStatusEnabled || gotOwned.CreatedBy != org.ShadowUserId {
		t.Fatalf("organization-owned key must not change when members leave: %+v", gotOwned)
	}
	if gotBob.Status != config.TokenStatusEnabled || !TokenIsOrgOwned(&gotBob) {
		t.Fatalf("bob's key must stay enabled and become owned by the organization: %+v", gotBob)
	}
	if gotCarol.Status != config.TokenStatusDisabled || gotCarol.CreatedBy != carol.Id {
		t.Fatalf("carol's key is disabled as before: %+v", gotCarol)
	}
	if gotPersonal.Status != config.TokenStatusEnabled || gotPersonal.CreatedBy != 0 {
		t.Fatalf("personal keys are never touched: %+v", gotPersonal)
	}

	// Transferring carol's disabled key keeps its status; keys of other accounts are left alone.
	if err := TransferOrgTokensToOrganization(org, []int{carolKey.Id, personal.Id}); err != nil {
		t.Fatalf("transfer: %v", err)
	}
	DB.First(&gotCarol, carolKey.Id)
	DB.First(&gotPersonal, personal.Id)
	if !TokenIsOrgOwned(&gotCarol) || gotCarol.Status != config.TokenStatusDisabled {
		t.Fatalf("transferred key is owned by the organization and keeps its status: %+v", gotCarol)
	}
	if gotPersonal.CreatedBy != 0 {
		t.Fatalf("a personal key must not be transferred: %+v", gotPersonal)
	}
}

// A personal key carrying its owner as created_by would look like a key the organization owns;
// the migration clears it and leaves real organization keys alone.
func TestClearPersonalTokenCreatedBy(t *testing.T) {
	setupOrgTestDB(t)
	owner := createOrgTestUser(t, "cp-owner")
	org, err := CreateOrganizationWithOwner("Clear Org", "", owner.Id)
	if err != nil {
		t.Fatal(err)
	}
	personal := &Token{UserId: owner.Id, CreatedBy: owner.Id, Key: "cp-personal", Name: "p"}
	owned := &Token{UserId: org.ShadowUserId, CreatedBy: org.ShadowUserId, Key: "cp-owned", Name: "o"}
	member := &Token{UserId: org.ShadowUserId, CreatedBy: owner.Id, Key: "cp-member", Name: "m"}
	for _, tk := range []*Token{personal, owned, member} {
		if err := DB.Create(tk).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := clearPersonalTokenCreatedBy().Migrate(DB); err != nil {
		t.Fatal(err)
	}
	read := func(id int) int {
		var got Token
		DB.First(&got, id)
		return got.CreatedBy
	}
	if got := read(personal.Id); got != 0 {
		t.Fatalf("personal key created_by must be cleared: %d", got)
	}
	if got := read(owned.Id); got != org.ShadowUserId {
		t.Fatalf("organization-owned key must be kept: %d", got)
	}
	if got := read(member.Id); got != owner.Id {
		t.Fatalf("member key must be kept: %d", got)
	}
}
