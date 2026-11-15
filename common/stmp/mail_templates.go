package stmp

import (
	"embed"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/lang"
)

// The emails the platform sends to users, in each of the console's languages (common/lang).
//
// The built-in text keeps one layout per email (below) and the words per language apart: the
// layout holds [[key]] slots that the words of the recipient's language fill, so the languages
// cannot drift apart. The words live in locales/<language>.json, flat keys "<template>.<slot>"
// with "shared.<slot>" for words several emails use; a missing word is looked up along the
// language's fallbacks and then in English (lang.Chain). A new language is a new file. The admin can rewrite any email in any language in the console (config.MailTemplates,
// {"<template>": {"<language>": {"subject", "body"}}}); an empty subject or body falls back to the
// built-in text of that language. Variables are written {{name}}; values are HTML-escaped in the
// body. The body is wrapped in the shared mail layout (template.go).

const (
	TemplateVerification   = "verification"
	TemplatePasswordReset  = "password_reset"
	TemplateQuotaLow       = "quota_low"
	TemplateQuotaExhausted = "quota_exhausted"
)

// MailTemplate is one email's subject and body.
type MailTemplate struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

//go:embed locales/*.json
var localeFiles embed.FS

// wordsByLang are the built-in words: language → "<template>.<slot>" → text.
var wordsByLang = func() map[string]map[string]string {
	out := map[string]map[string]string{}
	entries, err := localeFiles.ReadDir("locales")
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		data, err := localeFiles.ReadFile("locales/" + e.Name())
		if err != nil {
			panic(err)
		}
		words := map[string]string{}
		if err := json.Unmarshal(data, &words); err != nil {
			panic(fmt.Sprintf("common/stmp/locales/%s: %v", e.Name(), err))
		}
		out[strings.TrimSuffix(e.Name(), ".json")] = words
	}
	return out
}()

// word returns a built-in word of a template in a language, following the language's fallbacks:
// the template's own word, else the shared one.
func word(id, slot, lng string) (string, bool) {
	for _, l := range lang.Chain(lng) {
		if v, ok := wordsByLang[l][id+"."+slot]; ok {
			return v, true
		}
		if v, ok := wordsByLang[l]["shared."+slot]; ok {
			return v, true
		}
	}
	return "", false
}

// builtin is an email's built-in layout.
type builtin struct {
	subject string
	body    string
}

const quotaButtonLayout = `<p style="text-align: center; font-size: 13px;"><a target="__blank" href="{{link}}" class="button" style="color: #ffffff;">[[top_up]]</a></p>
<p style="color: #858585; padding-top: 15px;">[[fallback]]<br> {{link}}</p>`

const buttonLayout = `<p style="text-align: center; font-size: 13px;"><a target="__blank" href="{{link}}" class="button" style="color: #ffffff;">[[button]]</a></p>
<p style="color: #858585; padding-top: 15px;">[[fallback]]<br> {{link}}</p>`

const quotaLayout = `<p style="font-size: 30px">[[greeting]]</p>
<p>[[intro]]</p>
` + quotaButtonLayout

var builtins = map[string]builtin{
	TemplateVerification: {
		subject: "[[subject]]",
		body: `<p>[[intro]]</p>
<p style="text-align: center; font-size: 30px; color: #58a6ff;"><strong>{{code}}</strong></p>
<p style="color: #858585; padding-top: 15px;">[[validity]]</p>`,
	},
	TemplatePasswordReset: {
		subject: "[[subject]]",
		body: `<p style="font-size: 30px">[[greeting]]</p>
<p>[[intro]]</p>
` + buttonLayout + `
<p style="color: #858585;">[[validity]]</p>`,
	},
	TemplateQuotaLow: {
		subject: "[[subject]]",
		body:    quotaLayout,
	},
	TemplateQuotaExhausted: {
		subject: "[[subject]]",
		body:    quotaLayout,
	},
}

var slotPattern = regexp.MustCompile(`\[\[([a-z_]+)\]\]`)

// fillSlots puts the words of a template in lng into the [[slots]] of text. A slot without a word
// in any language stays as it is (TestEveryBuiltinIsComplete catches it).
func fillSlots(text, id, lng string) string {
	return slotPattern.ReplaceAllStringFunc(text, func(m string) string {
		if v, ok := word(id, m[2:len(m)-2], lng); ok {
			return v
		}
		return m
	})
}

// Builtin returns an email's built-in text in a language.
func Builtin(id, lng string) MailTemplate {
	b, ok := builtins[id]
	if !ok {
		return MailTemplate{}
	}
	return MailTemplate{Subject: fillSlots(b.subject, id, lng), Body: fillSlots(b.body, id, lng)}
}

// TemplateSpec is a template's built-in text per language and the variables it can use.
type TemplateSpec struct {
	ID        string                  `json:"id"`
	Defaults  map[string]MailTemplate `json:"defaults"`
	Variables []string                `json:"variables"`
	Sample    map[string]string       `json:"-"`
}

var templateVariables = []struct {
	id     string
	vars   []string
	sample map[string]string
}{
	{TemplateVerification, []string{"site", "code", "minutes"}, map[string]string{"code": "482913", "minutes": "10"}},
	{TemplatePasswordReset, []string{"site", "user", "link", "minutes"}, map[string]string{"user": "alice", "link": "https://example.com/reset?token=sample", "minutes": "10"}},
	{TemplateQuotaLow, []string{"site", "user", "quota", "link"}, map[string]string{"user": "alice", "quota": "1200", "link": "https://example.com/panel/billing?topup=1"}},
	{TemplateQuotaExhausted, []string{"site", "user", "quota", "link"}, map[string]string{"user": "alice", "quota": "0", "link": "https://example.com/panel/billing?topup=1"}},
}

// TemplateSpecs lists the emails in the order the console shows them.
var TemplateSpecs = func() []TemplateSpec {
	out := make([]TemplateSpec, 0, len(templateVariables))
	for _, t := range templateVariables {
		defaults := map[string]MailTemplate{}
		for _, lng := range lang.Supported {
			defaults[lng] = Builtin(t.id, lng)
		}
		out = append(out, TemplateSpec{ID: t.id, Defaults: defaults, Variables: t.vars, Sample: t.sample})
	}
	return out
}()

// TemplateSpecByID returns a template's spec, or nil for an unknown id.
func TemplateSpecByID(id string) *TemplateSpec {
	for i := range TemplateSpecs {
		if TemplateSpecs[i].ID == id {
			return &TemplateSpecs[i]
		}
	}
	return nil
}

// customTemplates are the admin's texts: template → language → text.
type customTemplates map[string]map[string]MailTemplate

// parseTemplates reads a MailTemplates value. A template stored in the shape from before the
// languages ({"subject", "body"} directly) counts as written in legacyLang. A broken value counts
// as none.
func parseTemplates(raw, legacyLang string) customTemplates {
	out := customTemplates{}
	var top map[string]json.RawMessage
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &top) != nil {
		return out
	}
	for id, value := range top {
		var inner map[string]json.RawMessage
		if json.Unmarshal(value, &inner) != nil {
			continue
		}
		_, hasSubject := inner["subject"]
		_, hasBody := inner["body"]
		if hasSubject || hasBody {
			var t MailTemplate
			if json.Unmarshal(value, &t) == nil {
				out[id] = map[string]MailTemplate{legacyLang: t}
			}
			continue
		}
		byLang := map[string]MailTemplate{}
		for lng, v := range inner {
			var t MailTemplate
			if json.Unmarshal(v, &t) == nil {
				byLang[lng] = t
			}
		}
		out[id] = byLang
	}
	return out
}

func storedTemplates() customTemplates { return parseTemplates(config.MailTemplates, lang.Default()) }

// ValidateTemplates checks a MailTemplates value before it is saved: an object of known templates,
// each an object of supported languages.
func ValidateTemplates(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out map[string]map[string]MailTemplate
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return err
	}
	for id, byLang := range out {
		if TemplateSpecByID(id) == nil {
			return fmt.Errorf("unknown mail template %q", id)
		}
		for lng := range byLang {
			if lang.Normalize(lng) != lng {
				return fmt.Errorf("unsupported language %q", lng)
			}
		}
	}
	return nil
}

// UpgradeTemplates rewrites a MailTemplates value from before the languages: each template goes
// under legacyLang, and the old quota_warning (one email for both quota states, with the state in
// {{status}}) becomes both quota emails. It reports whether anything changed.
func UpgradeTemplates(raw, legacyLang string) (string, bool) {
	parsed := parseTemplates(raw, legacyLang)
	old, hadWarning := parsed["quota_warning"]
	if hadWarning {
		delete(parsed, "quota_warning")
		for _, id := range []string{TemplateQuotaLow, TemplateQuotaExhausted} {
			if _, exists := parsed[id]; !exists {
				parsed[id] = old
			}
		}
	}
	if len(parsed) == 0 {
		return "", strings.TrimSpace(raw) != ""
	}
	data, _ := json.Marshal(parsed)
	return string(data), string(data) != raw
}

// effective returns the text to send: the admin's text in lng, field by field, else the built-in
// text in lng.
func effective(id, lng string, custom MailTemplate) MailTemplate {
	t := Builtin(id, lng)
	if strings.TrimSpace(custom.Subject) != "" {
		t.Subject = custom.Subject
	}
	if strings.TrimSpace(custom.Body) != "" {
		t.Body = custom.Body
	}
	return t
}

func fill(text string, vars map[string]string, escape bool) string {
	pairs := make([]string, 0, len(vars)*2)
	for k, v := range vars {
		if escape {
			v = html.EscapeString(v)
		}
		pairs = append(pairs, "{{"+k+"}}", v)
	}
	return strings.NewReplacer(pairs...).Replace(text)
}

// The quota emails' {{status}} ("<template>.status" in the locale files) is not offered any more
// (each quota state has its own email), but templates written before the split still use it.

// RenderTemplate fills a template in a language with vars ({{site}} is added) and returns the
// subject and the full HTML message. tmpl overrides the stored text when not nil (previews).
func RenderTemplate(id, lng string, vars map[string]string, tmpl *MailTemplate) (subject, body string) {
	lng = lang.Pick(lng)
	t := effective(id, lng, storedTemplates()[id][lng])
	if tmpl != nil {
		t = effective(id, lng, *tmpl)
	}
	all := map[string]string{"site": config.SystemName}
	if _, quota := wordsByLang[lang.English][id+".status"]; quota {
		all["status"], _ = word(id, "status", lng)
	}
	for k, v := range vars {
		all[k] = v
	}
	subject = strings.TrimSpace(fill(t.Subject, all, false))
	return subject, getDefaultTemplate(lng, fill(t.Body, all, true))
}

// PreviewTemplate renders a template in a language with sample values.
func PreviewTemplate(id, lng string, tmpl MailTemplate) (subject, body string) {
	spec := TemplateSpecByID(id)
	if spec == nil {
		return "", ""
	}
	return RenderTemplate(id, lng, spec.Sample, &tmpl)
}
