package model

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
)

// models.dev as it lists Fable 5.2 (Vercel spells it "anthropic/claude-fable-5.2", the lab
// "claude-fable-5-2"), a dated Opus whose lab spelling is shorter, and a model whose public name
// differs from its bare platform id.
const rehomeFixture = `{
  "anthropic": {"models": {
    "claude-fable-5-2": {"id":"claude-fable-5-2","name":"Claude Fable 5.2","modalities":{"input":["text"],"output":["text"]},"cost":{"input":8,"output":40}},
    "claude-opus-4-5":  {"id":"claude-opus-4-5","canonical_model_id":"anthropic/claude-opus-4-5-20251101"}
  }},
  "openai": {"models": {
    "gpt-9-sol": {"id":"gpt-9-sol","name":"GPT-9 Sol"}
  }},
  "vercel": {"models": {
    "anthropic/claude-fable-5.2":      {"id":"anthropic/claude-fable-5.2","canonical_model_id":"anthropic/claude-fable-5-2","name":"Claude Fable 5.2","modalities":{"input":["text"],"output":["text"]},"cost":{"input":8,"output":40}},
    "anthropic/claude-fable-5.2-fast": {"id":"anthropic/claude-fable-5.2-fast","canonical_model_id":"anthropic/claude-fable-5-2"},
    "anthropic/claude-opus-4.5":       {"id":"anthropic/claude-opus-4.5","canonical_model_id":"anthropic/claude-opus-4-5-20251101"},
    "openai/gpt-9":                    {"id":"openai/gpt-9","canonical_model_id":"openai/gpt-9-sol"}
  }}
}`

// useRehomeTestDB is usePreviewTestDB plus the library and alias tables, an empty library index
// and no price instance to refresh.
func useRehomeTestDB(t *testing.T) map[string]catalogProvider {
	t.Helper()
	usePreviewTestDB(t)
	if err := DB.AutoMigrate(&ModelLibraryEntry{}, &ModelLibraryHost{}, &ChannelModelAlias{}); err != nil {
		t.Fatal(err)
	}
	modelLibraryIndex.invalidate()
	oldPricing := PricingInstance
	PricingInstance = nil
	t.Cleanup(func() {
		PricingInstance = oldPricing
		modelLibraryIndex.invalidate()
	})
	data := map[string]catalogProvider{}
	if err := json.Unmarshal([]byte(rehomeFixture), &data); err != nil {
		t.Fatal(err)
	}
	return data
}

// catalogServer serves body as the models.dev catalog and counts the downloads.
func catalogServer(t *testing.T, status int, body string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	hits := &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, hits
}

func libraryCount(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := DB.Model(&ModelLibraryEntry{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestEnsureModelLibrary(t *testing.T) {
	useRehomeTestDB(t)

	// A failed download leaves the library empty and reports the error.
	broken, _ := catalogServer(t, http.StatusInternalServerError, "not json")
	if err := EnsureModelLibrary(broken.URL); err == nil {
		t.Error("want an error from a broken download")
	}
	if n := libraryCount(t); n != 0 {
		t.Errorf("library has %d rows after a broken download", n)
	}

	srv, hits := catalogServer(t, http.StatusOK, rehomeFixture)
	if err := EnsureModelLibrary(srv.URL); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Errorf("downloads = %d, want 1", hits.Load())
	}
	if n := libraryCount(t); n != 3 {
		t.Errorf("library has %d rows, want 3", n)
	}
	if got := LibraryPublicNames("anthropic/claude-fable-5.2"); len(got) == 0 || got[0] != "claude-fable-5-2" {
		t.Errorf("library public names = %v", got)
	}

	// A library with rows is left as it is: no download.
	if err := EnsureModelLibrary(srv.URL); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Errorf("downloads after the second call = %d, want still 1", hits.Load())
	}
}

func TestRehomeSyncedDrafts(t *testing.T) {
	data := useRehomeTestDB(t)
	if _, err := SyncModelLibrary(data); err != nil {
		t.Fatal(err)
	}
	for _, info := range []*ModelInfo{
		// Rename: named by the fallback before the library knew the public name.
		{Model: "claude-fable-5.2", Status: ModelStatusDraft, Source: ModelInfoSourceVercel},
		// Rename: a platform spelling the library does not know, kept by nothing but its own row.
		{Model: "vendor/thing-1", Status: ModelStatusDraft, Source: ModelInfoSourceOpenRouter},
		// Merge: the public name already has a row.
		{Model: "gpt-9", Status: ModelStatusDraft, Source: ModelInfoSourceOpenRouter},
		{Model: "gpt-9-sol", Status: ModelStatusPublished, Source: ModelInfoSourceManual},
		// Left alone: supplies that disagree, no supplies, published, locked, edited by an admin,
		// and a name the library knows as the lab's own spelling of a dated model.
		{Model: "mixed-1", Status: ModelStatusDraft, Source: ModelInfoSourceVercel},
		{Model: "claude-fable-5.2-fast", Status: ModelStatusDraft, Source: ModelInfoSourceVercel},
		{Model: "fable-published", Status: ModelStatusPublished, Source: ModelInfoSourceVercel},
		{Model: "fable-locked", Status: ModelStatusDraft, Source: ModelInfoSourceVercel, Locked: true},
		{Model: "fable-manual", Status: ModelStatusDraft, Source: ModelInfoSourceManual},
		{Model: "claude-opus-4-5", Status: ModelStatusDraft, Source: ModelInfoSourceVercel},
	} {
		if err := DB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range []*ModelSupply{
		{Model: "claude-fable-5.2", ChannelId: 1, UpstreamModel: "anthropic/claude-fable-5.2", Enabled: true},
		{Model: "claude-fable-5.2", ChannelId: 2, UpstreamModel: "anthropic/claude-fable-5.2-fast", Enabled: true},
		{Model: "vendor/thing-1", ChannelId: 1, UpstreamModel: "vendor/thing-1", Enabled: true},
		{Model: "gpt-9", ChannelId: 1, UpstreamModel: "openai/gpt-9", Enabled: true},
		// Channel 2 already supplies gpt-9-sol: its supply of the draft is a duplicate.
		{Model: "gpt-9", ChannelId: 2, UpstreamModel: "openai/gpt-9", Enabled: true},
		{Model: "gpt-9-sol", ChannelId: 2, UpstreamModel: "gpt-9-sol", Enabled: true},
		{Model: "mixed-1", ChannelId: 1, UpstreamModel: "anthropic/claude-fable-5.2", Enabled: true},
		{Model: "mixed-1", ChannelId: 2, UpstreamModel: "vendor/mixed-1", Enabled: true},
		{Model: "fable-published", ChannelId: 3, UpstreamModel: "anthropic/claude-fable-5.2", Enabled: true},
		{Model: "fable-locked", ChannelId: 3, UpstreamModel: "anthropic/claude-fable-5.2", Enabled: true},
		{Model: "fable-manual", ChannelId: 3, UpstreamModel: "anthropic/claude-fable-5.2", Enabled: true},
		{Model: "claude-opus-4-5", ChannelId: 1, UpstreamModel: "anthropic/claude-opus-4.5", Enabled: true},
	} {
		if err := DB.Create(s).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []*Price{
		{Model: "claude-fable-5.2", ChannelType: config.ChannelTypeVercelAIGateway, Input: 1},
		{Model: "gpt-9", ChannelType: config.ChannelTypeOpenRouter, Input: 2},
		{Model: "gpt-9-sol", ChannelType: config.ChannelTypeOpenAI, Input: 3},
	} {
		if err := DB.Create(p).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := SaveChannelAlias(1, "anthropic/claude-fable-5.2", "claude-fable-5.2", false); err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&SupplyPause{ChannelId: 1, Model: "claude-fable-5.2", Until: time.Now().Unix() + 3600}).Error; err != nil {
		t.Fatal(err)
	}

	out, err := RehomeSyncedDrafts()
	if err != nil {
		t.Fatal(err)
	}
	want := PlatformRowMigration{Renamed: 2, Merged: 1, SuppliesMoved: 4, SuppliesMerged: 1, PricesCopied: 1, AliasesMoved: 1, PausesMoved: 1}
	if out != want {
		t.Errorf("counts = %+v, want %+v", out, want)
	}
	wantCatalog := "claude-fable-5-2:draft,claude-fable-5.2-fast:draft,claude-opus-4-5:draft,fable-locked:draft," +
		"fable-manual:draft,fable-published:published,gpt-9-sol:published,mixed-1:draft,thing-1:draft"
	if got := catalogSummary(t); got != wantCatalog {
		t.Errorf("catalog\n got %s\nwant %s", got, wantCatalog)
	}
	wantSupplies := "claude-fable-5-2@1<-anthropic/claude-fable-5.2,claude-fable-5-2@2<-anthropic/claude-fable-5.2-fast," +
		"claude-opus-4-5@1<-anthropic/claude-opus-4.5," +
		"fable-locked@3<-anthropic/claude-fable-5.2,fable-manual@3<-anthropic/claude-fable-5.2," +
		"fable-published@3<-anthropic/claude-fable-5.2," +
		"gpt-9-sol@1<-openai/gpt-9,gpt-9-sol@2<-gpt-9-sol," +
		"mixed-1@1<-anthropic/claude-fable-5.2,mixed-1@2<-vendor/mixed-1,thing-1@1<-vendor/thing-1"
	if got := supplySummary(t); got != wantSupplies {
		t.Errorf("supplies\n got %s\nwant %s", got, wantSupplies)
	}
	// Prices under the old names stay; the renamed row gets a copy; gpt-9-sol keeps its own.
	wantPrices := "claude-fable-5-2=1,claude-fable-5.2=1,gpt-9-sol=3,gpt-9=2"
	if got := priceSummary(t); got != wantPrices {
		t.Errorf("prices\n got %s\nwant %s", got, wantPrices)
	}
	var aliases []*ChannelModelAlias
	if err := DB.Find(&aliases).Error; err != nil {
		t.Fatal(err)
	}
	if len(aliases) != 1 || aliases[0].Model != "claude-fable-5-2" {
		t.Errorf("alias decisions = %+v, want the one moved to claude-fable-5-2", aliases)
	}
	if !IsPaused(1, "claude-fable-5-2") || IsPaused(1, "claude-fable-5.2") {
		t.Error("the pause should have moved to claude-fable-5-2")
	}

	// Running it again changes nothing.
	again, err := RehomeSyncedDrafts()
	if err != nil {
		t.Fatal(err)
	}
	if again != (PlatformRowMigration{}) {
		t.Errorf("second run = %+v", again)
	}
	if got := catalogSummary(t); got != wantCatalog {
		t.Errorf("catalog after second run %s", got)
	}
}

// Without a library the draft keeps its fallback name: nothing better is known.
func TestRehomeSyncedDraftsWithoutLibrary(t *testing.T) {
	useRehomeTestDB(t)
	if err := DB.Create(&ModelInfo{Model: "claude-fable-5.2", Status: ModelStatusDraft, Source: ModelInfoSourceVercel}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&ModelSupply{Model: "claude-fable-5.2", ChannelId: 1, UpstreamModel: "anthropic/claude-fable-5.2", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	out, err := RehomeSyncedDrafts()
	if err != nil {
		t.Fatal(err)
	}
	if out != (PlatformRowMigration{}) {
		t.Errorf("counts = %+v, want nothing moved", out)
	}
}

// The channel-save flow on a fresh deployment. Before the fix, a channel saved while the library
// was empty got rows under fallback names that no later sync corrected; now the library is filled
// first, and drafts named before it are moved once it is.
func TestChannelSyncOnEmptyLibraryEndsWithPublicNames(t *testing.T) {
	data := useRehomeTestDB(t)
	PricingInstance = &Pricing{Prices: map[string]*Price{}, Match: []string{}}
	oldApply := config.PriceSyncAutoApply
	config.PriceSyncAutoApply = PriceSyncAutoApplyAll
	t.Cleanup(func() { config.PriceSyncAutoApply = oldApply })
	srv, hits := catalogServer(t, http.StatusOK, rehomeFixture)

	// A channel saved while the library is empty: the row gets the fallback name.
	early := []string{"anthropic/claude-fable-5.2"}
	if _, err := syncCatalogChannelModels(data, early, true); err != nil {
		t.Fatal(err)
	}
	ch := &Channel{Name: "vercel", Status: config.ChannelStatusEnabled, Group: "default", Models: strings.Join(early, ",")}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := AutoSupplyChannel(ch, early); err != nil {
		t.Fatal(err)
	}
	if got := supplySummary(t); got != "claude-fable-5.2@"+strconv.Itoa(ch.Id)+"<-anthropic/claude-fable-5.2" {
		t.Fatalf("supplies before the library sync: %s", got)
	}

	// The next channel save fills the library first; the early draft follows its public name.
	if err := EnsureModelLibrary(srv.URL); err != nil {
		t.Fatal(err)
	}
	if got := catalogSummary(t); got != "claude-fable-5-2:draft" {
		t.Errorf("catalog after the library sync: %s", got)
	}
	if got := supplySummary(t); got != "claude-fable-5-2@"+strconv.Itoa(ch.Id)+"<-anthropic/claude-fable-5.2" {
		t.Errorf("supplies after the library sync: %s", got)
	}

	// Names synced after the library is filled get their public names right away.
	later := []string{"anthropic/claude-fable-5.2", "openai/gpt-9"}
	if _, err := syncCatalogChannelModels(data, later, true); err != nil {
		t.Fatal(err)
	}
	if got := catalogSummary(t); got != "claude-fable-5-2:draft,gpt-9-sol:draft" {
		t.Errorf("catalog after the second sync: %s", got)
	}
	if err := EnsureModelLibrary(srv.URL); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Errorf("downloads = %d, want 1", hits.Load())
	}
}
