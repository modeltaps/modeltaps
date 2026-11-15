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
	oldRoot, oldServer := config.RootUserEmail, config.SMTPServer
	t.Cleanup(func() { SetSource(nil); config.RootUserEmail, config.SMTPServer = oldRoot, oldServer })

	config.RootUserEmail, config.SMTPServer = "root@example.com", ""
	SetSource(func() []Destination {
		return []Destination{
			{Name: "", Type: TypeEmail, Notifier: channel.NewEmail("")},
			{Name: "Ops group", Type: TypeTelegram, Notifier: namedNotifier("Telegram")},
		}
	})

	got := Targets()
	if len(got) < 2 || got[0].Type != TypeEmail || got[1].Name != "Ops group" {
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

func TestBuild(t *testing.T) {
	cases := []struct {
		typ    string
		values map[string]string
		ok     bool
	}{
		{TypeEmail, nil, true},
		{TypeWeCom, map[string]string{}, false},
		{TypeWeCom, map[string]string{"webhook": "https://example.com/hook"}, true},
		{TypeDingTalk, map[string]string{"token": "t"}, false},
		{TypeDingTalk, map[string]string{"token": "t", "keyword": "k"}, true},
		{TypeLark, map[string]string{"token": "t", "secret": "s"}, true},
		{TypePushDeer, map[string]string{"pushkey": " "}, false},
		{TypeTelegram, map[string]string{"bot_token": "b"}, false},
		{TypeTelegram, map[string]string{"bot_token": "b", "chat_id": "1"}, true},
		{"sms", nil, false},
	}
	for _, c := range cases {
		n, err := Build(c.typ, c.values)
		if (err == nil) != c.ok || (c.ok && n == nil) {
			t.Errorf("Build(%s, %v) = %v, %v; want ok=%v", c.typ, c.values, n, err, c.ok)
		}
	}
}

func TestDestinationWantsKinds(t *testing.T) {
	all := Destination{}
	only := Destination{Kinds: []string{KindLowBalance}}
	if !all.Wants(KindPriceChange) || !only.Wants(KindLowBalance) || only.Wants(KindPriceChange) {
		t.Fatal("kind filter")
	}
	if !only.Wants("") {
		t.Fatal("a notice without a kind goes everywhere")
	}
}

func TestBuildWebhookTypes(t *testing.T) {
	if _, err := Build(TypeSlack, map[string]string{"webhook": "ftp://x"}); err == nil {
		t.Fatal("accepted a non-http webhook")
	}
	if _, err := Build(TypeWebhook, map[string]string{"url": "https://example.com/hook"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(TypeDiscord, map[string]string{}); err == nil {
		t.Fatal("accepted a missing webhook")
	}
}
