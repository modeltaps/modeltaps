package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/logger"

	"go.uber.org/zap"
)

// A tag edit that adds keys must insert one channel per new key even when the
// request body carries a non-zero id (the old web UI submitted parseInt(tag)).
func TestUpdateChannelsTagInsertsNewKeysWithNonZeroID(t *testing.T) {
	setupChannelKeyTestDB(t)
	// ChannelGroup.Load() at the end of UpdateChannelsTag logs.
	oldLogger := logger.Logger
	logger.Logger = zap.NewNop()
	t.Cleanup(func() { logger.Logger = oldLogger })

	seed := []Channel{
		{Name: "a", Key: "k1", Tag: "g", Status: 1, Models: "m"},
		{Name: "b", Key: "k2", Tag: "g", Status: 1, Models: "m"},
	}
	if err := DB.Create(&seed).Error; err != nil {
		t.Fatal(err)
	}

	edit := seed[0]
	edit.Key = "k1\nk2\nk3\nk4"
	if err := UpdateChannelsTag("g", &edit); err != nil {
		t.Fatal(err)
	}

	var got []Channel
	if err := DB.Where("tag = ?", "g").Order("id").Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("want 4 channels in tag, got %d", len(got))
	}
	for i, want := range []string{"k1", "k2", "k3", "k4"} {
		if got[i].Key != want {
			t.Errorf("channel %d key = %q, want %q", got[i].Id, got[i].Key, want)
		}
	}
}

// A tag edit must keep each channel's remark, leave new channels' remark empty,
// and sync shared settings such as need2response_models to the whole tag.
func TestUpdateChannelsTagKeepsPerChannelRemark(t *testing.T) {
	setupChannelKeyTestDB(t)
	oldLogger := logger.Logger
	logger.Logger = zap.NewNop()
	t.Cleanup(func() { logger.Logger = oldLogger })

	seed := []Channel{
		{Name: "a", Key: "k1", Tag: "g", Remark: "remark-a", Status: 1, Models: "m"},
		{Name: "b", Key: "k2", Tag: "g", Remark: "remark-b", Status: 1, Models: "m"},
	}
	if err := DB.Create(&seed).Error; err != nil {
		t.Fatal(err)
	}

	routes := "gpt-5-pro"
	edit := seed[0]
	edit.Key = "k1\nk2\nk3"
	edit.Remark = "overwritten"
	edit.Need2ResponseModels = &routes
	if err := UpdateChannelsTag("g", &edit); err != nil {
		t.Fatal(err)
	}

	var got []Channel
	if err := DB.Order("id").Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 channels, got %d", len(got))
	}
	for i, want := range []string{"remark-a", "remark-b", ""} {
		if got[i].Remark != want {
			t.Errorf("channel %d remark = %q, want %q", got[i].Id, got[i].Remark, want)
		}
		if got[i].Need2ResponseModels == nil || *got[i].Need2ResponseModels != routes {
			t.Errorf("channel %d need2response_models not synced: %v", got[i].Id, got[i].Need2ResponseModels)
		}
	}
}
