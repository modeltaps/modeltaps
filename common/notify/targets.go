package notify

// Target is one place admin notices go, as the console shows it.
type Target struct {
	// Name is what the admin called the channel; empty means the type's own name.
	Name string `json:"name"`
	// Type is the channel type: email, wecom, dingtalk, lark, pushdeer or telegram.
	Type string `json:"type"`
	// Detail says where on that channel, when it is worth showing (the email recipient).
	Detail string `json:"detail,omitempty"`
	// Ready is false when the channel cannot send (email without SMTP or recipient).
	Ready bool `json:"ready"`
}

type recipientNotifier interface {
	Recipient() string
	Ready() bool
}

// TargetOf describes a notifier for the console.
func TargetOf(name, typ string, n Notifier) Target {
	target := Target{Name: name, Type: typ, Ready: true}
	if r, ok := n.(recipientNotifier); ok {
		target.Detail, target.Ready = r.Recipient(), r.Ready()
	}
	return target
}

// Targets lists the enabled channels notices are sent to, in the order they were set up.
func Targets() []Target {
	list := destinations()
	out := make([]Target, 0, len(list))
	for _, d := range list {
		if d.Notifier != nil {
			out = append(out, TargetOf(d.Name, d.Type, d.Notifier))
		}
	}
	return out
}
