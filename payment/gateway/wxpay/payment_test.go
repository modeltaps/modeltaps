package wxpay

// 说明（正路径豁免理由）：微信支付 APIv3 回调的验签正路径依赖微信平台证书链——
// SDK 的 SHA256WithRSAVerifier 从全局 downloader.MgrInstance() 取平台证书公钥验签，
// 且通知体 resource 为 AES-256-GCM 密文。要构造"验签通过"的用例需向全局单例注册
// 伪造平台证书并重造整套签名/加密报文，成本高且污染全局状态，故本文件只锁定
// 拒绝面（fail-closed）语义：配置解析失败与验签失败都必须 400 FAIL 且不产生 PayNotify。

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newWxpayNotifyContext(t *testing.T, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("POST", "/api/payment/notify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	return c, w
}

const wxpayTestConfig = `{"app_id":"wx1234567890","mch_id":"1900000000","mch_certificate_serial_number":"TESTSN","mch_apiv3_key":"0123456789abcdef0123456789abcdef","mch_private_key":"","notify_url":"","pay_type":"Native"}`

// 网关配置 JSON 解析失败 → 400 + FAIL 应答，且不产生 PayNotify。
func TestWxpayHandleCallback_BadGatewayConfig(t *testing.T) {
	c, w := newWxpayNotifyContext(t, `{}`)
	notify, err := (&WeChatPay{}).HandleCallback(c, "{not-json")
	if err == nil || notify != nil {
		t.Fatalf("配置解析失败必须拒绝, notify=%v err=%v", notify, err)
	}
	if w.Code != 400 {
		t.Fatalf("应返回 400，got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"FAIL"`) {
		t.Fatalf("应答应为 FAIL，got %q", w.Body.String())
	}
}

// 缺失微信签名头（Wechatpay-Signature/Serial/Timestamp/Nonce）→ 验签失败，400 + FAIL。
func TestWxpayHandleCallback_MissingSignatureHeadersRejected(t *testing.T) {
	body := `{"id":"evt-1","event_type":"TRANSACTION.SUCCESS","resource_type":"encrypt-resource","resource":{"algorithm":"AEAD_AES_256_GCM","ciphertext":"","nonce":"","associated_data":""}}`
	c, w := newWxpayNotifyContext(t, body)
	notify, err := (&WeChatPay{}).HandleCallback(c, wxpayTestConfig)
	if err == nil || notify != nil {
		t.Fatalf("无签名头必须拒绝, notify=%v err=%v", notify, err)
	}
	if !strings.Contains(err.Error(), "Signature verification failed") {
		t.Fatalf("应为验签失败错误: %v", err)
	}
	if w.Code != 400 {
		t.Fatalf("应返回 400，got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"FAIL"`) {
		t.Fatalf("应答应为 FAIL，got %q", w.Body.String())
	}
}

// 伪造签名头（平台证书序列号在本地证书管理器中不存在）→ 验签失败，400 + FAIL。
func TestWxpayHandleCallback_ForgedSignatureRejected(t *testing.T) {
	body := `{"id":"evt-1","event_type":"TRANSACTION.SUCCESS","resource_type":"encrypt-resource","resource":{"algorithm":"AEAD_AES_256_GCM","ciphertext":"","nonce":"","associated_data":""}}`
	c, w := newWxpayNotifyContext(t, body)
	c.Request.Header.Set("Wechatpay-Signature", "Zm9yZ2VkLXNpZ25hdHVyZQ==")
	c.Request.Header.Set("Wechatpay-Serial", "FAKE-PLATFORM-SERIAL")
	c.Request.Header.Set("Wechatpay-Timestamp", "1755302400")
	c.Request.Header.Set("Wechatpay-Nonce", "nonce123")
	c.Request.Header.Set("Wechatpay-Signature-Type", "WECHATPAY2-SHA256-RSA2048")

	notify, err := (&WeChatPay{}).HandleCallback(c, wxpayTestConfig)
	if err == nil || notify != nil {
		t.Fatalf("伪造签名必须拒绝, notify=%v err=%v", notify, err)
	}
	if w.Code != 400 {
		t.Fatalf("应返回 400，got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"FAIL"`) {
		t.Fatalf("应答应为 FAIL，got %q", w.Body.String())
	}
}

// RespondCallback：入账成功回 204 无应答体，微信支付停止重发；
// 入账失败回 5XX + FAIL 应答报文，微信支付稍后重发。
func TestWxpayRespondCallback(t *testing.T) {
	c, w := newWxpayNotifyContext(t, `{}`)
	(&WeChatPay{}).RespondCallback(c, true)
	if c.Writer.Status() != 204 || w.Body.Len() != 0 {
		t.Fatalf("成功应答应为 204 无应答体，got %d %q", c.Writer.Status(), w.Body.String())
	}

	c, w = newWxpayNotifyContext(t, `{}`)
	(&WeChatPay{}).RespondCallback(c, false)
	if w.Code != 500 {
		t.Fatalf("失败应答应为 500，got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"FAIL"`) {
		t.Fatalf("失败应答应为 FAIL，got %q", w.Body.String())
	}
}
