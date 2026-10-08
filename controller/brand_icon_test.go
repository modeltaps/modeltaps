package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/brandicon"
	"github.com/modeltaps/modeltaps/common/safefetch"
	"github.com/modeltaps/modeltaps/middleware"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupBrandIconRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	api.Use(middleware.NoCache())
	api.GET("/brand-icon/manifest", GetBrandIconManifest)
	api.GET("/brand-icon/:key", GetBrandIcon)
	return r
}

func doBrandIconRequest(r *gin.Engine, path string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestGetBrandIcon(t *testing.T) {
	r := setupBrandIconRouter()
	for _, path := range []string{"/api/brand-icon/openai", "/api/brand-icon/claude?variant=mono", "/api/brand-icon/Anthropic?variant=color"} {
		w := doBrandIconRequest(r, path, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: 期望 200，实际 %d", path, w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "image/svg+xml" {
			t.Fatalf("%s: Content-Type 应为 image/svg+xml，实际 %q", path, ct)
		}
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s: 缺少 nosniff", path)
		}
		if w.Header().Get("ETag") != brandicon.ETag() {
			t.Fatalf("%s: ETag 不对: %q", path, w.Header().Get("ETag"))
		}
		if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "max-age") || w.Header().Get("Pragma") != "" {
			t.Fatalf("%s: 应覆盖 NoCache，实际 Cache-Control=%q Pragma=%q", path, cc, w.Header().Get("Pragma"))
		}
		if !strings.HasPrefix(strings.TrimSpace(w.Body.String()), "<svg") {
			t.Fatalf("%s: 响应体不是 SVG", path)
		}
	}
}

func TestGetBrandIconErrors(t *testing.T) {
	r := setupBrandIconRouter()
	if w := doBrandIconRequest(r, "/api/brand-icon/no-such-brand", nil); w.Code != http.StatusNotFound {
		t.Fatalf("未知 key 期望 404，实际 %d", w.Code)
	}
	if w := doBrandIconRequest(r, "/api/brand-icon/openai?variant=rainbow", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("非法 variant 期望 400，实际 %d", w.Code)
	}
}

func TestGetBrandIconNotModified(t *testing.T) {
	r := setupBrandIconRouter()
	for _, path := range []string{"/api/brand-icon/openai", "/api/brand-icon/manifest"} {
		w := doBrandIconRequest(r, path, map[string]string{"If-None-Match": brandicon.ETag()})
		if w.Code != http.StatusNotModified || w.Body.Len() != 0 {
			t.Fatalf("%s: 期望 304 空响应，实际 %d / %d 字节", path, w.Code, w.Body.Len())
		}
	}
}

func TestGetBrandIconManifest(t *testing.T) {
	r := setupBrandIconRouter()
	w := doBrandIconRequest(r, "/api/brand-icon/manifest", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type 不对: %q", ct)
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("ETag") == "" {
		t.Fatal("manifest 缺少 nosniff 或 ETag")
	}
	var m brandicon.Manifest
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("manifest 不是合法 JSON: %v", err)
	}
	if m.Version == "" || len(m.Icons) == 0 || m.ChannelTypes["1"] != "openai" {
		t.Fatalf("manifest 内容不完整: version=%q icons=%d", m.Version, len(m.Icons))
	}
}

func TestGetBrandIconByDomain(t *testing.T) {
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&model.BrandIconCache{}); err != nil {
		t.Fatal(err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })

	png := []byte("\x89PNG\r\n\x1a\ndata")
	testDB.Create(&model.BrandIconCache{Domain: "foo.com", Status: model.BrandIconStatusOK, MimeType: "image/png", Data: png, Sha256: "abc"})
	testDB.Create(&model.BrandIconCache{Domain: "bad.com", Status: model.BrandIconStatusFailed})

	r := setupBrandIconRouter()
	r.GET("/api/brand-icon/domain/:domain", GetBrandIconByDomain)

	w := doBrandIconRequest(r, "/api/brand-icon/domain/api.Foo.com", nil)
	if w.Code != http.StatusOK || w.Body.String() != string(png) || w.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("hit: code=%d type=%q", w.Code, w.Header().Get("Content-Type"))
	}
	if w.Header().Get("ETag") != `"abc"` || !strings.Contains(w.Header().Get("Cache-Control"), "max-age=86400") ||
		w.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("unexpected headers %v", w.Header())
	}
	if w := doBrandIconRequest(r, "/api/brand-icon/domain/foo.com", map[string]string{"If-None-Match": `"abc"`}); w.Code != http.StatusNotModified {
		t.Errorf("etag: code=%d", w.Code)
	}
	for _, d := range []string{"bad.com", "missing.com"} {
		if w := doBrandIconRequest(r, "/api/brand-icon/domain/"+d, nil); w.Code != http.StatusNotFound {
			t.Errorf("%s: code=%d, want 404", d, w.Code)
		}
	}
	for _, d := range []string{"localhost", "127.0.0.1", "bad_host.com"} {
		if w := doBrandIconRequest(r, "/api/brand-icon/domain/"+d, nil); w.Code != http.StatusBadRequest {
			t.Errorf("%s: code=%d, want 400", d, w.Code)
		}
	}
	var count int64
	testDB.Model(&model.BrandIconCache{}).Count(&count)
	if count != 2 {
		t.Errorf("read endpoint must not write cache rows, count=%d", count)
	}
}

func TestGetBrandIconUpload(t *testing.T) {
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&model.BrandIconUpload{}); err != nil {
		t.Fatal(err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })

	png := []byte("\x89PNG\r\n\x1a\ndata")
	id, err := model.SaveBrandIconUpload(png)
	if err != nil {
		t.Fatal(err)
	}

	r := setupBrandIconRouter()
	r.GET("/api/brand-icon/upload/:assetId", GetBrandIconUpload)

	w := doBrandIconRequest(r, "/api/brand-icon/upload/"+id, nil)
	if w.Code != http.StatusOK || w.Body.String() != string(png) || w.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("hit: code=%d type=%q", w.Code, w.Header().Get("Content-Type"))
	}
	if w.Header().Get("ETag") != `"`+id+`"` || w.Header().Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(w.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("unexpected headers %v", w.Header())
	}
	if w := doBrandIconRequest(r, "/api/brand-icon/upload/"+id, map[string]string{"If-None-Match": `"` + id + `"`}); w.Code != http.StatusNotModified {
		t.Errorf("etag: code=%d", w.Code)
	}
	if w := doBrandIconRequest(r, "/api/brand-icon/upload/missing", nil); w.Code != http.StatusNotFound {
		t.Errorf("missing: code=%d, want 404", w.Code)
	}
}

func setupBrandIconAdminTest(t *testing.T) *gin.Engine {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&model.BrandIconUpload{}, &model.BrandIconCache{}); err != nil {
		t.Fatal(err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })
	r := setupBrandIconRouter()
	r.POST("/api/brand-icon/upload", UploadBrandIcon)
	r.POST("/api/brand-icon/refresh", RefreshBrandIcon)
	return r
}

func postBrandIconFile(r *gin.Engine, field, name string, data []byte) *httptest.ResponseRecorder {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if field != "" {
		fw, _ := mw.CreateFormFile(field, name)
		_, _ = fw.Write(data)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/brand-icon/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type brandIconAPIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Id     string `json:"id"`
		Icon   string `json:"icon"`
		Domain string `json:"domain"`
		Found  bool   `json:"found"`
	} `json:"data"`
}

func decodeBrandIconAPI(t *testing.T, w *httptest.ResponseRecorder) brandIconAPIResponse {
	t.Helper()
	var resp brandIconAPIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json %q: %v", w.Body.String(), err)
	}
	return resp
}

func TestUploadBrandIcon(t *testing.T) {
	r := setupBrandIconAdminTest(t)

	png := []byte("\x89PNG\r\n\x1a\ndata")
	w := postBrandIconFile(r, "file", "logo.png", png)
	resp := decodeBrandIconAPI(t, w)
	if w.Code != http.StatusOK || !resp.Success || resp.Data.Icon != "upload:"+resp.Data.Id {
		t.Fatalf("png: code=%d body=%s", w.Code, w.Body.String())
	}
	row, err := model.GetBrandIconUpload(resp.Data.Id)
	if err != nil || row.MimeType != "image/png" || string(row.Data) != string(png) {
		t.Fatalf("stored row=%+v err=%v", row, err)
	}
	if icon, err := model.NormalizeVendorIcon(resp.Data.Icon); err != nil || icon != resp.Data.Icon {
		t.Errorf("uploaded icon should be accepted by vendor form, icon=%q err=%v", icon, err)
	}

	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" onload="alert(1)"><script>alert(1)</script><path d="M0 0h24v24H0z"/></svg>`)
	w = postBrandIconFile(r, "file", "logo.svg", svg)
	resp = decodeBrandIconAPI(t, w)
	if w.Code != http.StatusOK || !resp.Success {
		t.Fatalf("svg: code=%d body=%s", w.Code, w.Body.String())
	}
	row, _ = model.GetBrandIconUpload(resp.Data.Id)
	if row == nil || row.MimeType != "image/svg+xml" || strings.Contains(string(row.Data), "script") ||
		strings.Contains(string(row.Data), "onload") || !strings.Contains(string(row.Data), "<path") {
		t.Fatalf("svg should be sanitized, row=%+v", row)
	}

	cases := []struct {
		name  string
		field string
		data  []byte
		code  int
	}{
		{"missing file", "", nil, http.StatusBadRequest},
		{"unsupported type", "file", []byte("GIF-not-really <html>"), http.StatusBadRequest},
		{"html disguised", "file", []byte("<html><script>alert(1)</script></html>"), http.StatusBadRequest},
		{"too large", "file", append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, safefetch.DefaultMaxBytes)...), http.StatusRequestEntityTooLarge},
	}
	for _, tc := range cases {
		w := postBrandIconFile(r, tc.field, "x.bin", tc.data)
		if w.Code != tc.code || decodeBrandIconAPI(t, w).Success {
			t.Errorf("%s: code=%d, want %d (body=%s)", tc.name, w.Code, tc.code, w.Body.String())
		}
	}
	var count int64
	model.DB.Model(&model.BrandIconUpload{}).Count(&count)
	if count != 2 {
		t.Errorf("rejected uploads must not be stored, count=%d", count)
	}
}

func postBrandIconRefresh(r *gin.Engine, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/brand-icon/refresh", strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRefreshBrandIcon(t *testing.T) {
	r := setupBrandIconAdminTest(t)
	oldSync := BrandIconSync
	t.Cleanup(func() { BrandIconSync = oldSync })

	BrandIconSync = nil
	if w := postBrandIconRefresh(r, ""); w.Code != http.StatusServiceUnavailable || decodeBrandIconAPI(t, w).Success {
		t.Errorf("no sync hook: code=%d body=%s", w.Code, w.Body.String())
	}

	calls := 0
	BrandIconSync = func(context.Context) error { calls++; return nil }
	if w := postBrandIconRefresh(r, "{}"); w.Code != http.StatusOK || !decodeBrandIconAPI(t, w).Success || calls != 1 {
		t.Errorf("sync: code=%d calls=%d body=%s", w.Code, calls, w.Body.String())
	}
	BrandIconSync = func(context.Context) error { return errors.New("upstream down") }
	if w := postBrandIconRefresh(r, ""); w.Code != http.StatusBadGateway || decodeBrandIconAPI(t, w).Success {
		t.Errorf("sync error: code=%d body=%s", w.Code, w.Body.String())
	}

	for _, body := range []string{`{"domain":"localhost"}`, `{"domain":"http://127.0.0.1:8080/v1"}`, `{"domain":"bad_host.com"}`, `not json`} {
		if w := postBrandIconRefresh(r, body); w.Code != http.StatusBadRequest || decodeBrandIconAPI(t, w).Success {
			t.Errorf("%s: code=%d, want 400 (body=%s)", body, w.Code, w.Body.String())
		}
	}
	var count int64
	model.DB.Model(&model.BrandIconCache{}).Count(&count)
	if count != 0 {
		t.Errorf("invalid domains must not touch the cache, count=%d", count)
	}
}
