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
