package alipay

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

var (
	testKeyOnce sync.Once
	testPrivKey *rsa.PrivateKey
)

// testKeys 懒生成一对 RSA 密钥（模拟"支付宝方"密钥对：私钥签通知、公钥配给网关验签）。
func testKeys(t *testing.T) (priv *rsa.PrivateKey, privB64, pubB64 string) {
	t.Helper()
	testKeyOnce.Do(func() {
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			panic(err)
		}
		testPrivKey = k
	})
	priv = testPrivKey
	privB64 = base64.StdEncoding.EncodeToString(x509.MarshalPKCS1PrivateKey(priv))
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("编码公钥失败: %v", err)
	}
	return priv, privB64, base64.StdEncoding.EncodeToString(pubDER)
}

// signNotifyParams 独立复刻支付宝异步通知签名规则（升序 k=v 以 & 连接，排除
// sign/sign_type/alipay_cert_sn，SHA256WithRSA PKCS1v15 后 base64），
// 不复用被测实现，避免测试镜像实现。
func signNotifyParams(t *testing.T, priv *rsa.PrivateKey, params url.Values) string {
	t.Helper()
	pairs := make([]string, 0, len(params))
	for k, vs := range params {
		if k == "sign" || k == "sign_type" || k == "alipay_cert_sn" {
			continue
		}
		for _, v := range vs {
			pairs = append(pairs, k+"="+v)
		}
	}
	sort.Strings(pairs)
	digest := sha256.Sum256([]byte(strings.Join(pairs, "&")))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	return base64.StdEncoding.EncodeToString(sig)
}

func alipayGatewayConfig(privB64, pubB64 string) string {
	return fmt.Sprintf(`{"app_id":"2021000000000000","private_key":%q,"public_key":%q,"pay_type":""}`, privB64, pubB64)
}

func newAlipayNotifyContext(t *testing.T, form url.Values) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("POST", "/api/payment/notify", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Request = req
	return c, w
}

func successNotifyParams() url.Values {
	return url.Values{
		"app_id":       {"2021000000000000"},
		"out_trade_no": {"CAP-TRADE-1"},
		"trade_no":     {"2026081622001400000000000001"},
		"trade_status": {"TRADE_SUCCESS"},
		"total_amount": {"12.34"},
		"sign_type":    {"RSA2"},
	}
}

// 网关配置 JSON 解析失败 → 拒绝并响应 failure。
func TestAlipayHandleCallback_BadGatewayConfig(t *testing.T) {
	c, w := newAlipayNotifyContext(t, successNotifyParams())
	notify, err := (&Alipay{}).HandleCallback(c, "{not-json")
	if err == nil || notify != nil {
		t.Fatalf("配置解析失败必须拒绝, notify=%v err=%v", notify, err)
	}
	if w.Body.String() != "failure" {
		t.Fatalf("应响应 failure，got %q", w.Body.String())
	}
}

// 配置里私钥非法 → 创建客户端失败，拒绝并响应 failure。
func TestAlipayHandleCallback_BadPrivateKey(t *testing.T) {
	c, w := newAlipayNotifyContext(t, successNotifyParams())
	notify, err := (&Alipay{}).HandleCallback(c, `{"app_id":"x","private_key":"bm90LWEta2V5","public_key":"bm90LWEta2V5","pay_type":""}`)
	if err == nil || notify != nil {
		t.Fatalf("非法密钥必须拒绝, notify=%v err=%v", notify, err)
	}
	if w.Body.String() != "failure" {
		t.Fatalf("应响应 failure，got %q", w.Body.String())
	}
}

// 缺失/伪造签名 → 验签失败，拒绝并响应 failure。
func TestAlipayHandleCallback_SignatureRejected(t *testing.T) {
	_, privB64, pubB64 := testKeys(t)
	cfg := alipayGatewayConfig(privB64, pubB64)

	cases := map[string]func(url.Values){
		"缺失签名": func(p url.Values) {},
		"伪造签名": func(p url.Values) {
			p.Set("sign", base64.StdEncoding.EncodeToString([]byte("forged-signature")))
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			params := successNotifyParams()
			mutate(params)
			c, w := newAlipayNotifyContext(t, params)
			notify, err := (&Alipay{}).HandleCallback(c, cfg)
			if err == nil || notify != nil {
				t.Fatalf("非法签名必须拒绝, notify=%v err=%v", notify, err)
			}
			if !strings.Contains(err.Error(), "Signature verification failed") {
				t.Fatalf("应为验签失败错误: %v", err)
			}
			if w.Body.String() != "failure" {
				t.Fatalf("应响应 failure，got %q", w.Body.String())
			}
		})
	}
}

// RSA 正签名 + TRADE_SUCCESS → 通过验签，PayNotify 字段正确，且此时还没有应答：
// 入账前就回 success，入账失败时支付宝也不会再重发。
func TestAlipayHandleCallback_ValidSignatureSuccess(t *testing.T) {
	priv, privB64, pubB64 := testKeys(t)
	params := successNotifyParams()
	params.Set("sign", signNotifyParams(t, priv, params))

	c, w := newAlipayNotifyContext(t, params)
	notify, err := (&Alipay{}).HandleCallback(c, alipayGatewayConfig(privB64, pubB64))
	if err != nil {
		t.Fatalf("正签名应通过验签: %v", err)
	}
	if notify == nil {
		t.Fatal("TRADE_SUCCESS 应返回 PayNotify")
	}
	if notify.TradeNo != "CAP-TRADE-1" {
		t.Fatalf("TradeNo 应取 out_trade_no，got %q", notify.TradeNo)
	}
	if notify.GatewayNo != "2026081622001400000000000001" {
		t.Fatalf("GatewayNo 应取 trade_no，got %q", notify.GatewayNo)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("入账前不应应答支付宝，got %q", w.Body.String())
	}
}

// 通知带未加载的 alipay_cert_sn → SDK 转去下载支付宝证书，且必须用请求 ctx：
// 请求已取消时不发出网络请求，即使签名本身正确也验签失败并响应 failure。
func TestAlipayHandleCallback_UnknownCertSNUsesRequestContext(t *testing.T) {
	priv, privB64, pubB64 := testKeys(t)
	params := successNotifyParams()
	params.Set("alipay_cert_sn", "unknown-cert-sn")
	params.Set("sign", signNotifyParams(t, priv, params))

	c, w := newAlipayNotifyContext(t, params)
	ctx, cancel := context.WithCancel(c.Request.Context())
	cancel()
	c.Request = c.Request.WithContext(ctx)

	notify, err := (&Alipay{}).HandleCallback(c, alipayGatewayConfig(privB64, pubB64))
	if err == nil || notify != nil {
		t.Fatalf("证书下载失败必须拒绝, notify=%v err=%v", notify, err)
	}
	if !strings.Contains(err.Error(), "Signature verification failed") ||
		!strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("应为请求取消导致的验签失败: %v", err)
	}
	if w.Body.String() != "failure" {
		t.Fatalf("应响应 failure，got %q", w.Body.String())
	}
}

// 正签名后篡改参数 → 验签必须失败。
func TestAlipayHandleCallback_TamperedParamRejected(t *testing.T) {
	priv, privB64, pubB64 := testKeys(t)
	params := successNotifyParams()
	params.Set("sign", signNotifyParams(t, priv, params))
	params.Set("out_trade_no", "CAP-TRADE-EVIL")

	c, w := newAlipayNotifyContext(t, params)
	notify, err := (&Alipay{}).HandleCallback(c, alipayGatewayConfig(privB64, pubB64))
	if err == nil || notify != nil {
		t.Fatalf("篡改参数必须拒绝, notify=%v err=%v", notify, err)
	}
	if w.Body.String() != "failure" {
		t.Fatalf("应响应 failure，got %q", w.Body.String())
	}
}

// 正签名但交易状态非成功 → 拒绝并响应 failure。
func TestAlipayHandleCallback_TradeStatusNotSuccess(t *testing.T) {
	priv, privB64, pubB64 := testKeys(t)
	params := successNotifyParams()
	params.Set("trade_status", "WAIT_BUYER_PAY")
	params.Set("sign", signNotifyParams(t, priv, params))

	c, w := newAlipayNotifyContext(t, params)
	notify, err := (&Alipay{}).HandleCallback(c, alipayGatewayConfig(privB64, pubB64))
	if err == nil || notify != nil {
		t.Fatalf("非成功状态必须拒绝, notify=%v err=%v", notify, err)
	}
	if !strings.Contains(err.Error(), "trade status not success") {
		t.Fatalf("应为状态错误: %v", err)
	}
	if w.Body.String() != "failure" {
		t.Fatalf("应响应 failure，got %q", w.Body.String())
	}
}

// RespondCallback：入账成功回 success，支付宝停止重发；入账失败回 failure，支付宝稍后重发。
func TestAlipayRespondCallback(t *testing.T) {
	for _, tc := range []struct {
		success bool
		want    string
	}{
		{success: true, want: "success"},
		{success: false, want: "failure"},
	} {
		c, w := newAlipayNotifyContext(t, successNotifyParams())
		(&Alipay{}).RespondCallback(c, tc.success)
		if w.Code != 200 || w.Body.String() != tc.want {
			t.Fatalf("RespondCallback(%v) 应响应 200 %q，got %d %q", tc.success, tc.want, w.Code, w.Body.String())
		}
	}
}
