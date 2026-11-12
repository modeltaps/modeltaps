package model

import (
	"fmt"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func followSummary(t *testing.T, channelId int) string {
	t.Helper()
	var supplies []*ModelSupply
	if err := DB.Where("channel_id = ?", channelId).Order("model").Find(&supplies).Error; err != nil {
		t.Fatal(err)
	}
	out := ""
	for _, s := range supplies {
		out += fmt.Sprintf("[%s p%d w%d]", s.Model, s.Priority, s.Weight)
	}
	return out
}

// Supplies that still have the channel's priority or weight move with the channel; a supply that
// was given its own number keeps it.
func TestSuppliesFollowChannel(t *testing.T) {
	usePreviewTestDB(t)
	priority, weight := int64(10), uint(1)
	ch := &Channel{Name: "c", Status: config.ChannelStatusEnabled, Group: "default", Tag: "team", Priority: &priority, Weight: &weight}
	other := &Channel{Name: "other", Status: config.ChannelStatusEnabled, Group: "default", Priority: &priority, Weight: &weight}
	for _, c := range []*Channel{ch, other} {
		if err := DB.Create(c).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range []*ModelSupply{
		{Model: "a", ChannelId: ch.Id, UpstreamModel: "a", Priority: 10, Weight: 1, Enabled: true}, // follows both
		{Model: "b", ChannelId: ch.Id, UpstreamModel: "b", Priority: 3, Weight: 1, Enabled: true},  // own priority
		{Model: "c", ChannelId: ch.Id, UpstreamModel: "c", Priority: 10, Weight: 7, Enabled: true}, // own weight
		{Model: "d", ChannelId: ch.Id, UpstreamModel: "d", Priority: 10, Weight: 0, Enabled: true}, // unset weight counts as 1
		{Model: "a", ChannelId: other.Id, UpstreamModel: "a", Priority: 10, Weight: 1, Enabled: true},
	} {
		if err := DB.Create(s).Error; err != nil {
			t.Fatal(err)
		}
	}

	// A partial update, as the console sends for "priority" and "weight".
	newPriority, newWeight := int64(20), uint(4)
	if err := (&Channel{Id: ch.Id, Priority: &newPriority, Weight: &newWeight}).Update(false); err != nil {
		t.Fatal(err)
	}
	if got, want := followSummary(t, ch.Id), "[a p20 w4][b p3 w4][c p20 w7][d p20 w4]"; got != want {
		t.Fatalf("after channel update\n got %s\nwant %s", got, want)
	}
	if got := followSummary(t, other.Id); got != "[a p10 w1]" {
		t.Fatalf("another channel's supplies changed: %s", got)
	}

	// An update that leaves the numbers alone changes nothing.
	if err := (&Channel{Id: ch.Id, Name: "renamed"}).Update(false); err != nil {
		t.Fatal(err)
	}
	if got := followSummary(t, ch.Id); got != "[a p20 w4][b p3 w4][c p20 w7][d p20 w4]" {
		t.Fatalf("rename moved supplies: %s", got)
	}

	// The tag-wide setters do the same for every channel of the tag.
	if err := UpdateChannelsTagPriority("team", 5); err != nil {
		t.Fatal(err)
	}
	if err := UpdateChannelsTagWeight("team", 2); err != nil {
		t.Fatal(err)
	}
	if got, want := followSummary(t, ch.Id), "[a p5 w2][b p3 w2][c p5 w7][d p5 w2]"; got != want {
		t.Fatalf("after tag update\n got %s\nwant %s", got, want)
	}
	if got := followSummary(t, other.Id); got != "[a p10 w1]" {
		t.Fatalf("tag update reached an untagged channel: %s", got)
	}

	views, err := GetModelSupplyViews("c", 0)
	if err != nil || len(views) != 1 || views[0].ChannelPriority != 5 || views[0].ChannelWeight != 2 {
		t.Fatalf("views = %+v, err %v", views, err)
	}
}
