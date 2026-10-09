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

	if err := c.verifySignature(params); err != nil {
		t.Fatalf("verifySignature() = %v, want nil for a correctly signed payload", err)
	}
	if err := validateNotifyParams(params); err != nil {
		t.Fatalf("validateNotifyParams() = %v, want nil for a genuine callback payload", err)
	}
}

func TestVerify_TamperedParamAfterSigning(t *testing.T) {
	c := newTestClient()
	params := validSignedPayload(c)

	// Tamper a signed field (out_trade_no) without re-signing.
	params["out_trade_no"] = "ORDER-EVIL-9999"

	if err := c.verifySignature(params); err == nil {
		t.Errorf("verifySignature() = nil, want error for a tampered param")
	}
}

func TestVerify_TamperedMoneyProvesAmountIsSigned(t *testing.T) {
	c := newTestClient()
	params := validSignedPayload(c)

	// Change the amount after signing; if money were outside the signed set this
	// would still verify. It must fail, proving money is inside the digest.
	params["money"] = "999999.99"

	if err := c.verifySignature(params); err == nil {
		t.Errorf("verifySignature() = nil, want error when money is tampered")
	}
}

func TestVerify_EmptyOrMissingSign(t *testing.T) {
	c := newTestClient()

	t.Run("empty sign", func(t *testing.T) {
		params := validSignedPayload(c)
		params["sign"] = ""
		if err := c.verifySignature(params); err == nil {
			t.Errorf("verifySignature() = nil, want error for empty sign")
		}
	})

	t.Run("missing sign", func(t *testing.T) {
		params := validSignedPayload(c)
		delete(params, "sign")
		if err := c.verifySignature(params); err == nil {
			t.Errorf("verifySignature() = nil, want error for missing sign")
		}
	})
}

// A validly signed payment-request payload replayed as a callback must be rejected:
// notify_url / return_url only appear in payment requests, and a value containing
// "&" could smuggle request params past the concatenated signature string.
func TestValidateNotifyParams_RejectsForgedShapes(t *testing.T) {
	c := newTestClient()

	cases := map[string]func(map[string]string){
		"notify_url present": func(p map[string]string) { p["notify_url"] = "https://evil.example.com/notify" },
		"return_url present": func(p map[string]string) { p["return_url"] = "https://evil.example.com/return" },
		"value contains &":   func(p map[string]string) { p["name"] = "x&trade_status=TRADE_SUCCESS" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			params := validSignedPayload(c)
			mutate(params)
			params["sign"] = c.Sign(params)
			if err := validateNotifyParams(params); err == nil {
				t.Errorf("validateNotifyParams() = nil, want error")
			}
		})
	}
}
