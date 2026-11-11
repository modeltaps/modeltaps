package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
)

func init() {
	for _, key := range []string{"HomePageContent", "Footer", "SystemName", "DocsLink", "ServerAddress"} {
		config.GlobalOption.RegisterValue(key, config.ScopeAdmin)
	}
}

func setOption(t *testing.T, key, value string) {
	t.Helper()
	old := config.GlobalOption.Get(key)
	if err := config.GlobalOption.Set(key, value); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = config.GlobalOption.Set(key, old) })
}

const testHomePage = `<title>__MT_BRAND__</title><link href="__MT_ORIGIN__/"><b class="__MT_WORDMARK_CLASS__">__MT_WORDMARK__</b>` +
	`<!--mt:docs--><a href="__MT_DOCS_URL__">Docs</a><!--/mt:docs--><i>LANG</i>`

func setupStaticHome() *gin.Engine {
	gin.SetMode(gin.TestMode)
	pages := map[string]string{}
	for _, lng := range homeLanguages {
		pages[lng] = strings.Replace(testHomePage, "LANG", lng, 1)
	}
	r := gin.New()
	registerStaticHome(r, pages, func(c *gin.Context) { c.String(http.StatusOK, "spa") })
	return r
}

func getHome(r *gin.Engine, target string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestStaticHomeLanguage(t *testing.T) {
	r := setupStaticHome()
	cases := []struct {
		name    string
		target  string
		headers map[string]string
		want    string
	}{
		{"no hints falls back to English", "/", nil, "en_US"},
		{"Accept-Language", "/", map[string]string{"Accept-Language": "ja,en;q=0.8"}, "ja_JP"},
		{"Accept-Language quality order", "/", map[string]string{"Accept-Language": "en;q=0.3, zh-TW;q=0.9"}, "zh_HK"},
		{"unsupported tags skipped", "/", map[string]string{"Accept-Language": "fr-FR, de;q=0.9, zh;q=0.5"}, "zh_CN"},
		{"cookie beats Accept-Language", "/", map[string]string{"Accept-Language": "ja", "Cookie": "appLanguage=zh_HK"}, "zh_HK"},
		{"query beats cookie", "/?lang=zh_CN", map[string]string{"Cookie": "appLanguage=ja_JP"}, "zh_CN"},
		{"invalid query ignored", "/?lang=xx", map[string]string{"Accept-Language": "ja"}, "ja_JP"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := getHome(r, tc.target, tc.headers)
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "<i>"+tc.want+"</i>") {
				t.Fatalf("got %d %q, want page %s", w.Code, w.Body.String(), tc.want)
			}
		})
	}
}

func TestStaticHomeQueryRemembersLanguage(t *testing.T) {
	r := setupStaticHome()
	w := getHome(r, "/?lang=ja_JP", nil)
	if got := w.Header().Get("Set-Cookie"); !strings.Contains(got, "appLanguage=ja_JP") {
		t.Fatalf("Set-Cookie = %q, want appLanguage=ja_JP", got)
	}
	if w := getHome(r, "/", nil); w.Header().Get("Set-Cookie") != "" {
		t.Fatalf("no explicit choice should not set a cookie")
	}
}

func TestStaticHomeFillsSiteValues(t *testing.T) {
	setOption(t, "ServerAddress", "https://gw.example.com/")
	r := setupStaticHome()

	t.Run("default brand, no docs link", func(t *testing.T) {
		setOption(t, "SystemName", "")
		setOption(t, "DocsLink", "")
		body := getHome(r, "/", nil).Body.String()
		want := `<title>Modeltaps</title><link href="https://gw.example.com/"><b class="brand-wordmark">modeltaps</b><i>en_US</i>`
		if body != want {
			t.Fatalf("got %q\nwant %q", body, want)
		}
	})

	t.Run("custom brand is escaped and shown as typed", func(t *testing.T) {
		setOption(t, "SystemName", `Acme <AI>`)
		setOption(t, "DocsLink", `https://docs.example.com/?a=1&b="2"`)
		body := getHome(r, "/", nil).Body.String()
		for _, want := range []string{
			"<title>Acme &lt;AI&gt;</title>",
			`<b class="font-semibold tracking-[-0.01em]">Acme &lt;AI&gt;</b>`,
			`<a href="https://docs.example.com/?a=1&amp;b=&#34;2&#34;">Docs</a>`,
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("body %q lacks %q", body, want)
			}
		}
	})

	t.Run("non-http docs link dropped", func(t *testing.T) {
		setOption(t, "DocsLink", "javascript:alert(1)")
		if body := getHome(r, "/", nil).Body.String(); strings.Contains(body, "Docs") || strings.Contains(body, "javascript") {
			t.Fatalf("docs link should be dropped: %q", body)
		}
	})
}

func TestStaticHomeFallsBackToApp(t *testing.T) {
	r := setupStaticHome()
	for _, key := range []string{"HomePageContent", "Footer"} {
		t.Run(key, func(t *testing.T) {
			setOption(t, key, "custom")
			if body := getHome(r, "/", nil).Body.String(); body != "spa" {
				t.Fatalf("with %s set, got %q, want the app", key, body)
			}
		})
	}

	gin.SetMode(gin.TestMode)
	noPages := gin.New()
	registerStaticHome(noPages, nil, func(c *gin.Context) { c.String(http.StatusOK, "spa") })
	if body := getHome(noPages, "/", nil).Body.String(); body != "spa" {
		t.Fatalf("without prerendered pages, got %q, want the app", body)
	}
}
