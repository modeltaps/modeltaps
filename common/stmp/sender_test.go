package stmp

import (
	"strings"
	"testing"
)

func TestSenderSplitsNameAndAddress(t *testing.T) {
	cases := []struct {
		name, from, fromName string
		wantName, wantAddr   string
	}{
		{"bare address", "noreply@example.com", "", "", "noreply@example.com"},
		{"display name option", "noreply@example.com", "模型水龙头", "模型水龙头", "noreply@example.com"},
		{"name written in From", "Modeltaps <noreply@example.com>", "", "Modeltaps", "noreply@example.com"},
		{"name option wins", "Old <noreply@example.com>", "New", "New", "noreply@example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &StmpConfig{From: tc.from, FromName: tc.fromName}
			name, addr, err := s.sender()
			if err != nil {
				t.Fatalf("sender() error: %v", err)
			}
			if name != tc.wantName || addr != tc.wantAddr {
				t.Fatalf("sender() = %q, %q; want %q, %q", name, addr, tc.wantName, tc.wantAddr)
			}
		})
	}
}

func TestSenderRejectsInvalidAddress(t *testing.T) {
	s := &StmpConfig{From: "not an address"}
	if _, _, err := s.sender(); err == nil {
		t.Fatal("sender() accepted an invalid address")
	}
}

func TestReferencesWithoutAt(t *testing.T) {
	if got := references("noreply"); !strings.HasSuffix(got, "@localhost>") {
		t.Fatalf("references() = %q", got)
	}
	if got := references("a@example.com"); !strings.HasPrefix(got, "<a.") || !strings.HasSuffix(got, "@example.com>") {
		t.Fatalf("references() = %q", got)
	}
}
