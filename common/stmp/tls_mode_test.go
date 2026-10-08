package stmp

import (
	"testing"

	"github.com/wneessen/go-mail"
)

func TestResolveTLSSettings(t *testing.T) {
	tests := []struct {
		name   string
		mode   string
		port   int
		expect tlsSettings
	}{
		{"auto 465 keeps implicit ssl", TLSModeAuto, 465, tlsSettings{ssl: true, policy: mail.TLSMandatory, auth: mail.SMTPAuthPlain}},
		{"auto 587 keeps starttls + login", TLSModeAuto, 587, tlsSettings{ssl: false, policy: mail.TLSMandatory, auth: mail.SMTPAuthLogin}},
		{"auto other port keeps starttls + plain", TLSModeAuto, 25, tlsSettings{ssl: false, policy: mail.TLSMandatory, auth: mail.SMTPAuthPlain}},
		{"empty mode behaves as auto", "", 465, tlsSettings{ssl: true, policy: mail.TLSMandatory, auth: mail.SMTPAuthPlain}},
		{"unknown mode falls back to auto", "bogus", 587, tlsSettings{ssl: false, policy: mail.TLSMandatory, auth: mail.SMTPAuthLogin}},
		{"ssl on non-465 port", TLSModeSSL, 2465, tlsSettings{ssl: true, policy: mail.TLSMandatory, auth: mail.SMTPAuthPlain}},
		{"starttls on 465 port", TLSModeStartTLS, 465, tlsSettings{ssl: false, policy: mail.TLSMandatory, auth: mail.SMTPAuthPlain}},
		{"opportunistic uses plain-noenc", TLSModeStartTLSOpportunistic, 587, tlsSettings{ssl: false, policy: mail.TLSOpportunistic, auth: mail.SMTPAuthPlainNoEnc}},
		{"none uses plain-noenc", TLSModeNone, 1025, tlsSettings{ssl: false, policy: mail.NoTLS, auth: mail.SMTPAuthPlainNoEnc}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveTLSSettings(tt.mode, tt.port)
			if got != tt.expect {
				t.Fatalf("resolveTLSSettings(%q, %d) = %+v, want %+v", tt.mode, tt.port, got, tt.expect)
			}
		})
	}
}

func TestIsValidTLSMode(t *testing.T) {
	valid := []string{"", TLSModeAuto, TLSModeSSL, TLSModeStartTLS, TLSModeStartTLSOpportunistic, TLSModeNone}
	for _, mode := range valid {
		if !IsValidTLSMode(mode) {
			t.Errorf("IsValidTLSMode(%q) = false, want true", mode)
		}
	}

	invalid := []string{"tls", "SSL", "starttls-opportunistic", "off"}
	for _, mode := range invalid {
		if IsValidTLSMode(mode) {
			t.Errorf("IsValidTLSMode(%q) = true, want false", mode)
		}
	}
}
