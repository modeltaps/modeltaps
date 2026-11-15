package notify

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/modeltaps/modeltaps/common/notify/channel"
)

// Notification channel types.
const (
	TypeEmail    = "email"
	TypeWeCom    = "wecom"
	TypeDingTalk = "dingtalk"
	TypeLark     = "lark"
	TypePushDeer = "pushdeer"
	TypeTelegram = "telegram"
	TypeSlack    = "slack"
	TypeDiscord  = "discord"
	TypeWebhook  = "webhook"
)

// Field is one setting of a channel type. Secret fields are stored apart and never sent back
// to the console.
type Field struct {
	Key      string `json:"key"`
	Secret   bool   `json:"secret,omitempty"`
	Required bool   `json:"required,omitempty"`
}

// Types lists the channel types and their settings, in the order the console offers them.
var Types = []struct {
	Type   string  `json:"type"`
	Fields []Field `json:"fields"`
}{
	{TypeEmail, []Field{{Key: "to"}}},
	{TypeWeCom, []Field{{Key: "webhook", Secret: true, Required: true}}},
	{TypeDingTalk, []Field{{Key: "token", Secret: true, Required: true}, {Key: "secret", Secret: true}, {Key: "keyword"}}},
	{TypeLark, []Field{{Key: "token", Secret: true, Required: true}, {Key: "secret", Secret: true}, {Key: "keyword"}}},
	{TypePushDeer, []Field{{Key: "pushkey", Secret: true, Required: true}, {Key: "url"}}},
	{TypeTelegram, []Field{{Key: "bot_token", Secret: true, Required: true}, {Key: "chat_id", Required: true}, {Key: "http_proxy"}}},
	{TypeSlack, []Field{{Key: "webhook", Secret: true, Required: true}}},
	{TypeDiscord, []Field{{Key: "webhook", Secret: true, Required: true}}},
	{TypeWebhook, []Field{{Key: "url", Secret: true, Required: true}, {Key: "secret", Secret: true}}},
}

// FieldsOf returns the settings of a channel type, or nil for an unknown type.
func FieldsOf(typ string) []Field {
	for _, t := range Types {
		if t.Type == typ {
			return t.Fields
		}
	}
	return nil
}

// Build turns a channel's settings into a notifier. It checks that the type is known and the
// required settings are there.
func Build(typ string, values map[string]string) (Notifier, error) {
	fields := FieldsOf(typ)
	if fields == nil {
		return nil, fmt.Errorf("unknown notification channel type %q", typ)
	}
	v := func(key string) string { return strings.TrimSpace(values[key]) }
	for _, f := range fields {
		if f.Required && v(f.Key) == "" {
			return nil, fmt.Errorf("%s is required", f.Key)
		}
	}
	switch typ {
	case TypeEmail:
		return channel.NewEmail(v("to")), nil
	case TypeWeCom:
		return channel.NewWeCom(v("webhook")), nil
	case TypeDingTalk, TypeLark:
		if v("secret") == "" && v("keyword") == "" {
			return nil, errors.New("a signing secret or a keyword is required")
		}
		if typ == TypeDingTalk {
			if v("secret") != "" {
				return channel.NewDingTalk(v("token"), v("secret")), nil
			}
			return channel.NewDingTalkWithKeyWord(v("token"), v("keyword")), nil
		}
		if v("secret") != "" {
			return channel.NewLark(v("token"), v("secret")), nil
		}
		return channel.NewLarkWithKeyWord(v("token"), v("keyword")), nil
	case TypePushDeer:
		return channel.NewPushdeer(v("pushkey"), v("url")), nil
	case TypeTelegram:
		return channel.NewTelegram(v("bot_token"), v("chat_id"), v("http_proxy")), nil
	case TypeSlack, TypeDiscord, TypeWebhook:
		key := "webhook"
		if typ == TypeWebhook {
			key = "url"
		}
		if err := checkWebhookURL(v(key)); err != nil {
			return nil, err
		}
		switch typ {
		case TypeSlack:
			return channel.NewSlack(v("webhook")), nil
		case TypeDiscord:
			return channel.NewDiscord(v("webhook")), nil
		default:
			return channel.NewWebhook(v("url"), v("secret")), nil
		}
	}
	return nil, fmt.Errorf("unknown notification channel type %q", typ)
}

// checkWebhookURL accepts only absolute http(s) URLs.
func checkWebhookURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("the webhook URL must start with http:// or https://")
	}
	return nil
}
