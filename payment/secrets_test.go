package payment

import (
	"encoding/json"
	"reflect"
	"testing"
)

func decode(t *testing.T, raw string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("invalid json %q: %v", raw, err)
	}
	return out
}

func TestMaskConfigMasksOnlySecretFields(t *testing.T) {
	raw := `{"secret_key":"dummy-secret-0123456789abcdefghijklmnop","webhook_secret":"dummy-webhook-abcdefghijklmnopqrstuvwxyz","pay_type":"card"}`
	got := decode(t, MaskConfig("stripe", raw))
	want := map[string]string{
		"secret_key":     "dumm****mnop",
		"webhook_secret": "dumm****wxyz",
		"pay_type":       "card",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMaskConfigPassThrough(t *testing.T) {
	cases := map[string]struct{ gatewayType, raw string }{
		"unknown gateway":  {"unknown", `{"key":"0123456789abcdefghij"}`},
		"empty config":     {"epay", ""},
		"invalid json":     {"epay", `{not json`},
		"empty secret":     {"epay", `{"key":"","partner_id":"1001"}`},
		"missing secret":   {"wxpay", `{"app_id":"wx1"}`},
		"non-string value": {"epay", `{"key":123}`},
	}
	for name, tc := range cases {
		if got := MaskConfig(tc.gatewayType, tc.raw); got != tc.raw {
			t.Errorf("%s: got %q, want unchanged %q", name, got, tc.raw)
		}
	}
}

func TestMergeConfigSecretsKeepsStoredWhenBlankOrMasked(t *testing.T) {
	stored := `{"app_id":"wx1","mch_id":"m1","mch_apiv3_key":"apiv3-0123456789abcdefghijklmn","mch_private_key":"-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----","pay_type":"Native"}`
	masked := decode(t, MaskConfig("wxpay", stored))
	submitted := `{"app_id":"wx2","mch_id":"m1","mch_apiv3_key":"","mch_private_key":"` + masked["mch_private_key"] + `","pay_type":"Native"}`

	got := decode(t, MergeConfigSecrets("wxpay", submitted, stored))
	want := decode(t, stored)
	want["app_id"] = "wx2"
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMergeConfigSecretsTakesNewValue(t *testing.T) {
	stored := `{"key":"old-key-0123456789abcdef","partner_id":"1001","pay_domain":"https://pay.example.com"}`
	submitted := `{"key":"new-key-0123456789abcdef","partner_id":"1001","pay_domain":"https://pay.example.com"}`
	if got := MergeConfigSecrets("epay", submitted, stored); got != submitted {
		t.Fatalf("got %q, want submitted unchanged", got)
	}
}

func TestMergeConfigSecretsPassThrough(t *testing.T) {
	cases := map[string]struct{ gatewayType, submitted, stored string }{
		"unknown gateway": {"unknown", `{"key":""}`, `{"key":"stored-0123456789abcdef"}`},
		"nothing stored":  {"epay", `{"key":""}`, ""},
		"stored blank":    {"epay", `{"key":""}`, `{"key":""}`},
		"type switched":   {"stripe", `{"secret_key":""}`, `{"key":"epay-0123456789abcdef"}`},
	}
	for name, tc := range cases {
		if got := MergeConfigSecrets(tc.gatewayType, tc.submitted, tc.stored); got != tc.submitted {
			t.Errorf("%s: got %q, want submitted unchanged %q", name, got, tc.submitted)
		}
	}
}
