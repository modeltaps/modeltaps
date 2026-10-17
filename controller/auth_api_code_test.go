package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// Sign-in and sign-up failures carry a code so the console can show them in the UI language;
// the English message stays as it was for API clients.
func TestAuthFailuresCarryCodes(t *testing.T) {
	setupGetUserTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	r.POST("/login", Login)
	r.POST("/register", Register)

	old := struct{ login, register, registerPassword bool }{config.PasswordLoginEnabled, config.RegisterEnabled, config.PasswordRegisterEnabled}
	t.Cleanup(func() {
		config.PasswordLoginEnabled, config.RegisterEnabled, config.PasswordRegisterEnabled = old.login, old.register, old.registerPassword
	})

	expect := func(t *testing.T, resp map[string]interface{}, code, message string) {
		t.Helper()
		if resp["success"] != false || resp["code"] != code || resp["message"] != message {
			t.Fatalf("want code %q / message %q, got %v", code, message, resp)
		}
	}

	config.PasswordLoginEnabled = true
	_, resp := postJSON(t, r, "/login", `{"username":"ghost"}`)
	expect(t, resp, common.APICodeInvalidParams, "Invalid parameters")

	config.PasswordLoginEnabled = false
	_, resp = postJSON(t, r, "/login", `{"username":"ghost","password":"whatever1"}`)
	expect(t, resp, common.APICodePasswordLoginDisabled, "The admin has disabled password sign-in")

	config.RegisterEnabled = false
	_, resp = postJSON(t, r, "/register", `{"username":"newbie","password":"password123"}`)
	expect(t, resp, common.APICodeRegisterDisabled, "The admin has disabled new user sign-up")

	config.RegisterEnabled, config.PasswordRegisterEnabled = true, true
	_, resp = postJSON(t, r, "/register", `{"username":"newbie","password":"short"}`)
	expect(t, resp, common.APICodePasswordTooShort, "Password must be at least 8 characters")
	if params, _ := resp["params"].(map[string]interface{}); params["min"] != "8" {
		t.Fatalf("params.min = %v, want 8", resp["params"])
	}

	if err := model.DB.Create(&model.User{Username: "taken", Password: "x", AccessToken: "tok-taken", AffCode: "tkn1"}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	_, resp = postJSON(t, r, "/register", `{"username":"taken","password":"password123"}`)
	expect(t, resp, common.APICodeUsernameTaken, "username already exists")
}

// Limits in the text and params come from the validate tag, so they cannot drift from the rule.
func TestFriendlyValidationErrorUsesTagLimits(t *testing.T) {
	err := common.Validate.Struct(&selfUpdateRequest{Password: "0123456789012345678901234567890123456789012345678901234567890123x"})
	if err == nil {
		t.Fatal("a 65-character password must be rejected")
	}
	body := common.APIErrorBody(friendlyValidationError(err))
	if body["code"] != common.APICodePasswordTooLong || body["message"] != "Password must not exceed 64 characters" {
		t.Fatalf("unexpected body: %v", body)
	}
	if params, _ := body["params"].(map[string]any); params["max"] != "64" {
		t.Fatalf("params.max = %v, want 64", body["params"])
	}
}

// Admin user lookups report a malformed or unknown id with a code instead of strconv or
// "record not found" text.
func TestAdminUserLookupCarriesCodes(t *testing.T) {
	setupGetUserTestDB(t)
	gin.SetMode(gin.TestMode)
	get := func(id string) map[string]interface{} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: id}}
		c.Request = httptest.NewRequest(http.MethodGet, "/api/user/"+id, nil)
		c.Set("role", config.RoleRootUser)
		GetUser(c)
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		return resp
	}
	if resp := get("abc"); resp["code"] != common.APICodeInvalidUserID {
		t.Fatalf("malformed id: %v", resp)
	}
	if resp := get("424242"); resp["code"] != common.APICodeUserNotFound {
		t.Fatalf("unknown id: %v", resp)
	}
}

// Model-level organization errors keep their English text and gain a code.
func TestOrganizationModelErrorsCarryCodes(t *testing.T) {
	err := model.ValidateOrgBudget(&model.QuotaResetSetting{Limit: 0})
	var apiErr *common.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != common.APICodeOrgBudgetInvalid || err.Error() != "budget must be greater than 0" {
		t.Fatalf("unexpected error: %#v", err)
	}
}
