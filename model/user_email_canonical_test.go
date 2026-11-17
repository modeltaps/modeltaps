package model

import (
	"errors"
	"testing"

	"github.com/modeltaps/modeltaps/common"

	"gorm.io/gorm"
)

func storedEmailCanonical(t *testing.T, id int) string {
	t.Helper()
	var user User
	if err := DB.Unscoped().Select("id", "email_canonical").First(&user, id).Error; err != nil {
		t.Fatalf("load user %d: %v", id, err)
	}
	return user.EmailCanonical
}

func wantEmailTaken(t *testing.T, err error, what string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s should be refused", what)
	}
	var apiErr *common.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != common.APICodeEmailTaken {
		t.Fatalf("%s: want the email taken error, got %v", what, err)
	}
}

// Addresses of one Gmail mailbox (dots, "+suffix", googlemail.com) cannot register a second
// account; other domains keep their dots, so first.last and firstlast are different mailboxes.
func TestRegisterRefusesAddressOfTakenMailbox(t *testing.T) {
	setupEmailTestDB(t, true)

	first := newEmailTestUser("first", "AB@gmail.com")
	if err := first.Insert(0); err != nil {
		t.Fatalf("first registration: %v", err)
	}
	if first.Email != "ab@gmail.com" || storedEmailCanonical(t, first.Id) != "ab@gmail.com" {
		t.Fatalf("stored email %q canonical %q, want ab@gmail.com for both", first.Email, storedEmailCanonical(t, first.Id))
	}

	for i, email := range []string{"a.b@gmail.com", "A.B@GoogleMail.com", "ab@googlemail.com"} {
		wantEmailTaken(t, newEmailTestUser("insert"+string(rune('a'+i)), email).Insert(0), "Insert of "+email)
		err := DB.Transaction(func(tx *gorm.DB) error {
			return newEmailTestUser("tx"+string(rune('a'+i)), email).InsertWithTx(tx, 0)
		})
		wantEmailTaken(t, err, "InsertWithTx of "+email)
		if !IsEmailAlreadyTaken(email) {
			t.Fatalf("IsEmailAlreadyTaken(%q) = false, want true", email)
		}
	}
	// "+" is already refused by the strict format check of the registration paths.
	if err := newEmailTestUser("plus", "a.b+x@gmail.com").Insert(0); err == nil {
		t.Fatal("registering a.b+x@gmail.com after ab@gmail.com should be refused")
	}
	// Addresses that skip the strict format check (verified emails from an identity provider)
	// are still matched through IsEmailAlreadyTaken.
	if !IsEmailAlreadyTaken("a.b+x@gmail.com") {
		t.Fatal("IsEmailAlreadyTaken(a.b+x@gmail.com) = false, want true")
	}

	if err := newEmailTestUser("dotted", "first.last@example.com").Insert(0); err != nil {
		t.Fatalf("first.last@example.com: %v", err)
	}
	if err := newEmailTestUser("undotted", "firstlast@example.com").Insert(0); err != nil {
		t.Fatalf("firstlast@example.com is another mailbox outside Gmail: %v", err)
	}
	if IsEmailAlreadyTaken("other@gmail.com") {
		t.Fatal("IsEmailAlreadyTaken(other@gmail.com) = true, want false")
	}
}

// Accounts that already shared a mailbox before the check stay editable, but no account can
// change its email to an address of a mailbox another account holds.
func TestUpdateChecksMailboxOnlyWhenEmailChanges(t *testing.T) {
	setupEmailTestDB(t, true)

	legacyA := newEmailTestUser("legacya", "ab@gmail.com")
	legacyB := newEmailTestUser("legacyb", "a.b@gmail.com")
	for _, user := range []*User{legacyA, legacyB} {
		user.EmailCanonical = common.CanonicalEmail(string(user.Email))
		if err := DB.Create(user).Error; err != nil {
			t.Fatalf("seed legacy user: %v", err)
		}
	}
	legacyB.DisplayName = "Renamed"
	if err := legacyB.Update(false); err != nil {
		t.Fatalf("editing an account that shares a mailbox from before: %v", err)
	}

	other := newEmailTestUser("other", "other@example.com")
	if err := other.Insert(0); err != nil {
		t.Fatalf("insert: %v", err)
	}
	other.Email = "A.B@googlemail.com"
	wantEmailTaken(t, other.Update(false), "changing the email to a taken mailbox")
	if got := storedEmailCanonical(t, other.Id); got != "other@example.com" {
		t.Fatalf("canonical after a refused change = %q, want other@example.com", got)
	}

	other.Email = "New.Address@gmail.com"
	if err := other.Update(false); err != nil {
		t.Fatalf("changing to a free mailbox: %v", err)
	}
	if got := storedEmailCanonical(t, other.Id); got != "newaddress@gmail.com" {
		t.Fatalf("canonical after the change = %q, want newaddress@gmail.com", got)
	}
}

// Email write-back through UpdateUser (identity provider sync) keeps the canonical column in
// step, and Delete releases it together with the email.
func TestUpdateUserAndDeleteKeepCanonicalEmail(t *testing.T) {
	setupEmailTestDB(t, true)

	user := newEmailTestUser("synced", "")
	if err := user.Insert(0); err != nil {
		t.Fatalf("insert: %v", err)
	}
	fields := map[string]interface{}{"email": NullableEmail("x.y+idp@gmail.com"), "email_verified": true}
	if err := UpdateUser(user.Id, fields); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if _, ok := fields["email_canonical"]; ok {
		t.Fatal("UpdateUser must not change the caller's map")
	}
	if got := storedEmailCanonical(t, user.Id); got != "xy@gmail.com" {
		t.Fatalf("canonical after UpdateUser = %q, want xy@gmail.com", got)
	}
	if !IsEmailAlreadyTaken("xy@googlemail.com") {
		t.Fatal("the synced mailbox should count as taken")
	}

	if err := user.Delete(); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := storedEmailCanonical(t, user.Id); got != "" {
		t.Fatalf("canonical after delete = %q, want empty", got)
	}
	if IsEmailAlreadyTaken("xy@gmail.com") {
		t.Fatal("a deleted account's mailbox should be free again")
	}
}

// Upgrading fills email_canonical for existing accounts, deleted ones included, and leaves
// accounts that share a mailbox in place. Running it again changes nothing.
func TestBackfillUserEmailCanonical(t *testing.T) {
	db := setupEmailTestDB(t, true)

	users := map[string]*User{
		"ab@gmail.com":           newEmailTestUser("gmaila", "ab@gmail.com"),
		"a.b@googlemail.com":     newEmailTestUser("gmailb", "a.b@googlemail.com"),
		"first.last@example.com": newEmailTestUser("dotted", "first.last@example.com"),
		"tag+x@example.com":      newEmailTestUser("tagged", "tag+x@example.com"),
		"":                       newEmailTestUser("nomail", ""),
	}
	for _, user := range users {
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("seed user %s: %v", user.Username, err)
		}
	}
	deleted := newEmailTestUser("gone", "gone.user@gmail.com")
	if err := db.Create(deleted).Error; err != nil {
		t.Fatalf("seed deleted user: %v", err)
	}
	if err := db.Delete(deleted).Error; err != nil {
		t.Fatalf("soft-delete user: %v", err)
	}

	want := map[string]string{
		"ab@gmail.com":           "ab@gmail.com",
		"a.b@googlemail.com":     "ab@gmail.com",
		"first.last@example.com": "first.last@example.com",
		"tag+x@example.com":      "tag@example.com",
		"":                       "",
	}
	migration := backfillUserEmailCanonical()
	for round := 1; round <= 2; round++ {
		if err := migration.Migrate(db); err != nil {
			t.Fatalf("migration round %d: %v", round, err)
		}
		for email, user := range users {
			if got := storedEmailCanonical(t, user.Id); got != want[email] {
				t.Fatalf("round %d: canonical of %q = %q, want %q", round, email, got, want[email])
			}
			if email != "" {
				var stored User
				db.Select("id", "email").First(&stored, user.Id)
				if string(stored.Email) != email {
					t.Fatalf("round %d: email changed from %q to %q", round, email, stored.Email)
				}
			}
		}
		if got := storedEmailCanonical(t, deleted.Id); got != "goneuser@gmail.com" {
			t.Fatalf("round %d: canonical of the deleted account = %q, want goneuser@gmail.com", round, got)
		}
	}
}
