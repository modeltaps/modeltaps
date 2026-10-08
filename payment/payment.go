package payment

import (
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/payment/gateway/alipay"
	"github.com/modeltaps/modeltaps/payment/gateway/epay"
	"github.com/modeltaps/modeltaps/payment/gateway/stripe"
	"github.com/modeltaps/modeltaps/payment/gateway/wxpay"
	"github.com/modeltaps/modeltaps/payment/types"

	"github.com/gin-gonic/gin"
)

type PaymentProcessor interface {
	Name() string
	Pay(config *types.PayConfig, gatewayConfig string) (*types.PayRequest, error)
	CreatedPay(notifyURL string, gatewayConfig *model.Payment) error
	// HandleCallback 验签并解析支付通知。返回 error 时的应答由网关自己处理，调用方不再写；
	// 返回 PayNotify（含 nil）时不写任何应答，由调用方入账后调用 RespondCallback。
	HandleCallback(c *gin.Context, gatewayConfig string) (*types.PayNotify, error)
	// RespondCallback 向网关应答已验签的通知：success 为 true 表示已入账，网关不再重发；
	// 为 false 写该网关的失败应答，让网关稍后重发。
	RespondCallback(c *gin.Context, success bool)
}

var Gateways = make(map[string]PaymentProcessor)

func init() {
	Gateways["epay"] = &epay.Epay{}
	Gateways["alipay"] = &alipay.Alipay{}
	Gateways["wxpay"] = &wxpay.WeChatPay{}
	Gateways["stripe"] = &stripe.Stripe{}
}
