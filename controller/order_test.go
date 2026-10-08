package controller

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/payment/gateway/epay"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// epayTestKey 是网关配置里的签名密钥，回调 query 用同一把 key 签名，Verify 才会通过。
const epayTestKey = "sec7-test-key-abcdef"

// setupEpayCallbackTestDB 打开内存 sqlite、迁移相关表并替换 model.DB，测试结束后还原。
// 迁移 UserGroup 是因为 CheckAndUpgradeUserGroup 会查询 promotion 组；不 seed 任何组即为 no-op。
// epayTestDBSeq 为每个测试生成独立的 shared-cache DSN，避免用例间共享同一内存库导致数据泄漏。
var epayTestDBSeq atomic.Int64

func setupEpayCallbackTestDB(t *testing.T) {
	t.Helper()
	// shared-cache 让并发 goroutine 共享同一内存库；不同 DSN 名保证用例间隔离。
	dsn := fmt.Sprintf("file:epay_test_%d?mode=memory&cache=shared", epayTestDBSeq.Add(1))
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	// 并发测试共享同一连接池会触发 sqlite "database is locked"；限制为单连接串行化写入。
	sqlDB, err := testDB.DB()
	if err != nil {
		t.Fatalf("获取底层 DB 失败: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	if err := testDB.AutoMigrate(&model.User{}, &model.Order{}, &model.Payment{}, &model.Log{}, &model.UserGroup{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })
}

// seedEpayFixtures 写入一个启用的 epay 网关、一个用户，以及一个指定状态的订单。
// 返回订单（含 TradeNo/Quota）供断言使用。
func seedEpayFixtures(t *testing.T, userID int, initialQuota int, orderQuota int, tradeNo string, status model.OrderStatus) *model.Order {
	t.Helper()

	// epay 网关配置：Config 是 EpayConfig 的 JSON，含签名 key。
	cfg, err := json.Marshal(map[string]any{
		"pay_type":   "alipay",
		"pay_domain": "https://pay.example.com",
		"partner_id": "1000",
		"key":        epayTestKey,
	})
	if err != nil {
		t.Fatalf("序列化网关配置失败: %v", err)
	}
	payment := &model.Payment{
		Type:     "epay",
		UUID:     "epay-uuid-fixed-000000000000000",
		Name:     "测试易支付",
		Currency: model.CurrencyTypeCNY,
		Config:   string(cfg),
	}
	return seedPaymentFixtures(t, payment, userID, initialQuota, orderQuota, tradeNo, status)
}

// seedPaymentFixtures 写入给定网关（置为启用）、一个用户，以及一个该网关下指定状态的订单。
func seedPaymentFixtures(t *testing.T, payment *model.Payment, userID int, initialQuota int, orderQuota int, tradeNo string, status model.OrderStatus) *model.Order {
	t.Helper()

	enable := true
	payment.Enable = &enable
	if err := model.DB.Create(payment).Error; err != nil {
		t.Fatalf("创建网关失败: %v", err)
	}

	user := &model.User{
		Id:        userID,
		Username:  "sec7user",
		Password:  "placeholder",
		Quota:     initialQuota,
		InviterId: 0, // 无邀请人，ProcessInviterReward 直接返回
		Group:     "default",
	}
	if err := model.DB.Create(user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	order := &model.Order{
		UserId:        userID,
		GatewayId:     payment.ID,
		TradeNo:       tradeNo,
		Amount:        1,
		OrderAmount:   1.00,
		OrderCurrency: model.CurrencyTypeCNY,
		Quota:         orderQuota,
		Status:        status,
	}
	if err := model.DB.Create(order).Error; err != nil {
		t.Fatalf("创建订单失败: %v", err)
	}
	return order
}

// signedEpayCallbackQuery 构造并签名一个 TRADE_SUCCESS 回调 query。
// 用与网关配置相同的 key 签名，使 epay.Verify 通过。
func signedEpayCallbackQuery(tradeNo, gatewayNo string) string {
	params := map[string]string{
		"pid":          "1000",
		"out_trade_no": tradeNo,
		"trade_no":     gatewayNo,
		"trade_status": "TRADE_SUCCESS",
		"type":         "alipay",
		"name":         tradeNo,
		"money":        "1.00",
	}

	// 复用真实 Sign 逻辑：直接调用 epay.Client.Sign，与回调 Verify 端完全一致，
	// 避免测试内自行复刻签名算法而与实现漂移。
	client := &epay.Client{Key: epayTestKey}
	sign := client.Sign(params)
	params["sign"] = sign
	params["sign_type"] = "MD5"

	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	return values.Encode()
}

// callEpayCallback 用给定的 query 触发一次 EpayCallback，返回 recorder。
func callEpayCallback(t *testing.T, rawQuery string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/user/epay/notify?"+rawQuery, nil)
	EpayCallback(c)
	return w
}

func getUserQuota(t *testing.T, userID int) int {
	t.Helper()
	var u model.User
	if err := model.DB.First(&u, userID).Error; err != nil {
		t.Fatalf("读取用户失败: %v", err)
	}
	return u.Quota
}

func countTopupLogs(t *testing.T, userID int) int64 {
	t.Helper()
	var n int64
	if err := model.DB.Model(&model.Log{}).
		Where("user_id = ? AND type = ?", userID, model.LogTypeTopup).
		Count(&n).Error; err != nil {
		t.Fatalf("统计充值日志失败: %v", err)
	}
	return n
}

func getOrderStatus(t *testing.T, tradeNo string) model.OrderStatus {
	t.Helper()
	o, err := model.GetOrderByTradeNo(tradeNo)
	if err != nil {
		t.Fatalf("读取订单失败: %v", err)
	}
	return o.Status
}

// assertTopupState 断言订单状态、用户额度与充值日志条数。
func assertTopupState(t *testing.T, userID int, tradeNo string, wantStatus model.OrderStatus, wantQuota int, wantLogs int64) {
	t.Helper()
	if got := getOrderStatus(t, tradeNo); got != wantStatus {
		t.Fatalf("订单状态应为 %q, 实际 %q", wantStatus, got)
	}
	if got := getUserQuota(t, userID); got != wantQuota {
		t.Fatalf("用户额度应为 %d, 实际 %d", wantQuota, got)
	}
	if got := countTopupLogs(t, userID); got != wantLogs {
		t.Fatalf("充值日志应为 %d 条, 实际 %d", wantLogs, got)
	}
}

// failUserQuotaUpdates 让之后所有改 users.quota 的语句在库里报错（模拟入账时加额度出错），
// 返回的函数撤掉故障。
func failUserQuotaUpdates(t *testing.T) (heal func()) {
	t.Helper()
	if err := model.DB.Exec(`CREATE TRIGGER fail_user_quota_update BEFORE UPDATE OF quota ON users
		BEGIN SELECT RAISE(ABORT, 'injected quota update failure'); END`).Error; err != nil {
		t.Fatalf("注入加额度故障失败: %v", err)
	}
	return func() {
		if err := model.DB.Exec(`DROP TRIGGER fail_user_quota_update`).Error; err != nil {
			t.Fatalf("撤掉加额度故障失败: %v", err)
		}
	}
}

// TestEpayCallbackConcurrentCreditsOnce 守护 SEC-7 修复：
// 对同一 pending 订单并发触发 N 次 EpayCallback，用户额度只应增加一次订单额度，
// 只应有一条充值日志，订单最终为 success。若锁内没有重取订单/重判状态，则会重复入账。
func TestEpayCallbackConcurrentCreditsOnce(t *testing.T) {
	setupEpayCallbackTestDB(t)

	const userID = 42
	const initialQuota = 1000
	const orderQuota = 5000
	const tradeNo = "sec7-concurrent-tradeno"

	seedEpayFixtures(t, userID, initialQuota, orderQuota, tradeNo, model.OrderStatusPending)

	const n = 8
	rawQuery := signedEpayCallbackQuery(tradeNo, "gw-concurrent-001")

	bodies := make([]string, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			bodies[i] = callEpayCallback(t, rawQuery).Body.String()
		}()
	}
	wg.Wait()

	// 入账的那次与之后读到已入账的那些都应确认，易支付不必再重发
	for i, body := range bodies {
		if body != "success" {
			t.Fatalf("第 %d 个回调应响应 success, 实际 %q", i, body)
		}
	}
	if got := getUserQuota(t, userID); got != initialQuota+orderQuota {
		t.Fatalf("并发回调应只入账一次: 期望额度 %d, 实际 %d (增量 %d, 订单额度 %d)",
			initialQuota+orderQuota, got, got-initialQuota, orderQuota)
	}
	if got := countTopupLogs(t, userID); got != 1 {
		t.Fatalf("应只有一条充值日志, 实际 %d", got)
	}
	if got := getOrderStatus(t, tradeNo); got != model.OrderStatusSuccess {
		t.Fatalf("订单状态应为 success, 实际 %q", got)
	}
}

// seedOrder 直接写入一条订单，供自助订单列表用例使用。
func seedOrder(t *testing.T, userID int, tradeNo string, status model.OrderStatus) {
	t.Helper()
	order := &model.Order{
		UserId:        userID,
		GatewayId:     1,
		TradeNo:       tradeNo,
		Amount:        1,
		OrderAmount:   1.00,
		OrderCurrency: model.CurrencyTypeCNY,
		Quota:         1000,
		Status:        status,
	}
	if err := model.DB.Create(order).Error; err != nil {
		t.Fatalf("创建订单失败: %v", err)
	}
}

// callGetUserOrderList 以 userID 的会话身份带 rawQuery 调用 GetUserOrderList。
func callGetUserOrderList(t *testing.T, userID int, rawQuery string) *model.DataResult[model.Order] {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/user/order?"+rawQuery, nil)
	c.Set("id", userID)

	GetUserOrderList(c)

	var resp struct {
		Success bool                           `json:"success"`
		Message string                         `json:"message"`
		Data    *model.DataResult[model.Order] `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v, body=%s", err, w.Body.String())
	}
	if !resp.Success {
		t.Fatalf("接口应成功, message=%q", resp.Message)
	}
	return resp.Data
}

// TestGetUserOrderListForcesSelfScope 守护自助订单列表的越权防护：
// 即使 query 里带上别人的 user_id，也只返回会话用户自己的订单。
func TestGetUserOrderListForcesSelfScope(t *testing.T) {
	setupEpayCallbackTestDB(t)

	const selfID = 11
	const otherID = 22
	seedOrder(t, selfID, "self-order-1", model.OrderStatusSuccess)
	seedOrder(t, selfID, "self-order-2", model.OrderStatusPending)
	seedOrder(t, otherID, "other-order-1", model.OrderStatusSuccess)

	data := callGetUserOrderList(t, selfID, "page=1&size=10&user_id="+fmt.Sprint(otherID))
	if data == nil || data.Data == nil {
		t.Fatalf("响应应包含分页数据")
	}
	if data.TotalCount != 2 {
		t.Fatalf("应只统计本人订单: 期望 2, 实际 %d", data.TotalCount)
	}
	for _, o := range *data.Data {
		if o.UserId != selfID {
			t.Fatalf("返回了他人订单: trade_no=%s user_id=%d", o.TradeNo, o.UserId)
		}
	}
}

// TestGetUserOrderListStatusFilter 状态过滤在自助作用域内仍生效。
func TestGetUserOrderListStatusFilter(t *testing.T) {
	setupEpayCallbackTestDB(t)

	const selfID = 33
	seedOrder(t, selfID, "filter-order-success", model.OrderStatusSuccess)
	seedOrder(t, selfID, "filter-order-pending", model.OrderStatusPending)

	data := callGetUserOrderList(t, selfID, "page=1&size=10&status=success")
	if data.TotalCount != 1 {
		t.Fatalf("状态过滤应只剩 1 条, 实际 %d", data.TotalCount)
	}
	if got := (*data.Data)[0].TradeNo; got != "filter-order-success" {
		t.Fatalf("返回了错误的订单: %q", got)
	}
}

// TestGetUserOrderListRejectsMissingSession 无有效会话用户时必须拒绝，
// 否则 user_id=0 会让 model.GetOrderList 不加用户条件而返回全站订单。
func TestGetUserOrderListRejectsMissingSession(t *testing.T) {
	setupEpayCallbackTestDB(t)

	seedOrder(t, 44, "no-session-order", model.OrderStatusSuccess)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/user/order?page=1&size=10", nil)

	GetUserOrderList(c)

	var resp struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v, body=%s", err, w.Body.String())
	}
	if resp.Success {
		t.Fatalf("无会话用户时不应返回成功: body=%s", w.Body.String())
	}
}

// TestEpayCallbackIdempotentOnSuccess 单线程幂等：
// 订单已是 success 时再收到一次合法回调，额度不变、不新增充值日志。
func TestEpayCallbackIdempotentOnSuccess(t *testing.T) {
	setupEpayCallbackTestDB(t)

	const userID = 7
	const initialQuota = 2000
	const orderQuota = 3000
	const tradeNo = "sec7-idempotent-tradeno"

	seedEpayFixtures(t, userID, initialQuota, orderQuota, tradeNo, model.OrderStatusSuccess)

	rawQuery := signedEpayCallbackQuery(tradeNo, "gw-idempotent-001")
	if body := callEpayCallback(t, rawQuery).Body.String(); body != "success" {
		t.Fatalf("已入账订单的重发通知应响应 success, 实际 %q", body)
	}

	if got := getUserQuota(t, userID); got != initialQuota {
		t.Fatalf("已 success 订单不应再入账: 期望额度 %d, 实际 %d", initialQuota, got)
	}
	if got := countTopupLogs(t, userID); got != 0 {
		t.Fatalf("已 success 订单不应新增充值日志, 实际 %d", got)
	}
}

// TestEpayCallbackRetriesAfterQuotaFailure 加额度失败时订单必须仍是 pending、额度与充值日志不变，
// 并回易支付 fail 让它重发；故障消失后重发的通知恰好入账一次，之后再重发只确认不入账。
func TestEpayCallbackRetriesAfterQuotaFailure(t *testing.T) {
	setupEpayCallbackTestDB(t)

	const userID = 51
	const initialQuota = 1000
	const orderQuota = 5000
	const tradeNo = "credit-failure-tradeno"

	seedEpayFixtures(t, userID, initialQuota, orderQuota, tradeNo, model.OrderStatusPending)
	rawQuery := signedEpayCallbackQuery(tradeNo, "gw-credit-failure-001")

	heal := failUserQuotaUpdates(t)
	if body := callEpayCallback(t, rawQuery).Body.String(); body != "fail" {
		t.Fatalf("入账失败应响应 fail 让易支付重发, 实际 %q", body)
	}
	assertTopupState(t, userID, tradeNo, model.OrderStatusPending, initialQuota, 0)

	heal()
	for i := 1; i <= 2; i++ {
		if body := callEpayCallback(t, rawQuery).Body.String(); body != "success" {
			t.Fatalf("第 %d 次重发应响应 success, 实际 %q", i, body)
		}
		assertTopupState(t, userID, tradeNo, model.OrderStatusSuccess, initialQuota+orderQuota, 1)
	}
}
