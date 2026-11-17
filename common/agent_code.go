package common

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Codes that hand an API key to an agent without the key passing through the user (AGENT-7).
//
// A pairing code is "pair-" followed by an 8-character lookup and a 32-character secret (base62).
// The lookup is stored as it is, so a wrong guess can be counted against the code it names; the
// secret only as its hash. The lookup alone gives nothing: 32 base62 characters are ~190 bits.
//
// The device flow (RFC 8628) uses a long random device code, which the agent keeps and polls with
// (stored as its hash), and a short user code the user types or follows a link with.
const (
	PairingCodePrefix       = "pair-"
	pairingCodeLookupLength = 8
	pairingCodeSecretLength = 32
	DeviceCodeLength        = 43
	deviceUserCodeLength    = 8
	// deviceUserCodeChars are the RFC 8628 §6.1 consonants: no vowels (no words), no lookalikes.
	deviceUserCodeChars = "BCDFGHJKLMNPQRSTVWXZ"
)

// NewPairingCode returns a new pairing code and its two parts.
func NewPairingCode() (code, lookup, secret string, err error) {
	lookup, err = randomBase62(pairingCodeLookupLength)
	if err != nil {
		return "", "", "", err
	}
	secret, err = randomBase62(pairingCodeSecretLength)
	if err != nil {
		return "", "", "", err
	}
	return PairingCodePrefix + lookup + secret, lookup, secret, nil
}

// ParsePairingCode splits a pairing code into its lookup and secret. Surrounding blanks and the
// prefix are optional; anything else malformed returns ok == false.
func ParsePairingCode(code string) (lookup, secret string, ok bool) {
	code = strings.TrimPrefix(strings.TrimSpace(code), PairingCodePrefix)
	if len(code) != pairingCodeLookupLength+pairingCodeSecretLength || !isBase62(code) {
		return "", "", false
	}
	return code[:pairingCodeLookupLength], code[pairingCodeLookupLength:], true
}

// NewDeviceCode returns a new device code for the device authorization flow.
func NewDeviceCode() (string, error) {
	return randomBase62(DeviceCodeLength)
}

// NewDeviceUserCode returns a new user code in its normalized form (8 letters, no dash).
func NewDeviceUserCode() (string, error) {
	limit := byte(256 - 256%len(deviceUserCodeChars))
	out := make([]byte, 0, deviceUserCodeLength)
	buf := make([]byte, 2*deviceUserCodeLength)
	for len(out) < deviceUserCodeLength {
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("failed to read random bytes: %w", err)
		}
		for _, b := range buf {
			if b >= limit {
				continue
			}
			out = append(out, deviceUserCodeChars[int(b)%len(deviceUserCodeChars)])
			if len(out) == deviceUserCodeLength {
				break
			}
		}
	}
	return string(out), nil
}

// NormalizeDeviceUserCode turns what a user typed ("abcd-efgh", "ABCD EFGH") into the stored form,
// or "" when it cannot be a user code.
func NormalizeDeviceUserCode(input string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(input) {
		if r == '-' || r == ' ' {
			continue
		}
		if !strings.ContainsRune(deviceUserCodeChars, r) {
			return ""
		}
		b.WriteRune(r)
	}
	if b.Len() != deviceUserCodeLength {
		return ""
	}
	return b.String()
}

// FormatDeviceUserCode is the form shown to people: "BCDF-GHJK".
func FormatDeviceUserCode(code string) string {
	if len(code) != deviceUserCodeLength {
		return code
	}
	return code[:4] + "-" + code[4:]
}

// HashAgentSecret is the stored form of a pairing code secret or a device code: hex SHA-256. Both
// carry far more entropy than could be brute-forced from the hash.
func HashAgentSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
