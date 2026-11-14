package model

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func TestCanonicalResolverPublicName(t *testing.T) {
	r := testResolver()
	cases := []struct{ upstream, want string }{
		// What Resolve finds: catalog alias, seed alias, bare id of a catalog row.
		{"anthropic/claude-opus-5", "claude-opus-5"},
		{"x-ai/grok-5", "grok-5"},
		{"OpenAI/GPT-6-Astra", "gpt-6-astra"},
		{"qwen/qwen3-max", "Qwen3-Max"},
		// Unknown "vendor/model": the model id after the last "/", in its own case.
		{"foo/Bar-1", "Bar-1"},
		{"accounts/fireworks/models/llama-v9", "llama-v9"},
		// A spelling with a catalog row of its own stays: the supply goes to that row.
		{"openrouter/some-vendor/made-up-9", "openrouter/some-vendor/made-up-9"},
		// Plain names, wildcards and names without a model id are unchanged.
		{"my-model", "my-model"},
		{"claude-opus-5", "claude-opus-5"},
		{"anthropic/claude-*", "anthropic/claude-*"},
		{"foo/", "foo/"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := r.PublicName(tc.upstream); got != tc.want {
			t.Errorf("PublicName(%q) = %q, want %q", tc.upstream, got, tc.want)
		}
	}
}

// A spelling the model library knows maps to the lab's name of the model.
func TestCanonicalResolverPublicNameFromLibrary(t *testing.T) {
	data := setupModelLibraryTestDB(t)
	if _, err := SyncModelLibrary(data); err != nil {
		t.Fatal(err)
	}
	r, err := NewCanonicalResolver()
	if err != nil {
		t.Fatal(err)
	}
	for upstream, want := range map[string]string{
		"claude-fable-5-2@default":      "claude-fable-5-2",
		"us.anthropic.claude-fable-5-2": "claude-fable-5-2",
		"anthropic/claude-fable-5.2":    "claude-fable-5-2",
	} {
		if got := r.PublicName(upstream); got != want {
			t.Errorf("PublicName(%q) = %q, want %q", upstream, got, want)
		}
	}
}

// A row created under the public name is the one Resolve and publicModel find for the spelling,
// so AutoSupplyChannel attaches the spelling to it.
func TestPublicNameAgreesWithResolve(t *testing.T) {
	base := testResolver()
	for _, upstream := range []string{"foo/Bar-1", "anthropic/claude-opus-5", "x-ai/grok-5", "OpenAI/GPT-6-Astra", "nested/a/b-2"} {
		public := base.PublicName(upstream)
		infos := []*ModelInfo{{Model: public}}
		for _, info := range base.byModel {
			if info.Model != public {
				infos = append(infos, info)
			}
		}
		seed := &CatalogSeed{Models: []CatalogSeedEntry{{Canonical: "grok-5", Aliases: []string{"x-ai/grok-5"}}}}
		after := newCanonicalResolver(infos, seed)
		info := after.publicModel(after.Resolve(upstream).Canonical)
		if info == nil || info.Model != public {
			t.Errorf("%q: row %q not found by Resolve (got %+v)", upstream, public, info)
		}
		if again := after.PublicName(upstream); again != public {
			t.Errorf("%q: public name moved from %q to %q once the row exists", upstream, public, again)
		}
	}
}

func infoNames(infos []*ModelInfo) map[string]string {
	out := map[string]string{}
	for _, info := range infos {
		out[info.Model] = info.Name
	}
	return out
}

func TestPublicNameInfos(t *testing.T) {
	r := newCanonicalResolver([]*ModelInfo{
		// An admin published this spelling before the seed listed it as an alias.
		{Model: "x-ai/grok-5", Status: ModelStatusPublished},
	}, &CatalogSeed{Models: []CatalogSeedEntry{
		{Canonical: "claude-opus-5", Aliases: []string{"anthropic/claude-opus-5"}},
		{Canonical: "gemini-3.1-pro", Aliases: []string{"Gemini-3.1-Pro-Exp", "google/gemini-3.1-pro"}},
		{Canonical: "grok-5", Aliases: []string{"x-ai/grok-5"}},
	}})
	groups := [][]*ModelInfo{
		{ // first source
			{Model: "anthropic/claude-opus-5", Name: "first opus"},
			{Model: "foo/bar-1", Name: "first bar"},
			{Model: "Gemini-3.1-Pro-Exp", Name: "exp"},
			{Model: "google/gemini-3.1-pro", Name: "google"},
			{Model: "azure/openai/gpt-x", Name: "long"},
			{Model: "openai/gpt-x", Name: "short"},
		},
		{ // second source
			{Model: "anthropic/claude-opus-5", Name: "second opus"},
			{Model: "bar-1", Name: "second bar"},
			{Model: "x-ai/grok-5", Name: "grok"},
			{Model: "plain-9", Name: "plain"},
		},
	}
	out := PublicNameInfos(r, groups)
	if len(out) != 2 {
		t.Fatalf("groups = %d", len(out))
	}
	first, second := infoNames(out[0]), infoNames(out[1])
	wantFirst := map[string]string{
		"claude-opus-5": "first opus", // the first source wins over the second
		"bar-1":         "first bar",  // even over a spelling that is the public name itself
		// The spelling whose bare id is the public name wins over a shorter alias.
		"gemini-3.1-pro": "google",
		"gpt-x":          "short", // then the shorter spelling
	}
	wantSecond := map[string]string{
		"grok-5":      "grok",
		"x-ai/grok-5": "grok", // the existing row under the spelling is refreshed, not created
		"plain-9":     "plain",
	}
	if len(first) != len(wantFirst) || len(second) != len(wantSecond) {
		t.Fatalf("first %v\nsecond %v", first, second)
	}
	for k, v := range wantFirst {
		if first[k] != v {
			t.Errorf("first[%q] = %q, want %q (all %v)", k, first[k], v, first)
		}
	}
	for k, v := range wantSecond {
		if second[k] != v {
			t.Errorf("second[%q] = %q, want %q (all %v)", k, second[k], v, second)
		}
	}
	// The input is not changed.
	if groups[0][0].Model != "anthropic/claude-opus-5" {
		t.Errorf("input renamed: %q", groups[0][0].Model)
	}
}

func TestPublicNamePrices(t *testing.T) {
	r := newCanonicalResolver(nil, &CatalogSeed{Models: []CatalogSeedEntry{
		{Canonical: "claude-opus-5", Aliases: []string{"anthropic/claude-opus-5"}},
	}})
	groups := [][]*Price{
		{
			{Model: "anthropic/claude-opus-5", ChannelType: config.ChannelTypeOpenRouter, Input: 1},
			{Model: "foo/bar-1", ChannelType: config.ChannelTypeOpenRouter, Input: 2},
			{Model: "openai/gpt-x", ChannelType: config.ChannelTypeOpenRouter, Input: 3},
		},
		{
			{Model: "anthropic/claude-opus-5", ChannelType: config.ChannelTypeVercelAIGateway, Input: 9},
			{Model: "vendor/qux-2", ChannelType: config.ChannelTypeVercelAIGateway, Input: 4},
			// Listed as is: written under its own name, so no copy for gpt-x.
			{Model: "gpt-x", ChannelType: config.ChannelTypeVercelAIGateway, Input: 5},
			{Model: "plain-9", ChannelType: config.ChannelTypeVercelAIGateway, Input: 6},
		},
	}
	got := map[string]*Price{}
	for _, p := range PublicNamePrices(r, groups) {
		got[p.Model] = p
	}
	if len(got) != 3 {
		t.Fatalf("copies = %v", got)
	}
	if p := got["claude-opus-5"]; p == nil || p.Input != 1 || p.ChannelType != config.ChannelTypeOpenRouter {
		t.Errorf("claude-opus-5 copy = %+v, want the first source's price", p)
	}
	if p := got["bar-1"]; p == nil || p.Input != 2 {
		t.Errorf("bar-1 copy = %+v", p)
	}
	if p := got["qux-2"]; p == nil || p.Input != 4 {
		t.Errorf("qux-2 copy = %+v", p)
	}
	if groups[0][0].Model != "anthropic/claude-opus-5" {
		t.Errorf("input renamed: %q", groups[0][0].Model)
	}

	oldPricing := PricingInstance
	PricingInstance = &Pricing{Prices: map[string]*Price{
		"claude-opus-5": {Model: "claude-opus-5", ChannelType: config.ChannelTypeAnthropic}, // official price
		"bar-1":         {Model: "bar-1", ChannelType: config.ChannelTypeVercelAIGateway},   // an aggregator's
	}}
	t.Cleanup(func() { PricingInstance = oldPricing })
	var kept []string
	for _, p := range KeepChannelPricedOrMissing(PublicNamePrices(r, groups)) {
		kept = append(kept, p.Model)
	}
	sort.Strings(kept)
	if strings.Join(kept, ",") != "bar-1,qux-2" {
		t.Errorf("kept = %v, want bar-1 (aggregator price) and qux-2 (no price)", kept)
	}
}

// End to end on the channel-save path: the models.dev sync writes catalog rows under public
// names, and AutoSupplyChannel then attaches each listed spelling to its public model.
func TestSyncCatalogChannelModelsUsesPublicNames(t *testing.T) {
	usePreviewTestDB(t)
	oldPricing := PricingInstance
	PricingInstance = &Pricing{Prices: map[string]*Price{}, Match: []string{}}
	t.Cleanup(func() { PricingInstance = oldPricing })
	oldApply := config.PriceSyncAutoApply
	config.PriceSyncAutoApply = PriceSyncAutoApplyAll
	t.Cleanup(func() { config.PriceSyncAutoApply = oldApply })

	const fx = `{
      "anthropic": {"models": {
        "claude-opus-5": {"id":"claude-opus-5","name":"Claude Opus 5","modalities":{"input":["text"],"output":["text"]},"cost":{"input":5,"output":25}}
      }},
      "openai": {"models": {
        "gpt-4.1-mini": {"id":"gpt-4.1-mini","name":"GPT-4.1 mini","modalities":{"input":["text"],"output":["text"]},"cost":{"input":0.4,"output":1.6}}
      }},
      "some-host": {"models": {
        "foo/bar-1": {"id":"foo/bar-1","name":"Bar 1","modalities":{"input":["text"],"output":["text"]},"cost":{"input":1,"output":1}}
      }}
    }`
	data := map[string]catalogProvider{}
	if err := json.Unmarshal([]byte(fx), &data); err != nil {
		t.Fatal(err)
	}
	names := []string{"anthropic/claude-opus-5", "openai/gpt-4.1-mini", "foo/bar-1"}
	if _, err := syncCatalogChannelModels(data, names, true); err != nil {
		t.Fatal(err)
	}
	var rows []*ModelInfo
	if err := DB.Order("model").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rows {
		got = append(got, r.Model+":"+r.Status+":"+r.Source)
	}
	want := "bar-1:draft:models.dev,claude-opus-5:draft:models.dev,gpt-4.1-mini:draft:models.dev"
	if strings.Join(got, ",") != want {
		t.Fatalf("catalog rows\n got %v\nwant %s", got, want)
	}
	// Prices stay under the listed spellings (billing without supply routing) and are copied to
	// the public names.
	for _, name := range append([]string{"claude-opus-5", "gpt-4.1-mini", "bar-1"}, names...) {
		if _, ok := PricingInstance.priceSnapshot(name); !ok {
			t.Errorf("no price under %q", name)
		}
	}
	// Once rows and prices exist under the public names, the channel's names are not missing.
	if missing := CatalogMissingModels(names); len(missing) != 0 {
		t.Errorf("missing = %v", missing)
	}

	ch := &Channel{Name: "aggregator", Status: config.ChannelStatusEnabled, Group: "default", Models: strings.Join(names, ",")}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	created, err := AutoSupplyChannel(ch, names)
	if err != nil {
		t.Fatal(err)
	}
	wantSupplies := "[bar-1<-foo/bar-1 p0 w1 true channel][claude-opus-5<-anthropic/claude-opus-5 p0 w1 true channel]" +
		"[gpt-4.1-mini<-openai/gpt-4.1-mini p0 w1 true channel]"
	if got := autoSupplySummary(created); got != wantSupplies {
		t.Fatalf("supplies\n got %s\nwant %s", got, wantSupplies)
	}

	// A second sync finds the rows and creates nothing new.
	if _, err := syncCatalogChannelModels(data, names, true); err != nil {
		t.Fatal(err)
	}
	var count int64
	DB.Model(&ModelInfo{}).Count(&count)
	if count != 3 {
		t.Fatalf("catalog has %d rows after the second sync, want 3", count)
	}
}

// Channel-sourced metadata and prices win over models.dev, compared on the public name.
func TestSyncCatalogChannelModelsKeepsChannelSourced(t *testing.T) {
	usePreviewTestDB(t)
	if err := DB.Create(&ModelInfo{Model: "bar-1", Name: "From aggregator", Source: ModelInfoSourceVercel}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&Price{Model: "bar-1", ChannelType: config.ChannelTypeVercelAIGateway, Input: 7, Output: 7}).Error; err != nil {
		t.Fatal(err)
	}
	oldPricing := PricingInstance
	PricingInstance = &Pricing{Prices: map[string]*Price{}, Match: []string{}}
	t.Cleanup(func() { PricingInstance = oldPricing })
	if err := PricingInstance.Init(); err != nil {
		t.Fatal(err)
	}
	data := map[string]catalogProvider{}
	if err := json.Unmarshal([]byte(`{"some-host":{"models":{"foo/bar-1":{"id":"foo/bar-1","name":"Bar 1","cost":{"input":1,"output":1}}}}}`), &data); err != nil {
		t.Fatal(err)
	}
	if _, err := syncCatalogChannelModels(data, []string{"foo/bar-1"}, false); err != nil {
		t.Fatal(err)
	}
	row, err := GetModelInfoByModel("bar-1")
	if err != nil || row.Name != "From aggregator" || row.Source != ModelInfoSourceVercel {
		t.Errorf("bar-1 row = %+v, %v", row, err)
	}
	if p, _ := PricingInstance.priceSnapshot("bar-1"); p == nil || p.Input != 7 {
		t.Errorf("bar-1 price = %+v", p)
	}
	var count int64
	DB.Model(&ModelInfo{}).Where("model = ?", "foo/bar-1").Count(&count)
	if count != 0 {
		t.Error("a row was created under the platform spelling")
	}
}
