package alipay

import (
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/payment/types"
	"encoding/json"
	"errors"
	"fmt"

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
	// 验证通知签名
	// SDK 仅在通知带有未加载的 alipay_cert_sn 时用 ctx 下载支付宝证书；请求取消只会让验签失败
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

	if noti.TradeStatus == alipay.TradeStatusSuccess {
		// 这里不回 success：入账提交后由调用方通过 RespondCallback 应答
		payNotify := &types.PayNotify{
			TradeNo:   noti.OutTradeNo,
			GatewayNo: noti.TradeNo,
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
