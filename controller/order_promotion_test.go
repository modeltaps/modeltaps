package controller

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// promotionOrderQuota 把分组晋级条件的单位（美元）折算成额度。
func promotionOrderQuota(units int) int {
	return units * int(config.QuotaPerUnit)
}

// seedCallbackPromotionGroups 写入两个自动晋级分组：silver 累计充值 [50, 150)，gold 150 起不设上限。
func seedCallbackPromotionGroups(t *testing.T) {
	t.Helper()
	enable := true
	for _, g := range []*model.UserGroup{
		{Symbol: "silver", Name: "Silver", Ratio: 1, Promotion: true, Min: 50, Max: 150, Enable: &enable},
		{Symbol: "gold", Name: "Gold", Ratio: 1, Promotion: true, Min: 150, Max: 0, Enable: &enable},
	} {
		if err := model.DB.Create(g).Error; err != nil {
			t.Fatalf("创建分组 %s 失败: %v", g.Symbol, err)
		}
	}
}

// enableBatchUpdateForTest 打开批量更新（不启动后台 ticker），用例结束时把排队的写入落到
// 测试库再恢复开关，免得漏进后面的用例。须在 setupEpayCallbackTestDB 之后调用。
func enableBatchUpdateForTest(t *testing.T) {
	t.Helper()
	old := config.BatchUpdateEnabled
	config.BatchUpdateEnabled = true
	t.Cleanup(func() {
		model.FlushAllBatches()
		config.BatchUpdateEnabled = old
	})
}

// notifyEpayPaymentRoute 以 /api/payment/notify/:uuid 路由触发一次 PaymentCallback。
func notifyEpayPaymentRoute(t *testing.T, uuid, rawQuery string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/payment/notify/"+uuid+"?"+rawQuery, nil)
	c.Params = gin.Params{{Key: "uuid", Value: uuid}}
	PaymentCallback(c)
}

// 支付回调入账后按累计充值判定晋级，本次充值只算一次：付 100 进 silver（重复计入会当成 200
// 进 gold），再付 50 累计 150 才进 gold。批量更新开启时也一样：晋级判定读的是库里的额度，
// 回调必须先把本次充值写进库，而不是留在批量队列里等下一次落库。
func TestPaymentCallbacksPromoteOnPaidQuotaOnce(t *testing.T) {
	callbacks := []struct {
		name   string
		notify func(t *testing.T, paymentUUID, rawQuery string)
	}{
		{"EpayCallback", func(t *testing.T, _ string, rawQuery string) { callEpayCallback(t, rawQuery) }},
		{"PaymentCallback", notifyEpayPaymentRoute},
	}
	for _, batch := range []bool{false, true} {
		for _, cb := range callbacks {
			t.Run(fmt.Sprintf("%s/batch_update=%v", cb.name, batch), func(t *testing.T) {
				setupEpayCallbackTestDB(t)
				if batch {
					enableBatchUpdateForTest(t)
				}
				seedCallbackPromotionGroups(t)

				const userID = 51
				first := seedEpayFixtures(t, userID, 0, promotionOrderQuota(100), "promotion-trade-100", model.OrderStatusPending)
				second := *first
				second.ID = 0
				second.TradeNo = "promotion-trade-050"
				second.Quota = promotionOrderQuota(50)
				if err := model.DB.Create(&second).Error; err != nil {
					t.Fatalf("创建第二笔订单失败: %v", err)
				}
				payment, err := model.GetPaymentByID(first.GatewayId)
				if err != nil {
					t.Fatalf("读取网关失败: %v", err)
				}

				for _, step := range []struct {
					order           *model.Order
					wantGroupSymbol string
				}{
					{first, "silver"},
					{&second, "gold"},
				} {
					cb.notify(t, payment.UUID, signedEpayCallbackQuery(step.order.TradeNo, "gw-"+step.order.TradeNo))
					if got := getOrderStatus(t, step.order.TradeNo); got != model.OrderStatusSuccess {
						t.Fatalf("订单 %s 应已入账, got %q", step.order.TradeNo, got)
					}
					group, err := model.GetUserGroup(userID)
					if err != nil {
						t.Fatalf("读取用户分组失败: %v", err)
					}
					if group != step.wantGroupSymbol {
						t.Fatalf("支付订单 %s 后分组应为 %q, got %q", step.order.TradeNo, step.wantGroupSymbol, group)
					}
				}

				model.FlushAllBatches()
				if got := getUserQuota(t, userID); got != promotionOrderQuota(150) {
					t.Fatalf("两笔订单应各入账一次: 期望额度 %d, 实际 %d", promotionOrderQuota(150), got)
				}
			})
		}
	}
}
