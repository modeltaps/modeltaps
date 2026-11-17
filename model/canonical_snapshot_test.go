package model

import (
	"fmt"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func snapshotTestResolver() *CanonicalResolver {
	return newCanonicalResolver([]*ModelInfo{
		{Model: "gpt-4o"},
		{Model: "gpt-4o-mini"},
		{Model: "gpt-5.5"},
		{Model: "claude-sonnet-4-5"},
		{Model: "claude-opus-4-5"},
		{Model: "claude-opus-4.5", AliasOf: "claude-opus-4-5"},
		// A snapshot with a catalog row of its own: it is priced apart from gpt-4o.
		{Model: "gpt-4o-2024-05-13"},
		{Model: "router", Kind: ModelInfoKindRoutingPolicy},
	}, nil)
}

// A name ending in a release date is a snapshot of the undated model when nothing else matches.
func TestCanonicalResolverSnapshot(t *testing.T) {
	r := snapshotTestResolver()
	snap := func(name string) CanonicalMatch { return CanonicalMatch{name, CanonicalSourceSnapshot} }
	none := func(name string) CanonicalMatch { return CanonicalMatch{name, CanonicalSourceNone} }
	cases := []struct {
		upstream string
		want     CanonicalMatch
	}{
		// Dated snapshots, with and without dashes, with a vendor prefix, through an alias.
		{"gpt-4o-2024-08-06", snap("gpt-4o")},
		{"openai/gpt-4o-2024-08-06", snap("gpt-4o")},
		{"claude-sonnet-4-5-20250929", snap("claude-sonnet-4-5")},
		{"anthropic/claude-sonnet-4-5-20250929", snap("claude-sonnet-4-5")},
		{"claude-opus-4.5-20251101", snap("claude-opus-4-5")},
		// A dated variant belongs to its own undated name, never to the base model.
		{"gpt-4o-mini-2024-07-18", snap("gpt-4o-mini")},
		{"gpt-5.5-fast-2026-01-15", none("gpt-5.5-fast-2026-01-15")},
		{"gpt-4o-search-preview-2025-03-11", none("gpt-4o-search-preview-2025-03-11")},
		{"gpt-5.5-fast", none("gpt-5.5-fast")},
		// A snapshot with its own catalog row stays that model.
		{"gpt-4o-2024-05-13", none("gpt-4o-2024-05-13")},
		{"openai/gpt-4o-2024-05-13", CanonicalMatch{"gpt-4o-2024-05-13", CanonicalSourceBare}},
		// Not a release date, an unknown base, a routing policy, a wildcard.
		{"gpt-4o-0613", none("gpt-4o-0613")},
		{"gpt-4o-12345678", none("gpt-4o-12345678")},
		{"gpt-4o-2024-13-01", none("gpt-4o-2024-13-01")},
		{"gpt-4o-2024-0806", none("gpt-4o-2024-0806")},
		{"made-up-2024-08-06", none("made-up-2024-08-06")},
		{"router-20250101", none("router-20250101")},
		{"openai/-2024-08-06", none("openai/-2024-08-06")},
		{"gpt-4o-2024-08-06*", none("gpt-4o-2024-08-06*")},
	}
	for _, tc := range cases {
		if got := r.Resolve(tc.upstream); got != tc.want {
			t.Errorf("Resolve(%q) = %+v, want %+v", tc.upstream, got, tc.want)
		}
	}
}

// Catalog syncs name rows by PublicName: a snapshot keeps its own name there, so its metadata and
// price never land on the undated model.
func TestPublicNameKeepsSnapshots(t *testing.T) {
	r := snapshotTestResolver()
	for upstream, want := range map[string]string{
		"gpt-4o-2024-08-06":                    "gpt-4o-2024-08-06",
		"openai/gpt-4o-2024-08-06":             "gpt-4o-2024-08-06",
		"anthropic/claude-sonnet-4-5-20250929": "claude-sonnet-4-5-20250929",
		"openai/gpt-4o-2024-05-13":             "gpt-4o-2024-05-13",
	} {
		if got := r.PublicName(upstream); got != want {
			t.Errorf("PublicName(%q) = %q, want %q", upstream, got, want)
		}
	}
}

// A channel that lists only dated snapshots supplies the undated public models; when it also
// lists the undated name, that one is sent. Variants are not attached to the base model, and a
// snapshot with a row of its own is supplied as that row.
func TestAutoSupplyChannelSnapshots(t *testing.T) {
	usePreviewTestDB(t)
	for _, info := range []*ModelInfo{
		{Model: "gpt-4o", Status: ModelStatusPublished},
		{Model: "gpt-4o-2024-05-13", Status: ModelStatusDraft},
		{Model: "claude-sonnet-4-5", Status: ModelStatusPublished},
		{Model: "gpt-5.5", Status: ModelStatusPublished},
	} {
		if err := DB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}
	ch := &Channel{Name: "c", Status: config.ChannelStatusEnabled, Group: "default"}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	names := []string{"openai/gpt-4o-2024-08-06", "gpt-4o-2024-05-13", "claude-sonnet-4-5-20250929", "gpt-5.5-fast-2026-01-15"}
	created, err := AutoSupplyChannel(ch, names)
	if err != nil {
		t.Fatal(err)
	}
	want := "[claude-sonnet-4-5<-claude-sonnet-4-5-20250929 p0 w1 true channel]" +
		"[gpt-4o<-openai/gpt-4o-2024-08-06 p0 w1 true channel]" +
		"[gpt-4o-2024-05-13<-gpt-4o-2024-05-13 p0 w1 true channel]"
	if got := autoSupplySummary(created); got != want {
		t.Fatalf("created\n got %s\nwant %s", got, want)
	}

	other := &Channel{Name: "other", Status: config.ChannelStatusEnabled, Group: "default"}
	if err := DB.Create(other).Error; err != nil {
		t.Fatal(err)
	}
	created, err = AutoSupplyChannel(other, []string{"gpt-4o-2024-08-06", "gpt-4o"})
	if err != nil {
		t.Fatal(err)
	}
	if got := autoSupplySummary(created); got != "[gpt-4o<-gpt-4o p0 w1 true channel]" {
		t.Fatalf("undated name listed too: %s", got)
	}
}

// The supply migration supplies a snapshot as the undated model without making it an alias, which
// would bill it at the undated model's price.
func TestPlanSupplyMigrationSnapshots(t *testing.T) {
	plan, report := planSupplyMigration(supplyMigrationInput{
		infos:    []*ModelInfo{{Id: 1, Model: "gpt-4o", Status: ModelStatusPublished}},
		channels: []*Channel{migrationChannel(1, "default", 0, 1, "gpt-4o-2024-08-06", "")},
	})
	if got := fmt.Sprint(supplyStrings(plan.supplies)); got != "[gpt-4o@1=gpt-4o-2024-08-06 p0 w1]" {
		t.Errorf("supplies = %s", got)
	}
	if len(report.NewAliases) != 0 {
		t.Errorf("new aliases = %+v, want none", report.NewAliases)
	}
}

// The upstream list and the model table show the undated public model for a snapshot.
func TestChannelViewsResolveSnapshots(t *testing.T) {
	usePreviewTestDB(t)
	if err := DB.Create(&ModelInfo{Model: "gpt-4o", Status: ModelStatusPublished}).Error; err != nil {
		t.Fatal(err)
	}
	r, err := NewCanonicalResolver()
	if err != nil {
		t.Fatal(err)
	}
	row := r.upstreamModelRow("openai/gpt-4o-2024-08-06", nil)
	if got := fmt.Sprintf("%s %s/%s", row.State, row.PublicModel, row.Source); got != UpstreamModelAvailable+" gpt-4o/snapshot" {
		t.Errorf("upstream row = %s", got)
	}
	detail := &ChannelModelDetail{ModelPickerItem: ModelPickerItem{Model: "gpt-4o-2024-08-06"}}
	applyUnofferedReason(detail, r, map[string]string{})
	if detail.Unoffered != ChannelModelNoSupply || detail.Suggested != "gpt-4o" {
		t.Errorf("detail = %q / %q, want no_supply / gpt-4o", detail.Unoffered, detail.Suggested)
	}
}
