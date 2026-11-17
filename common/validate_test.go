package common

import "testing"

func TestCanonicalEmail(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"   ", ""},
		{"Foo@Example.com", "foo@example.com"},
		{"  foo@example.com  ", "foo@example.com"},
		// Gmail ignores dots and "+suffix" in the local part, and googlemail.com is gmail.com.
		{"a.b@gmail.com", "ab@gmail.com"},
		{"A.B+promo@GMail.com", "ab@gmail.com"},
		{"ab+1@gmail.com", "ab@gmail.com"},
		{"a.b.c+x+y@googlemail.com", "abc@gmail.com"},
		{"ab@googlemail.com", "ab@gmail.com"},
		// Other domains keep their dots: only the case and the "+suffix" are dropped.
		{"first.last@example.com", "first.last@example.com"},
		{"first.last+tag@example.com", "first.last@example.com"},
		{"user@sub.gmail.com", "user@sub.gmail.com"},
		// Nothing usable before or after "@": the address is only normalized.
		{"+tag@example.com", "+tag@example.com"},
		{"...@gmail.com", "...@gmail.com"},
		{"no-at-sign", "no-at-sign"},
		{"@gmail.com", "@gmail.com"},
		{"ab@", "ab@"},
	}
	for _, tc := range cases {
		if got := CanonicalEmail(tc.in); got != tc.want {
			t.Errorf("CanonicalEmail(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
