package controller

import (
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/payment"

	"github.com/gin-gonic/gin"
)

type OrgQuotaTransferRequest struct {
	Quota int `json:"quota" binding:"required"`
}

// TransferQuotaToOrg Owner/Admin 将个人积分单向转入组织池(规格 §3.3 / a5)
func TransferQuotaToOrg(c *gin.Context) {
	if !config.OrganizationQuotaTransferEnabled {
		common.APIRespondWithError(c, http.StatusOK, errors.New("Quota transfer is not enabled"))
		return
	}
	var req OrgQuotaTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("Invalid parameters"))
		return
	}
	org, _ := getOrgFromContext(c)
	userId := c.GetInt("id")
	if err := model.TransferQuotaToOrganization(org, userId, req.Quota); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	model.RecordOrgAudit(org.Id, userId, "quota.transfer", fmt.Sprintf("Transferred %s of personal quota to the organization pool", common.LogQuota(req.Quota)))
	data := gin.H{}
	if quota, err := model.GetUserQuota(org.ShadowUserId); err == nil {
		data["quota"] = quota
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

type OrgTopUpRequest struct {
	Key string `json:"key" binding:"required"`
}

// OrgTopUp 兑换码入组织池(仅 Owner/Admin;入账主体为影子账户)
func OrgTopUp(c *gin.Context) {
	var req OrgTopUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("Invalid parameters"))
		return
	}
	org, _ := getOrgFromContext(c)
	quota, err := model.Redeem(req.Key, org.ShadowUserId, c.ClientIP())
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	model.RecordOrgAudit(org.Id, c.GetInt("id"), "billing.topup", fmt.Sprintf("Topped up the organization pool with %s via redemption code", common.LogQuota(quota)))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    quota,
	})
}

// CreateOrgOrder 组织在线充值下单(仅 Owner/Admin):订单归属影子账户,
// 支付回调复用现有流程,入账自动进组织池(order.user_id = 影子账户)
func CreateOrgOrder(c *gin.Context) {
	var orderReq OrderRequest
	if err := c.ShouldBindJSON(&orderReq); err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid request"))
		return
	}

	if orderReq.Amount <= 0 || orderReq.Amount < config.PaymentMinAmount {
		common.APIRespondWithError(c, http.StatusOK, fmt.Errorf("Amount must be at least %d", config.PaymentMinAmount))
		return
	}

	org, _ := getOrgFromContext(c)
	userId := c.GetInt("id")
	// 支付网关使用操作者(真实用户)信息,入账主体为影子账户
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

	// 创建订单(归属影子账户,回调入账即进组织池)
	order := &model.Order{
		UserId:        org.ShadowUserId,
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

	if err := order.Insert(); err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("Failed to create order, please try again later"))
		return
	}

	model.RecordOrgAudit(org.Id, userId, "billing.order_create", fmt.Sprintf("Created organization top-up order (amount: %d, order no: %s)", orderReq.Amount, tradeNo))

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

// CheckOrgOrderStatus 查询组织订单支付状态(仅 Owner/Admin)
func CheckOrgOrderStatus(c *gin.Context) {
	org, _ := getOrgFromContext(c)
	tradeNo := c.Query("trade_no")
	success := false

	if tradeNo != "" {
		order, err := model.GetUserOrder(org.ShadowUserId, tradeNo)
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

// GetOrgOrdersList 组织订单列表(仅 Owner/Admin;强制限定为影子账户订单)
func GetOrgOrdersList(c *gin.Context) {
	org, _ := getOrgFromContext(c)
	var params model.SearchOrderParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	params.UserId = org.ShadowUserId

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
