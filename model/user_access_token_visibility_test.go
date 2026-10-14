package model

import (
	"testing"
)

// The admin list and detail must not return access_token: with root's token an admin can act as
// root. Only the user's own /api/user/self (GetSelfById) keeps it.
func TestUserAccessTokenVisibility(t *testing.T) {
	setupOrgTestDB(t)

	root := newEmailTestUser("root-token-visibility", "root-token-visibility@example.com")
	if err := DB.Create(root).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if root.AccessToken == "" {
		t.Fatal("precondition: the user should have an access_token")
	}

	byId, err := GetUserById(root.Id, false)
	if err != nil {
		t.Fatalf("GetUserById: %v", err)
	}
	if byId.AccessToken != "" || byId.Password != "" {
		t.Fatalf("GetUserById(false) must not return access_token/password, got token=%q", byId.AccessToken)
	}

	full, err := GetUserById(root.Id, true)
	if err != nil {
		t.Fatalf("GetUserById(true): %v", err)
	}
	if full.AccessToken != root.AccessToken {
		t.Fatal("GetUserById(true) should return the full record (GenerateAccessToken relies on it)")
	}

	self, err := GetSelfById(root.Id)
	if err != nil {
		t.Fatalf("GetSelfById: %v", err)
	}
	if self.AccessToken != root.AccessToken {
		t.Fatal("GetSelfById should keep the user's own access_token")
	}
	if self.Password != "" {
		t.Fatal("GetSelfById must not return the password")
	}

	list, err := GetUsersList(&SearchUserParams{})
	if err != nil {
		t.Fatalf("GetUsersList: %v", err)
	}
	if len(*list.Data) == 0 {
		t.Fatal("user list should not be empty")
	}
	for _, u := range *list.Data {
		if u.AccessToken != "" || u.Password != "" {
			t.Fatalf("user list must not return access_token/password: %s", u.Username)
		}
	}
}
