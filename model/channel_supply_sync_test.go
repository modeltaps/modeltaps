package model

import (
	"sort"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func channelSupplyState(t *testing.T, channelId int) string {
	t.Helper()
	var supplies []*ModelSupply
	if err := DB.Where("channel_id = ?", channelId).Find(&supplies).Error; err != nil {
		t.Fatal(err)
	}
	var parts []string
	for _, s := range supplies {
		parts = append(parts, s.Model+"<-"+s.UpstreamModel+"("+s.Source+")")
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

func TestSyncChannelSupplies(t *testing.T) {
	usePreviewTestDB(t)
	for _, info := range []*ModelInfo{
		{Model: "m-a", Status: ModelStatusPublished},
		{Model: "m-b", Status: ModelStatusPublished},
		{Model: "m-c", Status: ModelStatusPublished},
		{Model: "m-d", Status: ModelStatusPublished},
		{Model: "m-e", Status: ModelStatusPublished},
		{Model: "e-alias", AliasOf: "m-e"},
		{Model: "m-new", Status: ModelStatusPublished},
	} {
		if err := DB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}
	mapping := `{"m-a":"deploy-a","m-b":"deploy-b"}`
	before := Channel{Name: "c", Status: config.ChannelStatusEnabled, Group: "default",
		Models: "m-a,m-b,m-c,m-d,m-e,e-alias", ModelMapping: &mapping}
	if err := DB.Create(&before).Error; err != nil {
		t.Fatal(err)
	}
	for _, s := range []*ModelSupply{
		{Model: "m-a", UpstreamModel: "deploy-a", Source: ModelSupplySourceChannel},  // follows the list
		{Model: "m-b", UpstreamModel: "deploy-b", Source: ModelSupplySourceManual},   // added by hand
		{Model: "m-c", UpstreamModel: "m-c", Source: ModelSupplySourceMigrated},      // follows the list
		{Model: "m-d", UpstreamModel: "renamed-d", Source: ModelSupplySourceChannel}, // renamed on the model page
		{Model: "m-e", UpstreamModel: "m-e", Source: ModelSupplySourceChannel},       // e-alias stays listed
	} {
		s.ChannelId, s.Enabled = before.Id, true
		if err := DB.Create(s).Error; err != nil {
			t.Fatal(err)
		}
	}

	// The mapping now sends m-a and m-b elsewhere; m-c, m-d and m-e leave the list; m-new joins.
	newMapping := `{"m-a":"deploy-a2","m-b":"deploy-b2"}`
	after := before
	after.Models, after.ModelMapping = "m-a,m-b,e-alias,m-new", &newMapping
	got, err := SyncChannelSupplies(&before, &after)
	if err != nil {
		t.Fatal(err)
	}
	if got != (ChannelSupplySync{Attached: 1, Updated: 1, Removed: 1}) {
		t.Fatalf("sync = %+v, want 1 attached, 1 updated, 1 removed", got)
	}
	// m-a follows the mapping, m-b (manual) and m-d (renamed) stay, m-c is removed, m-e stays
	// because e-alias still belongs to it, m-new is attached.
	want := "m-a<-deploy-a2(channel) m-b<-deploy-b(manual) m-d<-renamed-d(channel) m-e<-m-e(channel) m-new<-m-new(channel)"
	if state := channelSupplyState(t, before.Id); state != want {
		t.Fatalf("supplies\n got %s\nwant %s", state, want)
	}

	// Adding m-c back attaches it again.
	again := after
	again.Models = after.Models + ",m-c"
	got, err = SyncChannelSupplies(&after, &again)
	if err != nil || got != (ChannelSupplySync{Attached: 1}) {
		t.Fatalf("re-add = %+v, %v; want 1 attached", got, err)
	}

	// Saving without a change does nothing.
	if got, err := SyncChannelSupplies(&again, &again); err != nil || got != (ChannelSupplySync{}) {
		t.Fatalf("no change = %+v, %v", got, err)
	}
}
