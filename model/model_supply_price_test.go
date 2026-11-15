package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// Each supply row carries its channel's upstream price for the name it sends upstream, and says
// whether the channel has a price list at all.
func TestSupplyViewsCarryUpstreamPrices(t *testing.T) {
	usePreviewTestDB(t)
	priced := &Channel{Name: "gateway", Status: config.ChannelStatusEnabled, Group: "default"}
	plain := &Channel{Name: "direct", Status: config.ChannelStatusEnabled, Group: "default"}
	for _, c := range []*Channel{priced, plain} {
		if err := DB.Create(c).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := ReplaceChannelPrices(priced.Id, []*ChannelPrice{
		{Model: "vendor/m", Input: 3, Output: 15},
		{Model: "m-free"},
	}); err != nil {
		t.Fatal(err)
	}
	for _, s := range []*ModelSupply{
		{Model: "m", ChannelId: priced.Id, UpstreamModel: "vendor/m", Priority: 1, Weight: 1, Enabled: true},
		{Model: "m", ChannelId: plain.Id, UpstreamModel: "m", Priority: 1, Weight: 1, Enabled: true},
		{Model: "m-free", ChannelId: priced.Id, UpstreamModel: "", Priority: 1, Weight: 1, Enabled: true},
		{Model: "m-gone", ChannelId: priced.Id, UpstreamModel: "vendor/gone", Priority: 1, Weight: 1, Enabled: true},
	} {
		if err := DB.Create(s).Error; err != nil {
			t.Fatal(err)
		}
	}

	views, err := GetModelSupplyViews("", 0)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]*ModelSupplyView{}
	for _, v := range views {
		got[v.Model+"@"+v.ChannelName] = v
	}
	if v := got["m@gateway"]; v.UpstreamPrice == nil || v.UpstreamPrice.Input != 3 || v.UpstreamPrice.Output != 15 || !v.UpstreamPriced {
		t.Fatalf("gateway supply = %+v", v)
	}
	if v := got["m@direct"]; v.UpstreamPrice != nil || v.UpstreamPriced {
		t.Fatalf("channel without a price list = %+v", v)
	}
	if v := got["m-free@gateway"]; v.UpstreamPrice == nil || v.UpstreamPrice.Input != 0 {
		t.Fatalf("an empty upstream name should look up the model name: %+v", v)
	}
	if v := got["m-gone@gateway"]; v.UpstreamPrice != nil || !v.UpstreamPriced {
		t.Fatalf("a model missing from the list = %+v", v)
	}
}
