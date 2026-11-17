package encryption

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func testKey(fill byte) string {
	key := make([]byte, keySize)
	for i := range key {
		key[i] = fill + byte(i)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func configure(t *testing.T, current, previous string) {
	t.Helper()
	if err := Configure(current, previous); err != nil {
		t.Fatalf("configure: %v", err)
	}
	t.Cleanup(func() { _ = Configure("", "") })
}

func TestRoundTrip(t *testing.T) {
	configure(t, testKey(1), "")
	stored, err := Encrypt("sk-upstream-secret", "channels.key")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, Prefix) || strings.Contains(stored, "sk-upstream-secret") {
		t.Fatalf("stored value is not ciphertext: %q", stored)
	}
	again, _ := Encrypt("sk-upstream-secret", "channels.key")
	if again == stored {
		t.Error("two encryptions of the same value must use different nonces")
	}
	got, err := Decrypt(stored, "channels.key")
	if err != nil || got != "sk-upstream-secret" {
		t.Fatalf("decrypt = %q, %v", got, err)
	}
	if state, plain := Inspect(stored, "channels.key"); state != StateCurrent || plain != "sk-upstream-secret" {
		t.Errorf("inspect = %v %q", state, plain)
	}
}

func TestEmptyStaysEmpty(t *testing.T) {
	configure(t, testKey(1), "")
	stored, err := Encrypt("", "channels.key")
	if err != nil || stored != "" {
		t.Fatalf("encrypt empty = %q, %v", stored, err)
	}
	if state, _ := Inspect("", "channels.key"); state != StateEmpty {
		t.Errorf("inspect empty = %v", state)
	}
}

func TestTamperDetection(t *testing.T) {
	configure(t, testKey(1), "")
	stored, _ := Encrypt("sk-upstream-secret", "channels.key")
	raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, Prefix))
	raw[len(raw)-1] ^= 0x01
	tampered := Prefix + base64.StdEncoding.EncodeToString(raw)
	if _, err := Decrypt(tampered, "channels.key"); !errors.Is(err, ErrUndecryptable) {
		t.Errorf("tampered ciphertext: err = %v", err)
	}
	if _, err := Decrypt(Prefix+"!!not-base64!!", "channels.key"); !errors.Is(err, ErrUndecryptable) {
		t.Errorf("malformed ciphertext: err = %v", err)
	}
	if _, err := Decrypt(Prefix+"AAAA", "channels.key"); !errors.Is(err, ErrUndecryptable) {
		t.Errorf("short ciphertext: err = %v", err)
	}
	// The label binds the value to its column.
	if _, err := Decrypt(stored, "channels.manage_key"); !errors.Is(err, ErrUndecryptable) {
		t.Errorf("ciphertext moved to another column: err = %v", err)
	}
	if state, _ := Inspect(tampered, "channels.key"); state != StateUndecryptable {
		t.Errorf("inspect tampered = %v", state)
	}
}

func TestWrongKey(t *testing.T) {
	configure(t, testKey(1), "")
	stored, _ := Encrypt("sk-upstream-secret", "channels.key")
	configure(t, testKey(50), "")
	if _, err := Decrypt(stored, "channels.key"); !errors.Is(err, ErrUndecryptable) {
		t.Errorf("wrong key: err = %v", err)
	}
}

func TestLegacyPlaintextPassthrough(t *testing.T) {
	configure(t, testKey(1), "")
	got, err := Decrypt("sk-legacy-plaintext", "channels.key")
	if err != nil || got != "sk-legacy-plaintext" {
		t.Fatalf("legacy plaintext = %q, %v", got, err)
	}
	if state, plain := Inspect("sk-legacy-plaintext", "channels.key"); state != StatePlaintext || plain != "sk-legacy-plaintext" {
		t.Errorf("inspect legacy = %v %q", state, plain)
	}
}

func TestNoKeyPassthrough(t *testing.T) {
	configure(t, "", "")
	if Enabled() {
		t.Fatal("encryption must be off without a key")
	}
	stored, err := Encrypt("sk-upstream-secret", "channels.key")
	if err != nil || stored != "sk-upstream-secret" {
		t.Fatalf("encrypt without key = %q, %v", stored, err)
	}
	got, err := Decrypt("sk-upstream-secret", "channels.key")
	if err != nil || got != "sk-upstream-secret" {
		t.Fatalf("decrypt without key = %q, %v", got, err)
	}

	configure(t, testKey(1), "")
	cipherText, _ := Encrypt("sk-upstream-secret", "channels.key")
	configure(t, "", "")
	if _, err := Decrypt(cipherText, "channels.key"); !errors.Is(err, ErrNoKey) {
		t.Errorf("ciphertext without key: err = %v", err)
	}
	if state, _ := Inspect(cipherText, "channels.key"); state != StateUndecryptable {
		t.Errorf("inspect without key = %v", state)
	}
}

func TestPreviousKeyRotation(t *testing.T) {
	oldKey, newKey := testKey(1), testKey(100)
	configure(t, oldKey, "")
	underOld, _ := Encrypt("sk-upstream-secret", "channels.key")

	configure(t, newKey, oldKey)
	got, err := Decrypt(underOld, "channels.key")
	if err != nil || got != "sk-upstream-secret" {
		t.Fatalf("decrypt under previous key = %q, %v", got, err)
	}
	if state, _ := Inspect(underOld, "channels.key"); state != StatePrevious {
		t.Errorf("inspect under previous key = %v", state)
	}
	underNew, _ := Encrypt("sk-upstream-secret", "channels.key")
	if state, _ := Inspect(underNew, "channels.key"); state != StateCurrent {
		t.Errorf("inspect under current key = %v", state)
	}

	// Without the previous key the old value no longer reads.
	configure(t, newKey, "")
	if _, err := Decrypt(underOld, "channels.key"); !errors.Is(err, ErrUndecryptable) {
		t.Errorf("old value without previous key: err = %v", err)
	}
}

func TestConfigureRejectsBadKeys(t *testing.T) {
	t.Cleanup(func() { _ = Configure("", "") })
	key := make([]byte, keySize)
	for i := range key {
		key[i] = byte(i * 7)
	}
	for name, raw := range map[string]string{
		"base64":        base64.StdEncoding.EncodeToString(key),
		"raw base64":    base64.RawStdEncoding.EncodeToString(key),
		"url base64":    base64.URLEncoding.EncodeToString(key),
		"hex":           hex.EncodeToString(key),
		"padded spaces": "  " + base64.StdEncoding.EncodeToString(key) + "\n",
	} {
		if err := Configure(raw, ""); err != nil {
			t.Errorf("%s key rejected: %v", name, err)
		}
	}

	for name, raw := range map[string]string{
		"placeholder":  "<change-me-to-a-random-base64-32-byte-key>",
		"changeme":     "changeme",
		"too short":    base64.StdEncoding.EncodeToString(key[:16]),
		"too long":     base64.StdEncoding.EncodeToString(append(key, key...)),
		"not encoded":  "this is not a key at all!",
		"all zero":     base64.StdEncoding.EncodeToString(make([]byte, keySize)),
		"hex too long": hex.EncodeToString(append(key, 1)),
	} {
		if err := Configure(raw, ""); err == nil {
			t.Errorf("%s key accepted", name)
		}
	}

	if err := Configure("", testKey(1)); err == nil {
		t.Error("a previous key without a current key must be rejected")
	}
	if err := Configure(testKey(1), testKey(1)); err == nil {
		t.Error("a previous key equal to the current key must be rejected")
	}
	if err := Configure(testKey(1), "<placeholder>"); err == nil {
		t.Error("a malformed previous key must be rejected")
	}
}
