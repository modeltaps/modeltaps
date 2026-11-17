package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// MinSecretLength is the shortest session or token secret the gateway starts with. API keys
// are an HMAC of public ids, so a short user_token_secret can be recovered offline from one key
// and then used to forge a key for any user.
const MinSecretLength = 32

// placeholderMarkers are the fill-in values shipped in .env.example, config.example.yaml and the
// README, plus common stand-ins. A secret containing one was copied without being replaced.
var placeholderMarkers = []string{"change-me", "change_me", "changeme", "replace-me", "replace_me", "your-secret", "your_secret"}

// IsPlaceholderSecret reports whether a secret is a documentation placeholder.
func IsPlaceholderSecret(value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))
	if strings.HasPrefix(v, "<") && strings.HasSuffix(v, ">") {
		return true
	}
	for _, m := range placeholderMarkers {
		if strings.Contains(v, m) {
			return true
		}
	}
	return false
}

func checkSecret(name, value string) error {
	if IsPlaceholderSecret(value) {
		return fmt.Errorf("%s is still the example placeholder; set it to a random string, e.g. openssl rand -base64 48", name)
	}
	if len(value) < MinSecretLength {
		return fmt.Errorf("%s is %d characters long; it must be at least %d, e.g. openssl rand -base64 48", name, len(value), MinSecretLength)
	}
	return nil
}

// ValidateSecrets checks the configured secrets at startup. It returns the problems that must stop
// the gateway, and warnings that only need to be logged.
//
// session_secret is checked only when set (an unset one is random per run). Changing it signs
// everyone out, nothing worse.
//
// user_token_secret is required. A placeholder always stops startup. A short one stops startup too,
// unless user_token_secret_allow_short is true: replacing the secret invalidates every API key
// issued so far, so an existing deployment can keep running with a warning until it rotates.
func ValidateSecrets() (errs []error, warnings []string) {
	if SessionSecretExplicit {
		if err := checkSecret("SESSION_SECRET", viper.GetString("session_secret")); err != nil {
			errs = append(errs, err)
		}
	}

	token := viper.GetString("user_token_secret")
	if token != "" {
		if err := checkSecret("USER_TOKEN_SECRET", token); err != nil {
			if !IsPlaceholderSecret(token) && viper.GetBool("user_token_secret_allow_short") {
				warnings = append(warnings, err.Error()+" (allowed by USER_TOKEN_SECRET_ALLOW_SHORT; rotating it invalidates all API keys)")
			} else {
				errs = append(errs, err)
			}
		}
	}
	return errs, warnings
}
