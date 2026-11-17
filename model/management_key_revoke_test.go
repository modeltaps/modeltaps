package model

import (
	"errors"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
)

// setupRevokeTestDB is setupOrgTestDB plus the sessions table, which a password reset clears.
func setupRevokeTestDB(t *testing.T) {
	t.Helper()
	setupOrgTestDB(t)
	if err := DB.AutoMigrate(&UserSession{}); err != nil {
		t.Fatalf("migrate user sessions: %v", err)
	}
}

// issueTestCredentials gives the user a legacy access token and a read management key.
func issueTestCredentials(t *testing.T, userId int) (legacy string, managementKey string) {
	t.Helper()
	legacy, err := IssueLegacyAccessToken(userId)
	if err != nil {
		t.Fatalf("issue legacy access token: %v", err)
	}
	key, err := CreateManagementKey(userId, "automation", ManagementKeyScopeRead, 0)
	if err != nil {
		t.Fatalf("create management key: %v", err)
	}
	return legacy, key.Key
}

// SEC-71: a password change or reset ends the legacy access token, which is not tied to a session;
// management keys the user created on purpose keep working.
func TestPasswordChangeRevokesLegacyAccessToken(t *testing.T) {
	t.Run("password change", func(t *testing.T) {
		setupRevokeTestDB(t)
		user := createOrgTestUser(t, "pwchange")
		legacy, mk := issueTestCredentials(t, user.Id)

		update := &User{Id: user.Id, DisplayName: "renamed"}
		if err := update.Update(false); err != nil {
			t.Fatalf("update without password: %v", err)
		}
		if ValidateAccessToken(legacy) == nil {
			t.Fatal("an update that keeps the password must not revoke the legacy access token")
		}

		update = &User{Id: user.Id, Password: "new-password-123"}
		if err := update.Update(true); err != nil {
			t.Fatalf("change password: %v", err)
		}
		if ValidateAccessToken(legacy) != nil {
			t.Fatal("a password change must revoke the legacy access token")
		}
		if _, _, err := ValidateManagementKey(mk); err != nil {
			t.Fatalf("a password change must keep management keys, got %v", err)
		}
	})

	t.Run("password reset by email", func(t *testing.T) {
		setupRevokeTestDB(t)
		user := createOrgTestUser(t, "pwreset")
		legacy, mk := issueTestCredentials(t, user.Id)
		if err := ResetUserPasswordByEmail(string(user.Email), "reset-password-123"); err != nil {
			t.Fatalf("reset password: %v", err)
		}
		if ValidateAccessToken(legacy) != nil {
			t.Fatal("a password reset must revoke the legacy access token")
		}
		if _, _, err := ValidateManagementKey(mk); err != nil {
			t.Fatalf("a password reset must keep management keys, got %v", err)
		}
	})
}

// SEC-71: a legacy access token no longer lives forever. A new one gets an expiry, and one stored
// without (every migrated token) stops LegacyAccessTokenMaxLifetime after it was created; the
// console list shows that date.
func TestLegacyAccessTokenLifetimeCapped(t *testing.T) {
	setupRevokeTestDB(t)
	user := createOrgTestUser(t, "legacycap")
	now := time.Now()
	maxSeconds := int64(LegacyAccessTokenMaxLifetime / time.Second)

	issued, err := IssueLegacyAccessToken(user.Id)
	if err != nil {
		t.Fatalf("issue legacy access token: %v", err)
	}
	var stored ManagementKey
	if err := DB.Where("key_hash = ?", common.HashManagementKey(issued)).First(&stored).Error; err != nil {
		t.Fatalf("load issued token: %v", err)
	}
	if want := now.Unix() + maxSeconds; stored.ExpiredTime < want-5 || stored.ExpiredTime > want+5 {
		t.Fatalf("a new legacy access token should expire after %v, got expired_time %d (now %d)",
			LegacyAccessTokenMaxLifetime, stored.ExpiredTime, now.Unix())
	}

	recent := "0123456789abcdef0123456789abcde1"
	old := "0123456789abcdef0123456789abcde2"
	for _, row := range []*ManagementKey{
		{UserId: user.Id, Scope: ManagementKeyScopeLegacy, Key: recent, CreatedTime: now.Unix() - 86400, ExpiredTime: -1},
		{UserId: user.Id, Scope: ManagementKeyScopeLegacy, Key: old, CreatedTime: now.Unix() - maxSeconds - 1, ExpiredTime: -1},
	} {
		if err := DB.Create(row).Error; err != nil {
			t.Fatalf("create migrated token: %v", err)
		}
	}
	if ValidateAccessToken(recent) == nil {
		t.Fatal("a migrated token younger than the cap should still work")
	}
	if _, _, err := ValidateManagementKey(old); !errors.Is(err, ErrManagementKeyExpired) {
		t.Fatalf("a migrated token older than the cap should be expired, got %v", err)
	}

	keys, err := ListManagementKeys(user.Id)
	if err != nil {
		t.Fatalf("list management keys: %v", err)
	}
	for _, key := range keys {
		if key.ExpiredTime == -1 {
			t.Fatalf("the list should show the capped expiry, got -1 for %+v", key)
		}
		if key.KeyHint == common.ManagementKeyHint(recent) && key.ExpiredTime != now.Unix()-86400+maxSeconds {
			t.Fatalf("a migrated token should be listed with created_time + cap, got %d", key.ExpiredTime)
		}
	}
}
