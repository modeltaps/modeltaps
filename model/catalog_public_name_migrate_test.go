package model

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func catalogSummary(t *testing.T) string {
	t.Helper()
	var rows []*ModelInfo
	if err := DB.Order("model").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, r := range rows {
		out = append(out, r.Model+":"+r.Status)
	}
	return strings.Join(out, ",")
}

func supplySummary(t *testing.T) string {
	t.Helper()
	var supplies []*ModelSupply
	if err := DB.Order("model, channel_id").Find(&supplies).Error; err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, s := range supplies {
		out = append(out, fmt.Sprintf("%s@%d<-%s", s.Model, s.ChannelId, s.UpstreamModel))
	}
	return strings.Join(out, ",")
}

func priceSummary(t *testing.T) string {
	t.Helper()
	var prices []*Price
	if err := DB.Find(&prices).Error; err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, p := range prices {
		out = append(out, fmt.Sprintf("%s=%g", p.Model, p.Input))
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

func TestMigratePlatformSpellingRows(t *testing.T) {
	useSupplyTestDB(t)
	for _, info := range []*ModelInfo{
		// Rename: no row under the public name yet (seed: anthropic/claude-opus-5 -> claude-opus-5).
		{Model: "anthropic/claude-opus-5", Status: ModelStatusDraft, Source: ModelInfoSourceVercel},
		// Merge: the public name has a row; the draft's supplies move to it.
		{Model: "openai/gpt-4.1-mini", Status: ModelStatusDraft, Source: ModelInfoSourceVercel},
		{Model: "gpt-4.1-mini", Status: ModelStatusPublished, Source: ModelInfoSourceManual},
		// Two spellings of one unknown model: OpenRouter's is renamed, Vercel's merged into it.
		{Model: "foo/bar-1", Status: ModelStatusDraft, Source: ModelInfoSourceOpenRouter},
		{Model: "other/bar-1", Status: ModelStatusDraft, Source: ModelInfoSourceVercel},
		// Left alone: published, locked, edited by an admin, an alias.
		{Model: "vendor/published-1", Status: ModelStatusPublished, Source: ModelInfoSourceOpenRouter},
		{Model: "vendor/locked-1", Status: ModelStatusDraft, Source: ModelInfoSourceOpenRouter, Locked: true},
		{Model: "vendor/manual-1", Status: ModelStatusDraft, Source: ModelInfoSourceManual},
		{Model: "vendor/alias-1", Status: ModelStatusDraft, Source: ModelInfoSourceOpenRouter, AliasOf: "gpt-4.1-mini"},
		// The bare id is an alias row: the draft follows the alias and is merged into the renamed
		// claude-opus-5 row.
		{Model: "vendor/opus", Status: ModelStatusDraft, Source: ModelInfoSourceVercel},
		{Model: "opus", AliasOf: "claude-opus-5"},
	} {
		if err := DB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range []*ModelSupply{
		{Model: "anthropic/claude-opus-5", ChannelId: 1, UpstreamModel: "anthropic/claude-opus-5", Enabled: true},
		{Model: "openai/gpt-4.1-mini", ChannelId: 1, UpstreamModel: "openai/gpt-4.1-mini", Enabled: true},
		// Channel 2 already supplies gpt-4.1-mini: its supply of the draft is a duplicate.
		{Model: "openai/gpt-4.1-mini", ChannelId: 2, UpstreamModel: "openai/gpt-4.1-mini", Enabled: true},
		{Model: "gpt-4.1-mini", ChannelId: 2, UpstreamModel: "gpt-4.1-mini", Enabled: true},
		{Model: "foo/bar-1", ChannelId: 1, UpstreamModel: "foo/bar-1", Enabled: true},
		{Model: "other/bar-1", ChannelId: 2, UpstreamModel: "other/bar-1", Enabled: true},
		{Model: "vendor/published-1", ChannelId: 1, UpstreamModel: "vendor/published-1", Enabled: true},
	} {
		if err := DB.Create(s).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []*Price{
		{Model: "anthropic/claude-opus-5", ChannelType: config.ChannelTypeVercelAIGateway, Input: 1},
		{Model: "openai/gpt-4.1-mini", ChannelType: config.ChannelTypeVercelAIGateway, Input: 2},
		{Model: "gpt-4.1-mini", ChannelType: config.ChannelTypeOpenAI, Input: 3, Locked: true},
		{Model: "other/bar-1", ChannelType: config.ChannelTypeVercelAIGateway, Input: 4},
	} {
		if err := DB.Create(p).Error; err != nil {
			t.Fatal(err)
		}
	}

	out, err := migratePlatformSpellingRows(DB)
	if err != nil {
		t.Fatal(err)
	}
	want := PlatformRowMigration{Renamed: 2, Merged: 3, SuppliesMoved: 4, SuppliesMerged: 1, PricesCopied: 2}
	if out != want {
		t.Errorf("counts = %+v, want %+v", out, want)
	}
	wantCatalog := "bar-1:draft,claude-opus-5:draft,gpt-4.1-mini:published,opus:draft," +
		"vendor/alias-1:draft,vendor/locked-1:draft,vendor/manual-1:draft,vendor/published-1:published"
	if got := catalogSummary(t); got != wantCatalog {
		t.Errorf("catalog\n got %s\nwant %s", got, wantCatalog)
	}
	// Supplies keep the platform spelling as the upstream name.
	wantSupplies := "bar-1@1<-foo/bar-1,bar-1@2<-other/bar-1,claude-opus-5@1<-anthropic/claude-opus-5," +
		"gpt-4.1-mini@1<-openai/gpt-4.1-mini,gpt-4.1-mini@2<-gpt-4.1-mini,vendor/published-1@1<-vendor/published-1"
	if got := supplySummary(t); got != wantSupplies {
		t.Errorf("supplies\n got %s\nwant %s", got, wantSupplies)
	}
	// Prices under spellings stay; public names without a price get a copy; the locked public
	// price is untouched.
	wantPrices := "anthropic/claude-opus-5=1,bar-1=4,claude-opus-5=1,gpt-4.1-mini=3,openai/gpt-4.1-mini=2,other/bar-1=4"
	if got := priceSummary(t); got != wantPrices {
		t.Errorf("prices\n got %s\nwant %s", got, wantPrices)
	}

	// Running it again changes nothing.
	again, err := migratePlatformSpellingRows(DB)
	if err != nil {
		t.Fatal(err)
	}
	if again != (PlatformRowMigration{}) {
		t.Errorf("second run = %+v", again)
	}
	if got := catalogSummary(t); got != wantCatalog {
		t.Errorf("catalog after second run %s", got)
	}

	// The migration is registered once, under its own ID.
	if err := migratePlatformSpellingDrafts().Migrate(DB); err != nil {
		t.Fatal(err)
	}
}

// A draft whose public name is an alias row is left alone: merging would attach supplies to an
// alias, which serves nothing itself. Here the seed names claude-opus-5 as the public name of
// "anthropic/claude-opus-5", but the catalog made claude-opus-5 an alias of a dated id.
func TestMigratePlatformSpellingRowsSkipsAliasTargets(t *testing.T) {
	useSupplyTestDB(t)
	for _, info := range []*ModelInfo{
		{Model: "claude-opus-5-20261101", Status: ModelStatusPublished},
		{Model: "claude-opus-5", AliasOf: "claude-opus-5-20261101"},
		{Model: "anthropic/claude-opus-5", Status: ModelStatusDraft, Source: ModelInfoSourceOpenRouter},
	} {
		if err := DB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := DB.Create(&ModelSupply{Model: "anthropic/claude-opus-5", ChannelId: 1, UpstreamModel: "anthropic/claude-opus-5", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	out, err := migratePlatformSpellingRows(DB)
	if err != nil {
		t.Fatal(err)
	}
	if out != (PlatformRowMigration{Skipped: 1}) {
		t.Errorf("counts = %+v, want the draft skipped", out)
	}
	if got := catalogSummary(t); got != "anthropic/claude-opus-5:draft,claude-opus-5:draft,claude-opus-5-20261101:published" {
		t.Errorf("catalog %s", got)
	}
	if got := supplySummary(t); got != "anthropic/claude-opus-5@1<-anthropic/claude-opus-5" {
		t.Errorf("supplies %s", got)
	}
}
