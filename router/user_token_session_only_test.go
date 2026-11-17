package router

import (
	"net/http"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/model"
)

// SEC-71: a legacy access token cannot issue the next legacy access token; only a console
// session can.
func TestUserTokenNeedsSession(t *testing.T) {
	f := setupManagementKeyRouter(t, false)
	legacy := "fedcba9876543210fedcba9876543210"
	if err := model.DB.Create(&model.ManagementKey{UserId: f.user.Id, Name: model.LegacyAccessTokenName, Scope: model.ManagementKeyScopeLegacy, Key: legacy, ExpiredTime: -1}).Error; err != nil {
		t.Fatalf("create legacy token: %v", err)
	}
	expectDenied(t, f.do(http.MethodGet, "/api/user/token", legacy, nil), http.StatusForbidden, common.APICodeManagementKeyForbidden, "legacy token on /api/user/token")
	expectAllowed(t, f.do(http.MethodGet, "/api/user/token", "", nil, f.login(f.user.Id)...), "console session on /api/user/token")
}
