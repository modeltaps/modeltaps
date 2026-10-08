package model

import (
	"fmt"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/redis"

	"gorm.io/gorm"
)

type OrderStatus string

const (
	OrderStatusPending OrderStatus = "pending"
	OrderStatusSuccess OrderStatus = "success"
	OrderStatusFailed  OrderStatus = "failed"
	OrderStatusClosed  OrderStatus = "closed"
)

type Order struct {
	ID            int          `json:"id"`
	UserId        int          `json:"user_id"`
	GatewayId     int          `json:"gateway_id"`
	TradeNo       string       `json:"trade_no" gorm:"type:varchar(50);uniqueIndex"`
	GatewayNo     string       `json:"gateway_no" gorm:"type:varchar(100)"`
	Amount        int          `json:"amount" gorm:"default:0"`
	OrderAmount   float64      `json:"order_amount" gorm:"type:decimal(10,2);default:0"`
	OrderCurrency CurrencyType `json:"order_currency" gorm:"type:varchar(16)"`
	// ExchangeRate 下单时使用的人民币收款汇率（1 USD = ? CNY）；USD 网关恒为 1，历史订单为 0。
	ExchangeRate float64        `json:"exchange_rate" gorm:"type:decimal(10,4);default:0"`
	Quota        int            `json:"quota" gorm:"type:bigint;default:0"`
	Fee          float64        `json:"fee" gorm:"type:decimal(10,2);default:0"`
	Discount     float64        `json:"discount" gorm:"type:decimal(10,2);default:0"`
	Status       OrderStatus    `json:"status" gorm:"type:varchar(32)"`
	CreatedAt    int            `json:"created_at"`
	UpdatedAt    int            `json:"-"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

// 查询并关闭未完成的订单
func CloseUnfinishedOrder() error {
	// 关闭超过 3 小时未支付的订单
	unixTime := time.Now().Unix() - 3*3600
	return DB.Model(&Order{}).Where("status = ? AND created_at < ?", OrderStatusPending, unixTime).Update("status", OrderStatusClosed).Error
}

func GetOrderByTradeNo(tradeNo string) (*Order, error) {
	var order Order
	err := DB.Where("trade_no = ?", tradeNo).First(&order).Error
	return &order, err
}

func GetUserOrder(userId int, tradeNo string) (*Order, error) {
	var order Order
	err := DB.Where("user_id = ? AND trade_no = ?", userId, tradeNo).First(&order).Error
	return &order, err
}

func (o *Order) Insert() error {
	return DB.Create(o).Error
}

func (o *Order) Update() error {
	return DB.Save(o).Error
}

// CreditPendingOrder 支付成功入账：同一事务里把订单由 pending 置为 success 并给订单用户加额度，
// 同成同败——加额度失败时订单仍是 pending，网关重发的通知可以再次入账。
// 置 success 用 status = pending 条件更新，即使跨进程并发也只有一次转换生效；
// 返回 false 表示订单已不是 pending（已入账或已关闭），本次没有入账。
func CreditPendingOrder(order *Order, gatewayNo string) (bool, error) {
	credited := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&Order{}).
			Where("id = ? AND status = ?", order.ID, OrderStatusPending).
			Updates(map[string]interface{}{"status": OrderStatusSuccess, "gateway_no": gatewayNo})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if err := IncreaseUserQuotaWithTx(tx, order.UserId, order.Quota); err != nil {
			return err
		}
		credited = true
		return nil
	})
	if err != nil || !credited {
		return false, err
	}
	order.Status = OrderStatusSuccess
	order.GatewayNo = gatewayNo
	if config.RedisEnabled {
		redis.RedisDel(fmt.Sprintf(UserQuotaCacheKey, order.UserId))
	}
	return true, nil
}

var allowedOrderFields = map[string]bool{
	"id":         true,
	"gateway_id": true,
	"user_id":    true,
	"status":     true,
	"created_at": true,
}

type SearchOrderParams struct {
	UserId         int    `form:"user_id"`
	GatewayId      int    `form:"gateway_id"`
	TradeNo        string `form:"trade_no"`
	GatewayNo      string `form:"gateway_no"`
	Status         string `form:"status"`
	StartTimestamp int64  `form:"start_timestamp"`
	EndTimestamp   int64  `form:"end_timestamp"`
	PaginationParams
}

func GetOrderList(params *SearchOrderParams) (*DataResult[Order], error) {
	var orders []*Order

	db := DB.Omit("key")
	if params.GatewayId != 0 {
		db = db.Where("gateway_id = ?", params.GatewayId)
	}
	if params.UserId != 0 {
		db = db.Where("user_id = ?", params.UserId)
	}

	if params.TradeNo != "" {
		db = db.Where("trade_no = ?", params.TradeNo)
	}

	if params.GatewayNo != "" {
		db = db.Where("gateway_no = ?", params.GatewayNo)
	}

	if params.Status != "" {
		db = db.Where("status = ?", params.Status)
	}

	if params.StartTimestamp != 0 {
		db = db.Where("created_at >= ?", params.StartTimestamp)
	}
	if params.EndTimestamp != 0 {
		db = db.Where("created_at <= ?", params.EndTimestamp)
	}

	return PaginateAndOrder(db, &params.PaginationParams, &orders, allowedOrderFields)
}

type OrderStatistics struct {
	Quota         int64   `json:"quota"`
	Money         float64 `json:"money"`
	OrderCurrency string  `json:"order_currency"`
}

func GetStatisticsOrder() (orderStatistics []*OrderStatistics, err error) {
	err = DB.Model(&Order{}).Select("sum(quota) as quota, sum(order_amount) as money, order_currency").Where("status = ?", OrderStatusSuccess).Group("order_currency").Scan(&orderStatistics).Error
	return orderStatistics, err
}

type OrderStatisticsGroup struct {
	Date          string  `json:"date"`
	Quota         int64   `json:"quota"`
	Money         float64 `json:"money"`
	OrderCurrency string  `json:"order_currency"`
}

func GetStatisticsOrderByPeriod(startTimestamp, endTimestamp int64) (orderStatistics []*OrderStatisticsGroup, err error) {
	groupSelect := getTimestampGroupsSelect("created_at", "day", "date")

	err = DB.Raw(`
		SELECT `+groupSelect+`,
		sum(quota) as quota,
		sum(order_amount) as money,
		order_currency
		FROM orders
		WHERE status= ?
		AND created_at BETWEEN ? AND ?
		GROUP BY date, order_currency
		ORDER BY date, order_currency
	`, OrderStatusSuccess, startTimestamp, endTimestamp).Scan(&orderStatistics).Error

	return orderStatistics, err
}
