package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// ORG-11:「成员可见全员用量」是组织上的普通开关,未设置按关闭处理,不再回退站点默认值。
func TestOrganizationIsUsageVisibleToMembers(t *testing.T) {
	cases := []struct {
		name    string
		visible *bool
		want    bool
	}{
		{"unset is off", nil, false},
		{"explicit on", utils.GetPointer(true), true},
		{"explicit off", utils.GetPointer(false), false},
	}
	for _, tc := range cases {
		org := &Organization{}
		org.Setting.Set(OrganizationSetting{UsageVisibleToMembers: tc.visible})
		if got := org.IsUsageVisibleToMembers(); got != tc.want {
			t.Fatalf("%s: IsUsageVisibleToMembers() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// 新建组织默认成员只看自己。
func TestNewOrganizationUsageHiddenFromMembers(t *testing.T) {
	setupOrgTestDB(t)
	owner := createOrgTestUser(t, "uv-owner")
	org, err := CreateOrganizationWithOwner("UV Org", "", owner.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}
	if org.IsUsageVisibleToMembers() {
		t.Fatal("新建组织默认不应让成员看到全员用量")
	}
}

func setupUsageVisibleMigrationDB(t *testing.T, optionValue string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Organization{}, &Option{}); err != nil {
		t.Fatal(err)
	}
	if optionValue != "" {
		if err := db.Create(&Option{Key: orgUsageVisibleDefaultOptionKey, Value: optionValue}).Error; err != nil {
			t.Fatal(err)
		}
	}
	insert := func(slug string, visible *bool) {
		org := &Organization{Name: slug, Slug: slug, ShadowUserId: len(slug) + int(slug[0]), CreatedBy: 1}
		org.Setting.Set(OrganizationSetting{UsageVisibleToMembers: visible, ModelWhitelist: []string{"keep-*"}})
		if err := db.Create(org).Error; err != nil {
			t.Fatal(err)
		}
	}
	insert("unset", nil)
	insert("on", utils.GetPointer(true))
	insert("off", utils.GetPointer(false))
	return db
}

func readUsageVisible(t *testing.T, db *gorm.DB, slug string) (*bool, []string) {
	t.Helper()
	var org Organization
	if err := db.Unscoped().Where("slug = ?", slug).First(&org).Error; err != nil {
		t.Fatal(err)
	}
	s := org.Setting.Data()
	return s.UsageVisibleToMembers, s.ModelWhitelist
}

// 迁移:未设置的存量组织写入迁移时的站点值(行为不变),已显式设置的保持不变,其余设置不受影响,
// 站点选项随后删除。
func TestMigrateOrgUsageVisibleExplicit(t *testing.T) {
	for _, tc := range []struct {
		name        string
		optionValue string
		wantUnset   bool
	}{
		{"site default saved as false", "false", false},
		{"site default saved as true", "true", true},
		{"site default never saved uses built-in true", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupUsageVisibleMigrationDB(t, tc.optionValue)
			if err := migrateOrgUsageVisibleExplicit().Migrate(db); err != nil {
				t.Fatalf("迁移失败: %v", err)
			}

			got, whitelist := readUsageVisible(t, db, "unset")
			if got == nil || *got != tc.wantUnset {
				t.Fatalf("未设置的组织应写入站点值 %v, got %v", tc.wantUnset, got)
			}
			if len(whitelist) != 1 || whitelist[0] != "keep-*" {
				t.Fatalf("其余组织设置不应被改动, got %v", whitelist)
			}
			if got, _ := readUsageVisible(t, db, "on"); got == nil || !*got {
				t.Fatal("显式开启的组织应保持开启")
			}
			if got, _ := readUsageVisible(t, db, "off"); got == nil || *got {
				t.Fatal("显式关闭的组织应保持关闭")
			}

			var count int64
			db.Model(&Option{}).Where(&Option{Key: orgUsageVisibleDefaultOptionKey}).Count(&count)
			if count != 0 {
				t.Fatal("站点选项应在迁移后删除")
			}
		})
	}
}
