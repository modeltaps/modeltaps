package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupPromotionTestDB 迁移用户、分组、兑换码与日志表。:memory: 每个连接都是独立的库，
// 限制为单连接，事务内外的读写才会落在同一个库。
func setupPromotionTestDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	sqlDB, err := testDB.DB()
	if err != nil {
		t.Fatalf("获取底层 DB 失败: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := testDB.AutoMigrate(&User{}, &UserGroup{}, &Redemption{}, &Log{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
}

// promotionQuota 把分组晋级条件的单位（美元）折算成额度。
func promotionQuota(units int) int {
	return units * int(config.QuotaPerUnit)
}

// seedPromotionGroups 写入两个自动晋级分组：silver 累计充值 [50, 150)，gold 150 起不设上限。
func seedPromotionGroups(t *testing.T) {
	t.Helper()
	enable := true
	for _, g := range []*UserGroup{
		{Symbol: "silver", Name: "Silver", Ratio: 1, Promotion: true, Min: 50, Max: 150, Enable: &enable},
		{Symbol: "gold", Name: "Gold", Ratio: 1, Promotion: true, Min: 150, Max: 0, Enable: &enable},
	} {
		if err := DB.Create(g).Error; err != nil {
			t.Fatalf("创建分组 %s 失败: %v", g.Symbol, err)
		}
	}
}

func seedPromotionUser(t *testing.T, id, quota, usedQuota int) {
	t.Helper()
	user := &User{Id: id, Username: "promotion", Password: "placeholder", Quota: quota, UsedQuota: usedQuota, Group: "default"}
	if err := DB.Create(user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
}

func userGroupSymbol(t *testing.T, userId int) string {
	t.Helper()
	group, err := GetUserGroup(userId)
	if err != nil {
		t.Fatalf("读取用户分组失败: %v", err)
	}
	return group
}

// 充值先同步入账（批量更新关闭时 IncreaseUserQuota 直接写库），再判定晋级：累计充值 = 入账后的
// quota + used_quota，本次充值只算一次。重复计入时 100 会被当成 200 直接进 gold。
func TestCheckAndUpgradeUserGroupCountsCreditedRechargeOnce(t *testing.T) {
	if config.BatchUpdateEnabled {
		t.Fatal("本用例需要 IncreaseUserQuota 同步写库")
	}
	cases := []struct {
		name            string
		quota, used     int // 充值前的余额与已用额度（单位：美元）
		recharge        int
		wantGroupSymbol string
	}{
		{"first recharge of 100 lands in silver", 0, 0, 100, "silver"},
		{"recharge below every min keeps the group", 0, 0, 40, "default"},
		{"used quota counts toward the total", 0, 60, 90, "gold"},
		{"just under max stays in silver", 10, 39, 100, "silver"},
		{"max is exclusive", 20, 30, 100, "gold"},
		{"max 0 has no upper bound", 0, 900, 100, "gold"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupPromotionTestDB(t)
			seedPromotionGroups(t)
			const userId = 7
			seedPromotionUser(t, userId, promotionQuota(tc.quota), promotionQuota(tc.used))

			if err := IncreaseUserQuota(userId, promotionQuota(tc.recharge)); err != nil {
				t.Fatalf("入账失败: %v", err)
			}
			if err := CheckAndUpgradeUserGroup(userId); err != nil {
				t.Fatalf("判定晋级失败: %v", err)
			}
			if got := userGroupSymbol(t, userId); got != tc.wantGroupSymbol {
				t.Fatalf("累计充值 %d 后分组应为 %q，got %q",
					tc.quota+tc.used+tc.recharge, tc.wantGroupSymbol, got)
			}
		})
	}
}

// 兑换码在同一事务里加额度并提交后才判定晋级：第一次兑换 100 进 silver 而不是 gold，
// 再兑换 50 累计 150 才进 gold。
func TestRedeemPromotesOnRedeemedQuotaOnce(t *testing.T) {
	setupPromotionTestDB(t)
	seedPromotionGroups(t)
	const userId = 8
	seedPromotionUser(t, userId, 0, 0)

	for _, step := range []struct {
		key             string
		units           int
		wantGroupSymbol string
	}{
		{"PROMOTIONCODE0000000000000000100", 100, "silver"},
		{"PROMOTIONCODE0000000000000000050", 50, "gold"},
	} {
		code := &Redemption{Key: step.key, Status: config.RedemptionCodeStatusEnabled, Quota: promotionQuota(step.units)}
		if err := DB.Create(code).Error; err != nil {
			t.Fatalf("创建兑换码失败: %v", err)
		}
		if _, err := Redeem(step.key, userId, "127.0.0.1"); err != nil {
			t.Fatalf("兑换 %d 失败: %v", step.units, err)
		}
		if got := userGroupSymbol(t, userId); got != step.wantGroupSymbol {
			t.Fatalf("兑换 %d 后分组应为 %q，got %q", step.units, step.wantGroupSymbol, got)
		}
	}
}
