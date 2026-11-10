package model

import (
	"fmt"
	"sort"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/datatypes"
)

func migrationChannel(id int, group string, priority int64, weight uint, models, mapping string, snapshot ...string) *Channel {
	ch := &Channel{
		Id: id, Status: config.ChannelStatusEnabled, Group: group, Priority: &priority, Weight: &weight,
		Models: models, ModelMapping: strPtr(mapping),
	}
	if len(snapshot) > 0 {
		drift := datatypes.NewJSONType(ModelDriftResult{UpstreamSnapshot: snapshot})
		ch.ModelDrift = &drift
	}
	return ch
}

// migrationFixture covers every case the report has to catch: a mapped name, an alias listed in
// place of its public name, a "+" mapping, a name with no catalog row, a hidden model, a
// wildcard with and without an upstream snapshot, the same model listed twice on one channel,
// and a routing policy name listed on a channel.
func migrationFixture() supplyMigrationInput {
	return supplyMigrationInput{
		infos: []*ModelInfo{
			{Id: 1, Model: "claude-opus-5", Status: ModelStatusDraft},
			{Id: 2, Model: "anthropic/claude-opus-5", AliasOf: "claude-opus-5"},
			{Id: 3, Model: "gpt-4o-mini", Hidden: true, Status: ModelStatusDraft},
			{Id: 4, Model: "auto/coder", Kind: ModelInfoKindRoutingPolicy},
		},
		channels: []*Channel{
			migrationChannel(1, "default", 10, 1, "claude-opus-5,gpt-4o-mini", `{"claude-opus-5":"claude-opus-5-20261001"}`),
			migrationChannel(2, "default,vip", 0, 3, "anthropic/claude-opus-5,deepseek-chat", `{"deepseek-chat":"+deepseek-v4"}`),
			migrationChannel(3, "default", 0, 1, "gpt-4o*", "", "gpt-4o", "gpt-4o-mini", "o3"),
			migrationChannel(4, "default", 0, 1, "qwen-*", ""),
			migrationChannel(5, "default", 0, 1, "claude-opus-5,anthropic/claude-opus-5", ""),
			migrationChannel(6, "default", 0, 1, "auto/coder", ""),
		},
		prices: []*Price{
			{Model: "claude-opus-5-20261001", Type: TokensPriceType, Input: 5, Output: 25},
			{Model: "anthropic/claude-opus-5", Type: TokensPriceType, Input: 6, Output: 30},
			{Model: "gpt-4o-mini", Type: TokensPriceType, Input: 0.15, Output: 0.6},
			{Model: "gpt-4o*", Type: TokensPriceType, Input: 2.5, Output: 10},
			{Model: "deepseek-v4", Type: TokensPriceType, Input: 0.3, Output: 1.2},
		},
		policies: []*RoutingPolicy{{
			Name: "auto/coder", Enabled: true, DecisionModel: "gpt-4o-mini", FallbackModel: "claude-opus-5",
			Routes: datatypes.JSONSlice[RoutingRoute]{{Label: "big", TargetModel: "o3"}},
		}},
	}
}

func supplyStrings(supplies []*ModelSupply) []string {
	out := make([]string, 0, len(supplies))
	for _, s := range supplies {
		out = append(out, fmt.Sprintf("%s@%d=%s p%d w%d", s.Model, s.ChannelId, s.UpstreamModel, s.Priority, s.Weight))
	}
	sort.Strings(out)
	return out
}

func TestPlanSupplyMigration(t *testing.T) {
	plan, report := planSupplyMigration(migrationFixture())

	wantSupplies := []string{
		"claude-opus-5@1=claude-opus-5-20261001 p10 w1",
		"claude-opus-5@2=anthropic/claude-opus-5 p0 w3",
		"claude-opus-5@5=claude-opus-5 p0 w1",
		"deepseek-chat@2=deepseek-v4 p0 w3",
		"gpt-4o-mini@1=gpt-4o-mini p10 w1",
		"gpt-4o-mini@3=gpt-4o-mini p0 w1",
		"gpt-4o@3=gpt-4o p0 w1",
	}
	if got := supplyStrings(plan.supplies); fmt.Sprint(got) != fmt.Sprint(wantSupplies) {
		t.Fatalf("supplies\n got %v\nwant %v", got, wantSupplies)
	}
	for _, s := range plan.supplies {
		if !s.Enabled || s.Source != ModelSupplySourceMigrated {
			t.Fatalf("supply %+v not enabled or not marked migrated", s)
		}
	}

	if fmt.Sprint(report.NewModels) != "[deepseek-chat gpt-4o]" {
		t.Errorf("new models = %v", report.NewModels)
	}
	if len(report.StatusChanges) != 1 || report.StatusChanges[0] != (SupplyStatusChange{"claude-opus-5", ModelStatusDraft, ModelStatusPublished}) {
		t.Errorf("status changes = %+v (the hidden gpt-4o-mini must stay a draft)", report.StatusChanges)
	}
	copies := map[string]string{}
	for _, c := range report.PriceCopies {
		copies[c.Model] = c.From
	}
	if len(copies) != 2 || copies["claude-opus-5"] != "claude-opus-5-20261001" || copies["deepseek-chat"] != "deepseek-v4" {
		t.Errorf("price copies = %+v (gpt-4o is priced by the gpt-4o* row and needs none)", report.PriceCopies)
	}
	if len(report.PriceConflicts) != 1 || report.PriceConflicts[0].Model != "claude-opus-5" {
		t.Errorf("price conflicts = %+v", report.PriceConflicts)
	}
	if len(report.BillingChanges) != 1 || report.BillingChanges[0].ChannelId != 2 || report.BillingChanges[0].LegacyBilledAs != "anthropic/claude-opus-5" {
		t.Errorf("billing changes = %+v, want channel 2 moving from the anthropic/ price to the public one", report.BillingChanges)
	}
	if len(report.PlusMappings) != 1 || report.PlusMappings[0].Entry != "deepseek-chat" {
		t.Errorf("plus mappings = %+v", report.PlusMappings)
	}
	if len(report.ExpandedWildcards) != 1 || fmt.Sprint(report.ExpandedWildcards[0].Models) != "[gpt-4o gpt-4o-mini]" {
		t.Errorf("expanded wildcards = %+v", report.ExpandedWildcards)
	}
	if len(report.UnresolvedWildcards) != 1 || report.UnresolvedWildcards[0].Pattern != "qwen-*" {
		t.Errorf("unresolved wildcards = %+v", report.UnresolvedWildcards)
	}
	if len(report.DuplicateEntries) != 1 || report.DuplicateEntries[0].ChannelId != 5 || report.DuplicateEntries[0].Entry != "anthropic/claude-opus-5" {
		t.Errorf("duplicates = %+v, want the alias on channel 5 dropped in favour of the public name", report.DuplicateEntries)
	}
	if len(report.SkippedEntries) != 1 || report.SkippedEntries[0].Entry != "auto/coder" {
		t.Errorf("skipped = %+v", report.SkippedEntries)
	}
	if len(report.PolicyGaps) != 1 || report.PolicyGaps[0] != (SupplyPolicyGap{"auto/coder", "route:big", "o3"}) {
		t.Errorf("policy gaps = %+v", report.PolicyGaps)
	}

	diffs := map[string]SupplyRoutingDiff{}
	for _, d := range report.RoutingDifferences {
		diffs[d.Group+"/"+d.Model] = d
	}
	want := map[string]string{
		"default/claude-opus-5": "changed",         // channel 2 joins through its alias
		"vip/claude-opus-5":     "newly_reachable", // only the alias was listed in vip
		"default/gpt-4o":        "wildcard",
		"default/gpt-4o-mini":   "changed", // channel 3 joins through the expanded wildcard
	}
	if len(diffs) != len(want) {
		t.Errorf("routing differences = %+v", report.RoutingDifferences)
	}
	for key, kind := range want {
		if diffs[key].Kind != kind {
			t.Errorf("%s: kind %q, want %q (all: %+v)", key, diffs[key].Kind, kind, report.RoutingDifferences)
		}
	}
	if got := fmt.Sprint(diffs["default/claude-opus-5"].New); got != "[[1] [2 5]]" {
		t.Errorf("default/claude-opus-5 new tiers = %s", got)
	}
}

// A second run plans nothing new for supplies that already exist.
func TestPlanSupplyMigrationIsIdempotent(t *testing.T) {
	first, _ := planSupplyMigration(migrationFixture())
	in := migrationFixture()
	in.existing = first.supplies
	second, report := planSupplyMigration(in)
	if len(second.supplies) != 0 || report.SuppliesExisting != len(first.supplies) {
		t.Fatalf("second run planned %d supplies (existing %d)", len(second.supplies), report.SuppliesExisting)
	}
}

// A model listed the same way before and after produces no difference.
func TestPlanSupplyMigrationEquivalentConfigHasNoDifferences(t *testing.T) {
	_, report := planSupplyMigration(supplyMigrationInput{
		infos: []*ModelInfo{{Id: 1, Model: "gpt-4o-mini", Status: ModelStatusPublished}},
		channels: []*Channel{
			migrationChannel(1, "default", 10, 1, "gpt-4o-mini", ""),
			migrationChannel(2, "default", 0, 1, "gpt-4o-mini", `{"gpt-4o-mini":"openai/gpt-4o-mini"}`),
		},
		prices: []*Price{{Model: "gpt-4o-mini", Type: TokensPriceType, Input: 0.15, Output: 0.6}},
	})
	if len(report.RoutingDifferences) != 0 || len(report.StatusChanges) != 0 || len(report.NewModels) != 0 {
		t.Fatalf("expected no differences, got %+v", report)
	}
	if len(report.BillingChanges) != 1 || report.BillingChanges[0].LegacyBilledAs != "openai/gpt-4o-mini" || report.BillingChanges[0].LegacyPrice != nil {
		t.Fatalf("billing changes = %+v, want channel 2 moving from the unpriced upstream name to the public price", report.BillingChanges)
	}
}
