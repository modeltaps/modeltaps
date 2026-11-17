package alipay

import (
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/payment/types"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/smartwalle/alipay/v3"
)

type Alipay struct{}

type AlipayConfig struct {
	AppID      string  `json:"app_id"`
	PrivateKey string  `json:"private_key"`
	PublicKey  string  `json:"public_key"`
	PayType    PayType `json:"pay_type"`
}

const isProduction bool = true

func (a *Alipay) Name() string {
	return "Alipay"
}

func (a *Alipay) SecretConfigKeys() []string {
	return []string{"private_key"}
}

// createClient 创建支付宝客户端，每次调用都创建新实例
func (a *Alipay) createClient(config *AlipayConfig) (*alipay.Client, error) {
	client, err := alipay.New(config.AppID, config.PrivateKey, isProduction)
	if err != nil {
		return nil, err
	}
	err = client.LoadAliPayPublicKey(config.PublicKey)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (a *Alipay) Pay(config *types.PayConfig, gatewayConfig string) (*types.PayRequest, error) {
	alipayConfig, err := getAlipayConfig(gatewayConfig)
	if err != nil {
		return nil, err
	}

	client, err := a.createClient(alipayConfig)
	if err != nil {
		return nil, err
	}

	switch alipayConfig.PayType {
	case PagePay:
		return a.handlePagePay(config, client)
	case WapPay:
		return a.handleWapPay(config, client)
	default:
		return a.handleTradePreCreate(config, client)
	}
}

func (a *Alipay) HandleCallback(c *gin.Context, gatewayConfig string) (*types.PayNotify, error) {
	alipayConfig, err := getAlipayConfig(gatewayConfig)
	if err != nil {
		c.Writer.Write([]byte("failure"))
		return nil, err
	}

	client, err := a.createClient(alipayConfig)
	if err != nil {
		c.Writer.Write([]byte("failure"))
		return nil, err
	}

	// 获取通知参数
	params := c.Request.URL.Query()
	if err := c.Request.ParseForm(); err != nil {
		c.Writer.Write([]byte("failure"))
		return nil, fmt.Errorf("Alipay params failed: %v", err)
	}
	for k, v := range c.Request.PostForm {
		params[k] = v
	}
	// This site uses the Alipay public key mode, so a notification is only ever verified against
	// the configured public key. alipay_cert_sn is not covered by the signature; left in, the SDK
	// would download the certificate it names, calling Alipay as the merchant for any POST.
	params.Del("alipay_cert_sn")
	// 验证通知签名
	ctx := c.Request.Context()
	if err := client.VerifySign(ctx, params); err != nil {
		c.Writer.Write([]byte("failure"))
		return nil, fmt.Errorf("Alipay Signature verification failed: %v", err)
	}
	//解析通知内容
	noti, err := client.DecodeNotification(ctx, params)
	if err != nil {
		c.Writer.Write([]byte("failure"))
		return nil, fmt.Errorf("Alipay Error decoding notification: %v", err)
	}

	// The signature only shows the notification comes from Alipay, not that it is for this app:
	// a payment to another app with the same out_trade_no must not credit the order here.
	if reason := notificationAppMismatch(noti, alipayConfig); reason != "" {
		c.Writer.Write([]byte("failure"))
		return nil, fmt.Errorf("Alipay notification %s, out_trade_no: %s, trade_no: %s", reason, noti.OutTradeNo, noti.TradeNo)
	}

	if noti.TradeStatus == alipay.TradeStatusSuccess {
		// 这里不回 success：入账提交后由调用方通过 RespondCallback 应答
		payNotify := &types.PayNotify{
			TradeNo:   noti.OutTradeNo,
			GatewayNo: noti.TradeNo,
			PaidCents: types.YuanToCents(noti.TotalAmount),
		}
		return payNotify, nil
	}
	c.Writer.Write([]byte("failure"))
	return nil, fmt.Errorf("trade status not success")
}

// RespondCallback 回 success 后支付宝不再重发通知；回 failure 则按支付宝的重试策略稍后重发
func (a *Alipay) RespondCallback(c *gin.Context, success bool) {
	if success {
		alipay.ACKNotification(c.Writer)
		return
	}
	c.Writer.Write([]byte("failure"))
}

// notificationAppMismatch returns why the notification is not for the configured app, or "".
// auth_app_id is the app the payment was made through; Alipay sends it equal to app_id for an
// app that collects for itself, which is the only mode this gateway places orders in.
func notificationAppMismatch(noti *alipay.Notification, config *AlipayConfig) string {
	appID := strings.TrimSpace(config.AppID)
	if appID == "" || noti.AppId != appID {
		return "app_id mismatch"
	}
	if noti.AuthAppId != "" && noti.AuthAppId != appID {
		return "auth_app_id mismatch"
	}
	return ""
}

func getAlipayConfig(gatewayConfig string) (*AlipayConfig, error) {
	var alipayConfig AlipayConfig
	if err := json.Unmarshal([]byte(gatewayConfig), &alipayConfig); err != nil {
		return nil, errors.New("config error")
	}

	return &alipayConfig, nil
}

func (a *Alipay) CreatedPay(_ string, _ *model.Payment) error {
	return nil
}
