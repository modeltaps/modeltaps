package model

import (
	"reflect"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
)

func setupChannelModelDetailsDB(t *testing.T) {
	t.Helper()
	setupModelInfoTestDB(t)
	if err := DB.AutoMigrate(&Statistics{}, &ModelSupply{}, &ModelLibraryEntry{}, &ModelLibraryHost{}); err != nil {
		t.Fatal(err)
	}
	old := config.ModelSupplyEnabled
	t.Cleanup(func() { config.ModelSupplyEnabled = old })
}

func detailNames(details []ChannelModelDetail) []string {
	out := make([]string, 0, len(details))
	for _, d := range details {
		out = append(out, d.Model)
	}
	return out
}

// Families this site requests most come first, then the mainstream fallback tiers (so a list
// does not start with whatever sorts first by name); inside a family the latest version leads.
func TestBuildChannelModelDetailsOrder(t *testing.T) {
	setupChannelModelDetailsDB(t)
	config.ModelSupplyEnabled = false
		if err := DB.Create(&Statistics{Date: time.Now(), UserId: 1, ChannelId: 1, ModelName: "deepseek-v4-pro", RequestCount: 50}).Error; err != nil {
		t.Fatal(err)
	}
	models := []string{
		"alibaba/qwen-3-14b", "anthropic/claude-opus-4.8", "anthropic/claude-opus-5.5",
		"deepseek/deepseek-v4-pro", "acme/zzz-1", "openai/gpt-5",
	}
	details, err := BuildChannelModelDetails(0, models, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := detailNames(details)
	want := []string{"deepseek/deepseek-v4-pro", "anthropic/claude-opus-5.5", "anthropic/claude-opus-4.8", "openai/gpt-5", "alibaba/qwen-3-14b", "acme/zzz-1"}
	// claude-opus and gpt share the top fallback tier; only their relative order to the rest is fixed.
	if got[0] != want[0] || got[len(got)-2] != "alibaba/qwen-3-14b" || got[len(got)-1] != "acme/zzz-1" {
		t.Errorf("order = %v, want like %v", got, want)
	}
	if indexOf(got, "anthropic/claude-opus-5.5") > indexOf(got, "anthropic/claude-opus-4.8") {
		t.Errorf("latest opus should lead its family: %v", got)
	}
	for i, d := range details {
		if d.Rank != i+1 {
			t.Errorf("%s rank = %d, want %d", d.Model, d.Rank, i+1)
		}
	}
	if details[0].FamilyRequests != 50 {
		t.Errorf("family requests = %d, want 50", details[0].FamilyRequests)
	}
	if !reflect.DeepEqual(detailPublicNames(details[0]), []string{"deepseek/deepseek-v4-pro"}) {
		t.Errorf("without supplies the listed name is public: %+v", details[0].Public)
	}
}

// While supplies route requests, a listed model serves the public models whose supply on this
// channel sends its name upstream, the mapped name included.
func TestBuildChannelModelDetailsSupplies(t *testing.T) {
	setupChannelModelDetailsDB(t)
	config.ModelSupplyEnabled = true
	for _, s := range []ModelSupply{
		{Model: "gpt-5", ChannelId: 7, UpstreamModel: "openai/gpt-5", Enabled: true},
		{Model: "gpt-5-latest", ChannelId: 7, UpstreamModel: "openai/gpt-5", Enabled: false},
		{Model: "opus", ChannelId: 7, UpstreamModel: "anthropic/claude-opus-5.5", Enabled: true},
		{Model: "gpt-5", ChannelId: 8, UpstreamModel: "openai/gpt-5", Enabled: true},
	} {
		s := s
		if err := DB.Create(&s).Error; err != nil {
			t.Fatal(err)
		}
	}
	details, err := BuildChannelModelDetails(7, []string{"openai/gpt-5", "my-opus", "unused/model"}, map[string]string{"my-opus": "+anthropic/claude-opus-5.5"})
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]ChannelModelDetail{}
	for _, d := range details {
		byName[d.Model] = d
	}
	if got := detailPublicNames(byName["openai/gpt-5"]); !reflect.DeepEqual(got, []string{"gpt-5", "gpt-5-latest"}) {
		t.Errorf("gpt-5 public = %v", got)
	}
	if p := byName["openai/gpt-5"].Public; p[1].Enabled {
		t.Errorf("disabled supply reported enabled: %+v", p[1])
	}
	if got := detailPublicNames(byName["my-opus"]); !reflect.DeepEqual(got, []string{"opus"}) {
		t.Errorf("mapped model public = %v", got)
	}
	if got := byName["unused/model"].Public; len(got) != 0 {
		t.Errorf("unsupplied model public = %v", got)
	}
}

func detailPublicNames(d ChannelModelDetail) []string {
	out := []string{}
	for _, p := range d.Public {
		out = append(out, p.Model)
	}
	return out
}

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

// A listed model with no supply says why: no public model matches it, a public model matches
// but the channel does not supply it, or it is a wildcard.
func TestBuildChannelModelDetailsUnoffered(t *testing.T) {
	setupChannelModelDetailsDB(t)
	config.ModelSupplyEnabled = true
	if err := DB.Create(&ModelInfo{Model: "gpt-5", Name: "gpt-5", Status: ModelStatusPublished}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&ModelSupply{Model: "gpt-5", ChannelId: 7, UpstreamModel: "openai/gpt-5", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	details, err := BuildChannelModelDetails(9, []string{"openai/gpt-5", "openai/gpt-5-fast", "claude-*"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][4]string{}
	for _, d := range details {
		got[d.Model] = [4]string{d.Unoffered, d.Suggested, d.SameAs, d.SuppliedBy}
	}
	want := map[string][4]string{
		"openai/gpt-5":      {ChannelModelNoSupply, "gpt-5", "", ""},
		"openai/gpt-5-fast": {ChannelModelNotInCatalog, "gpt-5-fast", "", ""},
		"claude-*":          {ChannelModelWildcard, "", "", ""},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unoffered = %v, want %v", got, want)
	}

	// Supplied on its own channel, the name serves a public model and has no reason.
	details, err = BuildChannelModelDetails(7, []string{"openai/gpt-5"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d := details[0]; d.Unoffered != "" || d.Suggested != "" {
		t.Errorf("supplied model unoffered = %q / %q", d.Unoffered, d.Suggested)
	}

	// Another spelling of a model the channel already supplies cannot be supplied too: it is
	// offered as a catalog model of its own instead.
	details, err = BuildChannelModelDetails(7, []string{"OpenAI/GPT-5"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d := details[0]
	if got := [4]string{d.Unoffered, d.Suggested, d.SameAs, d.SuppliedBy}; got != [4]string{ChannelModelDuplicate, "GPT-5", "gpt-5", "openai/gpt-5"} {
		t.Errorf("duplicate = %v", got)
	}
}
