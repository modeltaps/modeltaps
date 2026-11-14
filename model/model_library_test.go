package model

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Fable 5.2 as models.dev lists it: the lab entry has no canonical id, the platforms point at it.
const libraryFixture = `{
  "anthropic": {"models": {
    "claude-fable-5-2": {"id":"claude-fable-5-2","name":"Claude Fable 5.2","limit":{"context":1000000,"output":128000},"modalities":{"input":["text","image"],"output":["text"]},"cost":{"input":8,"output":40},"release_date":"2026-10-30"},
    "claude-opus-4-5":  {"id":"claude-opus-4-5","canonical_model_id":"anthropic/claude-opus-4-5-20251101"}
  }},
  "google-vertex-anthropic": {"models": {
    "claude-fable-5-2@default": {"id":"claude-fable-5-2@default","name":"Fable on Vertex","canonical_model_id":"anthropic/claude-fable-5-2"},
    "claude-opus-4-5@20251101": {"id":"claude-opus-4-5@20251101","canonical_model_id":"anthropic/claude-opus-4-5-20251101"}
  }},
  "amazon-bedrock": {"models": {
    "us.anthropic.claude-fable-5-2": {"id":"us.anthropic.claude-fable-5-2","canonical_model_id":"anthropic/claude-fable-5-2"}
  }},
  "openrouter": {"models": {
    "anthropic/claude-fable-5.2": {"id":"anthropic/claude-fable-5.2","canonical_model_id":"anthropic/claude-fable-5-2"}
  }},
  "openai": {"models": {
    "gpt-5.6": {"id":"gpt-5.6","canonical_model_id":"openai/gpt-5.6-sol"}
  }},
  "some-relay": {"models": {
    "gpt-5.6-sol": {"id":"gpt-5.6-sol","canonical_model_id":"openai/gpt-5.6-sol","name":"Relay copy"},
    "phi-4": {"id":"phi-4"}
  }}
}`

func setupModelLibraryTestDB(t *testing.T) map[string]catalogProvider {
	t.Helper()
	setupCatalogVendors(t)
	dsn := filepath.Join(t.TempDir(), "lib_test.db") + "?_busy_timeout=5000&_journal_mode=WAL"
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&ModelInfo{}, &ModelLibraryEntry{}, &ModelLibraryHost{}, &Channel{}, &ChannelModelAlias{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldUsing := DB, common.UsingSQLite
	DB, common.UsingSQLite = testDB, true
	modelLibraryIndex.invalidate()
	t.Cleanup(func() {
		DB, common.UsingSQLite = oldDB, oldUsing
		modelLibraryIndex.invalidate()
	})
	data := map[string]catalogProvider{}
	if err := json.Unmarshal([]byte(libraryFixture), &data); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestBuildModelLibraryDeduplicatesByCanonical(t *testing.T) {
	data := setupModelLibraryTestDB(t)
	entries, hosts := BuildModelLibrary(data, 1)
	byCanonical := map[string]*ModelLibraryEntry{}
	for _, e := range entries {
		byCanonical[e.Canonical] = e
	}
	if len(entries) != 3 {
		t.Fatalf("want 3 models (fable, opus 4.5, gpt-5.6-sol), got %d: %+v", len(entries), byCanonical)
	}
	fable := byCanonical["anthropic/claude-fable-5-2"]
	if fable == nil {
		t.Fatal("fable missing")
	}
	if fable.Model != "claude-fable-5-2" || fable.Name != "Claude Fable 5.2" || fable.Source != "anthropic" ||
		fable.VendorID != config.ChannelTypeAnthropic || fable.ContextLength != 1000000 || fable.InputPrice == nil || *fable.InputPrice != 8 {
		t.Errorf("fable should take the lab's own entry, got %+v", fable)
	}
	if sol := byCanonical["openai/gpt-5.6-sol"]; sol == nil || sol.Source != "openai" {
		t.Errorf("a lab entry beats a relay copy, got %+v", sol)
	}
	fableHosts := 0
	for _, h := range hosts {
		if h.Canonical == "anthropic/claude-fable-5-2" {
			fableHosts++
		}
		if h.HostModel == "phi-4" {
			t.Error("entries without a canonical id outside the labs are left out")
		}
	}
	if fableHosts != 4 {
		t.Errorf("fable spellings = %d, want 4", fableHosts)
	}
}

func TestLibraryLookups(t *testing.T) {
	data := setupModelLibraryTestDB(t)
	if _, err := SyncModelLibrary(data); err != nil {
		t.Fatal(err)
	}
	if got := LibraryPublicNames("claude-fable-5-2@default"); len(got) != 1 || got[0] != "claude-fable-5-2" {
		t.Errorf("vertex spelling -> %v", got)
	}
	if got := LibraryPublicNames("claude-opus-4-5@20251101"); len(got) != 2 || got[1] != "claude-opus-4-5" {
		t.Errorf("dated canonical should also offer the lab's name, got %v", got)
	}
	if got := LibraryPublicNames("gpt-5.6"); got != nil {
		t.Errorf("a lab's own name is not a platform spelling, got %v", got)
	}
	if got := LibraryUpstreamFor(config.ChannelTypeVertexAI, "claude-fable-5-2"); got != "claude-fable-5-2@default" {
		t.Errorf("vertex upstream = %q", got)
	}
	if got := LibraryUpstreamFor(config.ChannelTypeVertexAI, "claude-opus-4-5"); got != "claude-opus-4-5@20251101" {
		t.Errorf("vertex upstream for the lab's undated name = %q", got)
	}
	if got := LibraryUpstreamFor(config.ChannelTypeBedrock, "claude-fable-5-2"); got != "" {
		t.Errorf("bedrock translates names itself, got %q", got)
	}
	if got := LibraryUpstreamFor(config.ChannelTypeOpenRouter, "claude-fable-5-2"); got != "anthropic/claude-fable-5.2" {
		t.Errorf("openrouter upstream = %q", got)
	}
}

func TestCanonicalResolverUsesLibrary(t *testing.T) {
	data := setupModelLibraryTestDB(t)
	if _, err := SyncModelLibrary(data); err != nil {
		t.Fatal(err)
	}
	r := newCanonicalResolver([]*ModelInfo{{Model: "claude-opus-4-5"}}, nil)
	cases := map[string]string{
		"claude-fable-5-2@default":      "claude-fable-5-2",
		"us.anthropic.claude-fable-5-2": "claude-fable-5-2",
		"claude-opus-4-5@20251101":      "claude-opus-4-5", // the catalog's name wins over the dated id
		"gpt-5.6":                       "gpt-5.6",
	}
	for upstream, want := range cases {
		m := r.Resolve(upstream)
		if m.Canonical != want {
			t.Errorf("Resolve(%q) = %q (%s), want %q", upstream, m.Canonical, m.Source, want)
		}
	}
	if m := r.Resolve("claude-fable-5-2@default"); m.Source != CanonicalSourceLibrary {
		t.Errorf("source = %q, want library", m.Source)
	}
}

func TestSuggestSupplyUpstream(t *testing.T) {
	data := setupModelLibraryTestDB(t)
	if _, err := SyncModelLibrary(data); err != nil {
		t.Fatal(err)
	}
	mapping := `{"claude-fable-5-2":"+my-fable"}`
	cases := []struct {
		name    string
		channel *Channel
		want    SupplyUpstreamSuggestion
	}{
		{"mapping", &Channel{Type: config.ChannelTypeVertexAI, Models: "x", ModelMapping: &mapping}, SupplyUpstreamSuggestion{"my-fable", SupplyUpstreamFromMapping}},
		{"listed spelling", &Channel{Type: config.ChannelTypeCustom, Models: "a,claude-fable-5-2@default"}, SupplyUpstreamSuggestion{"claude-fable-5-2@default", SupplyUpstreamFromChannel}},
		{"listed as is", &Channel{Type: config.ChannelTypeVertexAI, Models: "claude-fable-5-2"}, SupplyUpstreamSuggestion{"claude-fable-5-2", SupplyUpstreamFromChannel}},
		{"platform", &Channel{Type: config.ChannelTypeVertexAI, Models: "gemini-x"}, SupplyUpstreamSuggestion{"claude-fable-5-2@default", SupplyUpstreamFromLibrary}},
		{"bedrock", &Channel{Type: config.ChannelTypeBedrock}, SupplyUpstreamSuggestion{"claude-fable-5-2", SupplyUpstreamAsIs}},
	}
	for _, c := range cases {
		if got := SuggestSupplyUpstream("claude-fable-5-2", c.channel); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

// The catalog sync refreshes models already in the catalog and creates none.
func TestExistingModelInfosKeepsOnlyCatalogRows(t *testing.T) {
	setupModelLibraryTestDB(t)
	if err := DB.Create(&ModelInfo{Model: "kept"}).Error; err != nil {
		t.Fatal(err)
	}
	got, err := existingModelInfos([]*ModelInfo{{Model: "kept"}, {Model: "new"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Model != "kept" {
		t.Errorf("got %+v", got)
	}
}

func TestAddLibraryModels(t *testing.T) {
	data := setupModelLibraryTestDB(t)
	if err := DB.AutoMigrate(&ModelSupply{}, &Price{}); err != nil {
		t.Fatal(err)
	}
	oldPricing := PricingInstance
	PricingInstance = &Pricing{Prices: map[string]*Price{}}
	t.Cleanup(func() { PricingInstance = oldPricing })
	if _, err := SyncModelLibrary(data); err != nil {
		t.Fatal(err)
	}
	vertex := &Channel{Name: "vertex", Type: config.ChannelTypeVertexAI, Status: config.ChannelStatusEnabled, Models: "claude-fable-5-2@default"}
	off := &Channel{Name: "off", Type: config.ChannelTypeAnthropic, Status: config.ChannelStatusManuallyDisabled, Models: "claude-fable-5-2"}
	for _, ch := range []*Channel{vertex, off} {
		if err := DB.Create(ch).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := DB.Create(&ModelInfo{Model: "gpt-5.6-sol", Status: ModelStatusDraft}).Error; err != nil {
		t.Fatal(err)
	}

	items, err := ListLibraryItems()
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		switch it.Model {
		case "claude-fable-5-2":
			if len(it.Channels) != 1 || it.Channels[0].Id != vertex.Id || it.Channels[0].UpstreamModel != "claude-fable-5-2@default" {
				t.Errorf("fable channels = %+v, want only the enabled vertex channel", it.Channels)
			}
		case "gpt-5.6-sol":
			if !it.InCatalog {
				t.Error("gpt-5.6-sol is in the catalog")
			}
		}
	}

	res, err := AddLibraryModels(LibraryAddRequest{
		Canonicals: []string{"anthropic/claude-fable-5-2", "openai/gpt-5.6-sol"}, Status: ModelStatusPublished, Supplies: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Added) != 1 || res.Added[0] != "claude-fable-5-2" || len(res.Skipped) != 1 || res.Supplies != 1 || res.Priced != 1 {
		t.Fatalf("result = %+v", res)
	}
	info, _ := GetModelInfoByModel("claude-fable-5-2")
	if info == nil || info.Status != ModelStatusPublished || info.Hidden || info.Source != ModelInfoSourceLibrary ||
		info.VendorID != config.ChannelTypeAnthropic || info.ContextLength != 1000000 {
		t.Errorf("catalog row = %+v", info)
	}
	var supply ModelSupply
	if err := DB.Where("model = ?", "claude-fable-5-2").First(&supply).Error; err != nil {
		t.Fatal(err)
	}
	if supply.ChannelId != vertex.Id || supply.UpstreamModel != "claude-fable-5-2@default" || supply.Source != ModelSupplySourceLibrary {
		t.Errorf("supply = %+v", supply)
	}
	var price Price
	if err := DB.Where("model = ?", "claude-fable-5-2").First(&price).Error; err != nil {
		t.Fatal(err)
	}
	if !floatEqM(price.Input, 4) || !floatEqM(price.Output, 20) {
		t.Errorf("price in/out = %g/%g, want 4/20 ($8/$40 per 1M)", price.Input, price.Output)
	}
	if _, err := AddLibraryModels(LibraryAddRequest{Canonicals: []string{"x"}, Status: "weird"}); err == nil {
		t.Error("an unknown status is refused")
	}
}

func TestStaleCatalogRows(t *testing.T) {
	setupModelLibraryTestDB(t)
	if err := DB.AutoMigrate(&ModelSupply{}); err != nil {
		t.Fatal(err)
	}
	mapping := `{"mapped-key":"x"}`
	if err := DB.Create(&Channel{Name: "c", Models: "listed", ModelMapping: &mapping}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&ModelSupply{Model: "supplied", ChannelId: 1, UpstreamModel: "supplied", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	draft := func(m string) ModelInfo {
		return ModelInfo{Model: m, Source: ModelInfoSourceModelsDev, Status: ModelStatusDraft}
	}
	rows := []ModelInfo{
		draft("stale-1"), draft("claude-fable-5-2@default"),
		draft("listed"), draft("mapped-key"), draft("supplied"), draft("alias-target"),
		{Model: "an-alias", Source: ModelInfoSourceModelsDev, Status: ModelStatusDraft, AliasOf: "alias-target"},
		{Model: "published", Source: ModelInfoSourceModelsDev, Status: ModelStatusPublished},
		{Model: "manual", Source: ModelInfoSourceManual, Status: ModelStatusDraft},
		{Model: "locked", Source: ModelInfoSourceModelsDev, Status: ModelStatusDraft, Locked: true},
	}
	for i := range rows {
		if err := DB.Create(&rows[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := cleanupSyncedCatalogDrafts().Migrate(DB); err != nil {
		t.Fatal(err)
	}
	var left []string
	DB.Model(&ModelInfo{}).Order("model").Pluck("model", &left)
	want := []string{"alias-target", "an-alias", "listed", "locked", "manual", "mapped-key", "published", "supplied"}
	if len(left) != len(want) {
		t.Fatalf("left = %v, want %v", left, want)
	}
	for i := range want {
		if left[i] != want[i] {
			t.Fatalf("left = %v, want %v", left, want)
		}
	}
}

func TestVendorIDFromNameRules(t *testing.T) {
	setupCatalogVendors(t)
	cases := map[string]int{
		"claude-fable-5-2":              config.ChannelTypeAnthropic,
		"us.anthropic.claude-fable-5-2": config.ChannelTypeAnthropic,
		"grok-4-20-reasoning":           config.ChannelTypeXAI,
		"gpt-5-codex":                   config.ChannelTypeOpenAI,
		"o3-mini":                       config.ChannelTypeOpenAI,
		"glm-5.2":                       config.ChannelTypeZhipu,
		"llama-3.3-70b-instruct":        config.ChannelTypeLLAMA,
		"codestral-2501":                config.ChannelTypeMistral,
		"cohere-command-a":              config.ChannelTypeCohere,
		"some-relay/deepseek-v4-pro":    config.ChannelTypeDeepseek,
		"phi-4":                         0,
		"omega-1":                       0,
	}
	for name, want := range cases {
		if got := VendorIDFromNameRules(name); got != want {
			t.Errorf("VendorIDFromNameRules(%q) = %d, want %d", name, got, want)
		}
	}
}
