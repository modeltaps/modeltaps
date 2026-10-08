package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// loginDirectContext 构造一次 /login 的浏览器导航请求；accept 为空表示不带 Accept 头。
func loginDirectContext(rawQuery string, accept string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/login?"+rawQuery, nil)
	if accept != "" {
		c.Request.Header.Set("Accept", accept)
	}
	return c
}

// 直达只看账号体系：外部身份提供方模式下跳到承担本站身份的那一个提供方；
// 内置账号模式永不直达；非浏览器导航请求也不直达。
func TestLoginDirectTarget(t *testing.T) {
	const browserAccept = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"
	one := []*model.OidcProvider{{Slug: "keycloak", Enabled: true}}
	twoUnmarked := []*model.OidcProvider{{Slug: "keycloak", Enabled: true}, {Slug: "authany", Enabled: true}}
	twoMarked := []*model.OidcProvider{{Slug: "keycloak", Enabled: true}, {Slug: "authany", Enabled: true, FirstParty: true}}

	cases := []struct {
		name      string
		accept    string
		providers []*model.OidcProvider
		external  bool
		wantSlug  string
	}{
		{name: "外部模式 + 唯一启用提供方", accept: browserAccept, providers: one, external: true, wantSlug: "keycloak"},
		{name: "外部模式 + 两个提供方，取标记为本站身份的", accept: browserAccept, providers: twoMarked, external: true, wantSlug: "authany"},
		{name: "外部模式 + 两个提供方都未标记", accept: browserAccept, providers: twoUnmarked, external: true},
		{name: "外部模式 + 无启用的提供方", accept: browserAccept, providers: nil, external: true},
		{name: "内置模式不直达", accept: browserAccept, providers: one},
		{name: "Accept 不含 text/html", accept: "application/json", providers: one, external: true},
		{name: "无 Accept 头", providers: one, external: true},
	}

	oldSystem := config.AccountSystem
	t.Cleanup(func() { config.AccountSystem = oldSystem })

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config.AccountSystem = config.AccountSystemBuiltin
			if tc.external {
				config.AccountSystem = config.AccountSystemExternal
			}

			got := loginDirectTarget(loginDirectContext("", tc.accept), tc.providers)
			if tc.wantSlug == "" {
				if got != nil {
					t.Fatalf("期望不直达，实际命中提供方 %q", got.Slug)
				}
				return
			}
			if got == nil || got.Slug != tc.wantSlug {
				t.Fatalf("期望直达 %q，实际 %+v", tc.wantSlug, got)
			}
		})
	}
}

// 服务端直达读不到界面语言，只能按 Accept-Language 首选项映射，规则与 web/src/i18n/uiLocale.js 一致。
func TestAcceptLanguageUILocale(t *testing.T) {
	cases := map[string]string{
		"zh-CN,zh;q=0.9": "zh-CN",
		"zh-Hans-CN":     "zh-CN",
		"zh-HK":          "zh-HK",
		"zh-TW,zh;q=0.9": "zh-HK",
		"zh-Hant":        "zh-HK",
		"en-US,en;q=0.9": "en",
		"en":             "en",
		"ja-JP":          "ja",
		"ja":             "ja",
		" EN-GB ;q=0.8":  "en",
		"fr-FR,fr;q=0.9": "",
		"zh":             "",
		"":               "",
		"de,en;q=0.9":    "",
		"*":              "",
	}
	for header, want := range cases {
		if got := acceptLanguageUILocale(header); got != want {
			t.Fatalf("acceptLanguageUILocale(%q) = %q，期望 %q", header, got, want)
		}
	}
}
