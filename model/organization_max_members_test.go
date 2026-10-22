package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// ORG-12:组织成员上限 NULL=跟随站点默认,0=不限制,正数=上限。
func TestOrganizationGetMaxMembers(t *testing.T) {
	old := config.OrganizationDefaultMaxMembers
	config.OrganizationDefaultMaxMembers = 10
	t.Cleanup(func() { config.OrganizationDefaultMaxMembers = old })

	cases := []struct {
		name string
		max  *int
		want int
	}{
		{"null follows site default", nil, 10},
		{"zero is unlimited", utils.GetPointer(0), 0},
		{"positive is the limit", utils.GetPointer(3), 3},
	}
	for _, tc := range cases {
		org := &Organization{MaxMembers: tc.max}
		if got := org.GetMaxMembers(); got != tc.want {
			t.Fatalf("%s: GetMaxMembers() = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestSetOrganizationMaxMembers(t *testing.T) {
	setupOrgTestDB(t)
	owner := createOrgTestUser(t, "maxm-owner")
	org, err := CreateOrganizationWithOwner("MaxM Org", "", owner.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}
	if org.MaxMembers != nil {
		t.Fatalf("新建组织应跟随站点默认(NULL), got %v", *org.MaxMembers)
	}

	reload := func() *Organization {
		got, err := GetOrganizationById(org.Id)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if reload().MaxMembers != nil {
		t.Fatal("库内新建组织的 max_members 应为 NULL")
	}

	if err := SetOrganizationMaxMembers(org, utils.GetPointer(0)); err != nil {
		t.Fatal(err)
	}
	if got := reload(); got.MaxMembers == nil || *got.MaxMembers != 0 {
		t.Fatalf("设为不限制后应存 0, got %v", got.MaxMembers)
	}

	if err := SetOrganizationMaxMembers(org, utils.GetPointer(5)); err != nil {
		t.Fatal(err)
	}
	if got := reload(); got.MaxMembers == nil || *got.MaxMembers != 5 {
		t.Fatalf("设为 5 后应存 5, got %v", got.MaxMembers)
	}

	if err := SetOrganizationMaxMembers(org, nil); err != nil {
		t.Fatal(err)
	}
	if reload().MaxMembers != nil {
		t.Fatal("恢复跟随站点默认后应存 NULL")
	}

	if err := SetOrganizationMaxMembers(org, utils.GetPointer(-1)); err == nil {
		t.Fatal("负数上限应被拒绝")
	}
}

// 组织单独设为不限制时,即使站点默认上限已满也能继续加人(此前 0 只能表示「跟随站点默认」)。
func TestOrganizationUnlimitedOverridesSiteDefault(t *testing.T) {
	setupOrgTestDB(t)
	old := config.OrganizationDefaultMaxMembers
	config.OrganizationDefaultMaxMembers = 1 // 仅 Owner
	t.Cleanup(func() { config.OrganizationDefaultMaxMembers = old })

	owner := createOrgTestUser(t, "unl-owner")
	bob := createOrgTestUser(t, "unl-bob")
	org, err := CreateOrganizationWithOwner("Unl Org", "", owner.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}
	inv := func(token string) *OrganizationInvitation {
		i := &OrganizationInvitation{OrganizationId: org.Id, InviterId: owner.Id, InviteeId: bob.Id, Role: OrgRoleMember, Token: token, MaxUses: 1}
		if err := i.Insert(); err != nil {
			t.Fatalf("创建邀请失败: %v", err)
		}
		return i
	}

	inv("tok-unl-1")
	if _, err := AcceptOrganizationInvitation("tok-unl-1", bob.Id); err == nil {
		t.Fatal("跟随站点默认(上限 1)时接受邀请应被拒绝")
	}

	if err := SetOrganizationMaxMembers(org, utils.GetPointer(0)); err != nil {
		t.Fatal(err)
	}
	inv("tok-unl-2")
	if _, err := AcceptOrganizationInvitation("tok-unl-2", bob.Id); err != nil {
		t.Fatalf("组织设为不限制后接受邀请应成功: %v", err)
	}
}

// 迁移:旧库 max_members 为 DEFAULT 0 的整型列,0 改写为 NULL(行为不变),正数保留;
// 迁移后 GORM 新建组织写入 NULL 而不是旧列默认值 0。
func TestMigrateOrgMaxMembersNullable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE organizations (
		id integer PRIMARY KEY AUTOINCREMENT,
		name varchar(100) NOT NULL,
		slug varchar(64) NOT NULL,
		avatar_url varchar(500) DEFAULT '',
		status int DEFAULT 1,
		shadow_user_id int NOT NULL,
		created_by int NOT NULL,
		max_members int DEFAULT 0,
		created_time bigint,
		updated_time bigint,
		deleted_at datetime,
		setting json,
		budget_used bigint NOT NULL DEFAULT 0,
		budget_start bigint NOT NULL DEFAULT 0
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO organizations (name, slug, shadow_user_id, created_by, max_members) VALUES
		('a', 'a', 1, 1, 0), ('b', 'b', 2, 1, 5)`).Error; err != nil {
		t.Fatal(err)
	}

	if err := migrateOrgMaxMembersNullable().Migrate(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	read := func(slug string) *int {
		var org Organization
		if err := db.Where("slug = ?", slug).First(&org).Error; err != nil {
			t.Fatal(err)
		}
		return org.MaxMembers
	}
	if read("a") != nil {
		t.Fatal("旧值 0(跟随站点默认)应迁移为 NULL")
	}
	if got := read("b"); got == nil || *got != 5 {
		t.Fatalf("正数上限应保留, got %v", got)
	}

	created := &Organization{Name: "c", Slug: "c", ShadowUserId: 3, CreatedBy: 1}
	if err := db.Create(created).Error; err != nil {
		t.Fatal(err)
	}
	if read("c") != nil {
		t.Fatal("迁移后新建组织应写入 NULL,而不是旧列默认值 0")
	}

	if err := migrateOrgMaxMembersNullable().Rollback(db); err != nil {
		t.Fatal(err)
	}
	if got := read("a"); got == nil || *got != 0 {
		t.Fatalf("回滚后 NULL 应还原为 0, got %v", got)
	}
}
