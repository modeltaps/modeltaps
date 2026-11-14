package notify

import "sort"

// Target is one place admin notices go: a notifier set up in the config file.
type Target struct {
	// Name is the notifier's name: Email, WeCom, DingTalk, Lark, PushDeer or Telegram.
	Name string `json:"name"`
	// Detail says where on that notifier, when it is worth showing (the email recipient).
	Detail string `json:"detail,omitempty"`
	// Ready is false when the notifier is on but cannot send (email without SMTP or recipient).
	Ready bool `json:"ready"`
}

type recipientNotifier interface {
	Recipient() string
	Ready() bool
}

// Targets lists the notifiers notices are sent to, sorted by name.
func Targets() []Target {
	out := make([]Target, 0, len(notifyChannels.notifiers))
	for name, n := range notifyChannels.notifiers {
		if n == nil {
			continue
		}
		target := Target{Name: name, Ready: true}
		if r, ok := n.(recipientNotifier); ok {
			target.Detail, target.Ready = r.Recipient(), r.Ready()
		}
		out = append(out, target)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
