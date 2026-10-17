package controller

import (
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// Settings and redemption failures carry a code; message stays English.
func TestSettingsAndRedemptionFailuresCarryCodes(t *testing.T) {
	setupGetUserTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/option", UpdateOption)

	oldTrust := config.LinuxDoOAuthEnabled
	config.LinuxDoOAuthEnabled = false
	t.Cleanup(func() { config.LinuxDoOAuthEnabled = oldTrust })

	_, resp := postJSON(t, r, "/option", `{"key":"AccountSystem","value":"other"}`)
	if resp["code"] != common.APICodeOptionAccountSystemInvalid || resp["message"] != "Account system must be builtin or external" {
		t.Fatalf("account system: %v", resp)
	}
	_, resp = postJSON(t, r, "/option", `{"key":"LinuxDoOAuthTrustLevelEnabled","value":"true"}`)
	if resp["code"] != common.APICodeOptionLinuxdoTrustRequiresOauth {
		t.Fatalf("trust level: %v", resp)
	}

	if _, err := model.Redeem("no-such-code", 1, "127.0.0.1"); err == nil || common.APIErrorBody(err)["code"] != common.APICodeRedemptionCodeInvalid {
		t.Fatalf("redeem unknown code: %v", err)
	}
}
