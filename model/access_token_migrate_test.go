package model

import (
	"errors"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
)

// The plaintext users.access_token column becomes legacy management keys stored as hashes; the
// tokens keep working and running the migration again changes nothing.
func TestMoveAccessTokensToManagementKeys(t *testing.T) {
	setupOrgTestDB(t)
	// The column as an older release created it.
	if err := DB.Exec("ALTER TABLE users ADD COLUMN access_token char(32)").Error; err != nil {
		t.Fatalf("add access_token column: %v", err)
	}
	live := &User{Username: "live", AffCode: "aff-live"}
	shadow := &User{Username: "shadow", AffCode: "aff-shadow", Type: config.UserTypeOrgShadow}
	gone := &User{Username: "gone", AffCode: "aff-gone"}
	empty := &User{Username: "empty", AffCode: "aff-empty"}
	for _, u := range []*User{live, shadow, gone, empty} {
		if err := DB.Create(u).Error; err != nil {
			t.Fatalf("create %s: %v", u.Username, err)
		}
	}
	tokens := map[int]string{
		live.Id:   "11111111111111111111111111111111",
		shadow.Id: "22222222222222222222222222222222",
		gone.Id:   "33333333333333333333333333333333",
		empty.Id:  "",
	}
	for id, token := range tokens {
		if err := DB.Exec("UPDATE users SET access_token = ? WHERE id = ?", token, id).Error; err != nil {
			t.Fatalf("seed token: %v", err)
		}
	}
	if err := DB.Delete(gone).Error; err != nil {
		t.Fatalf("delete user: %v", err)
	}

	for run := 1; run <= 2; run++ {
		if err := moveAccessTokensToManagementKeys(DB); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		var keys []ManagementKey
		if err := DB.Find(&keys).Error; err != nil {
			t.Fatalf("list keys: %v", err)
		}
		if len(keys) != 1 {
			t.Fatalf("run %d: want one migrated token (live user only), got %+v", run, keys)
		}
		key := keys[0]
		if key.UserId != live.Id || key.Scope != ManagementKeyScopeLegacy || key.ExpiredTime != -1 ||
			key.KeyHash != common.HashManagementKey(tokens[live.Id]) || key.KeyHint == tokens[live.Id] {
			t.Fatalf("run %d: unexpected migrated key %+v", run, key)
		}
		var left int64
		DB.Table("users").Where("access_token IS NOT NULL").Count(&left)
		if left != 0 {
			t.Fatalf("run %d: %d plaintext access tokens left", run, left)
		}
	}

	if user := ValidateAccessToken(tokens[live.Id]); user == nil || user.Id != live.Id {
		t.Fatal("the migrated access token should still sign in its user")
	}
	if user := ValidateAccessToken(tokens[shadow.Id]); user != nil {
		t.Fatal("a shadow account's token must not be migrated")
	}
}

// Databases created after AGENT-4 have no access_token column: nothing to do.
func TestMoveAccessTokensWithoutColumn(t *testing.T) {
	setupOrgTestDB(t)
	if err := moveAccessTokensToManagementKeys(DB); err != nil {
		t.Fatalf("migration on a new database: %v", err)
	}
}

func TestResolveManagementKeyExpiry(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	if got, err := ResolveManagementKeyExpiry(0, now); err != nil || got != now.Add(90*24*time.Hour).Unix() {
		t.Fatalf("default expiry: got %d, %v", got, err)
	}
	year := now.Add(365 * 24 * time.Hour).Unix()
	if got, err := ResolveManagementKeyExpiry(year, now); err != nil || got != year {
		t.Fatalf("one year should be accepted: got %d, %v", got, err)
	}
	cases := []struct {
		requested int64
		code      string
	}{
		{-1, common.APICodeManagementKeyExpiryRequired},
		{now.Unix(), common.APICodeManagementKeyExpiryRequired},
		{now.Unix() - 60, common.APICodeManagementKeyExpiryRequired},
		{now.Add(367 * 24 * time.Hour).Unix(), common.APICodeManagementKeyExpiryTooLong},
	}
	for _, c := range cases {
		_, err := ResolveManagementKeyExpiry(c.requested, now)
		var apiErr *common.APIError
		if !errors.As(err, &apiErr) || apiErr.Code != c.code {
			t.Fatalf("expiry %d: want %s, got %v", c.requested, c.code, err)
		}
	}
}

// Only legacy access tokens may never expire, and an expired key is told apart from an unknown one.
func TestValidateManagementKey(t *testing.T) {
	setupOrgTestDB(t)
	user := &User{Username: "mk-validate", AffCode: "aff-mk-validate", Status: config.UserStatusEnabled}
	if err := DB.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	key, err := CreateManagementKey(user.Id, "agent", ManagementKeyScopeRead, 0)
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	if got, gotUser, err := ValidateManagementKey("Bearer " + key.Key); err != nil || got.Id != key.Id || gotUser.Id != user.Id {
		t.Fatalf("valid key refused: %v", err)
	}
	if user := ValidateAccessToken(key.Key); user != nil {
		t.Fatal("a management key is not a legacy access token")
	}
	if _, _, err := ValidateManagementKey(key.Key + "x"); !errors.Is(err, ErrManagementKeyInvalid) {
		t.Fatalf("unknown key: want invalid, got %v", err)
	}

	DB.Model(&ManagementKey{}).Where("id = ?", key.Id).Update("expired_time", -1)
	if _, _, err := ValidateManagementKey(key.Key); !errors.Is(err, ErrManagementKeyInvalid) {
		t.Fatalf("a read key without expiry must be refused, got %v", err)
	}
	DB.Model(&ManagementKey{}).Where("id = ?", key.Id).Update("expired_time", time.Now().Unix()-1)
	if _, _, err := ValidateManagementKey(key.Key); !errors.Is(err, ErrManagementKeyExpired) {
		t.Fatalf("expired key: want expired, got %v", err)
	}
}
