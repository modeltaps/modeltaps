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

// seedLogIOOrg 创建组织影子账户 + 组织(组织默认 def),返回影子账户 ID。
func seedLogIOOrg(t *testing.T, def *bool) int {
	t.Helper()
	shadowId := seedLogIOUser(t, config.UserTypeOrgShadow, nil)
	slug := "org-" + utils.GetRandomString(8)
	org := &Organization{Name: slug, Slug: slug, ShadowUserId: shadowId, CreatedBy: 1, Status: OrganizationStatusEnabled}
	org.Setting.Set(OrganizationSetting{LogIODefault: def})
	if err := DB.Create(org).Error; err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}
	return shadowId
}

// TestResolveTokenLogIO 表驱动覆盖判定链:站点闸门、个人/组织上的开关(未设置为关)、令牌显式值被忽略。
func TestResolveTokenLogIO(t *testing.T) {
	setupLogIOInheritTestDB(t)

	userNil := seedLogIOUser(t, config.UserTypeNormal, nil)
	userOn := seedLogIOUser(t, config.UserTypeNormal, boolPtr(true))
	userOff := seedLogIOUser(t, config.UserTypeNormal, boolPtr(false))
	orgNil := seedLogIOOrg(t, nil)
	orgOn := seedLogIOOrg(t, boolPtr(true))
	orgOff := seedLogIOOrg(t, boolPtr(false))

	cases := []struct {
		name    string
		enabled bool
		token   *Token
		want    bool
	}{
		{"站点闸门关-恒不留存", false, &Token{UserId: userOn, LogIO: boolPtr(true)}, false},
		{"令牌显式开被忽略-取用户关", true, &Token{UserId: userOff, LogIO: boolPtr(true)}, false},
		{"令牌显式关被忽略-取用户开", true, &Token{UserId: userOn, LogIO: boolPtr(false)}, true},
		{"个人-用户开", true, &Token{UserId: userOn}, true},
		{"个人-用户关", true, &Token{UserId: userOff}, false},
		{"个人-用户未设置-不留存", true, &Token{UserId: userNil}, false},
		{"组织-组织开", true, &Token{UserId: orgOn}, true},
		{"组织-组织关", true, &Token{UserId: orgOff}, false},
		{"组织-组织未设置-不留存", true, &Token{UserId: orgNil}, false},
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

// TestTokenLogIOColumnPreservedButIgnored 验证两层简化后:存量 log_io 列仍保留(回读为 *bool),
// 但解析恒忽略令牌显式值,只看归属用户上的开关。以原始 SQL 写入模拟既有布尔列数据。
func TestTokenLogIOColumnPreservedButIgnored(t *testing.T) {
	setupLogIOInheritTestDB(t)
	uidOff := seedLogIOUser(t, config.UserTypeNormal, nil)
	uidOn := seedLogIOUser(t, config.UserTypeNormal, boolPtr(true))

	insert := func(uid int, name, value string) {
		if err := DB.Exec("INSERT INTO tokens (user_id, name, key_hash, log_io) VALUES (?,?,?,"+value+")", uid, name, utils.GetRandomString(48)).Error; err != nil {
			t.Fatalf("写入存量令牌 %s 失败: %v", name, err)
		}
	}
	insert(uidOff, "legacy-on", "1")
	insert(uidOn, "legacy-off", "0")
	insert(uidOff, "inherit", "NULL")

	read := func(name string) *Token {
		var tk Token
		if err := DB.Where("name = ?", name).First(&tk).Error; err != nil {
			t.Fatalf("回读令牌 %s 失败: %v", name, err)
		}
		return &tk
	}
	on, off, inherit := read("legacy-on"), read("legacy-off"), read("inherit")
	// 列保留:存量布尔仍如实回读,不被迁移抹除。
	if on.LogIO == nil || !*on.LogIO || off.LogIO == nil || *off.LogIO || inherit.LogIO != nil {
		t.Fatalf("存量列值未保留: on=%v off=%v inherit=%v", on.LogIO, off.LogIO, inherit.LogIO)
	}

	oldEnabled := config.LogIOEnabled
	t.Cleanup(func() { config.LogIOEnabled = oldEnabled })
	config.LogIOEnabled = true
	if ResolveTokenLogIO(on) || ResolveTokenLogIO(inherit) {
		t.Fatalf("用户未设置时不留存(令牌显式开被忽略)")
	}
	if !ResolveTokenLogIO(off) {
		t.Fatalf("用户开启时留存(令牌显式关被忽略)")
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
	orgNil := seedLogIOOrg(t, nil)
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
	if org.Setting.Data().LogIODefault != nil {
		t.Fatalf("organization = %v, want untouched (its site default was off)", org.Setting.Data().LogIODefault)
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
