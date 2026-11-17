// Package encryption encrypts stored credentials (upstream channel keys, payment gateway
// secrets, notification channel secrets, sign-in client secrets) with AES-256-GCM.
//
// The master key comes from ENCRYPTION_KEY. The data key is derived from it with HKDF-SHA256, so
// the master key itself never touches a cipher. A stored value carries a versioned prefix
// ("enc:v1:" followed by base64 of nonce|ciphertext|tag), which lets ciphertext and legacy
// plaintext live side by side: a value without the prefix is read back unchanged. Every value is
// bound to a label (the table and column it lives in), so a ciphertext copied into another column
// does not decrypt.
//
// Without ENCRYPTION_KEY nothing is encrypted and values are written in plaintext as before.
// ENCRYPTION_KEY_PREVIOUS is a decrypt-only key for rotation: values under it still read, and the
// startup migration re-encrypts them under the current key.
package encryption

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
)

// Prefix marks a stored value as ciphertext of format version 1.
const Prefix = "enc:v1:"

// keySize is the length of the master key and of the derived data key (AES-256).
const keySize = 32

// dataKeyInfo is the HKDF context of the v1 data key. Changing it makes every stored value
// unreadable.
const dataKeyInfo = "modeltaps stored secret encryption v1"

var (
	// ErrNoKey is returned when a stored value is encrypted but no ENCRYPTION_KEY is configured.
	ErrNoKey = errors.New("the value is encrypted but ENCRYPTION_KEY is not set")
	// ErrUndecryptable is returned when no configured key decrypts a stored value: the key is
	// wrong, or the value was changed.
	ErrUndecryptable = errors.New("the value cannot be decrypted with ENCRYPTION_KEY or ENCRYPTION_KEY_PREVIOUS (wrong key or tampered value)")
)

type keyring struct {
	current  cipher.AEAD
	previous cipher.AEAD
}

var ring atomic.Pointer[keyring]

// Configure sets the master key and the optional previous (decrypt-only) key. Both are base64 or
// hex encodings of 32 random bytes. An empty current key turns encryption off; a previous key
// without a current key is rejected.
func Configure(currentKey, previousKey string) error {
	currentKey, previousKey = strings.TrimSpace(currentKey), strings.TrimSpace(previousKey)
	if currentKey == "" {
		if previousKey != "" {
			return errors.New("ENCRYPTION_KEY_PREVIOUS is set but ENCRYPTION_KEY is not")
		}
		ring.Store(nil)
		return nil
	}
	current, err := newAEAD(currentKey)
	if err != nil {
		return fmt.Errorf("ENCRYPTION_KEY: %w", err)
	}
	next := &keyring{current: current}
	if previousKey != "" {
		if previousKey == currentKey {
			return errors.New("ENCRYPTION_KEY_PREVIOUS must differ from ENCRYPTION_KEY")
		}
		if next.previous, err = newAEAD(previousKey); err != nil {
			return fmt.Errorf("ENCRYPTION_KEY_PREVIOUS: %w", err)
		}
	}
	ring.Store(next)
	return nil
}

// Enabled reports whether an ENCRYPTION_KEY is configured.
func Enabled() bool { return ring.Load() != nil }

// IsEncrypted reports whether a stored value is ciphertext (carries the prefix).
func IsEncrypted(stored string) bool { return strings.HasPrefix(stored, Prefix) }

// Encrypt returns the value to store for plaintext under label. It returns plaintext unchanged
// when encryption is off or plaintext is empty, so empty columns stay empty.
func Encrypt(plaintext, label string) (string, error) {
	keys := ring.Load()
	if keys == nil || plaintext == "" {
		return plaintext, nil
	}
	return seal(keys.current, plaintext, label)
}

// Decrypt returns the plaintext of a stored value. A value without the prefix is legacy plaintext
// and returned unchanged.
func Decrypt(stored, label string) (string, error) {
	plaintext, _, err := open(stored, label)
	return plaintext, err
}

// State says how a stored value is held.
type State int

const (
	// StateEmpty is an empty value; it is never encrypted.
	StateEmpty State = iota
	// StatePlaintext is a legacy plaintext value.
	StatePlaintext
	// StateCurrent is ciphertext under ENCRYPTION_KEY.
	StateCurrent
	// StatePrevious is ciphertext under ENCRYPTION_KEY_PREVIOUS.
	StatePrevious
	// StateUndecryptable is ciphertext no configured key opens (or no key is configured).
	StateUndecryptable
)

// Inspect reports the state of a stored value and, when it can be read, its plaintext.
func Inspect(stored, label string) (State, string) {
	if stored == "" {
		return StateEmpty, ""
	}
	if !IsEncrypted(stored) {
		return StatePlaintext, stored
	}
	plaintext, previous, err := open(stored, label)
	switch {
	case err != nil:
		return StateUndecryptable, ""
	case previous:
		return StatePrevious, plaintext
	default:
		return StateCurrent, plaintext
	}
}

// ParseKey decodes a master key given as standard or URL-safe base64 (padded or not) or as hex.
// It must hold exactly 32 bytes and must not be an obvious placeholder.
func ParseKey(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if isPlaceholder(raw) {
		return nil, errors.New("the key is a documentation placeholder; generate one with: openssl rand -base64 32")
	}
	var key []byte
	if len(raw) == hex.EncodedLen(keySize) {
		if b, err := hex.DecodeString(raw); err == nil {
			key = b
		}
	}
	if key == nil {
		for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
			if b, err := enc.DecodeString(raw); err == nil {
				key = b
				break
			}
		}
	}
	if key == nil {
		return nil, errors.New("the key is neither base64 nor hex; generate one with: openssl rand -base64 32")
	}
	if len(key) != keySize {
		return nil, fmt.Errorf("the key decodes to %d bytes, want %d; generate one with: openssl rand -base64 32", len(key), keySize)
	}
	if bytes.Count(key, key[:1]) == len(key) {
		return nil, errors.New("the key repeats a single byte; generate one with: openssl rand -base64 32")
	}
	return key, nil
}

// isPlaceholder catches values copied unchanged from the example files (".env.example",
// "config.example.yaml", the docs), such as "<change-me-to-a-random-string>".
func isPlaceholder(raw string) bool {
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "<") || strings.HasSuffix(lower, ">") {
		return true
	}
	for _, marker := range []string{"change-me", "changeme", "change_me", "replace-me", "your-", "your_", "placeholder", "example"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func newAEAD(rawKey string) (cipher.AEAD, error) {
	master, err := ParseKey(rawKey)
	if err != nil {
		return nil, err
	}
	dataKey, err := hkdf.Key(sha256.New, master, nil, dataKeyInfo, keySize)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(dataKey)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func seal(aead cipher.AEAD, plaintext, label string) (string, error) {
	nonce := make([]byte, aead.NonceSize(), aead.NonceSize()+len(plaintext)+aead.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	sealed := aead.Seal(nonce, nonce, []byte(plaintext), []byte(label))
	return Prefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// open decrypts stored and reports whether the previous key was needed.
func open(stored, label string) (string, bool, error) {
	if !IsEncrypted(stored) {
		return stored, false, nil
	}
	keys := ring.Load()
	if keys == nil {
		return "", false, ErrNoKey
	}
	sealed, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, Prefix))
	if err != nil {
		return "", false, ErrUndecryptable
	}
	if plaintext, ok := openWith(keys.current, sealed, label); ok {
		return plaintext, false, nil
	}
	if keys.previous != nil {
		if plaintext, ok := openWith(keys.previous, sealed, label); ok {
			return plaintext, true, nil
		}
	}
	return "", false, ErrUndecryptable
}

func openWith(aead cipher.AEAD, sealed []byte, label string) (string, bool) {
	size := aead.NonceSize()
	if len(sealed) < size+aead.Overhead() {
		return "", false
	}
	plaintext, err := aead.Open(nil, sealed[:size], sealed[size:], []byte(label))
	if err != nil {
		return "", false
	}
	return string(plaintext), true
}
