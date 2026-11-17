package base

import "regexp"

// hostLabelPattern is what a channel setting may hold when a provider puts it into a hostname,
// such as a cloud region: lowercase letters, digits and hyphens only.
var hostLabelPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// IsHostLabel reports whether s can be put into a hostname without changing which host it names.
// A value with a dot, slash, "@", "?", "#" or ":" could point the request, and the channel key
// sent with it, at another host.
func IsHostLabel(s string) bool {
	return hostLabelPattern.MatchString(s)
}
