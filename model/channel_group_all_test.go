package model

import (
	"slices"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// useAllGroupsTestDB is the supply test database plus user groups, with the routing table, the
// user group cache and the supply switch restored afterwards.
func useAllGroupsTestDB(t *testing.T, symbols ...string) {
	t.Helper()
	useSupplyTestDB(t)
	if err := DB.AutoMigrate(&UserGroup{}); err != nil {
		t.Fatal(err)
	}
	oldEnabled := config.ModelSupplyEnabled
	ChannelGroup.RLock()
	oldRule, oldChannels, oldMatch, oldGroups, oldTypes, oldSupplies := ChannelGroup.Rule, ChannelGroup.Channels,
		ChannelGroup.Match, ChannelGroup.ModelGroup, ChannelGroup.ModelChannelType, ChannelGroup.Supplies
	ChannelGroup.RUnlock()
	GlobalUserGroupRatio.RLock()
	oldUserGroups, oldLimiter, oldPublic := GlobalUserGroupRatio.UserGroup, GlobalUserGroupRatio.APILimiter, GlobalUserGroupRatio.PublicGroup
	GlobalUserGroupRatio.RUnlock()
	t.Cleanup(func() {
		config.ModelSupplyEnabled = oldEnabled
		ChannelGroup.Lock()
		ChannelGroup.Rule, ChannelGroup.Channels, ChannelGroup.Match = oldRule, oldChannels, oldMatch
		ChannelGroup.ModelGroup, ChannelGroup.ModelChannelType, ChannelGroup.Supplies = oldGroups, oldTypes, oldSupplies
		ChannelGroup.Unlock()
		GlobalUserGroupRatio.Lock()
		GlobalUserGroupRatio.UserGroup, GlobalUserGroupRatio.APILimiter, GlobalUserGroupRatio.PublicGroup = oldUserGroups, oldLimiter, oldPublic
		GlobalUserGroupRatio.Unlock()
	})
	config.ModelSupplyEnabled = false
	for _, symbol := range symbols {
		if err := DB.Create(&UserGroup{Symbol: symbol, Name: symbol, Ratio: 1}).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func createGroupTestChannel(t *testing.T, name, group, models string) *Channel {
	t.Helper()
	weight := uint(1)
	channel := &Channel{Name: name, Status: config.ChannelStatusEnabled, Group: group, Weight: &weight, Models: models}
	if err := DB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	return channel
}

// routedChannels lists the channel ids the routing table holds for a group and model.
func routedChannels(group, modelName string) []int {
	ChannelGroup.RLock()
	defer ChannelGroup.RUnlock()
	var ids []int
	for _, tier := range ChannelGroup.Rule[group][modelName] {
		ids = append(ids, tier...)
	}
	slices.Sort(ids)
	return ids
}

func TestNormalizeChannelGroup(t *testing.T) {
	cases := map[string]string{
		"":                   "",
		"default":            "default",
		" vip , default,vip": "vip,default",
		"*":                  "*",
		"vip,*":              "*",
		" * ,default":        "*",
	}
	for in, want := range cases {
		if got := NormalizeChannelGroup(in); got != want {
			t.Errorf("NormalizeChannelGroup(%q) = %q, want %q", in, got, want)
		}
	}
}

// A channel open to all groups routes in every enabled group, picks up groups created later
// and drops groups that are disabled or deleted, without the channel being edited.
func TestAllGroupsChannelFollowsUserGroups(t *testing.T) {
	useAllGroupsTestDB(t, "default", "vip")
	all := createGroupTestChannel(t, "all", ChannelGroupAll, "m")
	vipOnly := createGroupTestChannel(t, "vip-only", "vip", "m")

	// At startup the routing table loads before the user group cache: expansion must not
	// depend on it.
	GlobalUserGroupRatio.Lock()
	GlobalUserGroupRatio.UserGroup = nil
	GlobalUserGroupRatio.Unlock()
	ChannelGroup.Load()
	if got := routedChannels("default", "m"); !slices.Equal(got, []int{all.Id}) {
		t.Fatalf("default routes %v, want only the all-groups channel %d", got, all.Id)
	}
	if got := routedChannels("vip", "m"); !slices.Equal(got, []int{all.Id, vipOnly.Id}) {
		t.Fatalf("vip routes %v", got)
	}
	if _, ok := ChannelGroup.Rule[ChannelGroupAll]; ok {
		t.Fatal("the routing table has a literal * group")
	}

	team := &UserGroup{Symbol: "team", Name: "team", Ratio: 1}
	if err := team.Create(); err != nil {
		t.Fatal(err)
	}
	if got := routedChannels("team", "m"); !slices.Equal(got, []int{all.Id}) {
		t.Fatalf("new group routes %v, want the all-groups channel without editing it", got)
	}

	if err := ChangeUserGroupEnable(team.Id, false); err != nil {
		t.Fatal(err)
	}
	if got := routedChannels("team", "m"); len(got) != 0 {
		t.Fatalf("disabled group still routes %v", got)
	}
	if err := ChangeUserGroupEnable(team.Id, true); err != nil {
		t.Fatal(err)
	}
	if got := routedChannels("team", "m"); !slices.Equal(got, []int{all.Id}) {
		t.Fatalf("re-enabled group routes %v", got)
	}

	if err := team.Delete(); err != nil {
		t.Fatal(err)
	}
	if got := routedChannels("team", "m"); len(got) != 0 {
		t.Fatalf("deleted group still routes %v", got)
	}
}

func TestAllGroupsChannelRoutesSupplies(t *testing.T) {
	useAllGroupsTestDB(t, "default", "vip")
	config.ModelSupplyEnabled = true
	channel := createGroupTestChannel(t, "all", ChannelGroupAll, "")
	if err := DB.Create(&ModelInfo{Model: "m", Status: ModelStatusPublished}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&ModelSupply{Model: "m", ChannelId: channel.Id, UpstreamModel: "vendor/m", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	ChannelGroup.Load()
	for _, group := range []string{"default", "vip"} {
		if got := routedChannels(group, "m"); !slices.Equal(got, []int{channel.Id}) {
			t.Fatalf("%s routes %v from supplies", group, got)
		}
	}
	ChannelGroup.RLock()
	groups := ChannelGroup.ModelGroup["m"]
	ChannelGroup.RUnlock()
	if !groups["default"] || !groups["vip"] || groups[ChannelGroupAll] {
		t.Fatalf("model groups = %v", groups)
	}
}

func TestChannelSaveNormalizesGroup(t *testing.T) {
	useAllGroupsTestDB(t, "default", "vip")
	weight := uint(1)
	channel := &Channel{Name: "c", Status: config.ChannelStatusEnabled, Group: "vip, *", Weight: &weight, Models: "m"}
	if err := channel.Insert(); err != nil {
		t.Fatal(err)
	}
	stored, err := GetChannelById(channel.Id)
	if err != nil || stored.Group != ChannelGroupAll {
		t.Fatalf("inserted group = %q, err %v", stored.Group, err)
	}

	batch := []Channel{{Name: "b", Status: config.ChannelStatusEnabled, Group: "default,default,vip", Weight: &weight, Models: "m"}}
	if err := BatchInsertChannels(batch); err != nil {
		t.Fatal(err)
	}
	if stored, _ := GetChannelById(batch[0].Id); stored.Group != "default,vip" {
		t.Fatalf("batch inserted group = %q", stored.Group)
	}

	update := &Channel{Id: channel.Id, Group: "default , vip"}
	if err := update.Update(false); err != nil {
		t.Fatal(err)
	}
	if stored, _ := GetChannelById(channel.Id); stored.Group != "default,vip" {
		t.Fatalf("updated group = %q", stored.Group)
	}
}

func TestBatchAddUserGroupWithAllGroups(t *testing.T) {
	useAllGroupsTestDB(t, "default", "vip")
	all := createGroupTestChannel(t, "all", ChannelGroupAll, "m")
	listed := createGroupTestChannel(t, "listed", "default", "m")

	count, err := BatchAddUserGroupToChannels(&BatchChannelsParams{Value: "vip", Ids: []int{all.Id, listed.Id}})
	if err != nil || count != 1 {
		t.Fatalf("add vip: count %d, err %v", count, err)
	}
	if stored, _ := GetChannelById(all.Id); stored.Group != ChannelGroupAll {
		t.Fatalf("all-groups channel became %q", stored.Group)
	}

	if _, err := BatchAddUserGroupToChannels(&BatchChannelsParams{Value: ChannelGroupAll, Ids: []int{listed.Id}}); err != nil {
		t.Fatal(err)
	}
	if stored, _ := GetChannelById(listed.Id); stored.Group != ChannelGroupAll {
		t.Fatalf("adding * gave %q, want *", stored.Group)
	}
}

func TestChannelListGroupFilterMatchesAllGroups(t *testing.T) {
	useAllGroupsTestDB(t, "default", "vip")
	all := createGroupTestChannel(t, "all", ChannelGroupAll, "m")
	createGroupTestChannel(t, "default-only", "default", "m")
	vip := createGroupTestChannel(t, "vip-only", "vip", "m")

	params := &SearchChannelsParams{}
	params.Group = "vip"
	result, err := GetChannelsList(params)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int
	for _, c := range *result.Data {
		ids = append(ids, c.Id)
	}
	slices.Sort(ids)
	if !slices.Equal(ids, []int{all.Id, vip.Id}) {
		t.Fatalf("vip filter returned %v, want %v", ids, []int{all.Id, vip.Id})
	}
}

func TestChannelGroupLossesExpandsAllGroups(t *testing.T) {
	useAllGroupsTestDB(t)
	for _, g := range []*UserGroup{{Symbol: "cheap", Name: "cheap", Ratio: 0.5}, {Symbol: "full", Name: "full", Ratio: 2}} {
		if err := DB.Create(g).Error; err != nil {
			t.Fatal(err)
		}
	}
	GlobalUserGroupRatio.Load()
	cost := 1.0
	losses := ChannelGroupLosses(&Channel{Group: ChannelGroupAll, CostRatio: &cost})
	if len(losses) != 1 || losses[0].Group != "cheap" {
		t.Fatalf("losses = %+v, want only the cheap group", losses)
	}
}
