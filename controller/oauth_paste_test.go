package controller

import (
	"errors"
	"testing"

	"github.com/modeltaps/modeltaps/common"
)

func TestParseOAuthPaste(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  oauthPaste
	}{
		{
			name:  "codex callback url",
			input: "  http://localhost:1455/auth/callback?code=ac_abcdefghijkl&scope=openid&state=st-1\n",
			want:  oauthPaste{Code: "ac_abcdefghijkl", State: "st-1"},
		},
		{
			name:  "callback url without scheme typed as query",
			input: "?code=ac_abcdefghijkl&state=st-1",
			want:  oauthPaste{Code: "ac_abcdefghijkl", State: "st-1"},
		},
		{
			name:  "claude code with state",
			input: "abcdefghijklmnop#st-2",
			want:  oauthPaste{Code: "abcdefghijklmnop", State: "st-2"},
		},
		{
			name:  "bare code",
			input: "abcdefghijklmnop",
			want:  oauthPaste{Code: "abcdefghijklmnop"},
		},
		{
			name:  "parameters in the fragment",
			input: "https://example.com/cb#code=abcdefghijkl&state=st-3",
			want:  oauthPaste{Code: "abcdefghijkl", State: "st-3"},
		},
		{
			name:  "provider error",
			input: "http://localhost:1455/auth/callback?error=access_denied&error_description=User+cancelled&state=st-4",
			want:  oauthPaste{Error: "access_denied", ErrorDescription: "User cancelled", State: "st-4"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseOAuthPaste(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParseOAuthPasteRejectsInputWithoutCode(t *testing.T) {
	for _, input := range []string{
		"",
		"   ",
		"short",
		// The authorization link itself, pasted before approving.
		"https://auth.openai.com/oauth/authorize?client_id=app_x&response_type=code&state=st",
		"two words here",
	} {
		if _, err := parseOAuthPaste(input); !errors.Is(err, errOAuthPasteNoCode) {
			t.Errorf("parseOAuthPaste(%q) error = %v, want errOAuthPasteNoCode", input, err)
		}
	}
}

func TestCheckOAuthPaste(t *testing.T) {
	cases := []struct {
		name, input, wantCode, wantErr string
	}{
		{"matching state", "http://localhost:1455/auth/callback?code=ac_abcdefghijkl&state=good", "ac_abcdefghijkl", ""},
		{"bare code is accepted", "ac_abcdefghijkl", "ac_abcdefghijkl", ""},
		{"other attempt", "http://localhost:1455/auth/callback?code=ac_abcdefghijkl&state=old", "", common.APICodeChannelOAuthStateMismatch},
		{"claude other attempt", "abcdefghijklmnop#old", "", common.APICodeChannelOAuthStateMismatch},
		{"denied", "http://localhost:1455/auth/callback?error=access_denied&state=good", "", common.APICodeChannelOAuthAccessDenied},
		{"garbage", "hello", "", common.APICodeChannelOAuthInvalidInput},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, apiErr := checkOAuthPaste(tc.input, "good")
			errCode := ""
			if apiErr != nil {
				errCode = apiErr.Code
			}
			if code != tc.wantCode || errCode != tc.wantErr {
				t.Fatalf("got (%q, %q), want (%q, %q)", code, errCode, tc.wantCode, tc.wantErr)
			}
		})
	}
}

func TestCheckOAuthPasteKeepsProviderReason(t *testing.T) {
	_, apiErr := checkOAuthPaste("http://localhost:1455/auth/callback?error=access_denied&error_description=User+cancelled", "good")
	if apiErr == nil || apiErr.Params["reason"] != "access_denied (User cancelled)" {
		t.Fatalf("got %+v", apiErr)
	}
}

func TestIsOAuthCodeRejected(t *testing.T) {
	if !isOAuthCodeRejected(400, []byte(`{"error":"invalid_grant","error_description":"Code already used"}`)) {
		t.Fatal("invalid_grant should be treated as a rejected code")
	}
	if isOAuthCodeRejected(500, []byte(`invalid_grant`)) || isOAuthCodeRejected(400, []byte(`{"error":"invalid_request"}`)) {
		t.Fatal("only 400/401 invalid_grant means the code is unusable")
	}
}
