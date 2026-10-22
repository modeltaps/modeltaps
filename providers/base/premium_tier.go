package base

import (
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// PremiumTierRule names a request field that can move the upstream to a pricier
// processing tier, and the values of it that keep the standard price.
type PremiumTierRule struct {
	Field    string
	Standard []string
}

// OpenAIPremiumTierRules: service_tier "priority" / "fast" bill 2x and "ultrafast" 6x;
// "auto" follows the upstream project default, "flex" is cheaper.
var OpenAIPremiumTierRules = []PremiumTierRule{
	{Field: "service_tier", Standard: []string{"auto", "default", "flex"}},
}

// ClaudePremiumTierRules: speed "fast" bills 2x and inference_geo "us" 1.1x.
// Claude's service_tier only chooses Priority Tier capacity that is paid in advance,
// so it is left alone.
var ClaudePremiumTierRules = []PremiumTierRule{
	{Field: "speed", Standard: []string{"standard"}},
	{Field: "inference_geo", Standard: []string{"global"}},
}

// AllowsPremiumTier reports whether the channel forwards premium tier fields as sent.
func (p *BaseProvider) AllowsPremiumTier() bool {
	return p.Channel != nil && p.Channel.AllowServiceTier
}

// StripPremiumTierFields removes premium tier values from a raw request body unless
// the channel allows them, so a client cannot raise the upstream price on its own.
// Usage billing still follows the tier the upstream reports.
func (p *BaseProvider) StripPremiumTierFields(body []byte, rules []PremiumTierRule) []byte {
	if p.AllowsPremiumTier() {
		return body
	}
	return StripPremiumTierFields(body, rules)
}

// StripPremiumTierFields removes each rule's field whose value is not a standard one.
func StripPremiumTierFields(body []byte, rules []PremiumTierRule) []byte {
	for _, rule := range rules {
		value := gjson.GetBytes(body, rule.Field)
		if !value.Exists() || isStandardTierValue(value, rule.Standard) {
			continue
		}
		if patched, err := sjson.DeleteBytes(body, rule.Field); err == nil {
			body = patched
		}
	}
	return body
}

// StripPremiumTierMap is StripPremiumTierFields for a decoded request body.
func (p *BaseProvider) StripPremiumTierMap(body map[string]any, rules []PremiumTierRule) {
	if p.AllowsPremiumTier() || body == nil {
		return
	}
	for _, rule := range rules {
		value, ok := body[rule.Field]
		if !ok {
			continue
		}
		if text, isText := value.(string); isText && containsFold(rule.Standard, text) {
			continue
		}
		delete(body, rule.Field)
	}
}

func isStandardTierValue(value gjson.Result, standard []string) bool {
	return value.Type == gjson.String && containsFold(standard, value.String())
}

func containsFold(values []string, s string) bool {
	for _, v := range values {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}
