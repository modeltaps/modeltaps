package controller

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func TestSMTPTestTokenReusesSavedOnlyForSavedServer(t *testing.T) {
	oldServer, oldAccount, oldToken := config.SMTPServer, config.SMTPAccount, config.SMTPToken
	t.Cleanup(func() { config.SMTPServer, config.SMTPAccount, config.SMTPToken = oldServer, oldAccount, oldToken })
	config.SMTPServer, config.SMTPAccount, config.SMTPToken = "smtp.resend.com", "resend", "saved-secret"

	if got := smtpTestToken(smtpTestRequest{Server: "smtp.resend.com", Account: "resend", Token: "typed"}); got != "typed" {
		t.Fatalf("typed token not used: %q", got)
	}
	if got := smtpTestToken(smtpTestRequest{Server: "SMTP.resend.com ", Account: "resend"}); got != "saved-secret" {
		t.Fatalf("saved token not reused for the saved server: %q", got)
	}
	if got := smtpTestToken(smtpTestRequest{Server: "evil.example.com", Account: "resend"}); got != "" {
		t.Fatalf("saved token handed to another host: %q", got)
	}
	if got := smtpTestToken(smtpTestRequest{Server: "smtp.resend.com", Account: "someone-else"}); got != "" {
		t.Fatalf("saved token reused for another account: %q", got)
	}
}
