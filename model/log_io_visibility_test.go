package model

import (
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// TestCanViewTokenLogIO 覆盖 T50e 三档可见性矩阵(后端权威鉴权源,D2=b):
// 站点管理员全见 / 个人仅本人 / 组织 owner-admin 全见、member 一律拒绝(含自建组织令牌)。
func TestCanViewTokenLogIO(t *testing.T) {
	const viewer = 7
	selfToken := &Token{UserId: viewer, CreatedBy: viewer}
	othersToken := &Token{UserId: 99, CreatedBy: 99}
	memberSelfToken := &Token{UserId: 1000, CreatedBy: viewer} // 组织影子账户名下、由本成员创建
	memberOthersToken := &Token{UserId: 1000, CreatedBy: 42}   // 同组织、由他人创建

	cases := []struct {
		name       string
		viewerRole int
		orgRole    string
		token      *Token
		want       bool
	}{
		{"站点管理员-个人上下文-他人令牌全见", config.RoleAdminUser, "", othersToken, true},
		{"站点管理员-组织上下文-他人令牌全见", config.RoleAdminUser, OrgRoleMember, memberOthersToken, true},
		{"个人-本人令牌可见", config.RoleCommonUser, "", selfToken, true},
		{"个人-他人令牌不可见", config.RoleCommonUser, "", othersToken, false},
		{"组织Owner-组织内全部可见", config.RoleCommonUser, OrgRoleOwner, memberOthersToken, true},
		{"组织Admin-组织内全部可见", config.RoleCommonUser, OrgRoleAdmin, memberOthersToken, true},
		{"组织Member-自建组织令牌一律不可见", config.RoleCommonUser, OrgRoleMember, memberSelfToken, false},
		{"组织Member-他人自建不可见", config.RoleCommonUser, OrgRoleMember, memberOthersToken, false},
		{"nil 令牌-恒不可见", config.RoleAdminUser, OrgRoleOwner, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CanViewTokenLogIO(viewer, tc.viewerRole, tc.orgRole, tc.token)
			if got != tc.want {
				t.Fatalf("CanViewTokenLogIO(viewer=%d, role=%d, orgRole=%q) = %v, 期望 %v",
					viewer, tc.viewerRole, tc.orgRole, got, tc.want)
			}
		})
	}
}

func setupLogIOTestDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&LogDetail{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
}

// TestCleanupOldLogDetails 验证 T50d 的 30 天 TTL 删除边界:
// 超期记录被清理、未超期记录保留(以删除语义而非真实等到 03:00 cron 触发)。
func TestCleanupOldLogDetails(t *testing.T) {
	setupLogIOTestDB(t)
	now := time.Now()
	seed := []struct {
		logId     int
		createdAt int64
	}{
		{1, now.AddDate(0, 0, -LogIORetentionDays-1).Unix()}, // 31 天前:超期
		{2, now.AddDate(0, 0, -LogIORetentionDays+1).Unix()}, // 29 天前:保留
		{3, now.Unix()}, // 当前:保留
	}
	for _, s := range seed {
		d := &LogDetail{LogId: s.logId, TokenId: 1, UserId: 1, RequestBody: "req", ResponseBody: "resp", CreatedAt: s.createdAt}
		if err := DB.Create(d).Error; err != nil {
			t.Fatalf("写入 log_detail(log_id=%d)失败: %v", s.logId, err)
		}
	}

	deleted, err := CleanupOldLogDetails()
	if err != nil {
		t.Fatalf("CleanupOldLogDetails 失败: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("应删除 1 条超期记录,实际删除 %d 条", deleted)
	}

	var remaining []LogDetail
	if err := DB.Order("log_id").Find(&remaining).Error; err != nil {
		t.Fatalf("回读剩余记录失败: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("应保留 2 条未超期记录,实际剩余 %d 条", len(remaining))
	}
	for _, r := range remaining {
		if r.LogId == 1 {
			t.Fatalf("超期记录(log_id=1)未被删除")
		}
	}

	// 幂等:再次清理无可删记录。
	deleted, err = CleanupOldLogDetails()
	if err != nil {
		t.Fatalf("二次 CleanupOldLogDetails 失败: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("二次清理应删除 0 条,实际 %d 条", deleted)
	}
}
