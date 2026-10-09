package controller

import (
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/payment"
	"github.com/modeltaps/modeltaps/payment/types"

	"github.com/gin-gonic/gin"
)

type OrderRequest struct {
	UUID   string `json:"uuid" binding:"required"`
	Amount int    `json:"amount" binding:"required"`
}

type OrderResponse struct {
	TradeNo string `json:"trade_no"`
	*types.PayRequest
}

// CreateOrder
func CreateOrder(c *gin.Context) {
	var orderReq OrderRequest
	if err := c.ShouldBindJSON(&orderReq); err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid request"))

		return
	}

	if orderReq.Amount <= 0 || orderReq.Amount < config.PaymentMinAmount {
		common.APIRespondWithError(c, http.StatusOK, fmt.Errorf("Amount must be at least %d", config.PaymentMinAmount))

		return
	}

	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("User not found"))
		return
	}

	// 关闭用户未完成的订单
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SysError(fmt.Sprintf("CloseUnfinishedOrder panic: %v\n%s", r, debug.Stack()))
			}
		}()
		if err := model.CloseUnfinishedOrder(); err != nil {
			logger.SysError("Failed to close pending orders: " + err.Error())
		}
	}()

	paymentService, err := payment.NewPaymentService(orderReq.UUID)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	// 获取手续费和支付金额
	discount, fee, payMoney := calculateOrderAmount(paymentService.Payment, orderReq.Amount)
	// 开始支付
	tradeNo := utils.GenerateTradeNo()
	payRequest, err := paymentService.Pay(tradeNo, payMoney, user)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("Failed to create payment, please try again later"))
		return
	}

	// 创建订单
	order := &model.Order{
		UserId:        userId,
		GatewayId:     paymentService.Payment.ID,
		TradeNo:       tradeNo,
		Amount:        orderReq.Amount,
		OrderAmount:   payMoney,
		OrderCurrency: paymentService.Payment.Currency,
		ExchangeRate:  orderExchangeRate(paymentService.Payment.Currency),
		Fee:           fee,
		Discount:      discount,
		Status:        model.OrderStatusPending,
		Quota:         orderReq.Amount * int(config.QuotaPerUnit),
	}

	err = order.Insert()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("Failed to create order, please try again later"))
		return
	}

	orderResp := &OrderResponse{
		TradeNo:    tradeNo,
		PayRequest: payRequest,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    orderResp,
	})
}

// orderLockEntry 订单锁条目，带时间戳用于清理。
// lastUsed 用原子的 UnixNano 存储：LockOrder/UnlockOrder 持锁前后与 cleanupOrderLocks
// 会并发读写它，若用普通字段会构成数据竞争。
type orderLockEntry struct {
	lock     *sync.Mutex
	lastUsed atomic.Int64
}

// touch 记录本条目最近一次使用时间（纳秒）。
func (e *orderLockEntry) touch() {
	e.lastUsed.Store(time.Now().UnixNano())
}

// lastUsedTime 返回最近一次使用时间。
func (e *orderLockEntry) lastUsedTime() time.Time {
	return time.Unix(0, e.lastUsed.Load())
}

var orderLocks sync.Map
var createLock sync.Mutex

func init() {
	go cleanupOrderLocks()
}

// cleanupOrderLocks 定期清理长时间未使用的订单锁
func cleanupOrderLocks() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		orderLocks.Range(func(key, value interface{}) bool {
			if entry := value.(*orderLockEntry); now.Sub(entry.lastUsedTime()) > 30*time.Minute {
				orderLocks.Delete(key)
			}
			return true
		})
	}
}

// LockOrder 尝试对给定订单号加锁
func LockOrder(tradeNo string) {
	var entry *orderLockEntry
	val, ok := orderLocks.Load(tradeNo)
	if !ok {
		createLock.Lock()
		val, ok = orderLocks.Load(tradeNo)
		if !ok {
			entry = &orderLockEntry{
				lock: new(sync.Mutex),
			}
			entry.touch()
			orderLocks.Store(tradeNo, entry)
		} else {
			entry = val.(*orderLockEntry)
		}
		createLock.Unlock()
	} else {
		entry = val.(*orderLockEntry)
	}
	entry.touch()
	entry.lock.Lock()
}

// UnlockOrder 释放给定订单号的锁
func UnlockOrder(tradeNo string) {
	val, ok := orderLocks.Load(tradeNo)
	if ok {
		entry := val.(*orderLockEntry)
		entry.touch()
		entry.lock.Unlock()
	}
}

func PaymentCallback(c *gin.Context) {
	uuid := c.Param("uuid")
	paymentService, err := payment.NewPaymentService(uuid)
	if err != nil {
		// 不能回 2xx：Stripe / 微信支付把 2xx 当作已送达，查库出错时就再也不会重发
		common.APIRespondWithError(c, http.StatusNotFound, errors.New("payment not found"))
		return
	}

	payNotify, err := paymentService.HandleCallback(c, paymentService.Payment.Config)
	if err != nil {
		return
	}
	// 验签通过但无需入账的通知（Stripe 的其它事件类型）直接确认
	if payNotify == nil {
		paymentService.RespondCallback(c, true)
		return
	}

	paymentService.RespondCallback(c, creditPaidOrder(c, payNotify) == nil)
}

// creditPaidOrder 为已验签的支付成功通知入账。返回 nil 表示订单已入账（本次或之前的通知），
// 可以向网关确认；返回 error 表示本次没有入账，调用方应回网关失败应答让其重发。
func creditPaidOrder(c *gin.Context, payNotify *types.PayNotify) error {
	LockOrder(payNotify.TradeNo)
	defer UnlockOrder(payNotify.TradeNo)

	// 锁内读取订单再判状态：锁外读到的快照可能已被并发回调入账
	order, err := model.GetOrderByTradeNo(payNotify.TradeNo)
	if err != nil {
		logger.SysError(fmt.Sprintf("payment callback failed to find order, trade_no: %s, error: %s", payNotify.TradeNo, err.Error()))
		return err
	}
	if order.Status != model.OrderStatusPending {
		if order.Status != model.OrderStatusSuccess {
			logger.SysError(fmt.Sprintf("payment callback for a %s order, paid amount not credited, trade_no: %s, gateway_no: %s", order.Status, payNotify.TradeNo, payNotify.GatewayNo))
		}
		return nil
	}

	credited, err := model.CreditPendingOrder(order, payNotify.GatewayNo)
	if err != nil {
		logger.SysError(fmt.Sprintf("payment callback failed to credit order, trade_no: %s, error: %s", payNotify.TradeNo, err.Error()))
		return err
	}
	if !credited {
		return nil
	}

	// 以下动作在入账提交之后执行，失败只记日志：订单已是 success，网关重发也不会再走到这里
	err = model.CheckAndUpgradeUserGroup(order.UserId)
	if err != nil {
		logger.SysError(fmt.Sprintf("failed to check and upgrade user group, trade_no: %s, error: %s", payNotify.TradeNo, err.Error()))
	}

	model.RecordQuotaLog(order.UserId, model.LogTypeTopup, order.Quota, c.ClientIP(), fmt.Sprintf("Online top-up successful, quota: %d, amount paid: %.2f %s", order.Quota, order.OrderAmount, order.OrderCurrency))

	// 处理邀请人充值返利
	err = model.ProcessInviterReward(order.UserId, order.Quota, c.ClientIP())
	if err != nil {
		logger.SysError(fmt.Sprintf("failed to process inviter reward, trade_no: %s, error: %s", payNotify.TradeNo, err.Error()))
	}
	return nil
}

func CheckOrderStatus(c *gin.Context) {
	tradeNo := c.Query("trade_no")
	userId := c.GetInt("id")
	success := false

	if tradeNo != "" {
		order, err := model.GetUserOrder(userId, tradeNo)
		if err == nil {
			if order.Status == model.OrderStatusSuccess {
				success = true
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": success,
		"message": "",
	})
}

// orderExchangeRate 返回下单当时应记录的收款汇率：USD 网关不折算，恒为 1
func orderExchangeRate(currency model.CurrencyType) float64 {
	if currency == model.CurrencyTypeUSD {
		return 1
	}
	return config.PaymentUSDRate
}

// discountMoney优惠金额 fee手续费，payMoney实付金额
func calculateOrderAmount(payment *model.Payment, amount int) (discountMoney, fee, payMoney float64) {
	// 获取折扣
	discount := common.GetRechargeDiscount(strconv.Itoa(amount))
	newMoney := float64(amount) * discount // 折后价值
	oldTotal := float64(amount)            //原价值
	if payment.PercentFee > 0 {
		//手续费=（原始价值*折扣*手续费率）
		fee = utils.Decimal(newMoney*payment.PercentFee, 2) //折后手续
		oldTotal = utils.Decimal(oldTotal*(1+payment.PercentFee), 2)
	} else if payment.FixedFee > 0 {
		//固定费率不计算折扣
		fee = payment.FixedFee
	}

	//实际费用=（折后价+折后手续费）*汇率*网关倍率
	total := utils.Decimal(newMoney+fee, 2)

	// 获取网关倍率，默认为1
	currencyRate := payment.CurrencyRate
	if currencyRate <= 0 {
		currencyRate = 1
	}

	if payment.Currency == model.CurrencyTypeUSD {
		oldTotal = utils.Decimal(oldTotal*currencyRate, 2)
		payMoney = utils.Decimal(total*currencyRate, 2)
	} else {
		oldTotal = utils.Decimal(oldTotal*config.PaymentUSDRate*currencyRate, 2)
		payMoney = utils.Decimal(total*config.PaymentUSDRate*currencyRate, 2)
	}
	discountMoney = oldTotal - payMoney //折扣金额 = 原价值-实际支付价值
	return
}

func GetOrderList(c *gin.Context) {
	var params model.SearchOrderParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	payments, err := model.GetOrderList(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    payments,
	})
}

// GetUserOrderList 返回当前登录用户自己的充值订单列表。
// 查询参数与管理端一致，但 user_id 强制取自会话：客户端传入的 user_id 一律被覆盖，
// 避免越权查看他人订单。
func GetUserOrderList(c *gin.Context) {
	var params model.SearchOrderParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	userId := c.GetInt("id")
	if userId <= 0 {
		// user_id 为 0 时 model.GetOrderList 不会加 user_id 条件，会返回全站订单
		common.APIRespondWithError(c, http.StatusOK, errors.New("Invalid user"))
		return
	}
	params.UserId = userId

	orders, err := model.GetOrderList(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    orders,
	})
}

// EpayCallback 固定的易支付回调接口
func EpayCallback(c *gin.Context) {
	tradeNo := c.Query("out_trade_no")
	if tradeNo == "" {
		c.String(http.StatusOK, "fail")
		return
	}

	order, err := model.GetOrderByTradeNo(tradeNo)
	if err != nil {
		logger.SysError(fmt.Sprintf("epay callback failed to find order, trade_no: %s", tradeNo))
		c.String(http.StatusOK, "fail")
		return
	}

	gatewayPayment, err := model.GetPaymentByID(order.GatewayId)
	if err != nil {
		logger.SysError(fmt.Sprintf("epay callback failed to find payment, trade_no: %s, gateway_id: %d", tradeNo, order.GatewayId))
		c.String(http.StatusOK, "fail")
		return
	}

	paymentService, err := payment.NewPaymentService(gatewayPayment.UUID)
	if err != nil {
		logger.SysError(fmt.Sprintf("epay callback failed to create payment service, trade_no: %s", tradeNo))
		c.String(http.StatusOK, "fail")
		return
	}

	payNotify, err := paymentService.HandleCallback(c, paymentService.Payment.Config)
	if err != nil {
		return
	}

	// 上面按单号取的 order 只用来找网关，入账时在锁内重新读取（见 creditPaidOrder）
	paymentService.RespondCallback(c, creditPaidOrder(c, payNotify) == nil)
}
