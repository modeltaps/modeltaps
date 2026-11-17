package model

import (
	"fmt"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func autoSupplySummary(supplies []*ModelSupply) string {
	out := ""
	for _, s := range supplies {
		out += fmt.Sprintf("[%s<-%s p%d w%d %v %s]", s.Model, s.UpstreamModel, s.Priority, s.Weight, s.Enabled, s.Source)
	}
	return out
}

func TestAutoSupplyChannel(t *testing.T) {
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
	priority, weight := int64(7), uint(3)
	mapping := `{"gpt-9":"+deploy-9"}`
	ch := &Channel{Name: "c", Status: config.ChannelStatusEnabled, Group: "default", Priority: &priority, Weight: &weight, ModelMapping: &mapping}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	// gpt-9-mini is supplied already and must be left as it is.
	if err := DB.Create(&ModelSupply{Model: "gpt-9-mini", ChannelId: ch.Id, UpstreamModel: "mini-old", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}

	names := []string{"opus", "claude-opus-5", "gpt-9", "gpt-9-mini", "auto/coder", "unknown-model", "gpt-*", " ", "claude-opus-5"}
	created, err := AutoSupplyChannel(ch, names)
	if err != nil {
		t.Fatal(err)
	}
	// The name spelled like the model wins over its alias; the mapping gives gpt-9 its upstream
	// name without the "+"; policies, unknown names and wildcards are skipped.
	want := "[claude-opus-5<-claude-opus-5 p7 w3 true channel][gpt-9<-deploy-9 p7 w3 true channel]"
	if got := autoSupplySummary(created); got != want {
		t.Fatalf("created\n got %s\nwant %s", got, want)
	}

	// Running it again adds nothing.
	again, err := AutoSupplyChannel(ch, names)
	if err != nil || len(again) != 0 {
		t.Fatalf("second run created %s, err %v", autoSupplySummary(again), err)
	}
	var count int64
	DB.Model(&ModelSupply{}).Where("channel_id = ?", ch.Id).Count(&count)
	if count != 3 {
		t.Fatalf("channel has %d supplies, want 3", count)
	}

	// Only an alias is listed: the alias is what the channel is sent.
	other := &Channel{Name: "other", Status: config.ChannelStatusEnabled, Group: "default"}
	if err := DB.Create(other).Error; err != nil {
		t.Fatal(err)
	}
	created, err = AutoSupplyChannel(other, []string{"opus"})
	if err != nil {
		t.Fatal(err)
	}
	if got := autoSupplySummary(created); got != "[claude-opus-5<-opus p0 w1 true channel]" {
		t.Fatalf("alias only: %s", got)
	}

	if created, err := AutoSupplyChannel(other, nil); err != nil || created != nil {
		t.Fatalf("no names: %v, %v", created, err)
	}
}

// A new channel that costs more than the quote gets its supply switched off, waiting for review.
func TestAutoSupplyChannelHoldsSuppliesAboveQuote(t *testing.T) {
	usePreviewTestDB(t)
	for _, name := range []string{"cheap-model", "dear-model"} {
		if err := DB.Create(&ModelInfo{Model: name, Status: ModelStatusPublished}).Error; err != nil {
			t.Fatal(err)
		}
	}
	oldPricing := PricingInstance
	PricingInstance = &Pricing{Prices: map[string]*Price{
		"cheap-model": {Model: "cheap-model", Type: TokensPriceType, Input: 1, Output: 2},
		"dear-model":  {Model: "dear-model", Type: TokensPriceType, Input: 1, Output: 2},
	}}
	t.Cleanup(func() { PricingInstance = oldPricing })

	// The channel's cost ratio prices every model at 1.5x the quote; a 0.5 ratio channel costs less.
	dear := &Channel{Name: "dear", Status: config.ChannelStatusEnabled, Group: "default", CostRatio: floatPtr(1.5)}
	cheap := &Channel{Name: "cheap", Status: config.ChannelStatusEnabled, Group: "default", CostRatio: floatPtr(0.5)}
	unknown := &Channel{Name: "unknown", Status: config.ChannelStatusEnabled, Group: "default"}
	for _, ch := range []*Channel{dear, cheap, unknown} {
		if err := DB.Create(ch).Error; err != nil {
			t.Fatal(err)
		}
	}
	held, err := AutoSupplyChannel(dear, []string{"dear-model"})
	if err != nil || len(held) != 1 || held[0].Enabled || !held[0].Pending {
		t.Fatalf("above the quote: %+v, %v; want switched off and pending", held, err)
	}
	for _, ch := range []*Channel{cheap, unknown} {
		got, err := AutoSupplyChannel(ch, []string{"cheap-model"})
		if err != nil || len(got) != 1 || !got[0].Enabled || got[0].Pending {
			t.Fatalf("%s: %+v, %v; want enabled (below the quote, or cost unknown)", ch.Name, got, err)
		}
	}
}
