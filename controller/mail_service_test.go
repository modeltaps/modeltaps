package controller

import (
	"testing"

	"github.com/modeltaps/modeltaps/model"
)

// An empty password keeps the stored one only for the stored server and username, so a request
// cannot send the saved secret to another host.
func TestMailServiceRequestKeepsPasswordOnlyForSameEndpoint(t *testing.T) {
	stored := &model.MailService{Id: 3, Server: "smtp.resend.com", Port: 465, Account: "resend", Token: "re_saved", Active: true}
	cases := []struct {
		name string
		req  MailServiceRequest
		want string
	}{
		{"same endpoint", MailServiceRequest{Server: "SMTP.resend.com ", Port: 465, Account: "resend"}, "re_saved"},
		{"other host", MailServiceRequest{Server: "evil.example.com", Port: 465, Account: "resend"}, ""},
		{"other account", MailServiceRequest{Server: "smtp.resend.com", Port: 465, Account: "someone"}, ""},
		{"new password", MailServiceRequest{Server: "evil.example.com", Port: 465, Account: "x", Token: "typed"}, "typed"},
	}
	for _, tc := range cases {
		got := tc.req.build(stored)
		if got.Token != tc.want {
			t.Errorf("%s: token = %q, want %q", tc.name, got.Token, tc.want)
		}
		if got.Id != 3 || !got.Active {
			t.Errorf("%s: id/active not kept: %+v", tc.name, got)
		}
	}
	if stored.Token != "re_saved" || stored.Server != "smtp.resend.com" {
		t.Fatal("build changed the stored service")
	}
	if fresh := (&MailServiceRequest{Server: "a", Token: ""}).build(nil); fresh.Token != "" || fresh.TLSMode != "auto" {
		t.Fatalf("new service = %+v", fresh)
	}
}
