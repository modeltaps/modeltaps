package model

import (
	"errors"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// SEC-61: an organization key is owned by the organization's shadow account, but it stops working
// once the site bans or deletes the member who created it, the same as a personal key does when
// its owner is banned or deleted.
func TestOrganizationKeyFollowsCreatorStatus(t *testing.T) {
	setupOrgTestDB(t)
	if err := DB.AutoMigrate(&UserSession{}, &UserOidcIdentity{}); err != nil {
		t.Fatal(err)
	}
	owner := createOrgTestUser(t, "sec61-owner")
	member := createOrgTestUser(t, "sec61-member")
	org, err := CreateOrganizationWithOwner("SEC-61 Org", "", owner.Id)
	if err != nil {
		t.Fatalf("create organization: %v", err)
	}
	if err := (&OrganizationMember{OrganizationId: org.Id, UserId: member.Id, Role: OrgRoleMember}).Insert(); err != nil {
		t.Fatalf("add member: %v", err)
	}
	orgKey := &Token{UserId: org.ShadowUserId, CreatedBy: member.Id, Name: "org-key",
		Status: config.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true}
	if err := orgKey.Insert(); err != nil {
		t.Fatalf("create organization key: %v", err)
	}
	ownerKey := &Token{UserId: org.ShadowUserId, CreatedBy: owner.Id, Name: "owner-key",
		Status: config.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true}
	if err := ownerKey.Insert(); err != nil {
		t.Fatalf("create organization key: %v", err)
	}

	if _, err := ValidateUserToken(orgKey.Key); err != nil {
		t.Fatalf("the key of an enabled member must validate: %v", err)
	}

	// Banned: refused.
	if err := DB.Model(&User{}).Where("id = ?", member.Id).Update("status", config.UserStatusDisabled).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateUserToken(orgKey.Key); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("a banned member's organization key must be refused, got %v", err)
	}
	// Other members' keys of the same organization keep working.
	if _, err := ValidateUserToken(ownerKey.Key); err != nil {
		t.Fatalf("another member's organization key must keep working: %v", err)
	}

	// Enabled again: works again, like a personal key of a re-enabled user.
	if err := DB.Model(&User{}).Where("id = ?", member.Id).Update("status", config.UserStatusEnabled).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateUserToken(orgKey.Key); err != nil {
		t.Fatalf("the key must validate again once the member is enabled: %v", err)
	}

	// Deleted: refused.
	deleted, err := GetUserById(member.Id, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := deleted.Delete(); err != nil {
		t.Fatalf("delete member: %v", err)
	}
	if _, err := ValidateUserToken(orgKey.Key); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("a deleted member's organization key must be refused, got %v", err)
	}
}
