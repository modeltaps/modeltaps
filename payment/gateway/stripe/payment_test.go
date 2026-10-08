package stripe

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"
)

const testWebhookSecret = "whsec_test_secret"

func init() {
	gin.SetMode(gin.TestMode)
}

func testGatewayConfig() string {
	return fmt.Sprintf(`{"secret_key":"sk_test_x","webhook_secret":%q}`, testWebhookSecret)
}

func completedEventPayload(tradeNo, paymentIntentID string) []byte {
	return []byte(fmt.Sprintf(`{"id":"evt_1","object":"event","api_version":"`+stripe.APIVersion+`","type":"checkout.session.completed","data":{"object":{"id":"cs_test_1","object":"checkout.session","client_reference_id":%q,"payment_intent":%q}}}`, tradeNo, paymentIntentID))
}

// signHeader 用 stripe-go 官方测试辅助构造合法的 Stripe-Signature 头。
func signHeader(payload []byte, secret string) string {
	sp := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: payload, Secret: secret})
	return sp.Header
}

func newStripeCallbackContext(t *testing.T, body []byte, signature string) *gin.Context {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("POST", "/api/payment/notify", bytes.NewReader(body))
	if signature != "" {
		req.Header.Set("Stripe-Signature", signature)
	}
	c.Request = req
	return c
}

// 合法签名 + checkout.session.completed → PayNotify 字段正确映射。
func TestStripeHandleCallback_ValidSignatureCompleted(t *testing.T) {
	payload := completedEventPayload("CAP-TRADE-1", "pi_123")
	c := newStripeCallbackContext(t, payload, signHeader(payload, testWebhookSecret))

	notify, err := (&Stripe{}).HandleCallback(c, testGatewayConfig())
	if err != nil {
		t.Fatalf("合法签名应通过验签: %v", err)
	}
	if notify == nil {
		t.Fatal("completed 事件应返回 PayNotify")
	}
	if notify.TradeNo != "CAP-TRADE-1" {
		t.Fatalf("TradeNo 应取 client_reference_id，got %q", notify.TradeNo)
	}
	if notify.GatewayNo != "pi_123" {
		t.Fatalf("GatewayNo 应取 payment_intent ID，got %q", notify.GatewayNo)
	}
	if c.Writer.Written() || c.Writer.Status() != http.StatusOK {
		t.Fatalf("入账前不应应答 Stripe，got status %d", c.Writer.Status())
	}
}

// RespondCallback：入账成功回 2xx，Stripe 停止重发；入账失败回 5xx，Stripe 稍后重发该事件。
func TestStripeRespondCallback(t *testing.T) {
	for _, tc := range []struct {
		success bool
		want    int
	}{
		{success: true, want: http.StatusOK},
		{success: false, want: http.StatusInternalServerError},
	} {
		c := newStripeCallbackContext(t, nil, "")
		(&Stripe{}).RespondCallback(c, tc.success)
		if got := c.Writer.Status(); got != tc.want {
			t.Fatalf("RespondCallback(%v) 应响应 %d，got %d", tc.success, tc.want, got)
		}
	}
}

// 签名对应另一份 body（篡改 body）→ 必须拒绝。
func TestStripeHandleCallback_TamperedBodyRejected(t *testing.T) {
	original := completedEventPayload("CAP-TRADE-1", "pi_123")
	tampered := completedEventPayload("CAP-TRADE-EVIL", "pi_123")
	c := newStripeCallbackContext(t, tampered, signHeader(original, testWebhookSecret))

	notify, err := (&Stripe{}).HandleCallback(c, testGatewayConfig())
	if err == nil || notify != nil {
		t.Fatalf("篡改 body 必须拒绝, notify=%v err=%v", notify, err)
	}
	if !strings.Contains(err.Error(), "failed to verify webhook") {
		t.Fatalf("应为验签失败错误: %v", err)
	}
}

// 伪造/损坏的签名头 → 必须拒绝。
func TestStripeHandleCallback_BadSignatureHeaderRejected(t *testing.T) {
	payload := completedEventPayload("CAP-TRADE-1", "pi_123")
	for name, sig := range map[string]string{
		"空签名头":  "",
		"垃圾签名头": "t=123,v1=deadbeef",
	} {
		t.Run(name, func(t *testing.T) {
			c := newStripeCallbackContext(t, payload, sig)
			notify, err := (&Stripe{}).HandleCallback(c, testGatewayConfig())
			if err == nil || notify != nil {
				t.Fatalf("非法签名必须拒绝, notify=%v err=%v", notify, err)
			}
		})
	}
}

// WebhookSecret 未配置 → fail-closed，即使 body 是合法事件也拒绝。
func TestStripeHandleCallback_EmptyWebhookSecretFailClosed(t *testing.T) {
	payload := completedEventPayload("CAP-TRADE-1", "pi_123")
	c := newStripeCallbackContext(t, payload, signHeader(payload, testWebhookSecret))

	notify, err := (&Stripe{}).HandleCallback(c, `{"secret_key":"sk_test_x","webhook_secret":""}`)
	if err == nil || notify != nil {
		t.Fatalf("WebhookSecret 为空必须拒绝, notify=%v err=%v", notify, err)
	}
	if !strings.Contains(err.Error(), "webhook secret is not configured") {
		t.Fatalf("应为 fail-closed 错误: %v", err)
	}
}

// 网关配置 JSON 解析失败 → 拒绝。
func TestStripeHandleCallback_BadGatewayConfigRejected(t *testing.T) {
	payload := completedEventPayload("CAP-TRADE-1", "pi_123")
	c := newStripeCallbackContext(t, payload, signHeader(payload, testWebhookSecret))

	notify, err := (&Stripe{}).HandleCallback(c, "{not-json")
	if err == nil || notify != nil {
		t.Fatalf("配置解析失败必须拒绝, notify=%v err=%v", notify, err)
	}
}

// 合法签名但事件类型不是 checkout.session.completed → nil, nil（忽略且不报错）。
func TestStripeHandleCallback_NonCompletedEventIgnored(t *testing.T) {
	payload := []byte(`{"id":"evt_2","object":"event","api_version":"` + stripe.APIVersion + `","type":"payment_intent.succeeded","data":{"object":{"id":"pi_999","object":"payment_intent"}}}`)
	c := newStripeCallbackContext(t, payload, signHeader(payload, testWebhookSecret))

	notify, err := (&Stripe{}).HandleCallback(c, testGatewayConfig())
	if err != nil {
		t.Fatalf("非 completed 事件不应报错: %v", err)
	}
	if notify != nil {
		t.Fatalf("非 completed 事件不应产生 PayNotify: %+v", notify)
	}
}
