package model

import (
	"errors"
	"reflect"
	"testing"

	"github.com/modeltaps/modeltaps/common"
)

// ORG-15: a lower layer entry must be covered by the layer above.
func TestModelAllowlistCovers(t *testing.T) {
	tests := []struct {
		name   string
		parent []string
		entry  string
		want   bool
	}{
		{"empty parent covers everything", nil, "claude-*", true},
		{"exact under exact", []string{"gpt-4o"}, "GPT-4o", true},
		{"exact under prefix", []string{"claude-fable-*"}, "claude-fable-5-2", true},
		{"exact outside", []string{"claude-fable-*"}, "gpt-4o", false},
		{"narrower prefix under prefix", []string{"claude-*"}, "claude-fable-*", true},
		{"same prefix", []string{"Claude-Fable-*"}, "claude-fable-*", true},
		{"wider prefix not covered", []string{"claude-fable-*"}, "claude-*", false},
		{"prefix never covered by exact names", []string{"claude-fable-5-1", "claude-fable-5-2"}, "claude-fable-*", false},
		{"star covers all", []string{"*"}, "anything-*", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ModelAllowlistCovers(tt.parent, tt.entry); got != tt.want {
				t.Fatalf("ModelAllowlistCovers(%v, %q) = %v, want %v", tt.parent, tt.entry, got, tt.want)
			}
		})
	}
	if got := UncoveredModelAllowlistEntries([]string{"claude-*"}, []string{"claude-x", " ", "gpt-4o", "GPT-4O", "o-*"}); !reflect.DeepEqual(got, []string{"gpt-4o", "o-*"}) {
		t.Fatalf("uncovered entries = %v", got)
	}
}

type accessFixture struct {
	org    *Organization
	owner  *User
	member *User
}

func setupAccessFixture(t *testing.T) *accessFixture {
	t.Helper()
	setupOrgTestDB(t)
	owner := createOrgTestUser(t, "access-owner")
	member := createOrgTestUser(t, "access-member")
	org, err := CreateOrganizationWithOwner("Access Org", "", owner.Id)
	if err != nil {
		t.Fatal(err)
	}
	if err := (&OrganizationMember{OrganizationId: org.Id, UserId: member.Id, Role: OrgRoleMember}).Insert(); err != nil {
		t.Fatal(err)
	}
	return &accessFixture{org: org, owner: owner, member: member}
}

func (f *accessFixture) reload(t *testing.T) {
	t.Helper()
	org, err := GetOrganizationById(f.org.Id)
	if err != nil {
		t.Fatal(err)
	}
	f.org = org
}

func (f *accessFixture) guardrail(t *testing.T, userId int) *OrgMemberGuardrail {
	t.Helper()
	g, err := getOrgMemberGuardrailFromDB(f.org.ShadowUserId, userId)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func accessErrCode(err error) string {
	var apiErr *common.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code
	}
	return ""
}

func TestOrgModelAccessLayers(t *testing.T) {
	f := setupAccessFixture(t)

	// Nothing configured: everything allowed.
	g := f.guardrail(t, f.member.Id)
	if !g.ModelAllowed("gpt-4o") || !g.ApprovedAllows("gpt-4o") {
		t.Fatal("no layer configured should allow every model")
	}

	if _, err := SetOrganizationApprovedModels(f.org, []string{"claude-*", "gpt-4o"}); err != nil {
		t.Fatal(err)
	}
	f.reload(t)

	// Org default outside the approved models is refused; inside is saved.
	if err := UpdateOrgDefaultModelAccess(f.org, 0, []string{"gemini-2.5-pro"}); accessErrCode(err) != common.APICodeModelAccessNotCovered {
		t.Fatalf("default outside approved models: err = %v", err)
	}
	if err := UpdateOrgDefaultModelAccess(f.org, 0, []string{"claude-fable-*"}); err != nil {
		t.Fatal(err)
	}

	// A named list must stay inside the approved models too.
	bad := &ModelAccessList{Name: "Wide", Models: []string{"*"}}
	if err := CreateOrgModelAccessList(f.org, bad, f.owner.Id); accessErrCode(err) != common.APICodeModelAccessNotCovered {
		t.Fatalf("list wider than approved models: err = %v", err)
	}
	reasoning := &ModelAccessList{Name: "Reasoning", Models: []string{"claude-opus-*", "gpt-4o"}}
	if err := CreateOrgModelAccessList(f.org, reasoning, f.owner.Id); err != nil {
		t.Fatal(err)
	}
	if err := CreateOrgModelAccessList(f.org, &ModelAccessList{Name: "reasoning", Models: []string{"gpt-4o"}}, f.owner.Id); accessErrCode(err) != common.APICodeModelAccessListNameDuplicate {
		t.Fatalf("duplicate name: err = %v", err)
	}

	// Member without override: org default. New releases of the family follow the prefix.
	g = f.guardrail(t, f.member.Id)
	if !g.ModelAllowed("claude-fable-5-2") || g.ModelAllowed("gpt-4o") {
		t.Fatalf("member should follow the org default, got %v", g.ModelWhitelist)
	}

	// Member override through the named list replaces the default, still under approved models.
	if _, err := UpdateOrgMemberLimits(f.org, f.member.Id, nil, nil, reasoning.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateOrgMemberLimits(f.org, f.member.Id, nil, []string{"o3"}, 0); accessErrCode(err) != common.APICodeModelAccessNotCovered {
		t.Fatalf("inline override outside approved models: err = %v", err)
	}
	g = f.guardrail(t, f.member.Id)
	if !g.ModelAllowed("gpt-4o") || !g.ModelAllowed("claude-opus-5") || g.ModelAllowed("claude-fable-5-2") {
		t.Fatalf("member list should decide the member layer, got %v", g.ModelWhitelist)
	}

	// Keys: inside the member layer only.
	limit := LimitModelSetting{Enabled: true, Models: []string{"claude-fable-*"}}
	if err := ValidateOrgTokenModelAccess(f.org, f.member.Id, OrgRoleMember, &limit); accessErrCode(err) != common.APICodeModelAccessNotCovered {
		t.Fatalf("key outside the member layer: err = %v", err)
	}
	limit = LimitModelSetting{Enabled: true, Models: []string{"claude-opus-5-1"}}
	if err := ValidateOrgTokenModelAccess(f.org, f.member.Id, OrgRoleMember, &limit); err != nil {
		t.Fatalf("key inside the member layer: %v", err)
	}

	// Editing a list applies to every attachment at once.
	reasoning.Models = []string{"gpt-4o"}
	if err := UpdateOrgModelAccessList(f.org, reasoning); err != nil {
		t.Fatal(err)
	}
	if g = f.guardrail(t, f.member.Id); g.ModelAllowed("claude-opus-5") {
		t.Fatal("list edit should narrow the member at once")
	}
}

func TestOrgTokenAccessListRestricted(t *testing.T) {
	f := setupAccessFixture(t)
	open := &ModelAccessList{Name: "Open", Models: []string{"gpt-*"}}
	locked := &ModelAccessList{Name: "Locked", Models: []string{"gpt-4o"}, Restricted: true}
	for _, list := range []*ModelAccessList{open, locked} {
		if err := CreateOrgModelAccessList(f.org, list, f.owner.Id); err != nil {
			t.Fatal(err)
		}
	}
	limit := LimitModelSetting{AccessListId: locked.Id, Models: []string{"ignored"}}
	if err := ValidateOrgTokenModelAccess(f.org, f.member.Id, OrgRoleMember, &limit); accessErrCode(err) != common.APICodeModelAccessListRestricted {
		t.Fatalf("member attaching a restricted list: err = %v", err)
	}
	if err := ValidateOrgTokenModelAccess(f.org, f.member.Id, OrgRoleAdmin, &limit); err != nil {
		t.Fatalf("admin attaching a restricted list: %v", err)
	}
	if len(limit.Models) != 0 || limit.AccessListId != locked.Id {
		t.Fatalf("an attached list replaces the inline models, got %+v", limit)
	}
	limit = LimitModelSetting{AccessListId: open.Id}
	if err := ValidateOrgTokenModelAccess(f.org, f.member.Id, OrgRoleMember, &limit); err != nil {
		t.Fatalf("member attaching an open list: %v", err)
	}
	limit = LimitModelSetting{AccessListId: 9999}
	if err := ValidateOrgTokenModelAccess(f.org, f.member.Id, OrgRoleMember, &limit); accessErrCode(err) != common.APICodeModelAccessListNotFound {
		t.Fatalf("unknown list: err = %v", err)
	}

	// The key list is checked at request time and narrows the member layer.
	g := f.guardrail(t, f.member.Id)
	if !g.TokenListAllows(open.Id, "gpt-5") || g.TokenListAllows(locked.Id, "gpt-5") || !g.TokenListAllows(0, "o3") {
		t.Fatal("token list matching")
	}
	got := g.FilterModels([]string{"gpt-4o", "gpt-5", "claude-x", "policy-auto"}, locked.Id, func(name string) bool { return name == "policy-auto" })
	if !reflect.DeepEqual(got, []string{"gpt-4o"}) {
		t.Fatalf("filtered models = %v", got)
	}
}

func TestSetOrganizationApprovedModelsConflicts(t *testing.T) {
	f := setupAccessFixture(t)
	if err := UpdateOrgDefaultModelAccess(f.org, 0, []string{"gpt-4o"}); err != nil {
		t.Fatal(err)
	}
	if err := CreateOrgModelAccessList(f.org, &ModelAccessList{Name: "Claude", Models: []string{"claude-*"}}, f.owner.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateOrgMemberLimits(f.org, f.member.Id, nil, []string{"o3"}, 0); err != nil {
		t.Fatal(err)
	}

	conflicts, err := SetOrganizationApprovedModels(f.org, []string{"gpt-*"})
	if accessErrCode(err) != common.APICodeModelAccessCeilingInUse {
		t.Fatalf("narrowing below lower layers: err = %v", err)
	}
	kinds := map[string][]string{}
	for _, c := range conflicts {
		kinds[c.Kind] = c.Entries
	}
	if len(kinds) != 2 || !reflect.DeepEqual(kinds["list"], []string{"claude-*"}) || !reflect.DeepEqual(kinds["member"], []string{"o3"}) {
		t.Fatalf("conflicts = %+v", conflicts)
	}
	f.reload(t)
	if len(f.org.ApprovedModels) != 0 {
		t.Fatal("a refused change must not be saved")
	}

	if _, err := SetOrganizationApprovedModels(f.org, []string{"gpt-*", "claude-*", "o3", "o3"}); err != nil {
		t.Fatal(err)
	}
	f.reload(t)
	if !reflect.DeepEqual([]string(f.org.ApprovedModels), []string{"gpt-*", "claude-*", "o3"}) {
		t.Fatalf("approved models = %v", f.org.ApprovedModels)
	}
	// Clearing the approved models never conflicts.
	if _, err := SetOrganizationApprovedModels(f.org, nil); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteOrgModelAccessListDetaches(t *testing.T) {
	f := setupAccessFixture(t)
	list := &ModelAccessList{Name: "Shared", Models: []string{"gpt-4o"}}
	if err := CreateOrgModelAccessList(f.org, list, f.owner.Id); err != nil {
		t.Fatal(err)
	}
	if err := UpdateOrgDefaultModelAccess(f.org, list.Id, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateOrgMemberLimits(f.org, f.member.Id, nil, nil, list.Id); err != nil {
		t.Fatal(err)
	}
	token := &Token{UserId: f.org.ShadowUserId, CreatedBy: f.member.Id, Name: "k"}
	setting := token.Setting.Data()
	setting.Limits.LimitModelSetting = LimitModelSetting{AccessListId: list.Id}
	token.Setting.Set(setting)
	if err := DB.Create(token).Error; err != nil {
		t.Fatal(err)
	}

	usage, err := GetOrgModelAccessListUsage(f.org)
	if err != nil {
		t.Fatal(err)
	}
	if u := usage[list.Id]; u == nil || !u.OrgDefault || u.Members != 1 || u.Tokens != 1 {
		t.Fatalf("usage = %+v", usage[list.Id])
	}

	if err := DeleteOrgModelAccessList(f.org, list.Id); err != nil {
		t.Fatal(err)
	}
	f.reload(t)
	if f.org.Setting.Data().ModelAccessListId != 0 {
		t.Fatal("org default should be detached")
	}
	member, _ := GetOrganizationMember(f.org.Id, f.member.Id)
	if member.Setting.Data().ModelAccessListId != 0 {
		t.Fatal("member should be detached")
	}
	var saved Token
	DB.First(&saved, token.Id)
	if saved.Setting.Data().Limits.LimitModelSetting.AccessListId != 0 {
		t.Fatal("token should be detached")
	}
	if g := f.guardrail(t, f.member.Id); !g.ModelAllowed("o3") {
		t.Fatal("after detaching every layer falls back to no restriction")
	}
}

func TestTokenAvailableModelsWithApprovedModels(t *testing.T) {
	f := setupAccessFixture(t)
	prevRule := ChannelGroup.Rule
	ChannelGroup.Rule = map[string]map[string][][]int{"default": {"gpt-a": {{1}}, "gpt-b": {{1}}, "claude-x": {{1}}}}
	t.Cleanup(func() { ChannelGroup.Rule = prevRule })
	if err := DB.Model(&User{}).Where("id = ?", f.org.ShadowUserId).Update("group", "default").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := SetOrganizationApprovedModels(f.org, []string{"gpt-*"}); err != nil {
		t.Fatal(err)
	}
	list := &ModelAccessList{Name: "A only", Models: []string{"gpt-a"}}
	if err := CreateOrgModelAccessList(f.org, list, f.owner.Id); err != nil {
		t.Fatal(err)
	}
	token := &Token{UserId: f.org.ShadowUserId, CreatedBy: f.member.Id}
	if got, err := TokenAvailableModels(token); err != nil || !reflect.DeepEqual(got, []string{"gpt-a", "gpt-b"}) {
		t.Fatalf("approved models should apply, got %v err %v", got, err)
	}
	setting := token.Setting.Data()
	setting.Limits.LimitModelSetting = LimitModelSetting{AccessListId: list.Id}
	token.Setting.Set(setting)
	if got, err := TokenAvailableModels(token); err != nil || !reflect.DeepEqual(got, []string{"gpt-a"}) {
		t.Fatalf("key list should apply, got %v err %v", got, err)
	}
}

func TestLimitModelSettingChanged(t *testing.T) {
	a := LimitModelSetting{Enabled: true, Models: []string{"gpt-4o"}}
	if LimitModelSettingChanged(a, LimitModelSetting{Enabled: true, Models: []string{"GPT-4o"}}) {
		t.Fatal("case-only difference is not a change")
	}
	if !LimitModelSettingChanged(a, LimitModelSetting{AccessListId: 3}) {
		t.Fatal("switching to a list is a change")
	}
	if LimitModelSettingChanged(LimitModelSetting{}, LimitModelSetting{Enabled: false, Models: []string{"x"}}) {
		t.Fatal("a disabled inline list is the same as no limit")
	}
}
