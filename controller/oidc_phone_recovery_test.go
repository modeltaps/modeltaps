package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// SEC-27: an account created from a phone-only IdP account (verified phone_number, no email, no
// password) is no longer an island. The verified phone is stored on sign-up, so when the
// operator moves to another provider that links by verified phone, the new identity signs in to
// the same account instead of creating an empty one.
func TestOidcPhoneOnlyAccountRecoverableThroughPhoneLink(t *testing.T) {
	setupOidcTestDB(t)
	oldRegister := config.RegisterEnabled
	config.RegisterEnabled = true
	t.Cleanup(func() { config.RegisterEnabled = oldRegister })

	original := seedOidcProvider(t, "phone-idp", false)
	original.FirstParty = true
	r := newOidcPhoneRouter(t)
	r.GET("/register", func(c *gin.Context) {
		claims := map[string]interface{}{
			"preferred_username":    "phoneonly",
			"phone_number":          "+85251230000",
			"phone_number_verified": true,
		}
		oidcRegister(c, original, "", "sub-original", claims, oidcTrustedEmail(claims))
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/register", nil))

	var registered model.User
	if err := model.DB.First(&registered, "username = ?", "phoneonly").Error; err != nil {
		t.Fatalf("the phone-only account should be created: %v", err)
	}
	if registered.Email != "" || registered.Password != "" {
		t.Fatalf("the account should have no email and no password, got email=%q password set=%v", registered.Email, registered.Password != "")
	}
	if registered.PhoneNumber != "+85251230000" {
		t.Fatalf("the verified phone should be stored, got %q", registered.PhoneNumber)
	}

	replacement := seedOidcPhoneLinkProvider(t, "replacement-idp")
	user, outcome, err := resolveOidcUser(replacement, "sub-replacement", "", "+85251230000")
	if err != nil || outcome != oidcLinkLogin {
		t.Fatalf("the replacement provider should sign in to the account, outcome=%d err=%v", outcome, err)
	}
	if user.Id != registered.Id {
		t.Fatalf("the replacement identity should reach user %d, got %d", registered.Id, user.Id)
	}
	if identity, err := model.FindUserOidcIdentity(replacement.Id, "sub-replacement"); err != nil || identity.UserId != registered.Id {
		t.Fatalf("the replacement identity should be linked to user %d, got %v err=%v", registered.Id, identity, err)
	}
}
