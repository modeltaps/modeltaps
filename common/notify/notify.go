package notify

import (
	"context"
	"sync"

	"github.com/modeltaps/modeltaps/common/notify/channel"
)

// Notifier delivers one notice to one place.
type Notifier interface {
	Send(context.Context, string, string) error
	Name() string
}

// Destination is one place admin notices go: a notification channel an admin set up in the
// settings (see model.NotifyChannel), already turned into a notifier.
type Destination struct {
	// Name is what the admin called it; empty falls back to the type's own name.
	Name string
	// Type is the channel type (TypeEmail, TypeTelegram, ...).
	Type     string
	Notifier Notifier
	// Kinds limits the channel to these notice kinds; empty means every kind.
	Kinds []string
}

// Wants reports whether the channel takes notices of this kind. A notice sent without a kind
// (Send) goes everywhere.
func (d Destination) Wants(kind string) bool {
	if kind == "" || len(d.Kinds) == 0 {
		return true
	}
	for _, k := range d.Kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// Delivery is one notice handed to one channel, as the send record keeps it.
type Delivery struct {
	Kind  string
	Type  string
	Name  string
	Title string
	Err   error
}

// OnDelivered, when set, is told about every delivery after it succeeded or failed.
var OnDelivered func(Delivery)

// WithKind tells notifiers the kind of the notice they send (the generic webhook passes it on).
func WithKind(ctx context.Context, kind string) context.Context {
	return channel.WithKind(ctx, kind)
}

var (
	mu sync.RWMutex
	// source lists the enabled channels. It is read on every notice, so a change made on any
	// node applies everywhere without a reload; notices are rare and rate limited.
	source func() []Destination
	// fixed notifiers are added in-process (tests); they get every notice too.
	fixed []Notifier
)

// SetSource sets where the enabled notification channels come from.
func SetSource(fn func() []Destination) {
	mu.Lock()
	defer mu.Unlock()
	source = fn
}

// AddNotifiers adds notifiers that receive every notice besides the configured channels. A
// notifier whose name is already added is ignored.
func AddNotifiers(notifiers ...Notifier) {
	mu.Lock()
	defer mu.Unlock()
next:
	for _, n := range notifiers {
		if n == nil {
			continue
		}
		for _, f := range fixed {
			if f.Name() == n.Name() {
				continue next
			}
		}
		fixed = append(fixed, n)
	}
}

func destinations() []Destination {
	mu.RLock()
	fn, extra := source, append([]Notifier(nil), fixed...)
	mu.RUnlock()
	var out []Destination
	if fn != nil {
		out = fn()
	}
	for _, n := range extra {
		out = append(out, Destination{Name: n.Name(), Notifier: n})
	}
	return out
}
