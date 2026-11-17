package base

import (
	"strings"

	"github.com/tidwall/gjson"
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
//
// The top-level members are read one by one instead of through a path lookup, because a path
// lookup only sees the first of duplicate keys while an upstream parser may take the last one:
// {"speed":"standard","speed":"fast"} would otherwise pass. Keys are compared after JSON
// unescaping and without regard to case (some upstream parsers match keys case-insensitively).
// A field that appears more than once is removed in every occurrence, whatever its values, and
// so is every occurrence of a field when any of them holds a non-standard value. Bodies that
// need no change are returned as they are; otherwise the top-level object is rebuilt from the
// raw bytes of the members that are kept, in their original order.
func StripPremiumTierFields(body []byte, rules []PremiumTierRule) []byte {
	parsed := gjson.ParseBytes(body)
	if !parsed.IsObject() || len(rules) == 0 {
		return body
	}

	type member struct{ key, value gjson.Result }
	members := make([]member, 0, 16)
	parsed.ForEach(func(key, value gjson.Result) bool {
		members = append(members, member{key, value})
		return true
	})

	drop := make([]bool, len(rules))
	for i, rule := range rules {
		seen := 0
		for _, m := range members {
			if !strings.EqualFold(m.key.String(), rule.Field) {
				continue
			}
			seen++
			if seen > 1 || !isStandardTierValue(m.value, rule.Standard) {
				drop[i] = true
			}
		}
	}

	anyDropped := false
	for _, d := range drop {
		anyDropped = anyDropped || d
	}
	if !anyDropped {
		return body
	}

	dropped := func(key string) bool {
		for i, rule := range rules {
			if drop[i] && strings.EqualFold(key, rule.Field) {
				return true
			}
		}
		return false
	}

	var out strings.Builder
	out.Grow(len(body))
	out.WriteByte('{')
	first := true
	for _, m := range members {
		if dropped(m.key.String()) {
			continue
		}
		if !first {
			out.WriteByte(',')
		}
		first = false
		out.WriteString(m.key.Raw)
		out.WriteByte(':')
		out.WriteString(m.value.Raw)
	}
	out.WriteByte('}')
	return []byte(out.String())
}

// StripPremiumTierMap is StripPremiumTierFields for a decoded request body. Keys that differ
// only in case count as the same field, as they do there.
func (p *BaseProvider) StripPremiumTierMap(body map[string]any, rules []PremiumTierRule) {
	if p.AllowsPremiumTier() || body == nil {
		return
	}
	for _, rule := range rules {
		keys := make([]string, 0, 1)
		standard := true
		for key, value := range body {
			if !strings.EqualFold(key, rule.Field) {
				continue
			}
			keys = append(keys, key)
			if text, isText := value.(string); !isText || !containsFold(rule.Standard, text) {
				standard = false
			}
		}
		if len(keys) > 1 || !standard {
			for _, key := range keys {
				delete(body, key)
			}
		}
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
