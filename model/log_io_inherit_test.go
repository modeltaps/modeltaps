package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupLogIOInheritTestDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&User{}, &Token{}, &Organization{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
}

func boolPtr(b bool) *bool { return &b }

// seedLogIOUser 创建一个指定类型与个人默认的用户,返回其 ID。
func seedLogIOUser(t *testing.T, typ int, def *bool) int {
	t.Helper()
	u := &User{
		Username: utils.GetRandomString(10),
		AffCode:  utils.GetRandomString(8),
		Type:     typ,
	}
	u.Setting.Set(UserSetting{LogIODefault: def})
	if err := DB.Create(u).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	return u.Id
}

// seedLogIOOrg 创建组织影子账户 + 组织(留存策略 policy),返回影子账户 ID。
func seedLogIOOrg(t *testing.T, policy string) int {
	t.Helper()
	shadowId := seedLogIOUser(t, config.UserTypeOrgShadow, nil)
	slug := "org-" + utils.GetRandomString(8)
	org := &Organization{Name: slug, Slug: slug, ShadowUserId: shadowId, CreatedBy: 1, Status: OrganizationStatusEnabled}
	org.Setting.Set(OrganizationSetting{LogIOPolicy: policy})
	if err := DB.Create(org).Error; err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}
	return shadowId
}

// TestResolveTokenLogIO 表驱动覆盖判定链(ORG-10):站点闸门、个人账户开关、组织策略(全部 / 不留 / 选中的 key)。
func TestResolveTokenLogIO(t *testing.T) {
	setupLogIOInheritTestDB(t)

	userOn := seedLogIOUser(t, config.UserTypeNormal, boolPtr(true))
	userOff := seedLogIOUser(t, config.UserTypeNormal, nil)
	orgUnset := seedLogIOOrg(t, "")
	orgOn := seedLogIOOrg(t, LogIOPolicyOn)
	orgOff := seedLogIOOrg(t, LogIOPolicyOff)
	orgSel := seedLogIOOrg(t, LogIOPolicySelected)
	orgKey := func(shadow int, on *bool) *Token { return &Token{UserId: shadow, CreatedBy: 42, LogIO: on} }

	cases := []struct {
		name    string
		enabled bool
		token   *Token
		want    bool
	}{
		{"站点闸门关-恒不留存", false, &Token{UserId: userOn}, false},
		{"个人-账户开", true, &Token{UserId: userOn}, true},
		{"个人-账户关(key 上的值不读)", true, &Token{UserId: userOff, LogIO: boolPtr(true)}, false},
		{"组织未设置=不留存", true, orgKey(orgUnset, boolPtr(true)), false},
		{"组织全部留存", true, orgKey(orgOn, boolPtr(false)), true},
		{"组织不留存", true, orgKey(orgOff, boolPtr(true)), false},
		{"组织选中的 key", true, orgKey(orgSel, boolPtr(true)), true},
		{"组织未选中的 key", true, orgKey(orgSel, boolPtr(false)), false},
		{"nil 令牌恒不留存", true, nil, false},
	}

	oldEnabled := config.LogIOEnabled
	t.Cleanup(func() { config.LogIOEnabled = oldEnabled })
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config.LogIOEnabled = tc.enabled
			if got := ResolveTokenLogIO(tc.token); got != tc.want {
				t.Fatalf("ResolveTokenLogIO = %v, 期望 %v", got, tc.want)
			}
		})
	}
}

// migrateLogIOToOrgSelection: personal keys' column is cleared (the account switch decides); an
// organization where each key decided keeps the keys that were on as its selection.
func TestMigrateLogIOToOrgSelection(t *testing.T) {
	setupLogIOInheritTestDB(t)
	user := seedLogIOUser(t, config.UserTypeNormal, boolPtr(true))
	withKeys := seedLogIOOrg(t, legacyLogIOPolicyMember)
	noKeys := seedLogIOOrg(t, legacyLogIOPolicyMember)
	locked := seedLogIOOrg(t, LogIOPolicyOn)
	key := func(name string, userId, createdBy int, on bool) {
		tk := &Token{UserId: userId, CreatedBy: createdBy, Name: name, KeyHash: utils.GetRandomString(48), LogIO: boolPtr(on)}
		if err := DB.Create(tk).Error; err != nil {
			t.Fatal(err)
		}
	}
	key("personal", user, 0, true)
	key("sel-on", withKeys, user, true)
	key("sel-off", withKeys, user, false)
	key("none", noKeys, user, false)

	if err := migrateLogIOToOrgSelection().Migrate(DB); err != nil {
		t.Fatal(err)
	}
	var personal Token
	DB.Where("name = ?", "personal").First(&personal)
	if personal.LogIO == nil || *personal.LogIO {
		t.Fatalf("personal key column = %v, want cleared", personal.LogIO)
	}
	for shadow, want := range map[int]string{withKeys: LogIOPolicySelected, noKeys: LogIOPolicyOff, locked: LogIOPolicyOn} {
		var org Organization
		if err := DB.Where("shadow_user_id = ?", shadow).First(&org).Error; err != nil {
			t.Fatal(err)
		}
		if got := org.Setting.Data().LogIOPolicy; got != want {
			t.Errorf("organization %d: policy %q, want %q", org.Id, got, want)
		}
	}
	config.LogIOEnabled = true
	t.Cleanup(func() { config.LogIOEnabled = false })
	var selOn Token
	DB.Where("name = ?", "sel-on").First(&selOn)
	if !ResolveTokenLogIO(&selOn) {
		t.Fatal("the key that was on is no longer kept")
	}
}

// SetOrgLogIOSelection selects exactly the given keys of the organization and reports the changes.
func TestSetOrgLogIOSelection(t *testing.T) {
	setupLogIOInheritTestDB(t)
	user := seedLogIOUser(t, config.UserTypeNormal, nil)
	shadow := seedLogIOOrg(t, LogIOPolicySelected)
	var org Organization
	DB.Where("shadow_user_id = ?", shadow).First(&org)
	var ids []int
	for i, on := range []bool{true, false, false} {
		tk := &Token{UserId: shadow, CreatedBy: user, Name: utils.GetRandomString(6), KeyHash: utils.GetRandomString(48), LogIO: boolPtr(on)}
		if err := DB.Create(tk).Error; err != nil {
			t.Fatal(i, err)
		}
		ids = append(ids, tk.Id)
	}
	personal := &Token{UserId: user, Name: "p", KeyHash: utils.GetRandomString(48)}
	DB.Create(personal)

	changed, err := SetOrgLogIOSelection(&org, []int{ids[1], personal.Id})
	if err != nil || len(changed) != 2 {
		t.Fatalf("changed %d keys, err %v; want 2 (first off, second on)", len(changed), err)
	}
	var got Token
	DB.First(&got, personal.Id)
	if got.LogIO != nil && *got.LogIO {
		t.Fatal("a key of another owner was selected")
	}
}

// migrateLogIOToKeys gives every key the value that applied to it and turns the organization
// switch into a policy (ORG-10).
func TestMigrateLogIOToKeys(t *testing.T) {
	setupLogIOInheritTestDB(t)
	userOn := seedLogIOUser(t, config.UserTypeNormal, boolPtr(true))
	userOff := seedLogIOUser(t, config.UserTypeNormal, nil)
	seedOrg := func(legacy *bool) (*Organization, int) {
		shadow := seedLogIOOrg(t, "")
		var org Organization
		if err := DB.Where("shadow_user_id = ?", shadow).First(&org).Error; err != nil {
			t.Fatal(err)
		}
		org.Setting.Set(OrganizationSetting{LegacyLogIODefault: legacy})
		if err := DB.Model(&org).Update("setting", org.Setting).Error; err != nil {
			t.Fatal(err)
		}
		return &org, shadow
	}
	orgOn, shadowOn := seedOrg(boolPtr(true))
	orgOff, shadowOff := seedOrg(boolPtr(false))
	orgNil, shadowNil := seedOrg(nil)

	key := func(name string, userId, createdBy int, logIO string) {
		if err := DB.Exec("INSERT INTO tokens (user_id, created_by, name, key_hash, log_io) VALUES (?,?,?,?,"+logIO+")",
			userId, createdBy, name, utils.GetRandomString(48)).Error; err != nil {
			t.Fatal(err)
		}
	}
	key("personal-on", userOn, 0, "NULL")
	key("personal-legacy", userOff, 0, "1") // the column was ignored: overwritten with the user's value
	key("org-on", shadowOn, userOn, "0")
	key("org-off", shadowOff, userOn, "1")
	key("org-nil", shadowNil, userOn, "1")

	if err := migrateLogIOToKeys().Migrate(DB); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"personal-on": true, "personal-legacy": false, "org-on": true, "org-off": false, "org-nil": false}
	for name, w := range want {
		var tk Token
		if err := DB.Where("name = ?", name).First(&tk).Error; err != nil {
			t.Fatal(err)
		}
		if tk.LogIO == nil || *tk.LogIO != w {
			t.Errorf("%s: log_io = %v, want %v", name, tk.LogIO, w)
		}
	}
	for org, policy := range map[*Organization]string{orgOn: LogIOPolicyOn, orgOff: LogIOPolicyOff, orgNil: legacyLogIOPolicyMember} {
		var got Organization
		if err := DB.First(&got, org.Id).Error; err != nil {
			t.Fatal(err)
		}
		if got.Setting.Data().LogIOPolicy != policy || got.Setting.Data().LegacyLogIODefault != nil {
			t.Errorf("organization %d: setting %+v, want policy %s and the old switch cleared", org.Id, got.Setting.Data(), policy)
		}
	}
}

// foldLogIOSiteDefaults 把开着的站点默认写到未设置的用户 / 组织上,已有选择的不动,随后删除两个站点选项。
func TestFoldLogIOSiteDefaults(t *testing.T) {
	setupLogIOInheritTestDB(t)
	if err := DB.AutoMigrate(&Option{}); err != nil {
		t.Fatal(err)
	}
	userNil := seedLogIOUser(t, config.UserTypeNormal, nil)
	userOff := seedLogIOUser(t, config.UserTypeNormal, boolPtr(false))
	orgNil := seedLogIOOrg(t, "")
	DB.Create(&Option{Key: "LogIODefaultUser", Value: "true"})
	DB.Create(&Option{Key: "OrganizationLogIODefault", Value: "false"})

	if err := foldLogIOSiteDefaults().Migrate(DB); err != nil {
		t.Fatal(err)
	}
	userDefault := func(id int) *bool {
		var u User
		if err := DB.Select("setting").First(&u, id).Error; err != nil {
			t.Fatal(err)
		}
		return u.Setting.Data().LogIODefault
	}
	if v := userDefault(userNil); v == nil || !*v {
		t.Fatalf("user without a choice = %v, want on (the site default)", v)
	}
	if v := userDefault(userOff); v == nil || *v {
		t.Fatalf("user who chose off = %v, want off", v)
	}
	if v := userDefault(orgNil); v != nil {
		t.Fatalf("shadow account = %v, want untouched", v)
	}
	var org Organization
	if err := DB.Where("shadow_user_id = ?", orgNil).First(&org).Error; err != nil {
		t.Fatal(err)
	}
	if org.Setting.Data().LegacyLogIODefault != nil {
		t.Fatalf("organization = %v, want untouched (its site default was off)", org.Setting.Data().LegacyLogIODefault)
	}
	var left int64
	DB.Model(&Option{}).Where("key IN ?", []string{"LogIODefaultUser", "OrganizationLogIODefault"}).Count(&left)
	if left != 0 {
		t.Fatalf("%d site default options left", left)
	}
}

// TestUserSettingLogIOSentinel 覆盖重置哨兵:ApplyUpdate 把 "inherit"|"on"|"off" 映射为 *bool(nil/true/false),
// 可把已配置项重置回 nil;APIView 回显同样的三态字符串;非法取值报错。
func TestUserSettingLogIOSentinel(t *testing.T) {
	s := &UserSetting{LogIODefault: boolPtr(true)}

	// inherit 重置回 nil
	reset := "inherit"
	if err := s.ApplyUpdate(UserSettingUpdate{LogIODefault: &reset}); err != nil {
		t.Fatalf("ApplyUpdate(inherit) 报错: %v", err)
	}
	if s.LogIODefault != nil {
		t.Fatalf("inherit 未重置为 nil: def=%v", s.LogIODefault)
	}

	// on 映射
	on := "on"
	if err := s.ApplyUpdate(UserSettingUpdate{LogIODefault: &on}); err != nil {
		t.Fatalf("ApplyUpdate(on) 报错: %v", err)
	}
	if s.LogIODefault == nil || !*s.LogIODefault {
		t.Fatalf("on 映射错误: def=%v", s.LogIODefault)
	}

	// APIView 回显三态字符串
	view := s.APIView()
	if view["log_io_default"] != "on" {
		t.Fatalf("APIView 回显错误: %v", view)
	}

	// 非法取值报错
	bad := "yes"
	if err := s.ApplyUpdate(UserSettingUpdate{LogIODefault: &bad}); err == nil {
		t.Fatalf("非法取值应报错")
	}
}
