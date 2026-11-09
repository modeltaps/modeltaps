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

func TestCanonicalResolverCatalogAliasOnly(t *testing.T) {
	r := testResolver()
	if got, ok := r.CatalogAliasCanonical("anthropic/claude-opus-5"); !ok || got != "claude-opus-5" {
		t.Fatalf("catalog alias = %q %v", got, ok)
	}
	// Seed aliases and bare names are suggestions only: they are not rewritten by relay, so the
	// channel save guard must leave them alone.
	for _, name := range []string{"google/gemini-3.1-pro", "openai/gpt-6-astra", "claude-opus-5", "unknown"} {
		if got, ok := r.CatalogAliasCanonical(name); ok {
			t.Errorf("CatalogAliasCanonical(%q) = %q, want no match", name, got)
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

func TestCanonicalizeChannelModels(t *testing.T) {
	r := testResolver()

	t.Run("alias becomes canonical and maps back to the upstream name", func(t *testing.T) {
		models, mapping, changes, err := CanonicalizeChannelModels(r,
			"anthropic/claude-opus-5, openai/gpt-6-astra,gpt-6-astra-latest,anthropic/claude-made-up-9", "")
		if err != nil {
			t.Fatal(err)
		}
		// openai/gpt-6-astra is only a bare-name suggestion and stays as is.
		if models != "claude-opus-5,openai/gpt-6-astra,gpt-6-astra,anthropic/claude-made-up-9" {
			t.Fatalf("models = %q", models)
		}
		m := decodeMapping(t, mapping)
		if len(m) != 2 || m["claude-opus-5"] != "anthropic/claude-opus-5" || m["gpt-6-astra"] != "gpt-6-astra-latest" {
			t.Fatalf("mapping = %v", m)
		}
		if len(changes) != 2 || changes["anthropic/claude-opus-5"] != "claude-opus-5" || changes["gpt-6-astra-latest"] != "gpt-6-astra" {
			t.Fatalf("changes = %v", changes)
		}
	})

	t.Run("alias that is already a mapping key keeps its target", func(t *testing.T) {
		models, mapping, changes, err := CanonicalizeChannelModels(r,
			"anthropic/claude-opus-5", `{"anthropic/claude-opus-5":"+vendor-opus","other":"x"}`)
		if err != nil {
			t.Fatal(err)
		}
		m := decodeMapping(t, mapping)
		if models != "claude-opus-5" || m["claude-opus-5"] != "+vendor-opus" || m["other"] != "x" || len(m) != 2 {
			t.Fatalf("models = %q mapping = %v", models, m)
		}
		if len(changes) != 1 {
			t.Fatalf("changes = %v", changes)
		}
	})

	t.Run("canonical already listed or mapped is left alone", func(t *testing.T) {
		for _, tc := range []struct{ models, mapping string }{
			{"anthropic/claude-opus-5,claude-opus-5", ""},
			{"anthropic/claude-opus-5", `{"claude-opus-5":"elsewhere"}`},
		} {
			models, mapping, changes, err := CanonicalizeChannelModels(r, tc.models, tc.mapping)
			if err != nil || models != tc.models || mapping != tc.mapping || len(changes) != 0 {
				t.Fatalf("%+v: got %q %q %v %v", tc, models, mapping, changes, err)
			}
		}
	})

	t.Run("nothing to change returns the input untouched", func(t *testing.T) {
		models, mapping, changes, err := CanonicalizeChannelModels(r, "claude-opus-5 , gpt-6-astra", `{ }`)
		if err != nil || models != "claude-opus-5 , gpt-6-astra" || mapping != `{ }` || len(changes) != 0 {
			t.Fatalf("got %q %q %v %v", models, mapping, changes, err)
		}
	})

	t.Run("invalid mapping is an error and changes nothing", func(t *testing.T) {
		models, mapping, changes, err := CanonicalizeChannelModels(r, "anthropic/claude-opus-5", `not json`)
		if err == nil || models != "anthropic/claude-opus-5" || mapping != `not json` || len(changes) != 0 {
			t.Fatalf("got %q %q %v %v", models, mapping, changes, err)
		}
	})
}

// On a fresh install no alias row exists yet. apply_seed must create them, otherwise the seed's
// aliases do nothing and channels that list upstream names are not fixed.
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
