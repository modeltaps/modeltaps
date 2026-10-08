package epay

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// epayTestGatewayConfig 与 newTestClient 同一把 key，validSignedPayload 签出的通知才能验签通过。
const epayTestGatewayConfig = `{"pay_type":"alipay","pay_domain":"https://pay.example.com","partner_id":"1001","key":"test-secret-key"}`

func newEpayNotifyContext(t *testing.T, params map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	query := url.Values{}
	for k, v := range params {
		query.Set(k, v)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/user/epay/notify?"+query.Encode(), nil)
	return c, w
}

// 正签名 + TRADE_SUCCESS → PayNotify 字段正确，且此时还没有应答：
// 入账前就回 success，入账失败时易支付也不会再重发。
func TestEpayHandleCallback_ValidSignatureNotAckedYet(t *testing.T) {
	c, w := newEpayNotifyContext(t, validSignedPayload(newTestClient()))
	notify, err := (&Epay{}).HandleCallback(c, epayTestGatewayConfig)
	if err != nil {
		t.Fatalf("正签名应通过验签: %v", err)
	}
	if notify == nil || notify.TradeNo != "ORDER-20260701-0001" || notify.GatewayNo != "GATEWAY-TXN-777" {
		t.Fatalf("PayNotify 应取 out_trade_no / trade_no，got %+v", notify)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("入账前不应应答易支付，got %q", w.Body.String())
	}
}

// 验签不通过或配置解析失败 → 拒绝并照旧由网关自己响应 fail。
func TestEpayHandleCallback_RejectedRespondsFail(t *testing.T) {
	tampered := validSignedPayload(newTestClient())
	tampered["money"] = "999999.99"

	cases := map[string]struct {
		params map[string]string
		config string
	}{
		"篡改金额":   {params: tampered, config: epayTestGatewayConfig},
		"配置解析失败": {params: validSignedPayload(newTestClient()), config: "{not-json"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, w := newEpayNotifyContext(t, tc.params)
			notify, err := (&Epay{}).HandleCallback(c, tc.config)
			if err == nil || notify != nil {
				t.Fatalf("必须拒绝, notify=%v err=%v", notify, err)
			}
			if w.Body.String() != "fail" {
				t.Fatalf("应响应 fail，got %q", w.Body.String())
			}
		})
	}
}

// RespondCallback：入账成功回 success，易支付停止重发；入账失败回 fail，易支付稍后重发。
func TestEpayRespondCallback(t *testing.T) {
	for _, tc := range []struct {
		success bool
		want    string
	}{
		{success: true, want: "success"},
		{success: false, want: "fail"},
	} {
		c, w := newEpayNotifyContext(t, nil)
		(&Epay{}).RespondCallback(c, tc.success)
		if w.Code != 200 || w.Body.String() != tc.want {
			t.Fatalf("RespondCallback(%v) 应响应 200 %q，got %d %q", tc.success, tc.want, w.Code, w.Body.String())
		}
	}
}
