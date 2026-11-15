package stmp

import (
	"encoding/json"
	"html"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
)

// The emails the platform sends to users. Each has a subject and an HTML body that the admin can
// rewrite in the console (config.MailTemplates, a JSON object keyed by template id); an empty
// subject or body falls back to the default below. Variables are written {{name}}; values are
// HTML-escaped in the body. The body is wrapped in the shared mail layout (template.go).

const (
	TemplateVerification  = "verification"
	TemplatePasswordReset = "password_reset"
	TemplateQuotaWarning  = "quota_warning"
)

// MailTemplate is one email's subject and body.
type MailTemplate struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// TemplateSpec is a template's default text and the variables it can use.
type TemplateSpec struct {
	ID        string            `json:"id"`
	Default   MailTemplate      `json:"default"`
	Variables []string          `json:"variables"`
	Sample    map[string]string `json:"-"`
}

var TemplateSpecs = []TemplateSpec{
	{
		ID: TemplateVerification,
		Default: MailTemplate{
			Subject: "{{site}} email verification",
			Body: `<p>You are verifying your email. Your verification code is:</p>
<p style="text-align: center; font-size: 30px; color: #58a6ff;"><strong>{{code}}</strong></p>
<p style="color: #858585; padding-top: 15px;">The verification code is valid for {{minutes}} minutes. If you did not request this, please ignore this email.</p>`,
		},
		Variables: []string{"site", "code", "minutes"},
		Sample:    map[string]string{"code": "482913", "minutes": "10"},
	},
	{
		ID: TemplatePasswordReset,
		Default: MailTemplate{
			Subject: "{{site}} password reset",
			Body: `<p style="font-size: 30px">Hi <strong>{{user}},</strong></p>
<p>You requested a password reset. Click the button below to reset your password.</p>
<p style="text-align: center; font-size: 13px;"><a target="__blank" href="{{link}}" class="button" style="color: #ffffff;">Reset password</a></p>
<p style="color: #858585; padding-top: 15px;">If the button does not work, open the link below or copy it into your browser:<br> {{link}}</p>
<p style="color: #858585;">The reset link is valid for {{minutes}} minutes. If you did not request this, please ignore this email.</p>`,
		},
		Variables: []string{"site", "user", "link", "minutes"},
		Sample:    map[string]string{"user": "alice", "link": "https://example.com/reset?token=sample", "minutes": "10"},
	},
	{
		ID: TemplateQuotaWarning,
		Default: MailTemplate{
			Subject: "{{status}}",
			Body: `<p style="font-size: 30px">Hi <strong>{{user}},</strong></p>
<p>{{status}}. Your remaining quota is {{quota}}. Please top up soon to avoid service interruption.</p>
<p style="text-align: center; font-size: 13px;"><a target="__blank" href="{{link}}" class="button" style="color: #ffffff;">Top up</a></p>
<p style="color: #858585; padding-top: 15px;">If the button does not work, open the link below or copy it into your browser:<br> {{link}}</p>`,
		},
		Variables: []string{"site", "user", "quota", "status", "link"},
		Sample:    map[string]string{"user": "alice", "quota": "1200", "status": "Your quota is running low", "link": "https://example.com/panel/billing?topup=1"},
	},
}

// TemplateSpecByID returns a template's spec, or nil for an unknown id.
func TemplateSpecByID(id string) *TemplateSpec {
	for i := range TemplateSpecs {
		if TemplateSpecs[i].ID == id {
			return &TemplateSpecs[i]
		}
	}
	return nil
}

// customTemplates reads the admin's templates; a broken value counts as none.
func customTemplates() map[string]MailTemplate {
	out := map[string]MailTemplate{}
	if raw := strings.TrimSpace(config.MailTemplates); raw != "" {
		_ = json.Unmarshal([]byte(raw), &out)
	}
	return out
}

// ValidateTemplates checks a MailTemplates value before it is saved.
func ValidateTemplates(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out map[string]MailTemplate
	return json.Unmarshal([]byte(raw), &out)
}

// effective returns the template to send: the admin's text, field by field, else the default.
func effective(id string, custom MailTemplate) MailTemplate {
	t := MailTemplate{}
	if spec := TemplateSpecByID(id); spec != nil {
		t = spec.Default
	}
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

// RenderTemplate fills a template with vars ({{site}} is added) and returns the subject and the
// full HTML message. tmpl overrides the stored text when not nil (previews).
func RenderTemplate(id string, vars map[string]string, tmpl *MailTemplate) (subject, body string) {
	t := effective(id, customTemplates()[id])
	if tmpl != nil {
		t = effective(id, *tmpl)
	}
	all := map[string]string{"site": config.SystemName}
	for k, v := range vars {
		all[k] = v
	}
	subject = strings.TrimSpace(fill(t.Subject, all, false))
	return subject, getDefaultTemplate(fill(t.Body, all, true))
}

// PreviewTemplate renders a template with sample values.
func PreviewTemplate(id string, tmpl MailTemplate) (subject, body string) {
	spec := TemplateSpecByID(id)
	if spec == nil {
		return "", ""
	}
	return RenderTemplate(id, spec.Sample, &tmpl)
}
