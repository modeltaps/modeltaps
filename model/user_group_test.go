package model

import (
	"fmt"
	"sync/atomic"
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
	if err := testDB.AutoMigrate(&User{}, &UserGroup{}, &Redemption{}, &Log{}, &Order{}); err != nil {
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

// seedPromotionOrder writes an order of units dollars for the user in the given status.
func seedPromotionOrder(t *testing.T, userId, units int, status OrderStatus) *Order {
	t.Helper()
	order := &Order{
		UserId:  userId,
		TradeNo: fmt.Sprintf("promo-%d-%d-%s-%d", userId, units, status, promotionOrderSeq.Add(1)),
		Amount:  units,
		Quota:   promotionQuota(units),
		Status:  status,
	}
	if err := DB.Create(order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}
	return order
}

var promotionOrderSeq atomic.Int64

// Promotion counts only credited orders: gifted balance and used quota (sign-up and invite
// bonuses, admin grants) never count, nor do orders that were not paid.
func TestCheckAndUpgradeUserGroupCountsPaidOrdersOnly(t *testing.T) {
	cases := []struct {
		name            string
		gifted, used    int   // balance and used quota not bought through orders (dollars)
		paid            []int // credited orders (dollars)
		unpaid          []int // pending, failed and closed orders (dollars)
		wantGroupSymbol string
	}{
		{"gifted quota alone never promotes", 1000, 0, nil, nil, "default"},
		{"used gifted quota alone never promotes", 0, 1000, nil, nil, "default"},
		{"unpaid orders do not count", 0, 0, nil, []int{100, 100}, "default"},
		{"a paid order of 100 lands in silver", 500, 500, []int{100}, nil, "silver"},
		{"paid below every min keeps the group", 900, 0, []int{40}, []int{200}, "default"},
		{"paid orders add up", 0, 0, []int{60, 90}, nil, "gold"},
		{"just under max stays in silver", 0, 0, []int{149}, []int{1}, "silver"},
		{"max is exclusive", 0, 0, []int{150}, nil, "gold"},
		{"max 0 has no upper bound", 0, 0, []int{1000}, nil, "gold"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupPromotionTestDB(t)
			seedPromotionGroups(t)
			const userId = 7
			seedPromotionUser(t, userId, promotionQuota(tc.gifted), promotionQuota(tc.used))
			for _, units := range tc.paid {
				seedPromotionOrder(t, userId, units, OrderStatusSuccess)
			}
			for i, units := range tc.unpaid {
				status := []OrderStatus{OrderStatusPending, OrderStatusFailed, OrderStatusClosed}[i%3]
				seedPromotionOrder(t, userId, units, status)
			}

			if err := CheckAndUpgradeUserGroup(userId); err != nil {
				t.Fatalf("check promotion: %v", err)
			}
			if got := userGroupSymbol(t, userId); got != tc.wantGroupSymbol {
				t.Fatalf("group = %q, want %q", got, tc.wantGroupSymbol)
			}
		})
	}
}

// A credited order counts once toward the promotion: paying 100 lands in silver, not gold.
func TestCreditPendingOrderPromotesOnPaidQuotaOnce(t *testing.T) {
	setupPromotionTestDB(t)
	seedPromotionGroups(t)
	const userId = 9
	seedPromotionUser(t, userId, 0, 0)

	for _, step := range []struct {
		units           int
		wantGroupSymbol string
	}{
		{100, "silver"},
		{50, "gold"},
	} {
		order := seedPromotionOrder(t, userId, step.units, OrderStatusPending)
		credited, err := CreditPendingOrder(order, "gw-"+order.TradeNo)
		if err != nil || !credited {
			t.Fatalf("credit order of %d: credited=%v err=%v", step.units, credited, err)
		}
		if err := CheckAndUpgradeUserGroup(userId); err != nil {
			t.Fatalf("check promotion: %v", err)
		}
		if got := userGroupSymbol(t, userId); got != step.wantGroupSymbol {
			t.Fatalf("after paying %d, group = %q, want %q", step.units, got, step.wantGroupSymbol)
		}
	}
}

// Redemption codes are not paid top-ups: redeeming any amount leaves the group as it is.
func TestRedeemDoesNotPromote(t *testing.T) {
	setupPromotionTestDB(t)
	seedPromotionGroups(t)
	const userId = 8
	seedPromotionUser(t, userId, 0, 0)

	code := &Redemption{Key: "PROMOTIONCODE0000000000000000500", Status: config.RedemptionCodeStatusEnabled, Quota: promotionQuota(500)}
	if err := DB.Create(code).Error; err != nil {
		t.Fatalf("create redemption code: %v", err)
	}
	if _, err := Redeem(code.Key, userId, "127.0.0.1"); err != nil {
		t.Fatalf("redeem: %v", err)
	}
	if got := userGroupSymbol(t, userId); got != "default" {
		t.Fatalf("after redeeming 500, group = %q, want default", got)
	}
}
