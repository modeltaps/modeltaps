package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func TestCheaperUpstream(t *testing.T) {
	usePreviewTestDB(t)
	if err := DB.Create(&ModelInfo{Model: "claude-fable-5", Status: ModelStatusPublished}).Error; err != nil {
		t.Fatal(err)
	}
	requesty := previewChannel(t, "requesty", "default", config.ChannelStatusEnabled)
	DB.Model(requesty).Update("type", config.ChannelTypeRequesty)
	other := previewChannel(t, "other", "default", config.ChannelStatusEnabled)
	prices := []*ChannelPrice{
		{Model: "claude-fable-5", Input: 5.5, Output: 27.5},
		{Model: "bedrock/claude-fable-5", Input: 5.5, Output: 27.5},
		{Model: "anthropic/claude-fable-5", Input: 5, Output: 25},
		{Model: "anthropic/claude-fable-5@eu", Input: 1, Output: 1},
		{Model: "cheap/claude-fable-5", Input: 4, Output: 30},
	}
	for _, ch := range []*Channel{requesty, other} {
		if err := ReplaceChannelPrices(ch.Id, prices); err != nil {
			t.Fatal(err)
		}
	}
	for _, ch := range []*Channel{requesty, other} {
		s := &ModelSupply{Model: "claude-fable-5", ChannelId: ch.Id, UpstreamModel: "claude-fable-5", Priority: 1, Weight: 1, Enabled: true}
		if err := DB.Create(s).Error; err != nil {
			t.Fatal(err)
		}
	}
	views, err := GetModelSupplyViews("claude-fable-5", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range views {
		switch v.ChannelName {
		case "requesty":
			if v.CheaperUpstream == nil || v.CheaperUpstream.Model != "anthropic/claude-fable-5" || v.CheaperUpstream.Output != 25 {
				t.Errorf("requesty: %+v", v.CheaperUpstream)
			}
		case "other":
			if v.CheaperUpstream != nil {
				t.Errorf("a channel type with one id per model got %+v", v.CheaperUpstream)
			}
		}
	}

	// Already on the cheapest id: nothing to offer.
	DB.Model(&ModelSupply{}).Where("channel_id = ?", requesty.Id).Update("upstream_model", "anthropic/claude-fable-5")
	views, _ = GetModelSupplyViews("claude-fable-5", requesty.Id)
	if len(views) != 1 || views[0].CheaperUpstream != nil {
		t.Errorf("cheapest id still offered another: %+v", views[0].CheaperUpstream)
	}
}
