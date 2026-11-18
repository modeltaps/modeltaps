package model

import (
	"reflect"
	"testing"
)

func TestUpstreamRenamesPairsMissingNamesWithTheNewNameOfTheirPublicModel(t *testing.T) {
	r := newCanonicalResolver([]*ModelInfo{{Model: "gpt-5.5"}, {Model: "glm-5.3"}, {Model: "deepseek-v4-pro"}}, nil)
	snapshot := []string{"openai/gpt-5.5", "melious/glm-5.3", "deepinfra/glm-5.3", "openai/gpt-5.4"}
	got := upstreamRenames(r, []string{"gpt-5.5", "glm-5.3", "deepseek-v4-pro"}, map[string]string{}, map[string]string{}, snapshot)
	want := map[string]string{"gpt-5.5": "openai/gpt-5.5", "glm-5.3": "melious/glm-5.3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("renames = %v, want %v", got, want)
	}
}

func TestUpstreamRenamesFollowsTheSuppliedPublicModelAndTheMapping(t *testing.T) {
	r := newCanonicalResolver([]*ModelInfo{{Model: "gpt-5.5"}}, nil)
	// The listed name is an alias the mapping sends upstream as "gpt-5.5"; its supply serves gpt-5.5.
	got := upstreamRenames(r, []string{"my-gpt"}, map[string]string{"my-gpt": "gpt-5.5"}, map[string]string{"my-gpt": "gpt-5.5"},
		[]string{"vendor/gpt-5.5", "openai/gpt-5.5"})
	if got["my-gpt"] != "openai/gpt-5.5" && got["my-gpt"] != "vendor/gpt-5.5" {
		t.Fatalf("renames = %v", got)
	}
	if none := upstreamRenames(r, []string{"gpt-5.5"}, map[string]string{}, map[string]string{}, []string{"gpt-5.5-mini"}); none != nil {
		t.Fatalf("a different model is not a rename: %v", none)
	}
}

func TestDriftImpactAndAdoptingARename(t *testing.T) {
	useAllocationTestDB(t)
	for _, info := range []*ModelInfo{{Model: "gpt-5.5", Status: ModelStatusPublished}, {Model: "glm-5.3", Status: ModelStatusDraft}} {
		if err := DB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}
	ch := createGroupTestChannel(t, "requesty", "default", "gpt-5.5,glm-5.3,openai/gpt-5.5")
	for _, s := range []*ModelSupply{
		{Model: "gpt-5.5", ChannelId: ch.Id, UpstreamModel: "gpt-5.5", Enabled: true, UpstreamMissing: 2, UpstreamGoneAt: 100},
		{Model: "glm-5.3", ChannelId: ch.Id, UpstreamModel: "glm-5.3", Enabled: true},
	} {
		if err := DB.Create(s).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := ch.UpdateModelDrift(ModelDriftResult{
		MissingModels:    []string{"gpt-5.5", "glm-5.3"},
		NewModels:        []string{"openai/gpt-5.5"},
		UpstreamSnapshot: []string{"openai/gpt-5.5", "melious/glm-5.3"},
	}); err != nil {
		t.Fatal(err)
	}

	impacts, err := DriftImpacts([]*Channel{ch})
	if err != nil {
		t.Fatal(err)
	}
	impact := impacts[ch.Id]
	// glm-5.3 is a draft: nobody is served through it.
	if !reflect.DeepEqual(impact.Serving, []string{"gpt-5.5"}) {
		t.Fatalf("serving = %v", impact.Serving)
	}
	if impact.Renamed["gpt-5.5"] != "openai/gpt-5.5" || impact.Renamed["glm-5.3"] != "melious/glm-5.3" {
		t.Fatalf("renamed = %v", impact.Renamed)
	}

	switched, err := AdoptUpstreamRenames(ch, []string{"gpt-5.5"})
	if err != nil || switched != 1 {
		t.Fatalf("switched %d, err %v", switched, err)
	}
	var supply ModelSupply
	if err := DB.Where("channel_id = ? AND model = ?", ch.Id, "gpt-5.5").First(&supply).Error; err != nil {
		t.Fatal(err)
	}
	if supply.UpstreamModel != "openai/gpt-5.5" || supply.UpstreamGoneAt != 0 || supply.UpstreamMissing != 0 {
		t.Fatalf("supply = %+v", supply)
	}
	stored, err := GetChannelById(ch.Id)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Models != "glm-5.3,openai/gpt-5.5" {
		t.Fatalf("models = %q", stored.Models)
	}
	drift := stored.GetModelDrift()
	if !reflect.DeepEqual(drift.MissingModels, []string{"glm-5.3"}) || len(drift.NewModels) != 0 {
		t.Fatalf("drift = %+v", drift)
	}
}

func TestServedModelNamesWithSupplies(t *testing.T) {
	useAllocationTestDB(t)
	for _, info := range []*ModelInfo{{Model: "gpt-5.5", Status: ModelStatusPublished}, {Model: "glm-5.3", Status: ModelStatusDraft}, {Model: "hy3", Status: ModelStatusPublished}} {
		if err := DB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}
	ch := createGroupTestChannel(t, "c", "default", "openai/gpt-5.5,glm-5.3,hy3")
	for _, s := range []*ModelSupply{
		{Model: "gpt-5.5", ChannelId: ch.Id, UpstreamModel: "openai/gpt-5.5", Enabled: true},
		{Model: "glm-5.3", ChannelId: ch.Id, UpstreamModel: "glm-5.3", Enabled: true},
		{Model: "hy3", ChannelId: ch.Id, UpstreamModel: "hy3", Enabled: true, UpstreamGoneAt: 1},
	} {
		if err := DB.Create(s).Error; err != nil {
			t.Fatal(err)
		}
	}
	served, err := ServedModelNames()
	if err != nil {
		t.Fatal(err)
	}
	// glm-5.3 is a draft and hy3's only supply is gone upstream.
	if !reflect.DeepEqual(served, map[string]bool{"gpt-5.5": true}) {
		t.Fatalf("served = %v", served)
	}
}
