package model

import (
	"slices"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/datatypes"
)

func disabledList(names ...string) *datatypes.JSONSlice[string] {
	list := datatypes.JSONSlice[string](names)
	return &list
}

func TestDisabledModelsSkippedByChannelRouting(t *testing.T) {
	useAllGroupsTestDB(t, "default")
	config.ModelSupplyEnabled = false
	channel := createGroupTestChannel(t, "c", "default", "a,b")
	channel.DisabledModels = disabledList("b")
	if err := channel.Update(true); err != nil {
		t.Fatal(err)
	}
	if got := routedChannels("default", "a"); !slices.Equal(got, []int{channel.Id}) {
		t.Fatalf("a routes %v", got)
	}
	if got := routedChannels("default", "b"); len(got) != 0 {
		t.Fatalf("disabled b still routes %v", got)
	}

	channel.DisabledModels = disabledList()
	if err := channel.Update(true); err != nil {
		t.Fatal(err)
	}
	if got := routedChannels("default", "b"); !slices.Equal(got, []int{channel.Id}) {
		t.Fatalf("re-enabled b routes %v", got)
	}
}

func TestDisabledModelsSkippedBySupplyRouting(t *testing.T) {
	useAllGroupsTestDB(t, "default")
	config.ModelSupplyEnabled = true
	mapping := `{"m":"vendor/m"}`
	channel := createGroupTestChannel(t, "c", "default", "m,n")
	channel.ModelMapping = &mapping
	channel.DisabledModels = disabledList("m")
	if err := DB.Save(channel).Error; err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"m", "n"} {
		if err := DB.Create(&ModelInfo{Model: name, Status: ModelStatusPublished}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := DB.Create(&ModelSupply{Model: "m", ChannelId: channel.Id, UpstreamModel: "vendor/m", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&ModelSupply{Model: "n", ChannelId: channel.Id, UpstreamModel: "n", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	ChannelGroup.Load()
	if got := routedChannels("default", "m"); len(got) != 0 {
		t.Fatalf("supply of disabled m still routes %v", got)
	}
	if got := routedChannels("default", "n"); !slices.Equal(got, []int{channel.Id}) {
		t.Fatalf("n routes %v", got)
	}
}

func TestDisabledUpstreamsKeepSharedUpstream(t *testing.T) {
	mapping := `{"a":"up","b":"up"}`
	channel := &Channel{Models: "a,b,c", ModelMapping: &mapping, DisabledModels: disabledList("a", "c", "gone")}
	got := channel.DisabledUpstreams()
	if got["up"] || !got["c"] || got["gone"] || len(got) != 1 {
		t.Fatalf("disabled upstreams = %v", got)
	}
}

func TestUpdateKeepsOrPrunesDisabledModels(t *testing.T) {
	useAllGroupsTestDB(t, "default")
	channel := createGroupTestChannel(t, "c", "default", "a,b")
	channel.DisabledModels = disabledList("a", "b")
	if err := channel.Update(true); err != nil {
		t.Fatal(err)
	}

	// A save that does not send the list keeps it.
	reloaded, _ := GetChannelById(channel.Id)
	reloaded.DisabledModels = nil
	if err := reloaded.Update(true); err != nil {
		t.Fatal(err)
	}
	if reloaded.DisabledModels == nil || !slices.Equal([]string(*reloaded.DisabledModels), []string{"a", "b"}) {
		t.Fatalf("kept list = %v", reloaded.DisabledModels)
	}

	// Removing a model from the list drops it from the disabled ones.
	reloaded.Models = "a"
	if err := reloaded.Update(true); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal([]string(*reloaded.DisabledModels), []string{"a"}) {
		t.Fatalf("pruned list = %v", *reloaded.DisabledModels)
	}
}
