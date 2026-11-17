package common

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"strings"
	"sync"

	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/spf13/viper"
	"github.com/sqids/sqids-go"
)

// User API keys (SEC-47). Clients send "sk-" followed by the key; the prefix is optional and is
// not part of what is hashed. Three formats are accepted:
//
//   - current keys: UserTokenKeyLength characters drawn from crypto/rand (base62, about 333 bits);
//   - legacy signed keys (59 characters): a sqids payload of (token id, user id), "_" and an
//     HMAC-SHA256 of the payload under user_token_secret. They are no longer issued; the signature
//     is still checked as a cheap filter before the database is asked;
//   - legacy random keys (48 characters), issued by the oldest releases.
//
// Playground keys (PlaygroundKeyPrefix + PlaygroundKeySecretLength characters) are short-lived
// credentials of the console playground, see model.PlaygroundKey.
//
// Only HashUserTokenKey(key) and a masked hint are stored; the key itself is shown once.
const (
	UserTokenKeyLength        = 56
	LegacySignedTokenLength   = 59
	LegacyRandomTokenLength   = 48
	PlaygroundKeyPrefix       = "pg_"
	PlaygroundKeySecretLength = 48
)

const userTokenKeyChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var (
	hashidsMinLength = 15
	hashids          *sqids.Sqids

	jwtSecretBytes = []byte{}
	hmacPool       = sync.Pool{
		New: func() interface{} {
			return hmac.New(sha256.New, jwtSecretBytes)
		},
	}
)

func InitUserToken() error {
	tokenSecret := viper.GetString("user_token_secret")
	sqidsAlphabet := viper.GetString("hashids_salt")

	if tokenSecret == "" {
		return errors.New("token_secret or hashids_salt is not set")
	}

	var err error

	sqidsOptions := sqids.Options{
		MinLength: uint8(hashidsMinLength),
	}

	if sqidsAlphabet != "" {
		sqidsOptions.Alphabet = sqidsAlphabet
	}

	hashids, err = sqids.New(sqidsOptions)

	jwtSecretBytes = []byte(tokenSecret)

	return err
}

// randomBase62 returns n characters drawn uniformly from crypto/rand. Unlike
// utils.GetSecureRandomString it never falls back to a weaker source: it fails instead.
func randomBase62(n int) (string, error) {
	// Rejection sampling keeps the distribution uniform (256 is not a multiple of 62).
	limit := byte(256 - 256%len(userTokenKeyChars))
	out := make([]byte, 0, n)
	buf := make([]byte, n+n/4)
	for len(out) < n {
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("failed to read random bytes: %w", err)
		}
		for _, b := range buf {
			if b >= limit {
				continue
			}
			out = append(out, userTokenKeyChars[int(b)%len(userTokenKeyChars)])
			if len(out) == n {
				break
			}
		}
	}
	return string(out), nil
}

// NewUserTokenKey returns a new random API key, without the "sk-" prefix.
func NewUserTokenKey() (string, error) {
	return randomBase62(UserTokenKeyLength)
}

// NewPlaygroundKey returns a new random playground key, without the "sk-" prefix.
func NewPlaygroundKey() (string, error) {
	secret, err := randomBase62(PlaygroundKeySecretLength)
	if err != nil {
		return "", err
	}
	return PlaygroundKeyPrefix + secret, nil
}

// NormalizeUserTokenKey strips the optional "Bearer " and "sk-" prefixes and surrounding blanks.
func NormalizeUserTokenKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.TrimPrefix(key, "Bearer ")
	key = strings.TrimPrefix(key, "sk-")
	return strings.TrimSpace(key)
}

// HashUserTokenKey is the value stored in tokens.key_hash: the hex SHA-256 of the key without
// its "sk-" prefix. A plain hash is enough because every accepted key carries well over 192 bits
// of entropy, so there is nothing to brute-force from it.
func HashUserTokenKey(key string) string {
	sum := sha256.Sum256([]byte(NormalizeUserTokenKey(key)))
	return hex.EncodeToString(sum[:])
}

// UserTokenKeyHint is the non-secret form of a key kept for lists: the first and last four
// characters of "sk-<key>", the same mask the console uses for every secret.
func UserTokenKeyHint(key string) string {
	return utils.MaskSecret("sk-" + NormalizeUserTokenKey(key))
}

func isBase62(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return false
		}
	}
	return true
}

// IsUserTokenKeyFormat reports whether key (without "sk-") has the shape of a current API key.
func IsUserTokenKeyFormat(key string) bool {
	return len(key) == UserTokenKeyLength && isBase62(key)
}

// IsLegacyRandomTokenFormat reports whether key has the length of a 48-character legacy key. The
// characters are not checked: the oldest releases did not all use the same alphabet.
func IsLegacyRandomTokenFormat(key string) bool {
	return len(key) == LegacyRandomTokenLength
}

// IsPlaygroundKeyFormat reports whether key (without "sk-") has the shape of a playground key.
func IsPlaygroundKeyFormat(key string) bool {
	secret, ok := strings.CutPrefix(key, PlaygroundKeyPrefix)
	return ok && len(secret) == PlaygroundKeySecretLength && isBase62(secret)
}

// Management keys (AGENT-4) call the management API (/api/...) and MCP for their user; they can
// never call model APIs. They are ManagementKeyPrefix followed by ManagementKeySecretLength base62
// characters and, like API keys, only their hash and a masked hint are stored.
//
// Legacy access tokens (32 hex characters, the old per-user "system access token") are stored
// the same way after migration and are accepted for one release as a deprecated path.
const (
	ManagementKeyPrefix       = "mk-"
	ManagementKeySecretLength = 56
)

// NewManagementKey returns a new random management key, prefix included.
func NewManagementKey() (string, error) {
	secret, err := randomBase62(ManagementKeySecretLength)
	if err != nil {
		return "", err
	}
	return ManagementKeyPrefix + secret, nil
}

// NormalizeManagementKey strips the optional "Bearer " prefix and surrounding blanks. Unlike API
// keys the prefix ("mk-") is part of the key.
func NormalizeManagementKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.TrimPrefix(key, "Bearer ")
	return strings.TrimSpace(key)
}

// HashManagementKey is the value stored in management_keys.key_hash: the hex SHA-256 of the
// normalized key. Management keys carry about 333 bits of entropy and legacy access tokens 122,
// so a plain hash leaves nothing to brute-force.
func HashManagementKey(key string) string {
	sum := sha256.Sum256([]byte(NormalizeManagementKey(key)))
	return hex.EncodeToString(sum[:])
}

// ManagementKeyHint is the masked form of a management key kept for lists.
func ManagementKeyHint(key string) string {
	return utils.MaskSecret(NormalizeManagementKey(key))
}

// IsManagementKeyFormat reports whether key (normalized) has the shape of a management key.
func IsManagementKeyFormat(key string) bool {
	secret, ok := strings.CutPrefix(key, ManagementKeyPrefix)
	return ok && len(secret) == ManagementKeySecretLength && isBase62(secret)
}

// LooksLikeAPIKey reports whether a credential sent to the management API is an API key (a
// model-call key) rather than a management key or a legacy access token: anything sent with the
// "sk-" prefix, and every API key format with or without it.
func LooksLikeAPIKey(credential string) bool {
	credential = NormalizeManagementKey(credential)
	if strings.HasPrefix(credential, "sk-") {
		return true
	}
	// A management key has the length of a legacy signed key.
	if strings.HasPrefix(credential, ManagementKeyPrefix) {
		return false
	}
	return IsUserTokenKeyFormat(credential) || IsPlaygroundKeyFormat(credential) ||
		len(credential) == LegacySignedTokenLength || IsLegacyRandomTokenFormat(credential)
}

// ValidateToken checks the signature of a legacy 59-character key and returns the ids encoded in
// it. A valid signature does not make a key valid: it must still be found by its hash.
func ValidateToken(token string) (tokenID, userID int, err error) {
	parts := bytes.SplitN([]byte(token), []byte("_"), 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("Invalid API key")
	}

	payloadEncoded, receivedSignature := parts[0], parts[1]

	h := hmacPool.Get().(hash.Hash)
	defer func() {
		h.Reset()
		hmacPool.Put(h)
	}()

	h.Write(payloadEncoded)
	expectedSignature := h.Sum(nil)

	decodedSignature, err := base64.RawURLEncoding.DecodeString(string(receivedSignature))
	if err != nil {
		return 0, 0, fmt.Errorf("Failed to decode signature")
	}

	if !hmac.Equal(decodedSignature, expectedSignature) {
		return 0, 0, fmt.Errorf("Signature verification failed")
	}

	numbers := hashids.Decode(string(payloadEncoded))
	if len(numbers) != 2 {
		return 0, 0, fmt.Errorf("Invalid API key")
	}

	return int(numbers[0]), int(numbers[1]), nil
}
