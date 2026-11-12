package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

const day = int64(86400)

func setupLibraryRulesTestDB(t *testing.T, now int64) {
	t.Helper()
	setupModelLibraryTestDB(t)
	if err := DB.AutoMigrate(&ModelSupply{}, &Price{}, &ModelLibraryDecision{}); err != nil {
		t.Fatal(err)
	}
	oldPricing := PricingInstance
	PricingInstance = &Pricing{Prices: map[string]*Price{}}
	t.Cleanup(func() { PricingInstance = oldPricing })
	entries := []*ModelLibraryEntry{
		{Canonical: "anthropic/claude-new", Model: "claude-new", Lab: "anthropic", ReleasedAt: now - 5*day},
		{Canonical: "anthropic/claude-old", Model: "claude-old", Lab: "anthropic", ReleasedAt: now - 400*day},
		{Canonical: "google/gemini-x-preview", Model: "gemini-x-preview", Lab: "google", ReleasedAt: now - 2*day},
		{Canonical: "nvidia/nemo", Model: "nemo", Lab: "nvidia", ReleasedAt: now - 1*day},
		{Canonical: "openai/gpt-gone", Model: "gpt-gone", Lab: "openai", Deprecated: true},
		{Canonical: "openai/gpt-kept", Model: "gpt-kept", Lab: "openai", Deprecated: true},
		{Canonical: "openai/gpt-unsupplied", Model: "gpt-unsupplied", Lab: "openai", ReleasedAt: now - day},
	}
	if err := DB.Create(&entries).Error; err != nil {
		t.Fatal(err)
	}
	modelLibraryIndex.invalidate()
	ch := &Channel{Name: "c", Type: config.ChannelTypeOpenAI, Status: config.ChannelStatusEnabled,
		Models: "claude-new,claude-old,gemini-x-preview,nemo,gpt-gone,gpt-kept"}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	rows := []ModelInfo{
		{Model: "gpt-gone", Status: ModelStatusPublished, Source: ModelInfoSourceModelsDev},
		{Model: "gpt-kept", Status: ModelStatusPublished, Source: ModelInfoSourceManual},
	}
	if err := DB.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
}

func outcomesByModel(list []LibraryRuleOutcome) map[string]string {
	out := map[string]string{}
	for _, o := range list {
		out[o.Model] = o.Action
	}
	return out
}

func TestEvaluateDefaultLibraryRules(t *testing.T) {
	now := int64(1_800_000_000)
	setupLibraryRulesTestDB(t, now)
	got, err := EvaluateLibraryRules(DefaultLibraryRules(), now)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"claude-new":       LibraryActionPublish, // main lab, recent, supplied
		"claude-old":       LibraryActionDraft,   // supplied but old
		"gemini-x-preview": LibraryActionDraft,   // preview
		"nemo":             LibraryActionDraft,   // not a main lab
		"gpt-gone":         LibraryActionRetire,  // deprecated, synced row
	}
	m := outcomesByModel(got)
	if len(m) != len(want) {
		t.Fatalf("outcomes = %v, want %v", m, want)
	}
	for model, action := range want {
		if m[model] != action {
			t.Errorf("%s: %q, want %q", model, m[model], action)
		}
	}
	if _, ok := m["gpt-kept"]; ok {
		t.Error("a row an admin edited is never retired")
	}
	if _, ok := m["gpt-unsupplied"]; ok {
		t.Error("no channel supplies it: nothing to do")
	}
}

func TestRunLibraryRulesDryRunThenApply(t *testing.T) {
	now := int64(1_800_000_000)
	setupLibraryRulesTestDB(t, now)
	if err := SetCustomLibraryRules(""); err != nil {
		t.Fatal(err)
	}

	dry, err := RunLibraryRules(false)
	if err != nil {
		t.Fatal(err)
	}
	if dry.Reported != 5 || dry.Added != 0 {
		t.Fatalf("dry run = %+v", dry)
	}
	var count int64
	DB.Model(&ModelInfo{}).Count(&count)
	if count != 2 {
		t.Fatalf("a dry run changes nothing, catalog has %d rows", count)
	}
	again, _ := RunLibraryRules(false)
	if again.Reported != 0 {
		t.Errorf("a second dry run reports only what is new, got %d", again.Reported)
	}

	applied, err := RunLibraryRules(true)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Added != 4 || applied.Retired != 1 {
		t.Fatalf("apply = %+v", applied)
	}
	info, _ := GetModelInfoByModel("claude-new")
	if info == nil || info.Status != ModelStatusPublished {
		t.Errorf("claude-new = %+v", info)
	}
	gone, _ := GetModelInfoByModel("gpt-gone")
	if gone.Status != ModelStatusRetired || !gone.Hidden {
		t.Errorf("gpt-gone = %+v", gone)
	}

	// An admin removes a model a rule added: the next run leaves it out.
	DB.Where("model = ?", "nemo").Delete(&ModelInfo{})
	next, err := RunLibraryRules(true)
	if err != nil {
		t.Fatal(err)
	}
	if next.Added != 0 {
		t.Errorf("removed model added again: %+v", next)
	}
	if n, _ := GetModelInfoByModel("nemo"); n != nil {
		t.Error("nemo came back")
	}
}

func TestParseLibraryRules(t *testing.T) {
	if _, err := ParseLibraryRules(`[{"name":"x","action":"publish","patterns":["claude-*"]}]`); err != nil {
		t.Errorf("valid rules refused: %v", err)
	}
	for _, raw := range []string{
		`[{"action":"launch"}]`,
		`[{"action":"draft","patterns":["[bad"]}]`,
		`[{"action":"draft","endpoints":["telepathy"]}]`,
		`[{"action":"draft","released_within_days":-1}]`,
		`not json`,
	} {
		if _, err := ParseLibraryRules(raw); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	t.Cleanup(func() { _ = SetCustomLibraryRules("") })
	if err := SetCustomLibraryRules(`[]`); err != nil {
		t.Fatal(err)
	}
	if len(EffectiveLibraryRules()) != 0 {
		t.Error("saved empty rules mean no rules")
	}
	_ = SetCustomLibraryRules("")
	if len(EffectiveLibraryRules()) != len(DefaultLibraryRules()) {
		t.Error("cleared rules fall back to the defaults")
	}
}
