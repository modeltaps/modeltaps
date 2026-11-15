package stmp

import (
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/lang"
)

func TestParseManagedURL(t *testing.T) {
	s, err := ParseManagedURL("smtps://resend:re_key@smtp.resend.com")
	if err != nil {
		t.Fatal(err)
	}
	if s.Host != "smtp.resend.com" || s.Port != 465 || s.TLSMode != TLSModeSSL || s.Username != "resend" || s.Password != "re_key" {
		t.Fatalf("unexpected %+v", s)
	}
	s, err = ParseManagedURL("smtp://apikey:SG.x%2Fy@smtp.sendgrid.net:2525?tls=starttls")
	if err != nil {
		t.Fatal(err)
	}
	if s.Port != 2525 || s.TLSMode != TLSModeStartTLS || s.Password != "SG.x/y" {
		t.Fatalf("unexpected %+v", s)
	}
	for _, bad := range []string{"http://a.com", "smtp://", "smtp://h:99999", "smtp://h?tls=bogus"} {
		if _, err := ParseManagedURL(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestRenderTemplateFallsBackAndEscapes(t *testing.T) {
	old, oldName, oldLang := config.MailTemplates, config.SystemName, config.DefaultLanguage
	t.Cleanup(func() { config.MailTemplates, config.SystemName, config.DefaultLanguage = old, oldName, oldLang })
	config.SystemName, config.DefaultLanguage = "Taps", ""

	config.MailTemplates = ""
	subject, body := RenderTemplate(TemplateVerification, lang.English, map[string]string{"code": "123456", "minutes": "5"}, nil)
	if subject != "Taps email verification" || !strings.Contains(body, "123456") || !strings.Contains(body, "5 minutes") {
		t.Fatalf("default not used: %q", subject)
	}
	if !strings.Contains(body, `lang="en-US"`) {
		t.Fatal("layout lang attribute missing")
	}

	config.MailTemplates = `{"verification":{"zh_CN":{"subject":"【{{site}}】验证码 {{code}}","body":""}}}`
	subject, body = RenderTemplate(TemplateVerification, lang.SimplifiedChinese, map[string]string{"code": "<b>1</b>", "minutes": "5"}, nil)
	if subject != "【Taps】验证码 <b>1</b>" {
		t.Fatalf("custom subject not used: %q", subject)
	}
	if !strings.Contains(body, "&lt;b&gt;1&lt;/b&gt;") || strings.Contains(body, "<b>1</b>") {
		t.Fatal("body value not escaped")
	}
	// The built-in body of that language fills what the admin left empty.
	if !strings.Contains(body, "5 分钟内有效") {
		t.Fatal("built-in Chinese body not used")
	}
	// Another language keeps its own built-in text.
	if subject, _ = RenderTemplate(TemplateVerification, lang.Japanese, nil, nil); subject != "Taps メールアドレスの確認" {
		t.Fatalf("japanese subject: %q", subject)
	}

	config.MailTemplates = "{broken"
	if subject, _ = RenderTemplate(TemplatePasswordReset, "fr", nil, nil); subject != "Taps password reset" {
		t.Fatalf("broken value / unsupported language should fall back: %q", subject)
	}
	if ValidateTemplates("{broken") == nil || ValidateTemplates("") != nil {
		t.Fatal("ValidateTemplates")
	}
	if ValidateTemplates(`{"verification":{"fr":{"subject":"x"}}}`) == nil || ValidateTemplates(`{"nope":{}}`) == nil {
		t.Fatal("ValidateTemplates accepted an unknown language or template")
	}
}

func TestEveryBuiltinIsComplete(t *testing.T) {
	for _, spec := range TemplateSpecs {
		for _, lng := range lang.Supported {
			d := spec.Defaults[lng]
			if d.Subject == "" || d.Body == "" || strings.Contains(d.Subject+d.Body, "[[") {
				t.Errorf("%s/%s has an unfilled slot: %q", spec.ID, lng, d.Subject+d.Body)
			}
			for _, v := range spec.Variables {
				if v != "site" && !strings.Contains(d.Subject+d.Body, "{{"+v+"}}") {
					t.Errorf("%s/%s does not use {{%s}}", spec.ID, lng, v)
				}
			}
		}
	}
}

func TestUpgradeTemplates(t *testing.T) {
	raw := `{"verification":{"subject":"S","body":""},"quota_warning":{"subject":"{{status}}","body":"B {{status}}"}}`
	got, changed := UpgradeTemplates(raw, lang.SimplifiedChinese)
	if !changed {
		t.Fatal("not changed")
	}
	parsed := parseTemplates(got, lang.English)
	if parsed[TemplateVerification][lang.SimplifiedChinese].Subject != "S" {
		t.Fatalf("verification: %v", parsed)
	}
	for _, id := range []string{TemplateQuotaLow, TemplateQuotaExhausted} {
		if parsed[id][lang.SimplifiedChinese].Body != "B {{status}}" {
			t.Fatalf("%s: %v", id, parsed[id])
		}
	}
	if _, again := UpgradeTemplates(got, lang.SimplifiedChinese); again {
		t.Fatal("upgrading twice changed it again")
	}
	if ValidateTemplates(got) != nil {
		t.Fatal("upgraded value does not validate")
	}
	// {{status}} of an old template is still filled, in the email's language.
	config.MailTemplates = got
	t.Cleanup(func() { config.MailTemplates = "" })
	if subject, _ := RenderTemplate(TemplateQuotaExhausted, lang.SimplifiedChinese, nil, nil); subject != "你的额度已用完" {
		t.Fatalf("status: %q", subject)
	}
}

func TestPreviewUsesSamples(t *testing.T) {
	subject, body := PreviewTemplate(TemplateQuotaLow, lang.English, MailTemplate{})
	if !strings.Contains(subject, "quota is running low") || !strings.Contains(body, "1200") {
		t.Fatalf("preview: %q", subject)
	}
	if subject, _ = PreviewTemplate(TemplateQuotaExhausted, lang.TraditionalChinese, MailTemplate{}); !strings.Contains(subject, "額度已用完") {
		t.Fatalf("preview zh_HK: %q", subject)
	}
	if s, _ := PreviewTemplate("nope", lang.English, MailTemplate{}); s != "" {
		t.Fatal("unknown template rendered")
	}
}
