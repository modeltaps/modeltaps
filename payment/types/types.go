package types

import (
	"math"
	"strconv"
	"strings"

	"github.com/modeltaps/modeltaps/model"
)

// 支付网关的通用配置
type PayConfig struct {
	NotifyURL string             `json:"notify_url"`
	ReturnURL string             `json:"return_url"`
	TradeNo   string             `json:"trade_no"`
	Money     float64            `json:"money"`
	Currency  model.CurrencyType `json:"currency"`
	User      *model.User        `json:"user"`
}

// 请求支付时的数据结构
type PayRequest struct {
	Type int            `json:"type"` // 支付类型 1 url 2 qrcode
	Data PayRequestData `json:"data"`
}

type PayRequestData struct {
	URL    string `json:"url"`
	Method string `json:"method,omitempty"`
	Params any    `json:"params,omitempty"`
}

// 支付回调时的数据结构
type PayNotify struct {
	TradeNo   string `json:"trade_no"`
	GatewayNo string `json:"gateway_no"`
	// PaidCents 是网关通知里的实付金额（最小货币单位：分 / cent），入账前与订单金额比对。
	PaidCents int64 `json:"paid_cents"`
	// PaidCurrency 是网关通知里的币种；网关只收一种币种时留空。
	PaidCurrency string `json:"paid_currency,omitempty"`
}

// YuanToCents 把网关通知里以元为单位的金额字符串（如 "12.30"）换算成分，无法解析时返回 0。
func YuanToCents(amount string) int64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(amount), 64)
	if err != nil || v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return int64(math.Round(v * 100))
}
