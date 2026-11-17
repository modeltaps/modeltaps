package model

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/utils"

	"github.com/spf13/viper"
	"github.com/sqids/sqids-go"
)

// SEC-47: API keys are stored as a hash and a masked hint; the key itself is only returned by the
// request that created or reset it.

// legacySignedKey builds a key the way releases before SEC-47 derived it from (token id, user id)
// and user_token_secret, to stand in for keys already in use.
func legacySignedKey(t *testing.T, tokenID, userID int) string {
	t.Helper()
	encoder, err := sqids.New(sqids.Options{MinLength: 15})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := encoder.Encode([]uint64{uint64(tokenID), uint64(userID)})
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, []byte(viper.GetString("user_token_secret")))
	mac.Write([]byte(payload))
	key := payload + "_" + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if len(key) != common.LegacySignedTokenLength {
		t.Fatalf("legacy key has %d characters, want %d", len(key), common.LegacySignedTokenLength)
	}
	return key
}

func setupTokenKeyTestDB(t *testing.T) *User {
	t.Helper()
	setupOrgTestDB(t)
	if err := DB.AutoMigrate(&PlaygroundKey{}, &AgentPairingCode{}, &AgentKeyClaim{}, &AgentDeviceAuthorization{}); err != nil {
		t.Fatal(err)
	}
	return createOrgTestUser(t, "key-owner")
}

func newTestToken(t *testing.T, userId int, name string) *Token {
	t.Helper()
	token := &Token{UserId: userId, Name: name, Status: config.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true}
	if err := token.Insert(); err != nil {
		t.Fatal(err)
	}
	return token
}

func TestNewTokenStoresOnlyHashAndHint(t *testing.T) {
	user := setupTokenKeyTestDB(t)
	token := newTestToken(t, user.Id, "app")

	if !common.IsUserTokenKeyFormat(token.Key) {
		t.Fatalf("new key %q is not a %d-character random key", token.Key, common.UserTokenKeyLength)
	}
	var row struct {
		KeyHash string
		KeyHint string
	}
	if err := DB.Table("tokens").Select("key_hash", "key_hint").Where("id = ?", token.Id).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.KeyHash != common.HashUserTokenKey(token.Key) || len(row.KeyHash) != 64 {
		t.Fatalf("stored hash %q is not the SHA-256 of the key", row.KeyHash)
	}
	if want := "sk-" + token.Key[:1] + "****" + token.Key[len(token.Key)-4:]; row.KeyHint != want {
		t.Fatalf("stored hint %q, want %q", row.KeyHint, want)
	}
	if hasKey, err := tokensHaveKeyColumn(DB); err != nil || hasKey {
		t.Fatalf("a new database must not have a plaintext key column (%v)", err)
	}
	// On such a database the migration has nothing to do (and must not fail).
	if err := hashTokenKeys().Migrate(DB); err != nil {
		t.Fatal(err)
	}

	// Read back, the key is gone; serialized (lists, details, the Redis cache), it never appears.
	stored, err := GetTokenById(token.Id)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Key != "" {
		t.Fatal("the key must not be readable from the database")
	}
	body, _ := json.Marshal(stored)
	text := string(body)
	if strings.Contains(text, token.Key) || strings.Contains(text, row.KeyHash) || strings.Contains(text, `"key"`) {
		t.Fatalf("serialized token leaks the key or its hash: %s", text)
	}
	if !strings.Contains(text, `"key_hint":"`+row.KeyHint+`"`) {
		t.Fatalf("serialized token has no hint: %s", text)
	}
	// Even a token that still holds the key in memory does not serialize it.
	body, _ = json.Marshal(token)
	if strings.Contains(string(body), token.Key) {
		t.Fatal("the in-memory key must not be serialized")
	}
	// Two keys never share a secret.
	if other := newTestToken(t, user.Id, "app-2"); other.Key == token.Key {
		t.Fatal("two keys got the same secret")
	}
}

func TestValidateUserTokenByHash(t *testing.T) {
	user := setupTokenKeyTestDB(t)
	token := newTestToken(t, user.Id, "app")

	got, err := ValidateUserToken(common.NormalizeUserTokenKey("sk-" + token.Key))
	if err != nil || got.Id != token.Id {
		t.Fatalf("a new key must validate, got %v %v", got, err)
	}

	garbage := []string{
		"",
		strings.Repeat("a", common.UserTokenKeyLength),      // right shape, not issued
		token.Key[:len(token.Key)-1] + "!",                  // wrong alphabet
		token.Key + "x",                                     // wrong length
		strings.Repeat("a", common.LegacySignedTokenLength), // bad signature
	}
	for _, key := range garbage {
		if _, err := ValidateUserToken(key); !errors.Is(err, ErrTokenInvalid) {
			t.Fatalf("key %q: want ErrTokenInvalid, got %v", key, err)
		}
	}

	if err := DB.Model(&User{}).Where("id = ?", user.Id).Update("status", config.UserStatusDisabled).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateUserToken(token.Key); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("a disabled user's key must be refused, got %v", err)
	}
}

func TestResetKeyReplacesTheSecret(t *testing.T) {
	user := setupTokenKeyTestDB(t)
	token := newTestToken(t, user.Id, "app")
	oldKey, oldHint := token.Key, token.KeyHint

	newKey, err := token.ResetKey()
	if err != nil {
		t.Fatal(err)
	}
	if newKey == oldKey || !common.IsUserTokenKeyFormat(newKey) || token.Key != newKey {
		t.Fatalf("reset returned %q", newKey)
	}
	if _, err := ValidateUserToken(oldKey); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("the old key must stop working, got %v", err)
	}
	if got, err := ValidateUserToken(newKey); err != nil || got.Id != token.Id {
		t.Fatalf("the new key must work, got %v %v", got, err)
	}
	stored, _ := GetTokenById(token.Id)
	if stored.KeyHint == oldHint && oldHint != common.UserTokenKeyHint(newKey) {
		t.Fatal("the hint must follow the new key")
	}
	if stored.KeyHash != common.HashUserTokenKey(newKey) {
		t.Fatal("the stored hash must be the new key's")
	}
}

// The deterministic derivation of the old releases is no longer enough to make a working key.
func TestLegacyDerivationDoesNotMakeNewKeys(t *testing.T) {
	user := setupTokenKeyTestDB(t)
	token := newTestToken(t, user.Id, "app")

	forged := legacySignedKey(t, token.Id, user.Id)
	if _, _, err := common.ValidateToken(forged); err != nil {
		t.Fatalf("the forged key carries a valid legacy signature: %v", err)
	}
	if _, err := ValidateUserToken(forged); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("a derived key must not open a new token, got %v", err)
	}
}

func TestMigrateTokenKeysToHash(t *testing.T) {
	user := setupTokenKeyTestDB(t)
	// The schema of a release before SEC-47: a plaintext key column with a unique index.
	if err := DB.Exec("ALTER TABLE tokens ADD COLUMN `key` varchar(59)").Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Exec("CREATE UNIQUE INDEX idx_tokens_key ON tokens(`key`)").Error; err != nil {
		t.Fatal(err)
	}
	signed := legacySignedKey(t, 501, user.Id)
	random := utils.GetRandomString(common.LegacyRandomTokenLength)
	deleted := utils.GetRandomString(common.LegacyRandomTokenLength)
	rows := []struct {
		id  int
		key string
		del bool
	}{{501, signed, false}, {502, random, false}, {503, deleted, true}}
	for _, r := range rows {
		err := DB.Exec("INSERT INTO tokens (id, user_id, name, `key`, status, expired_time, unlimited_quota, deleted_at) VALUES (?, ?, ?, ?, ?, -1, 1, ?)",
			r.id, user.Id, "legacy-"+r.key[:4], r.key, config.TokenStatusEnabled, map[bool]any{true: time.Now(), false: nil}[r.del]).Error
		if err != nil {
			t.Fatal(err)
		}
	}
	// A row whose key was never written ('' collides on the unique index; it becomes NULL).
	if err := DB.Exec("INSERT INTO tokens (id, user_id, name, `key`, status) VALUES (504, ?, 'empty', '', 1)", user.Id).Error; err != nil {
		t.Fatal(err)
	}

	migration := hashTokenKeys()
	for run := 0; run < 2; run++ { // a second run finds nothing to move and changes nothing
		if err := migration.Migrate(DB); err != nil {
			t.Fatalf("run %d: %v", run+1, err)
		}
		var left int64
		DB.Table("tokens").Where("`key` IS NOT NULL").Count(&left)
		if left != 0 {
			t.Fatalf("run %d: %d plaintext keys left", run+1, left)
		}
		for _, r := range rows {
			var got struct {
				KeyHash string
				KeyHint string
			}
			DB.Table("tokens").Select("key_hash", "key_hint").Where("id = ?", r.id).Scan(&got)
			if got.KeyHash != common.HashUserTokenKey(r.key) || got.KeyHint != common.UserTokenKeyHint(r.key) {
				t.Fatalf("run %d: token %d has hash %q hint %q", run+1, r.id, got.KeyHash, got.KeyHint)
			}
		}
	}

	// Migrated keys keep working, by hash; the soft-deleted one stays refused.
	for _, key := range []string{signed, random} {
		if _, err := ValidateUserToken(key); err != nil {
			t.Fatalf("migrated key %q must keep working: %v", key[:4], err)
		}
	}
	if _, err := ValidateUserToken(deleted); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("a deleted key must stay refused, got %v", err)
	}
	// New keys made after the migration leave the old column NULL.
	if token := newTestToken(t, user.Id, "after"); token.KeyHash == "" {
		t.Fatal("a key made after the migration has no hash")
	}
}

func TestPlaygroundKey(t *testing.T) {
	user := setupTokenKeyTestDB(t)
	other := createOrgTestUser(t, "other-user")

	key, expiresAt, err := IssuePlaygroundKey(user.Id)
	if err != nil {
		t.Fatal(err)
	}
	if !common.IsPlaygroundKeyFormat(key) {
		t.Fatalf("playground key %q has the wrong shape", key)
	}
	if ttl := expiresAt - time.Now().Unix(); ttl <= 0 || ttl > int64(PlaygroundKeyTTL.Seconds()) {
		t.Fatalf("playground key lives %d seconds", ttl)
	}
	var row PlaygroundKey
	if err := DB.Where("user_id = ?", user.Id).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.KeyHash != common.HashUserTokenKey(key) {
		t.Fatal("only the hash of a playground key is stored")
	}

	// It authenticates as the user's playground API key, and only that.
	token, err := ValidateUserToken(key)
	if err != nil {
		t.Fatal(err)
	}
	if token.Name != PlaygroundTokenName || token.UserId != user.Id || token.Id != row.TokenId {
		t.Fatalf("playground key resolved to %+v", token)
	}
	// Each tab gets its own key; the playground API key is created once.
	second, _, err := IssuePlaygroundKey(user.Id)
	if err != nil || second == key {
		t.Fatalf("second playground key %q %v", second, err)
	}
	if again, err := ValidateUserToken(second); err != nil || again.Id != token.Id {
		t.Fatalf("second playground key must spend on the same API key: %v %v", again, err)
	}
	if _, _, err := IssuePlaygroundKey(other.Id); err != nil {
		t.Fatal(err)
	}
	var otherRow PlaygroundKey
	DB.Where("user_id = ?", other.Id).First(&otherRow)
	if otherRow.TokenId == token.Id {
		t.Fatal("each user spends on their own playground API key")
	}
	// The playground API key itself is never shown and cannot be read back.
	if stored, _ := GetTokenById(token.Id); stored.Key != "" {
		t.Fatal("the playground API key must not be readable")
	}

	// Expired: refused.
	DB.Model(&PlaygroundKey{}).Where("key_hash = ?", common.HashUserTokenKey(key)).Update("expires_at", time.Now().Unix()-1)
	if _, err := ValidateUserToken(key); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("an expired playground key must be refused, got %v", err)
	}
	// A playground key pointing at another API key than the playground's: refused.
	regular := newTestToken(t, user.Id, "regular")
	DB.Model(&PlaygroundKey{}).Where("key_hash = ?", common.HashUserTokenKey(second)).Update("token_id", regular.Id)
	if _, err := ValidateUserToken(second); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("a playground key must only open the playground API key, got %v", err)
	}
	DB.Model(&PlaygroundKey{}).Where("key_hash = ?", common.HashUserTokenKey(second)).Update("token_id", token.Id)
	// Disabled user: refused.
	DB.Model(&User{}).Where("id = ?", user.Id).Update("status", config.UserStatusDisabled)
	if _, err := ValidateUserToken(second); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("a disabled user's playground key must be refused, got %v", err)
	}
	DB.Model(&User{}).Where("id = ?", user.Id).Update("status", config.UserStatusEnabled)
	if _, err := ValidateUserToken(second); err != nil {
		t.Fatal(err)
	}
	// Deleting the playground API key revokes every playground key at once.
	if err := DeleteTokenById(token.Id, user.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateUserToken(second); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("deleting the playground API key must revoke its keys, got %v", err)
	}
	var live int64
	DB.Model(&PlaygroundKey{}).Where("token_id = ?", token.Id).Count(&live)
	if live != 0 {
		t.Fatalf("%d playground keys left after the API key was deleted", live)
	}
}

func TestPlaygroundKeysPerUserAreCapped(t *testing.T) {
	user := setupTokenKeyTestDB(t)
	for i := 0; i < maxLivePlaygroundKeys+5; i++ {
		if _, _, err := IssuePlaygroundKey(user.Id); err != nil {
			t.Fatal(err)
		}
	}
	var live int64
	DB.Model(&PlaygroundKey{}).Where("user_id = ?", user.Id).Count(&live)
	if live != maxLivePlaygroundKeys {
		t.Fatalf("%d live playground keys, want %d", live, maxLivePlaygroundKeys)
	}
}
