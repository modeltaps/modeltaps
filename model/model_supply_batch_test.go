package model

import (
	"errors"
	"fmt"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func TestSetModelSupplyMode(t *testing.T) {
	usePreviewTestDB(t)
	rows := []*ModelInfo{
		{Model: "a", Status: ModelStatusPublished},
		{Model: "b", Status: ModelStatusPublished},
		{Model: "alias", AliasOf: "a"},
		{Model: "auto/x", Kind: ModelInfoKindRoutingPolicy},
	}
	ids := []int{}
	for _, info := range rows {
		if err := DB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
		ids = append(ids, info.Id)
	}
	affected, err := SetModelSupplyMode(ids, SupplyModeFailover)
	if err != nil || affected != 2 {
		t.Fatalf("affected %d, err %v; want the two public models", affected, err)
	}
	if supplyModeOf("a") != SupplyModeFailover || supplyModeOf("b") != SupplyModeFailover {
		t.Fatal("models do not route with failover after the batch")
	}
	if affected, _ := SetModelSupplyMode(ids[:1], SupplyModeDefault); affected != 1 {
		t.Fatalf("back to default: affected %d", affected)
	}
	if _, err := SetModelSupplyMode(ids, "random"); !errors.Is(err, ErrModelSupplyMode) {
		t.Fatalf("invalid mode: err = %v", err)
	}
}

func TestApplySupplyBatch(t *testing.T) {
	usePreviewTestDB(t)
	for _, info := range []*ModelInfo{
		{Model: "a", Status: ModelStatusPublished},
		{Model: "b", Status: ModelStatusPublished},
		{Model: "c", Status: ModelStatusDraft},
		{Model: "alias", AliasOf: "a"},
	} {
		if err := DB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}
	priority, weight := int64(6), uint(2)
	mapping := `{"b":"deploy-b"}`
	ch := &Channel{Name: "c", Status: config.ChannelStatusEnabled, Group: "default", Priority: &priority, Weight: &weight, ModelMapping: &mapping}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&ModelSupply{Model: "a", ChannelId: ch.Id, UpstreamModel: "a-old", Priority: 1, Weight: 1, Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	summary := func() string {
		var supplies []*ModelSupply
		DB.Where("channel_id = ?", ch.Id).Order("model").Find(&supplies)
		out := ""
		for _, s := range supplies {
			out += fmt.Sprintf("[%s<-%s p%d w%d]", s.Model, s.UpstreamModel, s.Priority, s.Weight)
		}
		return out
	}

	added, err := ApplySupplyBatch(SupplyBatch{Action: "add", ChannelId: ch.Id, Models: []string{"a", "b", "c", "alias", "missing", "b", " "}})
	if err != nil {
		t.Fatal(err)
	}
	if added.Changed != 2 || fmt.Sprint(added.Skipped) != "[a alias missing]" {
		t.Fatalf("add: %+v", added)
	}
	// The existing supply is untouched; new ones take the mapping and the channel's numbers.
	if got, want := summary(), "[a<-a-old p1 w1][b<-deploy-b p6 w2][c<-c p6 w2]"; got != want {
		t.Fatalf("after add\n got %s\nwant %s", got, want)
	}

	removed, err := ApplySupplyBatch(SupplyBatch{Action: "remove", ChannelId: ch.Id, Models: []string{"a", "c", "missing"}})
	if err != nil {
		t.Fatal(err)
	}
	if removed.Changed != 2 || fmt.Sprint(removed.Skipped) != "[missing]" {
		t.Fatalf("remove: %+v", removed)
	}
	if got := summary(); got != "[b<-deploy-b p6 w2]" {
		t.Fatalf("after remove: %s", got)
	}

	if _, err := ApplySupplyBatch(SupplyBatch{Action: "swap", ChannelId: ch.Id, Models: []string{"a"}}); !errors.Is(err, ErrSupplyBatchAction) {
		t.Fatalf("bad action: err = %v", err)
	}
	if _, err := ApplySupplyBatch(SupplyBatch{Action: "add", ChannelId: 9999, Models: []string{"a"}}); !errors.Is(err, ErrSupplyChannelNotFound) {
		t.Fatalf("missing channel: err = %v", err)
	}
}
