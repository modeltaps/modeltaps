package model

import (
	"fmt"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/datatypes"
)

func TestGetChannelUpstreamModels(t *testing.T) {
	usePreviewTestDB(t)
	for _, info := range []*ModelInfo{
		{Model: "claude-opus-5", Status: ModelStatusPublished},
		{Model: "opus", AliasOf: "claude-opus-5"},
		{Model: "gpt-9", Status: ModelStatusDraft},
		{Model: "gpt-9-mini", Status: ModelStatusPublished},
		{Model: "auto/coder", Kind: ModelInfoKindRoutingPolicy},
	} {
		if err := DB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}
	snapshot := []string{"zeta-unknown", "opus", "gpt-9", "gpt-9-mini", "up-9", "auto/coder", "opus", " "}
	drift := datatypes.NewJSONType(ModelDriftResult{CheckedAt: 1700000000, UpstreamSnapshot: snapshot, OK: true})
	ch := &Channel{Name: "c", Status: config.ChannelStatusEnabled, Group: "default", ModelDrift: &drift}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	other := &Channel{Name: "other", Status: config.ChannelStatusEnabled, Group: "default"}
	if err := DB.Create(other).Error; err != nil {
		t.Fatal(err)
	}
	// gpt-9 is sent as "up-9" by an enabled supply; gpt-9-mini has a supply that is switched off.
	// A supply on another channel must not count. Enabled defaults to true, so switch it off after.
	off := &ModelSupply{Model: "gpt-9-mini", ChannelId: ch.Id, UpstreamModel: "gpt-9-mini", Enabled: true}
	for _, s := range []*ModelSupply{
		{Model: "gpt-9", ChannelId: ch.Id, UpstreamModel: "up-9", Enabled: true},
		off,
		{Model: "claude-opus-5", ChannelId: other.Id, UpstreamModel: "opus", Enabled: true},
	} {
		if err := DB.Create(s).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := DB.Model(off).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}

	list, err := GetChannelUpstreamModels(ch)
	if err != nil {
		t.Fatal(err)
	}
	got := ""
	for _, m := range list.Models {
		got += fmt.Sprintf("[%s %s %s/%s/%s]", m.Name, m.State, m.PublicModel, m.Source, m.PublicStatus)
	}
	want := "[auto/coder unknown //]" +
		"[gpt-9 available gpt-9//draft]" +
		"[gpt-9-mini off gpt-9-mini/supply/published]" +
		"[opus available claude-opus-5/alias/published]" +
		"[up-9 in_use gpt-9/supply/draft]" +
		"[zeta-unknown unknown //]"
	if got != want {
		t.Fatalf("models\n got %s\nwant %s", got, want)
	}
	if list.CheckedAt != 1700000000 {
		t.Fatalf("checked_at = %d", list.CheckedAt)
	}
	counts := fmt.Sprint(list.Counts)
	if counts != "map[available:2 in_use:1 off:1 unknown:2]" {
		t.Fatalf("counts = %s", counts)
	}

	// A channel that was never checked has an empty list, not an error.
	empty, err := GetChannelUpstreamModels(other)
	if err != nil || len(empty.Models) != 0 || empty.CheckedAt != 0 {
		t.Fatalf("unchecked channel: %+v, %v", empty, err)
	}
}
