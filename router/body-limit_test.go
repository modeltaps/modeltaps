package router

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/spf13/viper"
)

// SEC-57: /api, the dashboard and MCP routes cap request bodies; relay routes keep their own cap.

const bodyLimitRootToken = "bodylimitroottoken00000000000003"

// zeroBody serves n bytes and counts how many were read.
type zeroBody struct {
	left, read int64
}

func (b *zeroBody) Read(p []byte) (int, error) {
	if b.left <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > b.left {
		p = p[:b.left]
	}
	clear(p)
	b.left -= int64(len(p))
	b.read += int64(len(p))
	return len(p), nil
}

func sendSized(h http.Handler, method, target, token string, body io.Reader, size int64) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, body)
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestAPIBodyLimit(t *testing.T) {
	for _, key := range []string{"api_max_body_mb", "relay_max_body_mb"} {
		orig, set := viper.Get(key), viper.IsSet(key)
		t.Cleanup(func() {
			if set {
				viper.Set(key, orig)
			} else {
				viper.Set(key, nil)
			}
		})
	}
	viper.Set("api_max_body_mb", 2)
	viper.Set("relay_max_body_mb", 8)

	engine := setupModelInfoRouter(t)
	SetDashboardRouter(engine)
	SetRelayRouter(engine)
	root := &model.User{Username: "body-root", DisplayName: "root", Role: config.RoleRootUser,
		Status: config.UserStatusEnabled, AffCode: "affroot"}
	if err := model.DB.Create(root).Error; err != nil {
		t.Fatal(err)
	}
	if err := model.DB.Create(&model.ManagementKey{UserId: root.Id, Scope: model.ManagementKeyScopeLegacy,
		Key: bodyLimitRootToken, ExpiredTime: -1}).Error; err != nil {
		t.Fatal(err)
	}

	t.Run("oversized login is refused before it is read", func(t *testing.T) {
		body := &zeroBody{left: 200 << 20}
		w := sendSized(engine, http.MethodPost, "/api/user/login", "", body, 200<<20)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("want 413, got %d %s", w.Code, w.Body.String())
		}
		if body.read != 0 {
			t.Fatalf("the body must not be read, read %d bytes", body.read)
		}
		if !strings.Contains(w.Body.String(), `"message":"request body exceeds the 2 MB limit`) {
			t.Fatalf("unexpected error body: %s", w.Body.String())
		}
	})

	t.Run("chunked oversized login stops at the limit", func(t *testing.T) {
		body := &zeroBody{left: 200 << 20}
		w := sendSized(engine, http.MethodPost, "/api/user/login", "", body, -1)
		if w.Code == http.StatusOK && strings.Contains(w.Body.String(), `"success":true`) {
			t.Fatalf("chunked oversized login must fail: %s", w.Body.String())
		}
		if body.read > 2<<20+1 {
			t.Fatalf("read %d bytes, past the 2 MB limit", body.read)
		}
	})

	t.Run("normal login body passes the limit", func(t *testing.T) {
		raw := `{"username":"nobody","password":"wrong-password"}`
		w := sendSized(engine, http.MethodPost, "/api/user/login", "", strings.NewReader(raw), int64(len(raw)))
		if w.Code == http.StatusRequestEntityTooLarge {
			t.Fatalf("a normal body must not be refused: %s", w.Body.String())
		}
	})

	t.Run("import route takes a larger body", func(t *testing.T) {
		// The handler decodes the whole body before refusing the SMTP key, so the 3 MB value
		// is read in full.
		raw := []byte(`{"key":"SMTPServer","value":"` + strings.Repeat("a", 3<<20) + `"}`)
		w := sendSized(engine, http.MethodPut, "/api/option/", bodyLimitRootToken, bytes.NewReader(raw), int64(len(raw)))
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "smtp_use_mail_services") {
			t.Fatalf("3 MB option update should reach the handler: %d %s", w.Code, w.Body.String())
		}
		w = sendSized(engine, http.MethodPut, "/api/option/", bodyLimitRootToken, &zeroBody{left: 40 << 20}, 40<<20)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("40 MB option update should be 413, got %d", w.Code)
		}
		// Without an admin the larger body is still not read: AdminAuth answers first.
		body := &zeroBody{left: 3 << 20}
		w = sendSized(engine, http.MethodPost, "/api/prices/sync", "", body, 3<<20)
		if w.Code != http.StatusUnauthorized || body.read != 0 {
			t.Fatalf("anonymous price sync: %d, read %d bytes", w.Code, body.read)
		}
	})

	t.Run("payment notifications take 1 MB", func(t *testing.T) {
		w := sendSized(engine, http.MethodPost, "/api/payment/notify/abc", "", &zeroBody{left: 1<<20 + 1}, 1<<20+1)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("want 413, got %d %s", w.Code, w.Body.String())
		}
	})

	t.Run("dashboard routes are capped", func(t *testing.T) {
		w := sendSized(engine, http.MethodGet, "/v1/dashboard/billing/usage", "", &zeroBody{left: 3 << 20}, 3<<20)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("want 413, got %d %s", w.Code, w.Body.String())
		}
	})

	t.Run("relay limit is unchanged", func(t *testing.T) {
		body := &zeroBody{left: 3 << 20}
		w := sendSized(engine, http.MethodPost, "/v1/chat/completions", "", body, 3<<20)
		if w.Code == http.StatusRequestEntityTooLarge {
			t.Fatalf("3 MB is under relay_max_body_mb and must not be refused")
		}
		w = sendSized(engine, http.MethodPost, "/v1/chat/completions", "", &zeroBody{left: 9 << 20}, 9<<20)
		if w.Code != http.StatusRequestEntityTooLarge || !strings.Contains(w.Body.String(), "8 MB") {
			t.Fatalf("over relay_max_body_mb should be 413, got %d %s", w.Code, w.Body.String())
		}
	})
}

func TestMCPBodyLimit(t *testing.T) {
	orig, set := viper.Get("api_max_body_mb"), viper.IsSet("api_max_body_mb")
	t.Cleanup(func() {
		if set {
			viper.Set("api_max_body_mb", orig)
		} else {
			viper.Set("api_max_body_mb", nil)
		}
	})
	viper.Set("api_max_body_mb", 2)
	f := setupManagementKeyRouter(t, true)
	w := sendSized(f.engine, http.MethodPost, "/mcp", "", &zeroBody{left: 3 << 20}, 3<<20)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d %s", w.Code, w.Body.String())
	}
}

// Every per-route cap must name a registered route; a renamed route would silently fall back
// to the default.
func TestAPIRouteBodyLimitsAreRegistered(t *testing.T) {
	engine := setupModelInfoRouter(t)
	registered := map[string]bool{}
	for _, ri := range engine.Routes() {
		registered[ri.Path] = true
	}
	for path := range apiRouteBodyLimits() {
		if !registered[path] {
			t.Errorf("body limit for %s, which is not a registered route", path)
		}
	}
}
