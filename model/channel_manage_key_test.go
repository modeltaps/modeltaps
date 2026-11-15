package model

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// The management key never leaves the server: HideKey replaces it with its mask.
func TestHideKeyHidesManageKey(t *testing.T) {
	ch := Channel{Key: "sk-secret-value-9876", ManageKey: strPtr("mk-manage-value-4321")}
	ch.HideKey()
	if ch.ManageKey != nil {
		t.Error("HideKey must clear the management key")
	}
	if ch.KeyStatus.ManageMasked != "mk-m****4321" {
		t.Errorf("manage mask = %q", ch.KeyStatus.ManageMasked)
	}
	out, _ := json.Marshal(ch)
	if strings.Contains(string(out), "mk-manage-value-4321") || strings.Contains(string(out), `"manage_key"`) {
		t.Errorf("the response carries the management key: %s", out)
	}

	none := Channel{Key: "sk-secret-value-9876"}
	none.HideKey()
	if none.KeyStatus.ManageMasked != "" {
		t.Errorf("a channel without a management key shows %q", none.KeyStatus.ManageMasked)
	}
}

// A form save without manage_key keeps the stored one; a value replaces it; "" clears it.
func TestUpdateRawManageKey(t *testing.T) {
	setupChannelKeyTestDB(t)
	if err := DB.Create(&Channel{Id: 1, Name: "a", Key: "k", ManageKey: strPtr("manage-1"), Models: "m"}).Error; err != nil {
		t.Fatal(err)
	}
	stored := func() string {
		var got Channel
		DB.First(&got, 1)
		return got.GetManageKey()
	}

	ch := Channel{Id: 1, Name: "renamed", Models: "m"}
	if err := ch.UpdateRaw(true); err != nil {
		t.Fatal(err)
	}
	if got := stored(); got != "manage-1" {
		t.Errorf("a save without manage_key changed it to %q", got)
	}

	ch = Channel{Id: 1, Name: "renamed", ManageKey: strPtr("manage-2"), Models: "m"}
	if err := ch.UpdateRaw(true); err != nil {
		t.Fatal(err)
	}
	if got := stored(); got != "manage-2" {
		t.Errorf("manage key = %q, want manage-2", got)
	}

	ch = Channel{Id: 1, Name: "renamed", ManageKey: strPtr(""), Models: "m"}
	if err := ch.UpdateRaw(true); err != nil {
		t.Fatal(err)
	}
	if got := stored(); got != "" {
		t.Errorf("an empty manage_key must clear it, got %q", got)
	}
}

// A tag edit shares configuration, not credentials: each channel keeps its management key.
func TestUpdateChannelsTagKeepsManageKeys(t *testing.T) {
	seedTagChannels(t)
	if err := DB.Model(&Channel{}).Where("tag = ?", "grp").Update("manage_key", "manage-own").Error; err != nil {
		t.Fatal(err)
	}
	if err := UpdateChannelsTag("grp", &Channel{Tag: "grp", Name: "g", Models: "m2", ManageKey: strPtr("manage-other")}); err != nil {
		t.Fatal(err)
	}
	var got []Channel
	DB.Order("id").Find(&got)
	for _, ch := range got {
		if ch.GetManageKey() != "manage-own" {
			t.Errorf("channel %d management key = %q", ch.Id, ch.GetManageKey())
		}
	}
}

// The one-time migration splits "apiKey|manage..." keys of the types that used that form.
func TestSplitChannelManageKey(t *testing.T) {
	setupChannelKeyTestDB(t)
	for _, ch := range []*Channel{
		{Id: 1, Type: config.ChannelTypeRequesty, Key: "rq-inference | rq-manage"},
		{Id: 2, Type: config.ChannelTypeVolcengine, Key: "ark-key|AKX|SKX"},
		{Id: 3, Type: config.ChannelTypeRequesty, Key: "rq-only"},
		// Other types use "|" inside the key for their own purposes.
		{Id: 4, Type: config.ChannelTypeBaidu, Key: "app|secret"},
	} {
		if err := DB.Create(ch).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := splitChannelManageKey().Migrate(DB); err != nil {
		t.Fatal(err)
	}
	want := map[int][2]string{
		1: {"rq-inference", "rq-manage"},
		2: {"ark-key", "AKX|SKX"},
		3: {"rq-only", ""},
		4: {"app|secret", ""},
	}
	var got []Channel
	DB.Order("id").Find(&got)
	for _, ch := range got {
		if w := want[ch.Id]; ch.Key != w[0] || ch.GetManageKey() != w[1] {
			t.Errorf("channel %d: key = %q manage = %q, want %v", ch.Id, ch.Key, ch.GetManageKey(), w)
		}
	}
}
