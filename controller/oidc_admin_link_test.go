package controller

import (
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// SEC-53: a verified email or phone from an IdP never links to an admin or root account, so
// whoever controls an IdP cannot sign in as them; no identity row is written.
func TestResolveOidcUserRefusesAutoLinkToAdmins(t *testing.T) {
	roles := map[string]int{"admin": config.RoleAdminUser, "root": config.RoleRootUser}
	for name, role := range roles {
		t.Run(name+" by email", func(t *testing.T) {
			setupOidcTestDB(t)
			provider := seedOidcProvider(t, "authany", true)
			seeded := seedOidcUser(t, &model.User{
				Username: name + "-email", Email: model.NullableEmail(name + "@example.com"), EmailVerified: true,
				Role: role, Status: config.UserStatusEnabled, Type: config.UserTypeNormal, AffCode: "aff-e-" + name,
			})

			user, outcome, err := resolveOidcUser(provider, "attacker-sub", name+"@example.com", "")
			if err != nil || outcome != oidcLinkPrivileged {
				t.Fatalf("expected oidcLinkPrivileged, got outcome=%d err=%v", outcome, err)
			}
			if user == nil || user.Id != seeded.Id {
				t.Fatalf("expected the matched account %d, got %v", seeded.Id, user)
			}
			if _, err := model.FindUserOidcIdentity(provider.Id, "attacker-sub"); err == nil {
				t.Fatal("no identity row may be written for an admin account")
			}
		})

		t.Run(name+" by phone", func(t *testing.T) {
			setupOidcTestDB(t)
			provider := seedOidcPhoneLinkProvider(t, "authany")
			phone := "+8525123000" + map[string]string{"admin": "1", "root": "2"}[name]
			seeded := seedOidcUser(t, &model.User{
				Username: name + "-phone", Email: model.NullableEmail(name + "-phone@example.com"),
				Role: role, Status: config.UserStatusEnabled, Type: config.UserTypeNormal, AffCode: "aff-p-" + name,
				PhoneNumber: model.NullablePhone(phone),
			})

			user, outcome, err := resolveOidcUser(provider, "attacker-sub", "", phone)
			if err != nil || outcome != oidcLinkPrivileged {
				t.Fatalf("expected oidcLinkPrivileged, got outcome=%d err=%v", outcome, err)
			}
			if user == nil || user.Id != seeded.Id {
				t.Fatalf("expected the matched account %d, got %v", seeded.Id, user)
			}
			if _, err := model.FindUserOidcIdentity(provider.Id, "attacker-sub"); err == nil {
				t.Fatal("no identity row may be written for an admin account")
			}
		})
	}
}

// An admin whose identity is already linked keeps signing in through it; only automatic linking
// of a new subject is refused.
func TestResolveOidcUserAdminLinkedIdentityStillSignsIn(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "authany", true)
	root := seedOidcUser(t, &model.User{
		Username: "root", Email: "root@example.com", EmailVerified: true,
		Role: config.RoleRootUser, Status: config.UserStatusEnabled, Type: config.UserTypeNormal, AffCode: "aff-root",
	})
	seedOidcIdentity(t, root.Id, provider.Id, "root-sub")

	user, outcome, err := resolveOidcUser(provider, "root-sub", "root@example.com", "")
	if err != nil || outcome != oidcLinkLogin || user.Id != root.Id {
		t.Fatalf("a linked subject should sign in, got outcome=%d user=%v err=%v", outcome, user, err)
	}
}

// The rejection carries its own prefix for the login callback page and names neither the
// provider nor the matched email or phone.
func TestOidcLinkPrivilegedMessage(t *testing.T) {
	message := oidcLinkPrivilegedMessage()
	if !strings.HasPrefix(message, "OIDC_LINK_PRIVILEGED:") {
		t.Fatalf("message should start with OIDC_LINK_PRIVILEGED:, got %q", message)
	}
	if strings.Contains(message, "@") {
		t.Fatalf("message must not contain an email: %q", message)
	}
}
