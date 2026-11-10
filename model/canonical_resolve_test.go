package model

import (
	"encoding/json"
	"testing"
)

func testResolver() *CanonicalResolver {
	infos := []*ModelInfo{
		{Model: "claude-opus-5"},
		{Model: "anthropic/claude-opus-5", AliasOf: "claude-opus-5"},
		{Model: "gpt-6-astra"},
		{Model: "gpt-6-astra-latest", AliasOf: "gpt-6-astra-alias"},
		{Model: "gpt-6-astra-alias", AliasOf: "gpt-6-astra"},
		{Model: "Qwen3-Max"},
		{Model: "deepseek-v4-alias", AliasOf: "deepseek-v4"},
		{Model: "deepseek-v4"},
		// Synced upstream row without alias_of: names like this are what channels list today.
		{Model: "openrouter/some-vendor/made-up-9"},
	}
	seed := &CatalogSeed{Models: []CatalogSeedEntry{
		{Canonical: "gemini-3.1-pro", Aliases: []string{"google/gemini-3.1-pro", "Gemini-3.1-Pro-Exp"}},
		{Canonical: "grok-5", Aliases: []string{"x-ai/grok-5"}},
	}}
	return newCanonicalResolver(infos, seed)
}

func TestCanonicalResolverResolve(t *testing.T) {
	r := testResolver()
	cases := []struct {
		upstream string
		want     CanonicalMatch
	}{
		// 1. catalog alias, followed through a chain.
		{"anthropic/claude-opus-5", CanonicalMatch{"claude-opus-5", CanonicalSourceAlias}},
		{"gpt-6-astra-latest", CanonicalMatch{"gpt-6-astra", CanonicalSourceAlias}},
		// 2. seed alias with no catalog row yet (fresh install before apply_seed).
		{"google/gemini-3.1-pro", CanonicalMatch{"gemini-3.1-pro", CanonicalSourceSeed}},
		{"x-ai/grok-5", CanonicalMatch{"grok-5", CanonicalSourceSeed}},
		// 3. bare name after the vendor prefix, case-insensitive.
		{"openai/gpt-6-astra", CanonicalMatch{"gpt-6-astra", CanonicalSourceBare}},
		{"OpenAI/GPT-6-Astra", CanonicalMatch{"gpt-6-astra", CanonicalSourceBare}},
		{"some/gpt-6-astra-latest", CanonicalMatch{"gpt-6-astra", CanonicalSourceBare}},
		{"qwen/qwen3-max", CanonicalMatch{"Qwen3-Max", CanonicalSourceBare}},
		{"deepseek/deepseek-v4-alias", CanonicalMatch{"deepseek-v4", CanonicalSourceBare}},
		{"vertex/gemini-3.1-pro-exp", CanonicalMatch{"gemini-3.1-pro", CanonicalSourceBare}},
		{"xai/grok-5", CanonicalMatch{"grok-5", CanonicalSourceBare}},
		// No match, already canonical, wildcard, empty.
		{"anthropic/claude-made-up-9", CanonicalMatch{"anthropic/claude-made-up-9", CanonicalSourceNone}},
		{"openrouter/some-vendor/made-up-9", CanonicalMatch{"openrouter/some-vendor/made-up-9", CanonicalSourceNone}},
		{"claude-opus-5", CanonicalMatch{"claude-opus-5", CanonicalSourceNone}},
		{"GPT-6-ASTRA", CanonicalMatch{"GPT-6-ASTRA", CanonicalSourceNone}},
		{"anthropic/claude-*", CanonicalMatch{"anthropic/claude-*", CanonicalSourceNone}},
		{"", CanonicalMatch{"", CanonicalSourceNone}},
	}
	for _, tc := range cases {
		if got := r.Resolve(tc.upstream); got != tc.want {
			t.Errorf("Resolve(%q) = %+v, want %+v", tc.upstream, got, tc.want)
		}
	}
}

func decodeMapping(t *testing.T, raw string) map[string]string {
	t.Helper()
	m := map[string]string{}
	if raw == "" {
		return m
	}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("decode mapping %q: %v", raw, err)
	}
	return m
}

func TestApplyCatalogSeedCreatesMissingAliases(t *testing.T) {
	setupSeedTestDB(t)
	entry := firstSeedEntry(t)
	alias := entry.Aliases[0]
	if err := DB.Create(&ModelOwnedBy{Id: 900, Name: "Vendor", Slug: NullableSlug(entry.VendorSlug)}).Error; err != nil {
		t.Fatal(err)
	}
	oldInstance := ModelOwnedBysInstance
	ModelOwnedBysInstance = &ModelOwnedBys{}
	if err := ModelOwnedBysInstance.Load(); err != nil {
		t.Fatalf("load vendors: %v", err)
	}
	t.Cleanup(func() { ModelOwnedBysInstance = oldInstance })
	weight := uint(1)
	if err := DB.Create(&Channel{Id: 1, Status: 1, Name: "or", Models: alias, Weight: &weight}).Error; err != nil {
		t.Fatal(err)
	}

	dry, err := ApplyCatalogSeed(true)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if !containsString(dry.CreatedAliases, alias) || dry.ChannelsUpdated != 1 {
		t.Fatalf("dry run summary: created_aliases=%v channels=%d", dry.CreatedAliases, dry.ChannelsUpdated)
	}
	if hasCatalogRowInDB(t, alias) {
		t.Fatal("dry run must not create alias rows")
	}

	first, err := ApplyCatalogSeed(false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !containsString(first.CreatedAliases, alias) {
		t.Fatalf("created_aliases = %v, want %q", first.CreatedAliases, alias)
	}
	row, err := GetModelInfoByModel(alias)
	if err != nil {
		t.Fatalf("alias row: %v", err)
	}
	if row.AliasOf != entry.Canonical || row.VendorID != 900 || row.Hidden {
		t.Fatalf("alias row = %+v", row)
	}
	if canonical, isAlias := ResolveCanonicalModel(alias); !isAlias || canonical != entry.Canonical {
		t.Fatalf("ResolveCanonicalModel(%q) = %q %v", alias, canonical, isAlias)
	}
	var channel Channel
	DB.First(&channel, 1)
	if channel.Models != alias+","+entry.Canonical || decodeMapping(t, channel.GetModelMapping())[entry.Canonical] != alias {
		t.Fatalf("channel not fixed: models=%q mapping=%q", channel.Models, channel.GetModelMapping())
	}

	second, err := ApplyCatalogSeed(false)
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if len(second.CreatedAliases) != 0 || second.Created != 0 || second.ChannelsUpdated != 0 || len(second.AliasedModels) != 0 {
		t.Fatalf("second run must be a no-op: %+v", second)
	}
}

func hasCatalogRowInDB(t *testing.T, name string) bool {
	t.Helper()
	var count int64
	if err := DB.Model(&ModelInfo{}).Where("model = ?", name).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count > 0
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
