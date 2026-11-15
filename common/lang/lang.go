// Package lang picks the language of what the server writes for a person (emails): the console's
// languages, the site default, and the language of a request or a user.
package lang

import (
	"sort"
	"strconv"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
)

const (
	English            = "en_US"
	SimplifiedChinese  = "zh_CN"
	TraditionalChinese = "zh_HK"
	Japanese           = "ja_JP"
)

// Supported lists the console's languages; English is the last resort.
var Supported = []string{English, SimplifiedChinese, TraditionalChinese, Japanese}

// Cookie is the cookie the console writes when the user picks a language (the same name as its
// localStorage key), so requests from the console carry the choice.
const Cookie = "appLanguage"

// Normalize maps a language tag (zh-Hant-TW, ja, en-GB, zh_CN ...) to a supported language, or ""
// when it is none of them.
func Normalize(tag string) string {
	lower := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tag), "_", "-"))
	switch {
	case lower == "":
		return ""
	case strings.HasPrefix(lower, "zh"):
		for _, hant := range []string{"hant", "-tw", "-hk", "-mo"} {
			if strings.Contains(lower, hant) {
				return TraditionalChinese
			}
		}
		return SimplifiedChinese
	case strings.HasPrefix(lower, "ja"):
		return Japanese
	case strings.HasPrefix(lower, "en"):
		return English
	}
	return ""
}

// Negotiate picks the supported language an Accept-Language header ranks highest, or "" when it
// names none.
func Negotiate(header string) string {
	type candidate struct {
		lng string
		q   float64
	}
	var candidates []candidate
	for _, part := range strings.Split(header, ",") {
		fields := strings.Split(part, ";")
		lng := Normalize(fields[0])
		if lng == "" {
			continue
		}
		q := 1.0
		for _, param := range fields[1:] {
			if v, ok := strings.CutPrefix(strings.TrimSpace(param), "q="); ok {
				if parsed, err := strconv.ParseFloat(v, 64); err == nil {
					q = parsed
				}
			}
		}
		if q > 0 {
			candidates = append(candidates, candidate{lng, q})
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.SliceStable(candidates, func(a, b int) bool { return candidates[a].q > candidates[b].q })
	return candidates[0].lng
}

// Default is the site default language (config.DefaultLanguage), English when unset.
func Default() string {
	if lng := Normalize(config.DefaultLanguage); lng != "" {
		return lng
	}
	return English
}

// Pick returns the first of the given languages that is supported, else the site default.
func Pick(candidates ...string) string {
	for _, c := range candidates {
		if lng := Normalize(c); lng != "" {
			return lng
		}
	}
	return Default()
}

// OfRequest is the language of a request from the console: the language the user picked (the
// cookie), else the browser's, else the site default.
func OfRequest(cookie, acceptLanguage string) string {
	return Pick(cookie, Negotiate(acceptLanguage))
}
