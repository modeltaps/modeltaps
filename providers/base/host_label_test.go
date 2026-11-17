package base

import "testing"

func TestIsHostLabel(t *testing.T) {
	for _, ok := range []string{"us-central1", "global", "eu", "eastus", "europe-west4"} {
		if !IsHostLabel(ok) {
			t.Errorf("IsHostLabel(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "attacker.tld/x?", "evil.com", "a@b", "x#", "host:9977", "US-EAST1", "a b", "a\n", "a/b"} {
		if IsHostLabel(bad) {
			t.Errorf("IsHostLabel(%q) = true, want false", bad)
		}
	}
}
