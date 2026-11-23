// Package lang picks the language of what the server writes for a person (emails, the home page):
// the console's languages, the site default, and the language of a request or a user.
//
// The languages come from web/src/i18n/languages.json, the list the frontend uses too, so adding
// a language there (with its translation files) is all the code needs.
package lang

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/web"
)

// Language is one of the console's languages.
type Language struct {
	// Code is the app's code (en_US, zh_CN): locale file names, settings, the cookie.
	Code string `json:"code"`
	// Name is the language's own name, shown in pickers.
	Name string `json:"name"`
	// BCP47 is the tag sent to identity providers as ui_locales.
	BCP47 string `json:"bcp47"`
	// Match lists the lowercase browser tags (and their prefixes) that mean this language.
	Match []string `json:"match"`
	// Fallback is the language a missing text is looked up in next; English comes last anyway.
	Fallback string `json:"fallback"`
}

// English is the last resort for any missing text; the first language of the list.
const English = "en_US"

// Codes of the languages the server's own tests and built-in texts refer to.
const (
	SimplifiedChinese = "zh_CN"
	Japanese          = "ja_JP"
)

var (
	// Languages is the list, in the order the console shows it.
	Languages = mustLoad(web.LanguagesJSON)
	// Supported lists the codes, in the same order.
	Supported = codes(Languages)
)

func mustLoad(data []byte) []Language {
	var list []Language
	if err := json.Unmarshal(data, &list); err != nil {
		panic(fmt.Sprintf("web/src/i18n/languages.json: %v", err))
	}
	if len(list) == 0 || list[0].Code != English {
		panic("web/src/i18n/languages.json: English must come first")
	}
	return list
}

func codes(list []Language) []string {
	out := make([]string, len(list))
	for i, l := range list {
		out[i] = l.Code
	}
	return out
}

// Cookie is the cookie the console writes when the user picks a language (the same name as its
// localStorage key), so requests from the console carry the choice.
const Cookie = "appLanguage"

// Get returns a language by code.
func Get(code string) (Language, bool) {
	for _, l := range Languages {
		if l.Code == code {
			return l, true
		}
	}
	return Language{}, false
}

// IsSupported reports whether code is one of the console's languages.
func IsSupported(code string) bool {
	_, ok := Get(code)
	return ok
}

// Normalize maps a language tag (zh-Hant-TW, ja, en-GB, zh_CN ...) to a supported code, or ""
// when it is none of them. The longest matching entry wins; every Chinese tag (zh-TW, the zh_HK a
// user may have saved before) is Chinese.
func Normalize(tag string) string {
	tag = strings.TrimSpace(tag)
	if IsSupported(tag) {
		return tag
	}
	lower := strings.ToLower(strings.ReplaceAll(tag, "_", "-"))
	if lower == "" {
		return ""
	}
	best, bestLen := "", 0
	for _, l := range Languages {
		for _, m := range l.Match {
			if (lower == m || strings.HasPrefix(lower, m+"-")) && len(m) > bestLen {
				best, bestLen = l.Code, len(m)
			}
		}
	}
	return best
}

// Chain is the order a text is looked up in for a language: the language, its fallbacks, then
// English.
func Chain(code string) []string {
	var out []string
	seen := map[string]bool{}
	for c := code; c != "" && !seen[c]; {
		l, ok := Get(c)
		if !ok {
			break
		}
		out, seen[c] = append(out, c), true
		c = l.Fallback
	}
	if !seen[English] {
		out = append(out, English)
	}
	return out
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

// BCP47 returns the ui_locales tag of a code, or "".
func BCP47(code string) string {
	l, _ := Get(code)
	return l.BCP47
}

// FromBCP47 returns the code whose ui_locales tag is tag, or "".
func FromBCP47(tag string) string {
	for _, l := range Languages {
		if l.BCP47 == tag {
			return l.Code
		}
	}
	return ""
}
