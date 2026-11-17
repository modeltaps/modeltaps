package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

const goodSecret = "Zr8xQm2vN5pL7kT3wY9aB4cD6eF1gH0jK"

func TestIsPlaceholderSecret(t *testing.T) {
	for _, v := range []string{
		"<change-me-to-a-random-string>",
		"<change-me-to-a-32-char-random-string>",
		"change_me_to_a_32_char_random_string",
		"<anything in angle brackets that is long enough>",
		"CHANGEME-please-0123456789012345678",
	} {
		if !IsPlaceholderSecret(v) {
			t.Errorf("%q not detected as a placeholder", v)
		}
	}
	if IsPlaceholderSecret(goodSecret) {
		t.Errorf("random secret detected as a placeholder")
	}
}

func withSecrets(t *testing.T, session, token string, allowShort bool) {
	t.Helper()
	oldExplicit := SessionSecretExplicit
	viper.Set("session_secret", session)
	viper.Set("user_token_secret", token)
	viper.Set("user_token_secret_allow_short", allowShort)
	SessionSecretExplicit = session != ""
	t.Cleanup(func() {
		viper.Set("session_secret", "")
		viper.Set("user_token_secret", "")
		viper.Set("user_token_secret_allow_short", false)
		SessionSecretExplicit = oldExplicit
	})
}

func TestValidateSecrets(t *testing.T) {
	cases := []struct {
		name       string
		session    string
		token      string
		allowShort bool
		errs       int
		warnings   int
	}{
		{"both good", goodSecret, goodSecret, false, 0, 0},
		{"session unset is fine", "", goodSecret, false, 0, 0},
		{"placeholder session", "<change-me-to-a-random-string>", goodSecret, false, 1, 0},
		{"short session", "short-session-secret", goodSecret, false, 1, 0},
		{"placeholder token", goodSecret, "<change-me-to-a-32-char-random-string>", false, 1, 0},
		{"placeholder token even when short is allowed", goodSecret, "<change-me-to-a-32-char-random-string>", true, 1, 0},
		{"short token", goodSecret, "only-twenty-chars-xx", false, 1, 0},
		{"short token allowed", goodSecret, "only-twenty-chars-xx", true, 0, 1},
		{"both bad", "x", "y", false, 2, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withSecrets(t, tc.session, tc.token, tc.allowShort)
			errs, warnings := ValidateSecrets()
			if len(errs) != tc.errs || len(warnings) != tc.warnings {
				t.Fatalf("errs=%v warnings=%v, want %d errors and %d warnings", errs, warnings, tc.errs, tc.warnings)
			}
			for _, e := range errs {
				if strings.Contains(e.Error(), tc.token) && tc.token != "" || tc.session != "" && strings.Contains(e.Error(), tc.session) {
					t.Fatalf("error message leaks the secret: %v", e)
				}
			}
		})
	}
}
