package notify

import (
	"context"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/notify/channel"
)

type namedNotifier string

func (n namedNotifier) Send(context.Context, string, string) error { return nil }
func (n namedNotifier) Name() string                               { return string(n) }

func TestTargets(t *testing.T) {
	old := notifyChannels
	oldRoot, oldServer := config.RootUserEmail, config.SMTPServer
	t.Cleanup(func() { notifyChannels, config.RootUserEmail, config.SMTPServer = old, oldRoot, oldServer })

	notifyChannels = New()
	config.RootUserEmail, config.SMTPServer = "root@example.com", ""
	AddNotifiers(namedNotifier("Telegram"), channel.NewEmail(""))

	got := Targets()
	if len(got) != 2 || got[0].Name != "Email" || got[1].Name != "Telegram" {
		t.Fatalf("targets = %+v", got)
	}
	// Email falls back to the root user's address and is not ready without SMTP.
	if got[0].Detail != "root@example.com" || got[0].Ready {
		t.Errorf("email = %+v, want root address and not ready", got[0])
	}
	if !got[1].Ready {
		t.Errorf("telegram = %+v, want ready", got[1])
	}
}
