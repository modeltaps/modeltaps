package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestMaskChannelKey(t *testing.T) {
	cases := []struct {
		name string
		key  string
		want ChannelKeyStatus
	}{
		{"empty", "", ChannelKeyStatus{Configured: false, Count: 0, Masked: []string{}}},
		{"blank lines only", " \n\n", ChannelKeyStatus{Configured: false, Count: 0, Masked: []string{}}},
		{"single", "sk-test-abcd1234", ChannelKeyStatus{Configured: true, Count: 1, Masked: []string{"····1234"}}},
		{"multiline", "sk-first-key-a1b2\n\n  sk-second-key-c3d4  \n", ChannelKeyStatus{Configured: true, Count: 2, Masked: []string{"····a1b2", "····c3d4"}}},
		{"shorter than 4", "abc", ChannelKeyStatus{Configured: true, Count: 1, Masked: []string{"····"}}},
		{"short reveals nothing", "abcdefg", ChannelKeyStatus{Configured: true, Count: 1, Masked: []string{"····"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MaskChannelKey(tc.key)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("MaskChannelKey() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestMaskChannelKeyCapsEntries(t *testing.T) {
	lines := make([]string, 15)
	for i := range lines {
		lines[i] = "sk-some-long-key-value"
	}
	got := MaskChannelKey(strings.Join(lines, "\n"))
	if got.Count != 15 || len(got.Masked) != keyMaskMaxEntries {
		t.Errorf("count=%d masked=%d, want 15/%d", got.Count, len(got.Masked), keyMaskMaxEntries)
	}
}

func TestHideKeyClearsRawKey(t *testing.T) {
	ch := Channel{Key: "sk-secret-value-9876"}
	ch.HideKey()
	if ch.Key != "" {
		t.Error("HideKey must clear the raw key")
	}
	if ch.KeyStatus == nil || !ch.KeyStatus.Configured || ch.KeyStatus.Masked[0] != "····9876" {
		t.Errorf("unexpected key status %+v", ch.KeyStatus)
	}
}

func setupChannelKeyTestDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	if err := testDB.AutoMigrate(&Channel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
}

// 编辑时 key 留空（overwrite 模式）必须保留原 key，填了新值才覆盖。
func TestUpdateRawKeepsKeyWhenBlank(t *testing.T) {
	setupChannelKeyTestDB(t)
	if err := DB.Create(&Channel{Id: 1, Name: "a", Key: "original-key-0001", Models: "m"}).Error; err != nil {
		t.Fatal(err)
	}

	ch := Channel{Id: 1, Name: "renamed", Key: "", Models: "m"}
	if err := ch.UpdateRaw(true); err != nil {
		t.Fatal(err)
	}
	var got Channel
	DB.First(&got, 1)
	if got.Key != "original-key-0001" || got.Name != "renamed" {
		t.Errorf("blank key edit: name=%q keyKept=%v", got.Name, got.Key == "original-key-0001")
	}

	ch = Channel{Id: 1, Name: "renamed", Key: "replaced-key-0002", Models: "m"}
	if err := ch.UpdateRaw(true); err != nil {
		t.Fatal(err)
	}
	DB.First(&got, 1)
	if got.Key != "replaced-key-0002" {
		t.Error("non-blank key edit should replace the key")
	}
}

func seedTagChannels(t *testing.T) {
	t.Helper()
	setupChannelKeyTestDB(t)
	for _, ch := range []Channel{
		{Id: 1, Name: "g_0", Tag: "grp", Key: "tag-secret-key-aaaa1111", Models: "m", Status: 1},
		{Id: 2, Name: "g_1", Tag: "grp", Key: "tag-secret-key-bbbb2222", Models: "m", Status: 1},
	} {
		if err := DB.Create(&ch).Error; err != nil {
			t.Fatal(err)
		}
	}
}

// 标签子表格接口不得返回原始 key，只带 key_status。
func TestGetChannelsTagListHidesKey(t *testing.T) {
	seedTagChannels(t)
	result, err := GetChannelsTagList(&SearchChannelsTagParams{Tag: "grp", PaginationParams: PaginationParams{Page: 1, Size: 10}})
	if err != nil {
		t.Fatal(err)
	}
	channels := result.Data
	if channels == nil || len(*channels) != 2 {
		t.Fatal("expected 2 tag channels")
	}
	for _, ch := range *channels {
		if ch.Key != "" {
			t.Errorf("channel %d leaked raw key", ch.Id)
		}
		if ch.KeyStatus == nil || !ch.KeyStatus.Configured || ch.KeyStatus.Count != 1 {
			t.Errorf("channel %d unexpected key status %+v", ch.Id, ch.KeyStatus)
		}
	}
}

// 标签详情：HideKey 后 JSON 里既没有原始 key，也没有按 key 算出的 KeyMap。
func TestGetChannelsTagHidesKey(t *testing.T) {
	seedTagChannels(t)
	tag, err := GetChannelsTag("grp")
	if err != nil {
		t.Fatal(err)
	}
	tag.HideKey()
	raw, err := json.Marshal(tag)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if strings.Contains(body, "tag-secret-key") || strings.Contains(body, "KeyMap") {
		t.Error("tag detail response leaked key material")
	}
	if tag.KeyStatus == nil || tag.KeyStatus.Count != 2 {
		t.Errorf("unexpected key status %+v", tag.KeyStatus)
	}
}

// 批量更新标签时 key 留空：不增删渠道，各渠道已存 key 保持不变。
func TestUpdateChannelsTagKeepsKeysWhenBlank(t *testing.T) {
	seedTagChannels(t)
	if err := UpdateChannelsTag("grp", &Channel{Tag: "grp", Name: "g", Models: "m2"}); err != nil {
		t.Fatal(err)
	}
	var got []Channel
	DB.Order("id").Find(&got)
	if len(got) != 2 {
		t.Fatalf("blank key must not add/delete channels, got %d", len(got))
	}
	want := []string{"tag-secret-key-aaaa1111", "tag-secret-key-bbbb2222"}
	for i, ch := range got {
		if ch.Key != want[i] {
			t.Errorf("channel %d key changed", ch.Id)
		}
		if ch.Models != "m2" {
			t.Errorf("channel %d shared config not updated", ch.Id)
		}
	}
}
