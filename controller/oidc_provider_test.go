package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupOidcProviderTestDB 建一个只含 OIDC 相关表的内存库。
func setupOidcProviderTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&model.OidcProvider{}, &model.UserOidcIdentity{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })
	return testDB
}

// newFakeIdP 起一个只提供 .well-known/openid-configuration 的假 IdP，
// issuer 必须与实际地址一致，否则 go-oidc 会判为 issuer mismatch。
func newFakeIdP(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(nil)
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q,"jwks_uri":%q,"id_token_signing_alg_values_supported":["RS256"]}`,
			server.URL, server.URL+"/auth", server.URL+"/token", server.URL+"/keys")
	})
	server.Config.Handler = mux
	t.Cleanup(server.Close)
	return server.URL
}

// callOidcProvider 直接调用 handler，绕过 AdminAuth（鉴权由 router 分组统一挂载）。
func callOidcProvider(handler gin.HandlerFunc, method, target, body string, params gin.Params) (int, map[string]any) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = params
	handler(c)
	resp := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return w.Code, resp
}

// TestCreateOidcProviderDiscoveryFailure issuer 不可达时新建返回 4xx，且报错不含 client_secret。
func TestCreateOidcProviderDiscoveryFailure(t *testing.T) {
	setupOidcProviderTestDB(t)
	const secret = "super-secret-value"
	body := fmt.Sprintf(`{"slug":"bad","issuer":"https://127.0.0.1:1/idp","client_id":"cid","client_secret":%q,"scopes":"openid profile","enabled":true}`, secret)
	code, resp := callOidcProvider(CreateOidcProvider, http.MethodPost, "/api/oidc_provider/", body, nil)
	if code < 400 || code >= 500 {
		t.Fatalf("discovery 失败应返回 4xx，实际 %d: %v", code, resp)
	}
	message, _ := resp["message"].(string)
	if strings.Contains(message, secret) {
		t.Fatalf("错误信息泄露了 client_secret: %s", message)
	}
	var count int64
	model.DB.Model(&model.OidcProvider{}).Count(&count)
	if count != 0 {
		t.Fatalf("discovery 失败不应落库，实际 %d 行", count)
	}
}

// TestCreateOidcProviderForceSavesDisabled force=true 时允许保存，但一律停用。
func TestCreateOidcProviderForceSavesDisabled(t *testing.T) {
	setupOidcProviderTestDB(t)
	body := `{"slug":"bad","issuer":"https://127.0.0.1:1/idp","client_id":"cid","scopes":"openid","enabled":true}`
	code, resp := callOidcProvider(CreateOidcProvider, http.MethodPost, "/api/oidc_provider/?force=true", body, nil)
	if code != http.StatusOK || resp["success"] != true {
		t.Fatalf("force=true 应保存成功，实际 %d: %v", code, resp)
	}
	provider, err := model.GetOidcProviderBySlug("bad")
	if err != nil {
		t.Fatalf("读取提供方失败: %v", err)
	}
	if provider.Enabled {
		t.Fatal("force 保存必须落成 enabled=false")
	}
}

// TestOidcProviderSecretNeverReturned 读接口不得出现 client_secret；has_client_secret 反映是否已配。
func TestOidcProviderSecretNeverReturned(t *testing.T) {
	setupOidcProviderTestDB(t)
	issuer := newFakeIdP(t)
	const secret = "another-secret"
	body := fmt.Sprintf(`{"slug":"idp","display_name":"IdP","issuer":%q,"client_id":"cid","client_secret":%q,"scopes":"openid","enabled":true}`, issuer, secret)
	code, resp := callOidcProvider(CreateOidcProvider, http.MethodPost, "/api/oidc_provider/", body, nil)
	if code != http.StatusOK || resp["success"] != true {
		t.Fatalf("新建应成功，实际 %d: %v", code, resp)
	}
	provider, err := model.GetOidcProviderBySlug("idp")
	if err != nil {
		t.Fatalf("读取提供方失败: %v", err)
	}
	if provider.ClientSecret != secret {
		t.Fatalf("client_secret 未落库: %q", provider.ClientSecret)
	}

	params := gin.Params{{Key: "id", Value: fmt.Sprint(provider.Id)}}
	for name, call := range map[string]func() (int, map[string]any){
		"detail": func() (int, map[string]any) {
			return callOidcProvider(GetOidcProvider, http.MethodGet, "/api/oidc_provider/1", "", params)
		},
		"list": func() (int, map[string]any) {
			return callOidcProvider(GetOidcProvidersList, http.MethodGet, "/api/oidc_provider/", "", nil)
		},
	} {
		status, payload := call()
		if status != http.StatusOK {
			t.Fatalf("%s 应返回 200，实际 %d", name, status)
		}
		raw, _ := json.Marshal(payload)
		if strings.Contains(string(raw), secret) || strings.Contains(string(raw), `"client_secret"`) {
			t.Fatalf("%s 响应泄露密钥: %s", name, raw)
		}
		if !strings.Contains(string(raw), `"has_client_secret":true`) {
			t.Fatalf("%s 响应缺少 has_client_secret=true: %s", name, raw)
		}
	}
}

// TestUpdateOidcProviderKeepsSecretAndSlug 更新时空 client_secret 表示不修改，slug 不可变。
func TestUpdateOidcProviderKeepsSecretAndSlug(t *testing.T) {
	setupOidcProviderTestDB(t)
	issuer := newFakeIdP(t)
	const secret = "keep-me"
	provider := &model.OidcProvider{
		Slug: "idp", DisplayName: "IdP", Issuer: issuer, ClientId: "cid", ClientSecret: secret,
		Scopes: "openid", UsernameClaim: "sub", DisplayNameClaim: "name", AvatarClaim: "picture", Enabled: true,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("插入提供方失败: %v", err)
	}

	params := gin.Params{{Key: "id", Value: fmt.Sprint(provider.Id)}}
	body := fmt.Sprintf(`{"slug":"renamed","display_name":"New","issuer":%q,"client_id":"cid2","client_secret":"","scopes":"openid email","enabled":false}`, issuer)
	code, resp := callOidcProvider(UpdateOidcProvider, http.MethodPut, "/api/oidc_provider/1", body, params)
	if code != http.StatusOK || resp["success"] != true {
		t.Fatalf("更新应成功，实际 %d: %v", code, resp)
	}
	updated, err := model.GetOidcProviderById(provider.Id)
	if err != nil {
		t.Fatalf("读取提供方失败: %v", err)
	}
	if updated.Slug != "idp" {
		t.Fatalf("slug 不应被修改，实际 %q", updated.Slug)
	}
	if updated.ClientSecret != secret {
		t.Fatalf("空 client_secret 不应清空已配密钥，实际 %q", updated.ClientSecret)
	}
	if updated.ClientId != "cid2" || updated.DisplayName != "New" || updated.Enabled {
		t.Fatalf("其余字段未按入参更新: %+v", updated)
	}
}

// TestOidcProviderFirstPartyRoundTrip first_party 能落库、回显，并能从 true 改回 false
// （零值必须靠 Update 的 Select 列写回去）。
func TestOidcProviderFirstPartyRoundTrip(t *testing.T) {
	setupOidcProviderTestDB(t)
	issuer := newFakeIdP(t)
	body := fmt.Sprintf(`{"slug":"idp","issuer":%q,"client_id":"cid","client_secret":"s","scopes":"openid","first_party":true,"enabled":true}`, issuer)
	code, resp := callOidcProvider(CreateOidcProvider, http.MethodPost, "/api/oidc_provider/", body, nil)
	if code != http.StatusOK || resp["success"] != true {
		t.Fatalf("新建应成功，实际 %d: %v", code, resp)
	}
	provider, err := model.GetOidcProviderBySlug("idp")
	if err != nil {
		t.Fatalf("读取提供方失败: %v", err)
	}
	if !provider.FirstParty {
		t.Fatal("first_party=true 未落库")
	}

	params := gin.Params{{Key: "id", Value: fmt.Sprint(provider.Id)}}
	detailCode, detail := callOidcProvider(GetOidcProvider, http.MethodGet, "/api/oidc_provider/1", "", params)
	raw, _ := json.Marshal(detail)
	if detailCode != http.StatusOK || !strings.Contains(string(raw), `"first_party":true`) {
		t.Fatalf("读接口应回显 first_party=true，实际 %d: %s", detailCode, raw)
	}

	updateBody := fmt.Sprintf(`{"issuer":%q,"client_id":"cid","scopes":"openid","first_party":false,"enabled":true}`, issuer)
	updateCode, updateResp := callOidcProvider(UpdateOidcProvider, http.MethodPut, "/api/oidc_provider/1", updateBody, params)
	if updateCode != http.StatusOK || updateResp["success"] != true {
		t.Fatalf("更新应成功，实际 %d: %v", updateCode, updateResp)
	}
	updated, err := model.GetOidcProviderById(provider.Id)
	if err != nil {
		t.Fatalf("读取提供方失败: %v", err)
	}
	if updated.FirstParty {
		t.Fatal("first_party 应被改回 false")
	}
}

// TestOidcProviderAccountSettingsUrlRoundTrip account_settings_url 能落库、回显，
// 并能清空（空串是零值，必须靠 Update 的 Select 列写回去）。
func TestOidcProviderAccountSettingsUrlRoundTrip(t *testing.T) {
	setupOidcProviderTestDB(t)
	issuer := newFakeIdP(t)
	const settingsUrl = "https://idp.example.com/settings"
	body := fmt.Sprintf(`{"slug":"idp","issuer":%q,"client_id":"cid","client_secret":"s","scopes":"openid","account_settings_url":%q,"enabled":true}`, issuer, settingsUrl)
	code, resp := callOidcProvider(CreateOidcProvider, http.MethodPost, "/api/oidc_provider/", body, nil)
	if code != http.StatusOK || resp["success"] != true {
		t.Fatalf("新建应成功，实际 %d: %v", code, resp)
	}
	provider, err := model.GetOidcProviderBySlug("idp")
	if err != nil {
		t.Fatalf("读取提供方失败: %v", err)
	}
	if provider.AccountSettingsUrl != settingsUrl {
		t.Fatalf("account_settings_url 未落库，实际 %q", provider.AccountSettingsUrl)
	}

	params := gin.Params{{Key: "id", Value: fmt.Sprint(provider.Id)}}
	detailCode, detail := callOidcProvider(GetOidcProvider, http.MethodGet, "/api/oidc_provider/1", "", params)
	raw, _ := json.Marshal(detail)
	if detailCode != http.StatusOK || !strings.Contains(string(raw), settingsUrl) {
		t.Fatalf("读接口应回显 account_settings_url，实际 %d: %s", detailCode, raw)
	}

	updateBody := fmt.Sprintf(`{"issuer":%q,"client_id":"cid","scopes":"openid","account_settings_url":"","enabled":true}`, issuer)
	if updateCode, updateResp := callOidcProvider(UpdateOidcProvider, http.MethodPut, "/api/oidc_provider/1", updateBody, params); updateCode != http.StatusOK || updateResp["success"] != true {
		t.Fatalf("更新应成功，实际 %d: %v", updateCode, updateResp)
	}
	updated, err := model.GetOidcProviderById(provider.Id)
	if err != nil {
		t.Fatalf("读取提供方失败: %v", err)
	}
	if updated.AccountSettingsUrl != "" {
		t.Fatalf("account_settings_url 应被清空，实际 %q", updated.AccountSettingsUrl)
	}
}

// TestDeleteOidcProviderBlockedByIdentities 仍有身份行时删除被拒，且提供方与身份行都还在。
func TestDeleteOidcProviderBlockedByIdentities(t *testing.T) {
	setupOidcProviderTestDB(t)
	provider := &model.OidcProvider{Slug: "idp", Issuer: "https://idp.example.com", ClientId: "cid", Scopes: "openid"}
	if err := provider.Insert(); err != nil {
		t.Fatalf("插入提供方失败: %v", err)
	}
	identity := &model.UserOidcIdentity{UserId: 7, ProviderId: provider.Id, Subject: "sub-7"}
	if err := identity.Insert(); err != nil {
		t.Fatalf("插入身份失败: %v", err)
	}

	params := gin.Params{{Key: "id", Value: fmt.Sprint(provider.Id)}}
	code, resp := callOidcProvider(DeleteOidcProvider, http.MethodDelete, "/api/oidc_provider/1", "", params)
	if code != http.StatusConflict || resp["success"] != false {
		t.Fatalf("仍有身份行时删除应返回 409，实际 %d: %v", code, resp)
	}
	if _, err := model.GetOidcProviderById(provider.Id); err != nil {
		t.Fatalf("被拒的删除不应落库: %v", err)
	}
	var count int64
	model.DB.Model(&model.UserOidcIdentity{}).Count(&count)
	if count != 1 {
		t.Fatalf("身份行不应被删除，实际 %d 行", count)
	}
}

// TestUpdateOidcProviderStatus 启停不做 discovery：IdP 不可达时也必须能停用。
func TestUpdateOidcProviderStatus(t *testing.T) {
	setupOidcProviderTestDB(t)
	provider := &model.OidcProvider{Slug: "idp", Issuer: "https://127.0.0.1:1/idp", ClientId: "cid", Scopes: "openid", Enabled: true}
	if err := provider.Insert(); err != nil {
		t.Fatalf("插入提供方失败: %v", err)
	}
	params := gin.Params{{Key: "id", Value: fmt.Sprint(provider.Id)}}
	code, resp := callOidcProvider(UpdateOidcProviderStatus, http.MethodPut, "/api/oidc_provider/1/status", `{"enabled":false}`, params)
	if code != http.StatusOK || resp["success"] != true {
		t.Fatalf("启停应成功，实际 %d: %v", code, resp)
	}
	updated, err := model.GetOidcProviderById(provider.Id)
	if err != nil {
		t.Fatalf("读取提供方失败: %v", err)
	}
	if updated.Enabled {
		t.Fatal("enabled 应被置为 false")
	}
}

// TestOidcProviderDisableAutoRegisterRoundTrip 新建与更新都要能落库并回显 disable_auto_register，
// 且关闭时（零值）也必须写回去。
func TestOidcProviderDisableAutoRegisterRoundTrip(t *testing.T) {
	setupOidcProviderTestDB(t)
	issuer := newFakeIdP(t)

	body := fmt.Sprintf(`{"slug":"idp","issuer":%q,"client_id":"cid","scopes":"openid","enabled":true,"disable_auto_register":true}`, issuer)
	code, resp := callOidcProvider(CreateOidcProvider, http.MethodPost, "/api/oidc_provider/", body, nil)
	if code != http.StatusOK || resp["success"] != true {
		t.Fatalf("新建应成功，实际 %d: %v", code, resp)
	}
	created, err := model.GetOidcProviderBySlug("idp")
	if err != nil {
		t.Fatalf("读取提供方失败: %v", err)
	}
	if !created.DisableAutoRegister {
		t.Fatal("disable_auto_register=true 未落库")
	}

	params := gin.Params{{Key: "id", Value: fmt.Sprint(created.Id)}}
	status, payload := callOidcProvider(GetOidcProvider, http.MethodGet, "/api/oidc_provider/1", "", params)
	raw, _ := json.Marshal(payload)
	if status != http.StatusOK || !strings.Contains(string(raw), `"disable_auto_register":true`) {
		t.Fatalf("读接口应回显 disable_auto_register=true，实际 %d: %s", status, raw)
	}

	updateBody := fmt.Sprintf(`{"issuer":%q,"client_id":"cid","scopes":"openid","enabled":true,"disable_auto_register":false}`, issuer)
	if code, resp := callOidcProvider(UpdateOidcProvider, http.MethodPut, "/api/oidc_provider/1", updateBody, params); code != http.StatusOK || resp["success"] != true {
		t.Fatalf("更新应成功，实际 %d: %v", code, resp)
	}
	updated, err := model.GetOidcProviderById(created.Id)
	if err != nil {
		t.Fatalf("读取提供方失败: %v", err)
	}
	if updated.DisableAutoRegister {
		t.Fatal("disable_auto_register=false 应被写回")
	}
}

// TestOidcProviderRequestValidation 校验规则：slug、issuer 协议、scopes 含 openid、claim 缺省值。
func TestOidcProviderRequestValidation(t *testing.T) {
	setupOidcProviderTestDB(t)
	cases := []struct{ name, body string }{
		{"slug 非法", `{"slug":"BAD_SLUG","issuer":"https://idp.example.com","client_id":"cid","scopes":"openid"}`},
		{"issuer 非 https", `{"slug":"idp","issuer":"http://idp.example.com","client_id":"cid","scopes":"openid"}`},
		{"issuer 为空", `{"slug":"idp","issuer":"","client_id":"cid","scopes":"openid"}`},
		{"scopes 缺 openid", `{"slug":"idp","issuer":"https://idp.example.com","client_id":"cid","scopes":"profile email"}`},
		{"client_id 为空", `{"slug":"idp","issuer":"https://idp.example.com","client_id":"","scopes":"openid"}`},
		{"账号设置页非 https", `{"slug":"idp","issuer":"https://idp.example.com","client_id":"cid","scopes":"openid","account_settings_url":"http://idp.example.com/settings"}`},
		{"账号设置页非绝对 URL", `{"slug":"idp","issuer":"https://idp.example.com","client_id":"cid","scopes":"openid","account_settings_url":"/settings"}`},
	}
	for _, tc := range cases {
		code, resp := callOidcProvider(CreateOidcProvider, http.MethodPost, "/api/oidc_provider/", tc.body, nil)
		if code != http.StatusBadRequest {
			t.Fatalf("%s 应返回 400，实际 %d: %v", tc.name, code, resp)
		}
	}

	issuer := newFakeIdP(t)
	body := fmt.Sprintf(`{"slug":"idp","issuer":%q,"client_id":"cid","scopes":"openid"}`, issuer)
	if code, resp := callOidcProvider(CreateOidcProvider, http.MethodPost, "/api/oidc_provider/", body, nil); code != http.StatusOK {
		t.Fatalf("合法入参应成功，实际 %d: %v", code, resp)
	}
	created, err := model.GetOidcProviderBySlug("idp")
	if err != nil {
		t.Fatalf("读取提供方失败: %v", err)
	}
	if created.UsernameClaim != oidcProviderDefaultUsernameClaim ||
		created.DisplayNameClaim != oidcProviderDefaultDisplayNameClaim ||
		created.AvatarClaim != oidcProviderDefaultAvatarClaim {
		t.Fatalf("claim 缺省值未填充: %+v", created)
	}
	if created.DisplayName != "idp" {
		t.Fatalf("display_name 应缺省取 slug，实际 %q", created.DisplayName)
	}

	// slug 唯一：重复新建返回 409。
	if code, resp := callOidcProvider(CreateOidcProvider, http.MethodPost, "/api/oidc_provider/", body, nil); code != http.StatusConflict {
		t.Fatalf("重复 slug 应返回 409，实际 %d: %v", code, resp)
	}
}
