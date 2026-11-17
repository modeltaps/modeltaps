package common

import (
	"strings"
	"testing"
)

func TestManagementKeyFormat(t *testing.T) {
	key, err := NewManagementKey()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key, ManagementKeyPrefix) || !IsManagementKeyFormat(key) {
		t.Fatalf("new management key has the wrong shape: %q", key)
	}
	if HashManagementKey("Bearer "+key) != HashManagementKey(key) {
		t.Fatal("the Bearer prefix must not change the hash")
	}
	if hint := ManagementKeyHint(key); strings.Contains(hint, key[3:]) || !strings.HasPrefix(hint, "mk-") {
		t.Fatalf("hint leaks the key or lost its prefix: %q", hint)
	}
	if LooksLikeAPIKey(key) {
		t.Fatal("a management key (59 characters, like a legacy signed key) is not an API key")
	}
}

// Every API key format is recognized, so the management API can tell the caller to use a
// management key; legacy access tokens (32 hex characters) are not API keys.
func TestLooksLikeAPIKey(t *testing.T) {
	apiKey, _ := NewUserTokenKey()
	playground, _ := NewPlaygroundKey()
	for _, credential := range []string{
		"sk-anything",
		"Bearer sk-" + apiKey,
		apiKey,
		playground,
		strings.Repeat("a", LegacySignedTokenLength),
		strings.Repeat("b", LegacyRandomTokenLength),
	} {
		if !LooksLikeAPIKey(credential) {
			t.Fatalf("%q should look like an API key", credential)
		}
	}
	if LooksLikeAPIKey("0123456789abcdef0123456789abcdef") {
		t.Fatal("a legacy access token is not an API key")
	}
}
