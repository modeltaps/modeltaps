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
		Username:    utils.GetRandomString(10),
		AccessToken: utils.GetRandomString(32),
		AffCode:     utils.GetRandomString(8),
		Type:        typ,
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

// TestResolveTokenLogIO 表驱动覆盖两层判定链:站点闸门、个人/组织逐级回退、组织不受个人默认影响、令牌显式值被忽略。
func TestResolveTokenLogIO(t *testing.T) {
	setupLogIOInheritTestDB(t)

	userNil := seedLogIOUser(t, config.UserTypeNormal, nil)
	userOn := seedLogIOUser(t, config.UserTypeNormal, boolPtr(true))
	userOff := seedLogIOUser(t, config.UserTypeNormal, boolPtr(false))
	orgNil := seedLogIOOrg(t, nil)
	orgOn := seedLogIOOrg(t, boolPtr(true))
	orgOff := seedLogIOOrg(t, boolPtr(false))

	cases := []struct {
		name        string
		enabled     bool
		siteUserDef bool
		siteOrgDef  bool
		token       *Token
		want        bool
	}{
		{"站点闸门关-恒不留存", false, true, true, &Token{UserId: userNil, LogIO: boolPtr(true)}, false},
		{"令牌显式开被忽略-取用户默认关", true, false, false, &Token{UserId: userOff, LogIO: boolPtr(true)}, false},
		{"令牌显式关被忽略-取用户默认开", true, true, true, &Token{UserId: userOn, LogIO: boolPtr(false)}, true},
		{"个人-用户默认开", true, false, false, &Token{UserId: userOn, LogIO: nil}, true},
		{"个人-用户默认关", true, true, true, &Token{UserId: userOff, LogIO: nil}, false},
		{"个人-用户未配-站点开", true, true, false, &Token{UserId: userNil, LogIO: nil}, true},
		{"个人-用户未配-站点关", true, false, false, &Token{UserId: userNil, LogIO: nil}, false},
		{"组织-组织默认开", true, false, false, &Token{UserId: orgOn, LogIO: nil}, true},
		{"组织-组织默认关", true, true, true, &Token{UserId: orgOff, LogIO: nil}, false},
		{"组织-组织未配-站点开", true, false, true, &Token{UserId: orgNil, LogIO: nil}, true},
		{"组织-组织未配-站点关", true, true, false, &Token{UserId: orgNil, LogIO: nil}, false},
		{"组织不受成员个人默认影响", true, true, false, &Token{UserId: orgNil, LogIO: nil}, false},
		{"nil 令牌恒不留存", true, true, true, nil, false},
	}

	oldEnabled, oldUser, oldOrg := config.LogIOEnabled, config.LogIODefaultUser, config.OrganizationLogIODefault
	t.Cleanup(func() {
		config.LogIOEnabled, config.LogIODefaultUser, config.OrganizationLogIODefault = oldEnabled, oldUser, oldOrg
	})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config.LogIOEnabled = tc.enabled
			config.LogIODefaultUser = tc.siteUserDef
			config.OrganizationLogIODefault = tc.siteOrgDef
			if got := ResolveTokenLogIO(tc.token); got != tc.want {
				t.Fatalf("ResolveTokenLogIO = %v, 期望 %v", got, tc.want)
			}
		})
	}
}

// TestTokenLogIOColumnPreservedButIgnored 验证两层简化后:存量 log_io 列仍保留(回读为 *bool),
// 但解析恒忽略令牌显式值,只按站点/用户默认回退。以原始 SQL 写入模拟既有布尔列数据。
func TestTokenLogIOColumnPreservedButIgnored(t *testing.T) {
	setupLogIOInheritTestDB(t)
	uid := seedLogIOUser(t, config.UserTypeNormal, nil) // 个人默认未配,回退站点

	if err := DB.Exec("INSERT INTO tokens (user_id, name, `key`, log_io) VALUES (?,?,?,1)", uid, "legacy-on", utils.GetRandomString(48)).Error; err != nil {
		t.Fatalf("写入存量强制开令牌失败: %v", err)
	}
	if err := DB.Exec("INSERT INTO tokens (user_id, name, `key`, log_io) VALUES (?,?,?,0)", uid, "legacy-off", utils.GetRandomString(48)).Error; err != nil {
		t.Fatalf("写入存量强制关令牌失败: %v", err)
	}
	if err := DB.Exec("INSERT INTO tokens (user_id, name, `key`, log_io) VALUES (?,?,?,NULL)", uid, "inherit", utils.GetRandomString(48)).Error; err != nil {
		t.Fatalf("写入继承令牌失败: %v", err)
	}

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

	oldEnabled, oldUser := config.LogIOEnabled, config.LogIODefaultUser
	t.Cleanup(func() {
		config.LogIOEnabled, config.LogIODefaultUser = oldEnabled, oldUser
	})
	config.LogIOEnabled = true

	// 两层解析忽略令牌显式值:三者均随站点/用户默认。
	config.LogIODefaultUser = false
	if ResolveTokenLogIO(on) || ResolveTokenLogIO(off) || ResolveTokenLogIO(inherit) {
		t.Fatalf("站点默认关时三者都应不留存(令牌显式值被忽略)")
	}
	config.LogIODefaultUser = true
	if !ResolveTokenLogIO(on) || !ResolveTokenLogIO(off) || !ResolveTokenLogIO(inherit) {
		t.Fatalf("站点默认开时三者都应留存(令牌显式值被忽略)")
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
