package notify

import (
	"context"
	"sync"
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
