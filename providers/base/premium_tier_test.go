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
