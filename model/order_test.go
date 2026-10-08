package model

import (
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupOrderTestDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&Order{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
}

func newTestOrder() *Order {
	return &Order{
		UserId:        42,
		GatewayId:     7,
		TradeNo:       "CAP-TRADE-1",
		GatewayNo:     "GW-0001",
		Amount:        10,
		OrderAmount:   12.34,
		OrderCurrency: CurrencyTypeUSD,
		Quota:         5000000,
		Fee:           0.5,
		Discount:      0.25,
		Status:        OrderStatusPending,
		CreatedAt:     1755302400,
	}
}

// Insert → GetOrderByTradeNo / GetUserOrder 字段 round-trip。
func TestOrderInsertRoundTrip(t *testing.T) {
	setupOrderTestDB(t)
	want := newTestOrder()
	if err := want.Insert(); err != nil {
		t.Fatalf("插入订单失败: %v", err)
	}

	assertOrderEqual := func(got *Order) {
		t.Helper()
		if got.UserId != want.UserId || got.GatewayId != want.GatewayId {
			t.Fatalf("UserId/GatewayId 不一致: got %d/%d", got.UserId, got.GatewayId)
		}
		if got.TradeNo != want.TradeNo || got.GatewayNo != want.GatewayNo {
			t.Fatalf("TradeNo/GatewayNo 不一致: got %q/%q", got.TradeNo, got.GatewayNo)
		}
		if got.Amount != want.Amount || got.Quota != want.Quota {
			t.Fatalf("Amount/Quota 不一致: got %d/%d", got.Amount, got.Quota)
		}
		if got.OrderAmount != want.OrderAmount || got.Fee != want.Fee || got.Discount != want.Discount {
			t.Fatalf("金额字段不一致: got %v/%v/%v", got.OrderAmount, got.Fee, got.Discount)
		}
		if got.OrderCurrency != CurrencyTypeUSD {
			t.Fatalf("OrderCurrency 不一致: got %q", got.OrderCurrency)
		}
		if got.Status != OrderStatusPending {
			t.Fatalf("Status 不一致: got %q", got.Status)
		}
		if got.CreatedAt != want.CreatedAt {
			t.Fatalf("CreatedAt 不一致: got %d", got.CreatedAt)
		}
	}

	byTradeNo, err := GetOrderByTradeNo("CAP-TRADE-1")
	if err != nil {
		t.Fatalf("GetOrderByTradeNo 失败: %v", err)
	}
	assertOrderEqual(byTradeNo)

	byUser, err := GetUserOrder(42, "CAP-TRADE-1")
	if err != nil {
		t.Fatalf("GetUserOrder 失败: %v", err)
	}
	assertOrderEqual(byUser)
}

// Update 后状态与网关单号保持一致。
func TestOrderUpdateStatus(t *testing.T) {
	setupOrderTestDB(t)
	order := newTestOrder()
	if err := order.Insert(); err != nil {
		t.Fatalf("插入订单失败: %v", err)
	}

	order.Status = OrderStatusSuccess
	order.GatewayNo = "GW-PAID-9"
	if err := order.Update(); err != nil {
		t.Fatalf("更新订单失败: %v", err)
	}

	got, err := GetOrderByTradeNo(order.TradeNo)
	if err != nil {
		t.Fatalf("回读订单失败: %v", err)
	}
	if got.Status != OrderStatusSuccess {
		t.Fatalf("更新后 Status 应为 success，got %q", got.Status)
	}
	if got.GatewayNo != "GW-PAID-9" {
		t.Fatalf("更新后 GatewayNo 应为 GW-PAID-9，got %q", got.GatewayNo)
	}
}

// 不存在的订单查询 → 错误语义锁定为 gorm.ErrRecordNotFound。
func TestOrderNotFoundSemantics(t *testing.T) {
	setupOrderTestDB(t)
	order := newTestOrder()
	if err := order.Insert(); err != nil {
		t.Fatalf("插入订单失败: %v", err)
	}

	if _, err := GetOrderByTradeNo("NO-SUCH-TRADE"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetOrderByTradeNo 未命中应为 ErrRecordNotFound，实际: %v", err)
	}
	// 单号存在但归属其他用户 → 同样必须 NotFound（防越权读取他人订单）。
	if _, err := GetUserOrder(99999, "CAP-TRADE-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetUserOrder 跨用户应为 ErrRecordNotFound，实际: %v", err)
	}
}

// setupOrderCreditTestDB 迁移订单与用户表。:memory: 每个连接都是独立的库，限制为单连接，
// 事务内外的读写才会落在同一个库。
func setupOrderCreditTestDB(t *testing.T) {
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
	if err := testDB.AutoMigrate(&Order{}, &User{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
}

// seedPendingCreditOrder 写入一个用户与一笔属于他的 pending 订单（尚无网关单号）。
func seedPendingCreditOrder(t *testing.T, initialQuota int) *Order {
	t.Helper()
	order := newTestOrder()
	order.GatewayNo = ""
	user := &User{Id: order.UserId, Username: "credit", Password: "placeholder", Quota: initialQuota, Group: "default"}
	if err := DB.Create(user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	if err := order.Insert(); err != nil {
		t.Fatalf("插入订单失败: %v", err)
	}
	return order
}

// failUserQuotaUpdates 让之后所有改 users.quota 的语句在库里报错（模拟加额度时 DB 出错），
// 返回的函数撤掉故障。
func failUserQuotaUpdates(t *testing.T) (heal func()) {
	t.Helper()
	if err := DB.Exec(`CREATE TRIGGER fail_user_quota_update BEFORE UPDATE OF quota ON users
		BEGIN SELECT RAISE(ABORT, 'injected quota update failure'); END`).Error; err != nil {
		t.Fatalf("注入加额度故障失败: %v", err)
	}
	return func() {
		if err := DB.Exec(`DROP TRIGGER fail_user_quota_update`).Error; err != nil {
			t.Fatalf("撤掉加额度故障失败: %v", err)
		}
	}
}

func assertCreditState(t *testing.T, tradeNo string, userId int, wantStatus OrderStatus, wantGatewayNo string, wantQuota int) {
	t.Helper()
	got, err := GetOrderByTradeNo(tradeNo)
	if err != nil {
		t.Fatalf("回读订单失败: %v", err)
	}
	if got.Status != wantStatus || got.GatewayNo != wantGatewayNo {
		t.Fatalf("订单应为 %q / 网关单号 %q，got %q / %q", wantStatus, wantGatewayNo, got.Status, got.GatewayNo)
	}
	quota, err := GetUserQuota(userId)
	if err != nil {
		t.Fatalf("读取用户额度失败: %v", err)
	}
	if quota != wantQuota {
		t.Fatalf("用户额度应为 %d，got %d", wantQuota, quota)
	}
}

// pending 订单入账：订单置 success 并记下网关单号，用户额度增加订单额度。
// 再拿同一份仍显示 pending 的旧快照入账必须被条件更新挡掉——相当于另一个进程
// 在本进程入账前读到了 pending，进程内的订单锁拦不住它。
func TestCreditPendingOrderCreditsOnce(t *testing.T) {
	setupOrderCreditTestDB(t)
	order := seedPendingCreditOrder(t, 1000)
	stale := *order

	credited, err := CreditPendingOrder(order, "GW-PAID-1")
	if err != nil || !credited {
		t.Fatalf("pending 订单应入账成功, credited=%v err=%v", credited, err)
	}
	if order.Status != OrderStatusSuccess || order.GatewayNo != "GW-PAID-1" {
		t.Fatalf("入账后内存中的订单应同步为 success，got %q / %q", order.Status, order.GatewayNo)
	}
	assertCreditState(t, order.TradeNo, order.UserId, OrderStatusSuccess, "GW-PAID-1", 1000+order.Quota)

	credited, err = CreditPendingOrder(&stale, "GW-PAID-2")
	if err != nil || credited {
		t.Fatalf("已入账订单不得再次入账, credited=%v err=%v", credited, err)
	}
	assertCreditState(t, order.TradeNo, order.UserId, OrderStatusSuccess, "GW-PAID-1", 1000+order.Quota)
}

// 加额度失败时整笔回滚：订单仍是 pending、没有网关单号、额度不变，网关重发时还能入账；
// 故障消失后重试恰好入账一次。
func TestCreditPendingOrderRollsBackWhenQuotaUpdateFails(t *testing.T) {
	setupOrderCreditTestDB(t)
	order := seedPendingCreditOrder(t, 1000)
	heal := failUserQuotaUpdates(t)

	credited, err := CreditPendingOrder(order, "GW-PAID-1")
	if err == nil || credited {
		t.Fatalf("加额度失败必须返回错误, credited=%v err=%v", credited, err)
	}
	if order.Status != OrderStatusPending {
		t.Fatalf("入账失败时内存中的订单应保持 pending，got %q", order.Status)
	}
	assertCreditState(t, order.TradeNo, order.UserId, OrderStatusPending, "", 1000)

	heal()
	credited, err = CreditPendingOrder(order, "GW-PAID-1")
	if err != nil || !credited {
		t.Fatalf("故障消失后应入账成功, credited=%v err=%v", credited, err)
	}
	assertCreditState(t, order.TradeNo, order.UserId, OrderStatusSuccess, "GW-PAID-1", 1000+order.Quota)
}
