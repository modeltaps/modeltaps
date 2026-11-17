package controller

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// SEC-72: an organization admin cannot put an email nobody verified on an account they create,
// so the mailbox's real owner can still sign up with it or bind it. A site admin, who can set any
// account's email from the user admin page anyway, still can.
func TestCreateOrgMemberAccountDoesNotClaimEmail(t *testing.T) {
	const victim = "victim@corp.example"
	body := `{"username":"squatter","password":"secret123","email":"` + victim + `","role":"member"}`
	storedEmail := func(t *testing.T) string {
		t.Helper()
		var user model.User
		if err := model.DB.Where("username = ?", "squatter").First(&user).Error; err != nil {
			t.Fatalf("load created account: %v", err)
		}
		return string(user.Email)
	}

	t.Run("organization admin", func(t *testing.T) {
		setupOrgMemberAccountTestDB(t)
		resp := callCreateOrgMemberAccountAs(t, config.RoleCommonUser, body)
		if resp["success"] != true {
			t.Fatalf("the account should still be created, got %v", resp)
		}
		if data, _ := resp["data"].(map[string]interface{}); data["email_saved"] != false {
			t.Fatalf("the response should say the email was not saved, got %v", resp)
		}
		if got := storedEmail(t); got != "" {
			t.Fatalf("the account must not hold an unverified email, got %q", got)
		}
		if model.IsEmailAlreadyTaken(victim) {
			t.Fatal("the email must stay free for its owner")
		}
	})

	t.Run("site admin", func(t *testing.T) {
		setupOrgMemberAccountTestDB(t)
		resp := callCreateOrgMemberAccountAs(t, config.RoleAdminUser, body)
		if resp["success"] != true {
			t.Fatalf("the account should be created, got %v", resp)
		}
		if data, _ := resp["data"].(map[string]interface{}); data["email_saved"] != true {
			t.Fatalf("the response should say the email was saved, got %v", resp)
		}
		if got := storedEmail(t); got != victim {
			t.Fatalf("a site admin's email should be stored, got %q", got)
		}
	})
}
