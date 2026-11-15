package stmp

import (
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
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
	old, oldName := config.MailTemplates, config.SystemName
	t.Cleanup(func() { config.MailTemplates, config.SystemName = old, oldName })
	config.SystemName = "Taps"

	config.MailTemplates = ""
	subject, body := RenderTemplate(TemplateVerification, map[string]string{"code": "123456", "minutes": "5"}, nil)
	if subject != "Taps email verification" || !strings.Contains(body, "123456") || !strings.Contains(body, "5 minutes") {
		t.Fatalf("default not used: %q", subject)
	}

	config.MailTemplates = `{"verification":{"subject":"【{{site}}】验证码 {{code}}","body":""}}`
	subject, body = RenderTemplate(TemplateVerification, map[string]string{"code": "<b>1</b>", "minutes": "5"}, nil)
	if subject != "【Taps】验证码 <b>1</b>" {
		t.Fatalf("custom subject not used: %q", subject)
	}
	if !strings.Contains(body, "&lt;b&gt;1&lt;/b&gt;") || strings.Contains(body, "<b>1</b>") {
		t.Fatal("body value not escaped")
	}

	config.MailTemplates = "{broken"
	if subject, _ = RenderTemplate(TemplatePasswordReset, nil, nil); subject != "Taps password reset" {
		t.Fatalf("broken value should fall back: %q", subject)
	}
	if ValidateTemplates("{broken") == nil || ValidateTemplates("") != nil {
		t.Fatal("ValidateTemplates")
	}
}

func TestPreviewUsesSamples(t *testing.T) {
	subject, body := PreviewTemplate(TemplateQuotaWarning, MailTemplate{})
	if subject != "Your quota is running low" || !strings.Contains(body, "1200") {
		t.Fatalf("preview: %q", subject)
	}
	if s, _ := PreviewTemplate("nope", MailTemplate{}); s != "" {
		t.Fatal("unknown template rendered")
	}
}
