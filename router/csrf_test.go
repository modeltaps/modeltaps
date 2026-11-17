package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// SEC-52: console-session writes from another site's page are refused on /api and /mcp.
func TestConsoleWritesRefuseForeignOrigin(t *testing.T) {
	f := setupManagementKeyRouter(t, true)
	cookies := f.login(f.user.Id)

	send := func(method, target, origin string) mkResponse {
		req := httptest.NewRequest(method, target, bytes.NewReader([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		for _, c := range cookies {
			req.AddCookie(c)
		}
		w := httptest.NewRecorder()
		f.engine.ServeHTTP(w, req)
		resp := mkResponse{status: w.Code, header: w.Header(), raw: w.Body.String()}
		_ = json.Unmarshal(w.Body.Bytes(), &resp.body)
		return resp
	}

	for _, target := range []struct{ method, path string }{
		{http.MethodPost, "/api/user/token"},
		{http.MethodPut, "/api/user/self"},
		{http.MethodPost, "/api/user/logout"},
		{http.MethodPost, "/mcp"},
	} {
		expectDenied(t, send(target.method, target.path, "https://evil.example"), http.StatusForbidden, common.APICodePermissionDenied, "foreign origin on "+target.method+" "+target.path)
		expectDenied(t, send(target.method, target.path, ""), http.StatusForbidden, common.APICodePermissionDenied, "no origin on "+target.method+" "+target.path)
	}
	expectAllowed(t, send(http.MethodPost, "/api/user/token", "http://example.com"), "same-site origin on POST /api/user/token")
	// Reads are not checked.
	expectAllowed(t, send(http.MethodGet, "/api/user/self", "https://evil.example"), "foreign origin on GET /api/user/self")
}

// SEC-52: endpoints that change state are not served over GET, so a link or an image cannot
// trigger them. A GET either matches no route or falls through to a read route with a path
// parameter (/api/user/:id, /api/channel/:id), which refuses the name as an ID.
func TestStateChangingEndpointsRefuseGet(t *testing.T) {
	f := setupManagementKeyRouter(t, false)
	cookies := f.login(f.admin.Id)
	for path, readRouteCode := range map[string]string{
		"/api/verification":             "",
		"/api/reset_password":           "",
		"/api/oauth/wechat/bind":        "",
		"/api/oauth/email/bind":         "",
		"/api/oauth/linuxdo/bind":       "",
		"/api/user/logout":              "invalid_user_id",
		"/api/user/token":               "invalid_user_id",
		"/api/channel/test":             "invalid_channel_id",
		"/api/channel/test/1":           "",
		"/api/channel/check/1":          "",
		"/api/channel/update_balance":   "invalid_channel_id",
		"/api/channel/update_balance/1": "",
	} {
		r := f.do(http.MethodGet, path, "", nil, cookies...)
		if readRouteCode == "" && r.status != http.StatusNotFound {
			t.Errorf("GET %s: want 404, got %d %s", path, r.status, r.raw)
		}
		if readRouteCode != "" && (r.success() || r.code() != readRouteCode) {
			t.Errorf("GET %s: want the read route's %s, got %d %s", path, readRouteCode, r.status, r.raw)
		}
	}
}

// SEC-52: pprof (when enabled) is for the root user only.
func TestPprofNeedsRoot(t *testing.T) {
	f := setupManagementKeyRouter(t, false)
	SetPprofRouter(f.engine)
	root := f.createUser("mk-root", config.RoleRootUser)

	expectDenied(t, f.do(http.MethodGet, "/debug/pprof/cmdline", "", nil), http.StatusUnauthorized, common.APICodeNotSignedIn, "anonymous pprof")
	expectDenied(t, f.do(http.MethodGet, "/debug/pprof/cmdline", "", nil, f.login(f.admin.Id)...), http.StatusOK, common.APICodePermissionDenied, "admin pprof")
	rootKey := f.managementKey(root.Id, model.ManagementKeyScopeRead)
	expectDenied(t, f.do(http.MethodGet, "/debug/pprof/cmdline", rootKey, nil), http.StatusForbidden, common.APICodeManagementKeyForbidden, "root management key on pprof")

	r := f.do(http.MethodGet, "/debug/pprof/cmdline", "", nil, f.login(root.Id)...)
	if r.status != http.StatusOK || r.raw == "" || r.body != nil {
		t.Fatalf("root pprof: want the profile, got %d %s", r.status, r.raw)
	}
}
