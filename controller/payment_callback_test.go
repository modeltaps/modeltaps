package controller

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"
)

// callPaymentCallback 经真实路由调用 PaymentCallback：路由参数 uuid、处理结束后补写状态码都与线上一致。
func callPaymentCallback(t *testing.T, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Any("/api/payment/notify/:uuid", PaymentCallback)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

var (
	alipayTestKeyOnce sync.Once
	alipayTestKey     *rsa.PrivateKey
)

// seedAlipayFixtures 写入支付宝网关、一个用户与一个 pending 订单，返回模拟支付宝签通知的私钥与网关 UUID。
// 网关配置里的支付宝公钥与该私钥成对，验签走真实 SDK。
func seedAlipayFixtures(t *testing.T, userID, initialQuota, orderQuota int, tradeNo string) (*rsa.PrivateKey, string) {
	t.Helper()
	alipayTestKeyOnce.Do(func() {
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			panic(err)
		}
		alipayTestKey = k
	})
	pubDER, err := x509.MarshalPKIXPublicKey(&alipayTestKey.PublicKey)
	if err != nil {
		t.Fatalf("编码公钥失败: %v", err)
	}
	payment := &model.Payment{
		Type:     "alipay",
		UUID:     "alipay-uuid-fixed-0000000000000",
		Name:     "测试支付宝",
		Currency: model.CurrencyTypeCNY,
		Config: fmt.Sprintf(`{"app_id":"2021000000000000","private_key":%q,"public_key":%q,"pay_type":""}`,
			base64.StdEncoding.EncodeToString(x509.MarshalPKCS1PrivateKey(alipayTestKey)),
			base64.StdEncoding.EncodeToString(pubDER)),
	}
	seedPaymentFixtures(t, payment, userID, initialQuota, orderQuota, tradeNo, model.OrderStatusPending)
	return alipayTestKey, payment.UUID
}

// alipayNotifyRequest 构造一条签好名的 TRADE_SUCCESS 异步通知。签名规则按支付宝文档独立实现：
// 除 sign / sign_type 外的参数按 key 升序以 k=v 用 & 连接，SHA256WithRSA 后 base64。
func alipayNotifyRequest(t *testing.T, priv *rsa.PrivateKey, uuid, tradeNo, gatewayNo string) *http.Request {
	t.Helper()
	params := url.Values{
		"app_id":       {"2021000000000000"},
		"out_trade_no": {tradeNo},
		"trade_no":     {gatewayNo},
		"trade_status": {"TRADE_SUCCESS"},
		"total_amount": {"1.00"},
		"sign_type":    {"RSA2"},
	}
	pairs := make([]string, 0, len(params))
	for k, vs := range params {
		if k != "sign_type" {
			pairs = append(pairs, k+"="+vs[0])
		}
	}
	sort.Strings(pairs)
	digest := sha256.Sum256([]byte(strings.Join(pairs, "&")))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	params.Set("sign", base64.StdEncoding.EncodeToString(sig))

	req := httptest.NewRequest("POST", "/api/payment/notify/"+uuid, strings.NewReader(params.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

// TestPaymentCallbackAlipayRetriesAfterQuotaFailure 支付宝通知验签通过后，加额度失败时订单必须仍是
// pending，并回 failure 让支付宝重发（入账前就回 success 是此前的问题）；故障消失后重发的通知
// 恰好入账一次，之后再重发只确认不入账。
func TestPaymentCallbackAlipayRetriesAfterQuotaFailure(t *testing.T) {
	setupEpayCallbackTestDB(t)

	const userID = 61
	const initialQuota = 1000
	const orderQuota = 5000
	const tradeNo = "alipay-credit-failure"
	const gatewayNo = "2026092922001400000000000001"

	priv, uuid := seedAlipayFixtures(t, userID, initialQuota, orderQuota, tradeNo)

	heal := failUserQuotaUpdates(t)
	if body := callPaymentCallback(t, alipayNotifyRequest(t, priv, uuid, tradeNo, gatewayNo)).Body.String(); body != "failure" {
		t.Fatalf("入账失败应响应 failure 让支付宝重发, 实际 %q", body)
	}
	assertTopupState(t, userID, tradeNo, model.OrderStatusPending, initialQuota, 0)

	heal()
	for i := 1; i <= 2; i++ {
		if body := callPaymentCallback(t, alipayNotifyRequest(t, priv, uuid, tradeNo, gatewayNo)).Body.String(); body != "success" {
			t.Fatalf("第 %d 次重发应响应 success, 实际 %q", i, body)
		}
		assertTopupState(t, userID, tradeNo, model.OrderStatusSuccess, initialQuota+orderQuota, 1)
	}
}

// TestPaymentCallbackAlipayUnknownOrderNotAcked 验签通过但按单号找不到订单：没有入账，
// 不能回 success，否则支付宝不再重发。
func TestPaymentCallbackAlipayUnknownOrderNotAcked(t *testing.T) {
	setupEpayCallbackTestDB(t)

	priv, uuid := seedAlipayFixtures(t, 62, 1000, 5000, "alipay-known-order")
	req := alipayNotifyRequest(t, priv, uuid, "alipay-unknown-order", "2026092922001400000000000002")
	if body := callPaymentCallback(t, req).Body.String(); body != "failure" {
		t.Fatalf("找不到订单应响应 failure, 实际 %q", body)
	}
	assertTopupState(t, 62, "alipay-known-order", model.OrderStatusPending, 1000, 0)
}

// TestPaymentCallbackUnknownGatewayNotAcked 回调地址里的网关查不到（含查库出错）时必须回非 2xx：
// Stripe 与微信支付把 2xx 当作已送达，不会再重发。
func TestPaymentCallbackUnknownGatewayNotAcked(t *testing.T) {
	setupEpayCallbackTestDB(t)

	w := callPaymentCallback(t, httptest.NewRequest("POST", "/api/payment/notify/no-such-gateway", nil))
	if w.Code >= 200 && w.Code < 300 {
		t.Fatalf("网关不存在不应回 2xx, 实际 %d", w.Code)
	}
}

const stripeTestWebhookSecret = "whsec_test_secret"

// seedStripeFixtures 写入 Stripe 网关、一个用户与一个 pending 订单，返回网关 UUID。
func seedStripeFixtures(t *testing.T, userID, initialQuota, orderQuota int, tradeNo string) string {
	t.Helper()
	payment := &model.Payment{
		Type:     "stripe",
		UUID:     "stripe-uuid-fixed-0000000000000",
		Name:     "测试 Stripe",
		Currency: model.CurrencyTypeUSD,
		Config:   fmt.Sprintf(`{"secret_key":"sk_test_x","webhook_secret":%q}`, stripeTestWebhookSecret),
	}
	seedPaymentFixtures(t, payment, userID, initialQuota, orderQuota, tradeNo, model.OrderStatusPending)
	return payment.UUID
}

func stripeCompletedEvent(tradeNo, paymentIntentID string) []byte {
	return []byte(fmt.Sprintf(`{"id":"evt_1","object":"event","api_version":"`+stripe.APIVersion+`","type":"checkout.session.completed","data":{"object":{"id":"cs_test_1","object":"checkout.session","client_reference_id":%q,"payment_intent":%q}}}`, tradeNo, paymentIntentID))
}

// stripeEventRequest 用 stripe-go 官方测试辅助为 payload 生成合法的 Stripe-Signature 头。
func stripeEventRequest(uuid string, payload []byte) *http.Request {
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: payload, Secret: stripeTestWebhookSecret})
	req := httptest.NewRequest("POST", "/api/payment/notify/"+uuid, bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", signed.Header)
	return req
}

// TestPaymentCallbackStripeRetriesAfterQuotaFailure Stripe 只在入账提交后收到 2xx；加额度失败回 5xx
// 让 Stripe 重发该事件，故障消失后重发恰好入账一次，之后再重发只确认不入账。
func TestPaymentCallbackStripeRetriesAfterQuotaFailure(t *testing.T) {
	setupEpayCallbackTestDB(t)

	const userID = 71
	const initialQuota = 1000
	const orderQuota = 5000
	const tradeNo = "stripe-credit-failure"

	uuid := seedStripeFixtures(t, userID, initialQuota, orderQuota, tradeNo)
	payload := stripeCompletedEvent(tradeNo, "pi_credit_failure")

	heal := failUserQuotaUpdates(t)
	if code := callPaymentCallback(t, stripeEventRequest(uuid, payload)).Code; code != http.StatusInternalServerError {
		t.Fatalf("入账失败应响应 500 让 Stripe 重发, 实际 %d", code)
	}
	assertTopupState(t, userID, tradeNo, model.OrderStatusPending, initialQuota, 0)

	heal()
	for i := 1; i <= 2; i++ {
		if code := callPaymentCallback(t, stripeEventRequest(uuid, payload)).Code; code != http.StatusOK {
			t.Fatalf("第 %d 次重发应响应 200, 实际 %d", i, code)
		}
		assertTopupState(t, userID, tradeNo, model.OrderStatusSuccess, initialQuota+orderQuota, 1)
	}
}

// TestPaymentCallbackStripeIgnoredEventAcked 验签通过的其它事件类型没有 PayNotify：不入账，直接 2xx 确认。
func TestPaymentCallbackStripeIgnoredEventAcked(t *testing.T) {
	setupEpayCallbackTestDB(t)

	uuid := seedStripeFixtures(t, 72, 1000, 5000, "stripe-ignored-event")
	payload := []byte(`{"id":"evt_2","object":"event","api_version":"` + stripe.APIVersion + `","type":"payment_intent.succeeded","data":{"object":{"id":"pi_999","object":"payment_intent"}}}`)
	if code := callPaymentCallback(t, stripeEventRequest(uuid, payload)).Code; code != http.StatusOK {
		t.Fatalf("无需入账的事件应响应 200, 实际 %d", code)
	}
	assertTopupState(t, 72, "stripe-ignored-event", model.OrderStatusPending, 1000, 0)
}
