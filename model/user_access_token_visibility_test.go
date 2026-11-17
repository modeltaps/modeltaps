package model

import (
	"testing"
)

// The admin list and detail must not return the password hash; /api/user/self (GetSelfById)
// must not either. Access tokens are no longer on the user at all: they are management keys,
// stored as hashes (AGENT-4).
func TestUserSensitiveFieldsVisibility(t *testing.T) {
	setupOrgTestDB(t)

	root := newEmailTestUser("root-token-visibility", "root-token-visibility@example.com")
	root.Password = "hashed-secret"
	if err := DB.Create(root).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	byId, err := GetUserById(root.Id, false)
	if err != nil {
		t.Fatalf("GetUserById: %v", err)
	}
	if byId.Password != "" {
		t.Fatal("GetUserById(false) must not return the password")
	}

	self, err := GetSelfById(root.Id)
	if err != nil {
		t.Fatalf("GetSelfById: %v", err)
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
		if u.Password != "" {
			t.Fatalf("user list must not return the password: %s", u.Username)
		}
	}
}
