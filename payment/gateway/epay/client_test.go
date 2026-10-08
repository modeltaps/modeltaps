package epay

import (
	"crypto/md5"
	"encoding/hex"
	"sort"
	"strings"
	"testing"
)

// newTestClient returns a Client with a fixed key so signatures are deterministic.
func newTestClient() *Client {
	return &Client{
		PayDomain: "https://pay.example.com",
		PartnerID: "1001",
		Key:       "test-secret-key",
	}
}

// referenceSign recomputes the expected signature independently of client.Sign so
// the test does not merely mirror the implementation. It applies the documented
// canonical form: sorted non-empty params (excluding sign/sign_type) joined by "&"
// as k=v pairs, with Key appended, then MD5-hex lowercase.
func referenceSign(args map[string]string, key string) string {
	keys := make([]string, 0, len(args))
	for k, v := range args {
		if k == "sign" || k == "sign_type" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + args[k]
	}
	signStr := strings.Join(parts, "&") + key

	sum := md5.Sum([]byte(signStr))
	return hex.EncodeToString(sum[:])
}

func TestSign_CanonicalForm(t *testing.T) {
	c := newTestClient()

	params := map[string]string{
		"pid":          "1001",
		"out_trade_no": "ORDER-20260701-0001",
		"name":         "test order",
		"money":        "12.34",
		"trade_status": "TRADE_SUCCESS",
		// present but should be excluded from the digest:
		"sign":      "SHOULD_BE_IGNORED",
		"sign_type": "MD5",
		// empty-valued param must be dropped from the digest:
		"return_url": "",
	}

	got := c.Sign(params)
	want := referenceSign(params, c.Key)

	if got != want {
		t.Fatalf("Sign() = %q, want %q", got, want)
	}

	// MD5-hex lowercase: 32 hex chars, no uppercase.
	if len(got) != 32 {
		t.Errorf("Sign() length = %d, want 32", len(got))
	}
	if got != strings.ToLower(got) {
		t.Errorf("Sign() = %q, expected lowercase hex", got)
	}
}

func TestSign_DropsEmptyValuedParams(t *testing.T) {
	c := newTestClient()

	withEmpty := map[string]string{
		"pid":          "1001",
		"out_trade_no": "ORDER-1",
		"money":        "1.00",
		"return_url":   "", // dropped
		"notify_url":   "", // dropped
	}
	withoutEmpty := map[string]string{
		"pid":          "1001",
		"out_trade_no": "ORDER-1",
		"money":        "1.00",
	}

	if c.Sign(withEmpty) != c.Sign(withoutEmpty) {
		t.Errorf("Sign() should ignore empty-valued params, but digests differ")
	}
}

// validSignedPayload builds a TRADE_SUCCESS callback payload and signs it in-test
// via the real Sign(), then attaches sign + sign_type as a real gateway would.
func validSignedPayload(c *Client) map[string]string {
	params := map[string]string{
		"pid":          "1001",
		"trade_no":     "GATEWAY-TXN-777",
		"out_trade_no": "ORDER-20260701-0001",
		"type":         "alipay",
		"name":         "test order",
		"money":        "12.34",
		"trade_status": "TRADE_SUCCESS",
	}
	params["sign"] = c.Sign(params)
	params["sign_type"] = FormArgsSignType
	return params
}

func TestVerify_ValidPayload(t *testing.T) {
	c := newTestClient()
	params := validSignedPayload(c)

	result, ok := c.Verify(params)
	if !ok {
		t.Fatalf("Verify() ok = false, want true for a correctly signed TRADE_SUCCESS payload")
	}
	if result == nil {
		t.Fatalf("Verify() result = nil, want parsed PaymentResult")
	}

	if result.OutTradeNo != "ORDER-20260701-0001" {
		t.Errorf("OutTradeNo = %q, want %q", result.OutTradeNo, "ORDER-20260701-0001")
	}
	if result.TradeNo != "GATEWAY-TXN-777" {
		t.Errorf("TradeNo = %q, want %q", result.TradeNo, "GATEWAY-TXN-777")
	}
	if result.Money != "12.34" {
		t.Errorf("Money = %q, want %q", result.Money, "12.34")
	}
	if result.TradeStatus != TradeStatusSuccess {
		t.Errorf("TradeStatus = %q, want %q", result.TradeStatus, TradeStatusSuccess)
	}
}

func TestVerify_TamperedParamAfterSigning(t *testing.T) {
	c := newTestClient()
	params := validSignedPayload(c)

	// Tamper a signed field (out_trade_no) without re-signing.
	params["out_trade_no"] = "ORDER-EVIL-9999"

	result, ok := c.Verify(params)
	if ok || result != nil {
		t.Errorf("Verify() = (%v, %v), want (nil, false) for a tampered param", result, ok)
	}
}

func TestVerify_TamperedMoneyProvesAmountIsSigned(t *testing.T) {
	c := newTestClient()
	params := validSignedPayload(c)

	// Change the amount after signing; if money were outside the signed set this
	// would still verify. It must fail, proving money is inside the digest.
	params["money"] = "999999.99"

	result, ok := c.Verify(params)
	if ok || result != nil {
		t.Errorf("Verify() = (%v, %v), want (nil, false) when money is tampered", result, ok)
	}
}

func TestVerify_NonSuccessTradeStatus(t *testing.T) {
	c := newTestClient()

	// Build an otherwise-correct payload but with a failure status, signed correctly.
	params := map[string]string{
		"pid":          "1001",
		"trade_no":     "GATEWAY-TXN-778",
		"out_trade_no": "ORDER-2",
		"money":        "5.00",
		"trade_status": "TRADE_ERROR",
	}
	params["sign"] = c.Sign(params)
	params["sign_type"] = FormArgsSignType

	// Sanity: the sign itself is valid, so failure must be due to trade_status.
	if params["sign"] != c.Sign(params) {
		t.Fatalf("precondition failed: sign should be self-consistent")
	}

	result, ok := c.Verify(params)
	if ok || result != nil {
		t.Errorf("Verify() = (%v, %v), want (nil, false) for trade_status != TRADE_SUCCESS", result, ok)
	}
}

func TestVerify_EmptyOrMissingSign(t *testing.T) {
	c := newTestClient()

	t.Run("empty sign", func(t *testing.T) {
		params := validSignedPayload(c)
		params["sign"] = ""
		if result, ok := c.Verify(params); ok || result != nil {
			t.Errorf("Verify() = (%v, %v), want (nil, false) for empty sign", result, ok)
		}
	})

	t.Run("missing sign", func(t *testing.T) {
		params := validSignedPayload(c)
		delete(params, "sign")
		if result, ok := c.Verify(params); ok || result != nil {
			t.Errorf("Verify() = (%v, %v), want (nil, false) for missing sign", result, ok)
		}
	})
}
