package controller

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/middleware"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// callCreateOrgMemberAccountAs is callCreateOrgMemberAccount with the operator's site role set.
func callCreateOrgMemberAccountAs(t *testing.T, siteRole int, body string) map[string]interface{} {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/org/1/members/account", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", 1)
	c.Set("role", siteRole)
	c.Set(middleware.OrgContextKey, &model.Organization{Id: 1, Name: "Acme", ShadowUserId: 2})
	c.Set(middleware.OrgRoleContextKey, model.OrgRoleOwner)
	CreateOrgMemberAccount(c)
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

// SEC-60: the site's sign-up rules also bind accounts an organization admin creates, and the
// sign-up bonus cannot be asked for.
func TestCreateOrgMemberAccountSignupRules(t *testing.T) {
	saved := struct {
		register, passwordRegister, domainRestriction bool
		whitelist                                     []string
		accountSystem                                 string
		quotaForNewUser                               int
	}{config.RegisterEnabled, config.PasswordRegisterEnabled, config.EmailDomainRestrictionEnabled,
		config.EmailDomainWhitelist, config.AccountSystem, config.QuotaForNewUser}
	reset := func() {
		config.RegisterEnabled = saved.register
		config.PasswordRegisterEnabled = saved.passwordRegister
		config.EmailDomainRestrictionEnabled = saved.domainRestriction
		config.EmailDomainWhitelist = saved.whitelist
		config.AccountSystem = saved.accountSystem
		config.QuotaForNewUser = saved.quotaForNewUser
	}
	t.Cleanup(reset)

	const body = `{"username":"newmember","password":"secret123","email":"new@corp.example","role":"member"}`
	cases := []struct {
		name     string
		setup    func()
		siteRole int
		wantCode string // "" = created
	}{
		{"sign-up closed", func() { config.RegisterEnabled = false }, config.RoleCommonUser, common.APICodeRegisterDisabled},
		{"password sign-up closed", func() { config.PasswordRegisterEnabled = false }, config.RoleCommonUser, common.APICodePasswordRegisterDisabled},
		{"external account system", func() { config.AccountSystem = config.AccountSystemExternal }, config.RoleCommonUser, common.APICodePasswordRegisterDisabled},
		{"email domain not on the allowlist", func() {
			config.EmailDomainRestrictionEnabled = true
			config.EmailDomainWhitelist = []string{"other.example"}
		}, config.RoleCommonUser, common.APICodeEmailDomainNotAllowed},
		{"email domain on the allowlist", func() {
			config.EmailDomainRestrictionEnabled = true
			config.EmailDomainWhitelist = []string{"corp.example"}
		}, config.RoleCommonUser, ""},
		{"site admins are not bound by the sign-up rules", func() { config.RegisterEnabled = false }, config.RoleAdminUser, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reset()
			setupOrgMemberAccountTestDB(t)
			tc.setup()
			resp := callCreateOrgMemberAccountAs(t, tc.siteRole, body)
			if tc.wantCode == "" {
				if resp["success"] != true {
					t.Fatalf("want the account created, got %v", resp)
				}
				return
			}
			if resp["success"] != false || resp["code"] != tc.wantCode {
				t.Fatalf("want code %q, got %v", tc.wantCode, resp)
			}
			if n := countUsersByUsername(t, "newmember"); n != 0 {
				t.Fatalf("a refused request must not create the account, got %d", n)
			}
		})
	}

	t.Run("grant_signup_bonus is ignored", func(t *testing.T) {
		reset()
		setupOrgMemberAccountTestDB(t)
		config.QuotaForNewUser = 500
		resp := callCreateOrgMemberAccountAs(t, config.RoleCommonUser,
			`{"username":"bonusasker","password":"secret123","role":"member","grant_signup_bonus":true}`)
		if resp["success"] != true {
			t.Fatalf("want the account created, got %v", resp)
		}
		var user model.User
		if err := model.DB.Where("username = ?", "bonusasker").First(&user).Error; err != nil {
			t.Fatal(err)
		}
		if user.Quota != 0 {
			t.Fatalf("an account an organization creates must get no sign-up bonus, got quota %d", user.Quota)
		}
	})
}
