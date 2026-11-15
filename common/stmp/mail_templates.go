package stmp

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/lang"
)

// The emails the platform sends to users, in each of the console's languages (common/lang).
//
// The built-in text keeps one layout per email and the words per language apart: the layout holds
// [[key]] slots that the words of the recipient's language fill, so the languages cannot drift
// apart. The admin can rewrite any email in any language in the console (config.MailTemplates,
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

// words are the built-in texts of one language, by slot.
type words map[string]string

// builtin is an email's built-in layout and its words per language.
type builtin struct {
	subject string
	body    string
	words   map[string]words
}

// Words several emails share.
var sharedWords = map[string]words{
	lang.English: {
		"greeting": "Hi <strong>{{user}}</strong>,",
		"fallback": "If the button does not work, open the link below or copy it into your browser:",
		"top_up":   "Top up",
	},
	lang.SimplifiedChinese: {
		"greeting": "<strong>{{user}}</strong>，你好",
		"fallback": "如果按钮无法点击，请打开下面的链接，或复制到浏览器中打开：",
		"top_up":   "去充值",
	},
	lang.TraditionalChinese: {
		"greeting": "<strong>{{user}}</strong>，你好",
		"fallback": "如果按鈕無法使用，請打開下面的連結，或複製到瀏覽器中打開：",
		"top_up":   "去充值",
	},
	lang.Japanese: {
		"greeting": "<strong>{{user}}</strong> さん",
		"fallback": "ボタンが使えない場合は、次のリンクを開くか、ブラウザーに貼り付けてください。",
		"top_up":   "チャージする",
	},
}

const buttonLayout = `<p style="text-align: center; font-size: 13px;"><a target="__blank" href="{{link}}" class="button" style="color: #ffffff;">[[button]]</a></p>
<p style="color: #858585; padding-top: 15px;">[[fallback]]<br> {{link}}</p>`

const quotaLayout = `<p style="font-size: 30px">[[greeting]]</p>
<p>[[intro]]</p>
` + buttonLayout

var builtins = map[string]builtin{
	TemplateVerification: {
		subject: "[[subject]]",
		body: `<p>[[intro]]</p>
<p style="text-align: center; font-size: 30px; color: #58a6ff;"><strong>{{code}}</strong></p>
<p style="color: #858585; padding-top: 15px;">[[validity]]</p>`,
		words: map[string]words{
			lang.English: {
				"subject":  "{{site}} email verification",
				"intro":    "You are verifying your email. Your verification code is:",
				"validity": "The verification code is valid for {{minutes}} minutes. If you did not request this, please ignore this email.",
			},
			lang.SimplifiedChinese: {
				"subject":  "{{site}} 邮箱验证",
				"intro":    "你正在验证邮箱，验证码是：",
				"validity": "验证码 {{minutes}} 分钟内有效。如果不是你本人操作，请忽略这封邮件。",
			},
			lang.TraditionalChinese: {
				"subject":  "{{site}} 電郵驗證",
				"intro":    "你正在驗證電郵，驗證碼是：",
				"validity": "驗證碼 {{minutes}} 分鐘內有效。如果不是你本人操作，請忽略這封郵件。",
			},
			lang.Japanese: {
				"subject":  "{{site}} メールアドレスの確認",
				"intro":    "メールアドレスを確認しています。確認コードは次のとおりです。",
				"validity": "確認コードの有効期限は {{minutes}} 分です。お心当たりがない場合は、このメールを無視してください。",
			},
		},
	},
	TemplatePasswordReset: {
		subject: "[[subject]]",
		body: `<p style="font-size: 30px">[[greeting]]</p>
<p>[[intro]]</p>
` + buttonLayout + `
<p style="color: #858585;">[[validity]]</p>`,
		words: map[string]words{
			lang.English: {
				"subject":  "{{site}} password reset",
				"intro":    "You requested a password reset. Click the button below to set a new password.",
				"button":   "Reset password",
				"validity": "The reset link is valid for {{minutes}} minutes. If you did not request this, please ignore this email.",
			},
			lang.SimplifiedChinese: {
				"subject":  "{{site}} 重置密码",
				"intro":    "你申请了重置密码，点击下面的按钮设置新密码。",
				"button":   "重置密码",
				"validity": "链接 {{minutes}} 分钟内有效。如果不是你本人操作，请忽略这封邮件。",
			},
			lang.TraditionalChinese: {
				"subject":  "{{site}} 重設密碼",
				"intro":    "你申請了重設密碼，按下面的按鈕設定新密碼。",
				"button":   "重設密碼",
				"validity": "連結 {{minutes}} 分鐘內有效。如果不是你本人操作，請忽略這封郵件。",
			},
			lang.Japanese: {
				"subject":  "{{site}} パスワードの再設定",
				"intro":    "パスワードの再設定が申請されました。下のボタンから新しいパスワードを設定してください。",
				"button":   "パスワードを再設定",
				"validity": "リンクの有効期限は {{minutes}} 分です。お心当たりがない場合は、このメールを無視してください。",
			},
		},
	},
	TemplateQuotaLow: {
		subject: "[[subject]]",
		body:    quotaLayout,
		words: map[string]words{
			lang.English: {
				"subject": "Your {{site}} quota is running low",
				"intro":   "Your quota is running low: {{quota}} left. Please top up soon to avoid service interruption.",
				"button":  "[[top_up]]",
			},
			lang.SimplifiedChinese: {
				"subject": "你的 {{site}} 额度快用完了",
				"intro":   "你的额度快用完了，还剩 {{quota}}。请尽快充值，以免服务中断。",
				"button":  "[[top_up]]",
			},
			lang.TraditionalChinese: {
				"subject": "你的 {{site}} 額度快用完了",
				"intro":   "你的額度快用完了，還剩 {{quota}}。請盡快充值，以免服務中斷。",
				"button":  "[[top_up]]",
			},
			lang.Japanese: {
				"subject": "{{site}} の残高が少なくなっています",
				"intro":   "残高が少なくなっています（残り {{quota}}）。サービスが止まらないよう、早めにチャージしてください。",
				"button":  "[[top_up]]",
			},
		},
	},
	TemplateQuotaExhausted: {
		subject: "[[subject]]",
		body:    quotaLayout,
		words: map[string]words{
			lang.English: {
				"subject": "Your {{site}} quota has been used up",
				"intro":   "Your quota has been used up ({{quota}} left), so requests will fail until you top up.",
				"button":  "[[top_up]]",
			},
			lang.SimplifiedChinese: {
				"subject": "你的 {{site}} 额度已用完",
				"intro":   "你的额度已经用完（剩余 {{quota}}），充值之前请求会失败。",
				"button":  "[[top_up]]",
			},
			lang.TraditionalChinese: {
				"subject": "你的 {{site}} 額度已用完",
				"intro":   "你的額度已經用完（剩餘 {{quota}}），充值之前請求會失敗。",
				"button":  "[[top_up]]",
			},
			lang.Japanese: {
				"subject": "{{site}} の残高がなくなりました",
				"intro":   "残高がなくなりました（残り {{quota}}）。チャージするまでリクエストは失敗します。",
				"button":  "[[top_up]]",
			},
		},
	},
}

// fillSlots puts the words of lng (English for a slot it lacks) into the [[slots]] of text. A word
// may itself hold slots (a shared word), so it runs twice.
func fillSlots(text string, b builtin, lng string) string {
	pairs := []string{}
	add := func(set map[string]words) {
		for _, l := range []string{lng, lang.English} {
			for k, v := range set[l] {
				pairs = append(pairs, "[["+k+"]]", v)
			}
		}
	}
	add(b.words)
	add(sharedWords)
	r := strings.NewReplacer(pairs...)
	return r.Replace(r.Replace(text))
}

// Builtin returns an email's built-in text in a language.
func Builtin(id, lng string) MailTemplate {
	b, ok := builtins[id]
	if !ok {
		return MailTemplate{}
	}
	return MailTemplate{Subject: fillSlots(b.subject, b, lng), Body: fillSlots(b.body, b, lng)}
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

// quotaStatus is the {{status}} of the quota emails. It is not offered any more (each quota state
// has its own email), but templates written before the split still use it.
var quotaStatus = map[string]map[string]string{
	TemplateQuotaLow: {
		lang.English: "Your quota is running low", lang.SimplifiedChinese: "你的额度快用完了",
		lang.TraditionalChinese: "你的額度快用完了", lang.Japanese: "残高が少なくなっています",
	},
	TemplateQuotaExhausted: {
		lang.English: "Your quota has been used up", lang.SimplifiedChinese: "你的额度已用完",
		lang.TraditionalChinese: "你的額度已用完", lang.Japanese: "残高がなくなりました",
	},
}

// RenderTemplate fills a template in a language with vars ({{site}} is added) and returns the
// subject and the full HTML message. tmpl overrides the stored text when not nil (previews).
func RenderTemplate(id, lng string, vars map[string]string, tmpl *MailTemplate) (subject, body string) {
	lng = lang.Pick(lng)
	t := effective(id, lng, storedTemplates()[id][lng])
	if tmpl != nil {
		t = effective(id, lng, *tmpl)
	}
	all := map[string]string{"site": config.SystemName}
	if status, ok := quotaStatus[id]; ok {
		all["status"] = status[lng]
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
