package model

import (
	"errors"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func useGroupDeleteTestDB(t *testing.T) *UserGroup {
	t.Helper()
	useSupplyTestDB(t)
	if err := DB.AutoMigrate(&User{}, &Token{}, &UserGroup{}); err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"default", "vip", "svip"} {
		if err := DB.Create(&UserGroup{Symbol: symbol, Name: symbol, Ratio: 1}).Error; err != nil {
			t.Fatal(err)
		}
	}
	var vip UserGroup
	if err := DB.Where("symbol = ?", "vip").First(&vip).Error; err != nil {
		t.Fatal(err)
	}
	users := []*User{
		{Username: "a", Group: "vip"},
		{Username: "b", Group: "vip"},
		{Username: "org-shadow", Group: "vip", Type: config.UserTypeOrgShadow},
		{Username: "c", Group: "default"},
	}
	for i, u := range users {
		u.AffCode = string(rune('a' + i))
		if err := DB.Create(u).Error; err != nil {
			t.Fatal(err)
		}
	}
	tokens := []*Token{
		{Name: "t1", UserId: 1, KeyHash: "h1", Group: "vip"},
		{Name: "t2", UserId: 1, KeyHash: "h2", Group: "default", BackupGroup: "vip"},
		{Name: "t3", UserId: 2, KeyHash: "h3", Group: "vip", BackupGroup: "svip"},
		{Name: "t4", UserId: 3, KeyHash: "h4", Group: "svip", BackupGroup: "vip"},
		{Name: "t5", UserId: 3, KeyHash: "h5", Group: "default"},
	}
	for _, tk := range tokens {
		if err := DB.Create(tk).Error; err != nil {
			t.Fatal(err)
		}
	}
	channels := []*Channel{
		{Name: "only-vip", Group: "vip"},
		{Name: "vip-and-default", Group: "default,vip"},
		{Name: "all", Group: ChannelGroupAll},
		{Name: "vipx", Group: "vipx"}, // a group whose name contains "vip"
	}
	for _, ch := range channels {
		if err := DB.Create(ch).Error; err != nil {
			t.Fatal(err)
		}
	}
	return &vip
}

func TestCountUserGroupReferences(t *testing.T) {
	vip := useGroupDeleteTestDB(t)
	refs, err := CountUserGroupReferences(vip.Symbol)
	if err != nil {
		t.Fatal(err)
	}
	want := UserGroupReferences{Users: 2, ShadowAccounts: 1, Tokens: 2, BackupTokens: 2, Channels: 2}
	if refs != want {
		t.Fatalf("refs = %+v, want %+v", refs, want)
	}
	unused, _ := CountUserGroupReferences("svip")
	if unused.Users != 0 || unused.Channels != 0 || unused.Tokens != 1 {
		t.Fatalf("svip refs = %+v", unused)
	}
}

func TestDeleteUserGroupMovingMovesEveryReference(t *testing.T) {
	vip := useGroupDeleteTestDB(t)
	if err := DeleteUserGroupMoving(vip, "svip"); err != nil {
		t.Fatal(err)
	}
	groupOf := func(name string) string {
		var u User
		if err := DB.Where("username = ?", name).First(&u).Error; err != nil {
			t.Fatal(err)
		}
		return u.Group
	}
	for _, name := range []string{"a", "b", "org-shadow"} {
		if g := groupOf(name); g != "svip" {
			t.Errorf("%s group = %q, want svip", name, g)
		}
	}
	if g := groupOf("c"); g != "default" {
		t.Errorf("an account of another group moved: %q", g)
	}
	token := func(name string) Token {
		var tk Token
		if err := DB.Where("name = ?", name).First(&tk).Error; err != nil {
			t.Fatal(err)
		}
		return tk
	}
	cases := map[string][2]string{
		"t1": {"svip", ""},
		"t2": {"default", "svip"},
		"t3": {"svip", ""}, // its backup was svip: now its own group, so none
		"t4": {"svip", ""},
		"t5": {"default", ""},
	}
	for name, want := range cases {
		if tk := token(name); tk.Group != want[0] || tk.BackupGroup != want[1] {
			t.Errorf("%s = %q / %q, want %q / %q", name, tk.Group, tk.BackupGroup, want[0], want[1])
		}
	}
	channelGroup := func(name string) string {
		var ch Channel
		if err := DB.Where("name = ?", name).First(&ch).Error; err != nil {
			t.Fatal(err)
		}
		return ch.Group
	}
	for name, want := range map[string]string{"only-vip": "svip", "vip-and-default": "default", "all": ChannelGroupAll, "vipx": "vipx"} {
		if g := channelGroup(name); g != want {
			t.Errorf("channel %s group = %q, want %q", name, g, want)
		}
	}
	var n int64
	DB.Model(&UserGroup{}).Where("symbol = ?", "vip").Count(&n)
	if n != 0 {
		t.Error("the group was not deleted")
	}
}

func TestDeleteUserGroupMovingNeedsASuccessorWhenUsed(t *testing.T) {
	vip := useGroupDeleteTestDB(t)
	for _, successor := range []string{"", "vip", "nope"} {
		if err := DeleteUserGroupMoving(vip, successor); !errors.Is(err, ErrUserGroupSuccessor) {
			t.Errorf("successor %q: err = %v", successor, err)
		}
	}
	var n int64
	DB.Model(&UserGroup{}).Where("symbol = ?", "vip").Count(&n)
	if refs, _ := CountUserGroupReferences("vip"); n != 1 || refs.Users != 2 {
		t.Fatalf("a refused delete changed data: groups %d, refs %+v", n, refs)
	}
}

func TestDeleteUnusedUserGroupNeedsNoSuccessor(t *testing.T) {
	useGroupDeleteTestDB(t)
	extra := &UserGroup{Symbol: "trial", Name: "trial", Ratio: 1}
	if err := DB.Create(extra).Error; err != nil {
		t.Fatal(err)
	}
	if err := DeleteUserGroupMoving(extra, ""); err != nil {
		t.Fatal(err)
	}
}
