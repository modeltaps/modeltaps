package controller

import (
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// SEC-71: "sign out other devices" and a password change end the legacy access token, which a
// stolen session could have minted; management keys the user created on purpose keep working.
func TestSignOutAndPasswordChangeRevokeLegacyAccessToken(t *testing.T) {
	setup := func(t *testing.T, username string) (*model.User, string, string) {
		t.Helper()
		setupGetUserTestDB(t)
		user := createTestUser(t, username, config.RoleCommonUser, config.UserTypeNormal)
		legacy, err := model.IssueLegacyAccessToken(user.Id)
		if err != nil {
			t.Fatalf("issue legacy access token: %v", err)
		}
		key, err := model.CreateManagementKey(user.Id, "automation", model.ManagementKeyScopeWrite, 0)
		if err != nil {
			t.Fatalf("create management key: %v", err)
		}
		return user, legacy, key.Key
	}
	expectRevoked := func(t *testing.T, legacy string, managementKey string) {
		t.Helper()
		if model.ValidateAccessToken(legacy) != nil {
			t.Fatal("the legacy access token should be revoked")
		}
		if _, _, err := model.ValidateManagementKey(managementKey); err != nil {
			t.Fatalf("management keys should keep working, got %v", err)
		}
	}

	t.Run("sign out other devices", func(t *testing.T) {
		user, legacy, mk := setup(t, "signout")
		c, w := newSelfTestContext(t, "POST", "/api/user/sessions/logout_others", "", user.Id)
		LogoutOtherSessions(c)
		if resp := decodeResp(t, w); resp["success"] != true {
			t.Fatalf("sign out other devices should succeed, got %v", resp)
		}
		expectRevoked(t, legacy, mk)
	})

	t.Run("password change", func(t *testing.T) {
		oldEnabled := config.PasswordLoginEnabled
		t.Cleanup(func() { config.PasswordLoginEnabled = oldEnabled })
		config.PasswordLoginEnabled = true
		user, legacy, mk := setup(t, "pwself")
		hash, err := common.Password2Hash("old-password-1")
		if err != nil {
			t.Fatal(err)
		}
		if err := model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("password", hash).Error; err != nil {
			t.Fatal(err)
		}
		c, w := newSelfTestContext(t, "PUT", "/api/user/self",
			`{"password":"new-password-1","original_password":"old-password-1"}`, user.Id)
		UpdateSelf(c)
		if resp := decodeResp(t, w); resp["success"] != true {
			t.Fatalf("password change should succeed, got %v", resp)
		}
		expectRevoked(t, legacy, mk)
	})
}
