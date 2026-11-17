package base

import (
	"testing"

	"github.com/modeltaps/modeltaps/model"

	"github.com/tidwall/gjson"
)

func TestStripPremiumTierFields(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		rules []PremiumTierRule
		keep  map[string]bool
	}{
		{"openai priority", `{"model":"m","service_tier":"priority"}`, OpenAIPremiumTierRules, map[string]bool{"service_tier": false}},
		{"openai fast", `{"model":"m","service_tier":"fast"}`, OpenAIPremiumTierRules, map[string]bool{"service_tier": false}},
		{"openai ultrafast", `{"model":"m","service_tier":"ultrafast"}`, OpenAIPremiumTierRules, map[string]bool{"service_tier": false}},
		{"openai unknown value", `{"model":"m","service_tier":"turbo"}`, OpenAIPremiumTierRules, map[string]bool{"service_tier": false}},
		{"openai auto", `{"model":"m","service_tier":"auto"}`, OpenAIPremiumTierRules, map[string]bool{"service_tier": true}},
		{"openai flex", `{"model":"m","service_tier":"flex"}`, OpenAIPremiumTierRules, map[string]bool{"service_tier": true}},
		{"openai default upper case", `{"model":"m","service_tier":"Default"}`, OpenAIPremiumTierRules, map[string]bool{"service_tier": true}},
		{"claude fast and us", `{"model":"m","speed":"fast","inference_geo":"us","service_tier":"auto"}`, ClaudePremiumTierRules,
			map[string]bool{"speed": false, "inference_geo": false, "service_tier": true}},
		{"claude standard and global", `{"model":"m","speed":"standard","inference_geo":"global"}`, ClaudePremiumTierRules,
			map[string]bool{"speed": true, "inference_geo": true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := StripPremiumTierFields([]byte(tc.body), tc.rules)
			if !gjson.GetBytes(out, "model").Exists() {
				t.Fatalf("other fields dropped: %s", out)
			}
			for field, keep := range tc.keep {
				if gjson.GetBytes(out, field).Exists() != keep {
					t.Errorf("%s kept=%v, want %v (%s)", field, !keep, keep, out)
				}
			}
		})
	}
}

func TestBaseProviderStripPremiumTierRespectsChannel(t *testing.T) {
	body := []byte(`{"model":"m","speed":"fast"}`)

	p := &BaseProvider{Channel: &model.Channel{}}
	if gjson.GetBytes(p.StripPremiumTierFields(body, ClaudePremiumTierRules), "speed").Exists() {
		t.Fatal("speed forwarded on a channel that does not allow premium tiers")
	}

	p.Channel.AllowServiceTier = true
	if gjson.GetBytes(p.StripPremiumTierFields(body, ClaudePremiumTierRules), "speed").String() != "fast" {
		t.Fatal("speed dropped on a channel that allows premium tiers")
	}

	m := map[string]any{"model": "m", "service_tier": "priority"}
	p.Channel.AllowServiceTier = false
	p.StripPremiumTierMap(m, OpenAIPremiumTierRules)
	if _, ok := m["service_tier"]; ok {
		t.Fatal("service_tier kept in the decoded body")
	}
}

// gjson and sjson only see the first of duplicate keys, while an upstream parser may take the
// last one, so a duplicate must not let a premium value through (SEC-81).
func TestStripPremiumTierFieldsDuplicateKeys(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"standard then fast", `{"model":"m","speed":"standard","speed":"fast"}`, `{"model":"m"}`},
		{"fast twice", `{"model":"m","speed":"fast","speed":"fast"}`, `{"model":"m"}`},
		{"standard twice", `{"speed":"standard","model":"m","speed":"standard"}`, `{"model":"m"}`},
		{"escaped key", `{"model":"m","sp\u0065ed":"fast"}`, `{"model":"m"}`},
		{"other case", `{"model":"m","Speed":"fast","inference_geo":"global"}`, `{"model":"m","inference_geo":"global"}`},
		{"geo duplicate keeps speed", `{"inference_geo":"global","speed":"standard","inference_geo":"us"}`, `{"speed":"standard"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := string(StripPremiumTierFields([]byte(tc.body), ClaudePremiumTierRules))
			if out != tc.want {
				t.Fatalf("got %s, want %s", out, tc.want)
			}
		})
	}
}

// A body that needs no change keeps its bytes, and a rebuilt one keeps the other members'
// raw values and order.
func TestStripPremiumTierFieldsKeepsBytes(t *testing.T) {
	body := `{ "model": "m",  "speed": "standard", "metadata": {"user_id": "u"} }`
	if out := string(StripPremiumTierFields([]byte(body), ClaudePremiumTierRules)); out != body {
		t.Fatalf("unchanged body rewritten: %s", out)
	}

	body = `{"model":"m","max_tokens":1.0e3,"speed":"fast","metadata":{ "user_id" : "u" },"stream":true}`
	want := `{"model":"m","max_tokens":1.0e3,"metadata":{ "user_id" : "u" },"stream":true}`
	if out := string(StripPremiumTierFields([]byte(body), ClaudePremiumTierRules)); out != want {
		t.Fatalf("got %s, want %s", out, want)
	}

	for _, notObject := range []string{`[{"speed":"fast"}]`, `"speed"`, ``} {
		if out := string(StripPremiumTierFields([]byte(notObject), ClaudePremiumTierRules)); out != notObject {
			t.Fatalf("non-object body changed: %q -> %q", notObject, out)
		}
	}
}

func TestStripPremiumTierMapCaseVariants(t *testing.T) {
	p := &BaseProvider{Channel: &model.Channel{}}
	m := map[string]any{"model": "m", "service_tier": "auto", "Service_Tier": "priority"}
	p.StripPremiumTierMap(m, OpenAIPremiumTierRules)
	if len(m) != 1 {
		t.Fatalf("case variants of service_tier kept: %v", m)
	}
	m = map[string]any{"model": "m", "service_tier": "flex"}
	p.StripPremiumTierMap(m, OpenAIPremiumTierRules)
	if m["service_tier"] != "flex" {
		t.Fatalf("standard service_tier dropped: %v", m)
	}
}
