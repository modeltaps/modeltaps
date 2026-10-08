package epay

import (
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/payment/types"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Epay struct{}

type EpayConfig struct {
	PayType PayType `json:"pay_type"`
	Client
}

func (e *Epay) Name() string {
	return "Epay"
}

func (e *Epay) Pay(config *types.PayConfig, gatewayConfig string) (*types.PayRequest, error) {
	epayConfig, err := getEpayConfig(gatewayConfig)
	if err != nil {
		return nil, err
	}

	payArgs := &PayArgs{
		Type:       epayConfig.PayType,
		OutTradeNo: config.TradeNo,
		NotifyUrl:  config.NotifyURL,
		ReturnUrl:  config.ReturnURL,
		Name:       config.TradeNo,
		Money:      strconv.FormatFloat(config.Money, 'f', 2, 64),
	}

	formPayURL, formPayArgs, err := epayConfig.FormPay(payArgs)
	if err != nil {
		return nil, err
	}

	payRequest := &types.PayRequest{
		Type: 1,
		Data: types.PayRequestData{
			URL:    formPayURL,
			Params: &formPayArgs,
			Method: http.MethodPost,
		},
	}

	return payRequest, nil
}

func (e *Epay) HandleCallback(c *gin.Context, gatewayConfig string) (*types.PayNotify, error) {
	queryMap := make(map[string]string)
	for key, values := range c.Request.URL.Query() {
		if len(values) > 0 {
			queryMap[key] = values[0]
		}
	}

	epayConfig, err := getEpayConfig(gatewayConfig)
	if err != nil {
		c.Writer.Write([]byte("fail"))
		return nil, fmt.Errorf("tradeNo: %s, PaymentNo: %s,  err: %v", queryMap["out_trade_no"], queryMap["trade_no"], err)
	}

	paymentResult, success := epayConfig.Verify(queryMap)
	if paymentResult != nil && success {
		// 这里不回 success：入账提交后由调用方通过 RespondCallback 应答
		payNotify := &types.PayNotify{
			TradeNo:   paymentResult.OutTradeNo,
			GatewayNo: paymentResult.TradeNo,
		}
		return payNotify, nil
	}

	c.Writer.Write([]byte("fail"))
	return nil, fmt.Errorf("tradeNo: %s, PaymentNo: %s,  Verify Sign failed", queryMap["out_trade_no"], queryMap["trade_no"])
}

// RespondCallback 回 success 后易支付不再重发通知；回 fail 则稍后重发
func (e *Epay) RespondCallback(c *gin.Context, success bool) {
	if success {
		c.Writer.Write([]byte("success"))
		return
	}
	c.Writer.Write([]byte("fail"))
}

func getEpayConfig(gatewayConfig string) (*EpayConfig, error) {
	var epayConfig EpayConfig
	if err := json.Unmarshal([]byte(gatewayConfig), &epayConfig); err != nil {
		return nil, errors.New("config error")
	}

	return &epayConfig, nil
}
func (e *Epay) CreatedPay(_ string, _ *model.Payment) error {
	return nil
}
