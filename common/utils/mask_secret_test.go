package utils

import "testing"

func TestMaskSecret(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"   ", ""},
		{"short", "****"},
		{"1234567", "****"},
		{"12345678", "****5678"},
		{"abcdefghijklmno", "****lmno"},
		{"abcdefghijklmnop", "abcd****mnop"},
		{"sk-live-0123456789abcdefghijklmnopqrstuvwxyz", "sk-l****wxyz"},
		{"  sk-live-0123456789abcdefghijklmnopqrstuvwxyz\n", "sk-l****wxyz"},
		{"密钥密钥密钥密钥密钥密钥密钥密钥", "密钥密钥****密钥密钥"},
	}
	for _, tc := range cases {
		if got := MaskSecret(tc.in); got != tc.want {
			t.Errorf("MaskSecret(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
