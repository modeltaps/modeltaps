package middleware

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
)

func init() {
	config.GlobalOption.RegisterValue("AnalyticsCode", config.ScopeAdmin)
}

func setAnalyticsCode(t *testing.T, code string) {
	t.Helper()
	old := config.GlobalOption.Get("AnalyticsCode")
	if err := config.GlobalOption.Set("AnalyticsCode", code); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = config.GlobalOption.Set("AnalyticsCode", old) })
}

func scriptHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

// directive returns the sources of one directive of policy, or nil when it is missing.
func directive(policy, name string) []string {
	for _, part := range strings.Split(policy, ";") {
		fields := strings.Fields(part)
		if len(fields) > 0 && fields[0] == name {
			return fields[1:]
		}
	}
	return nil
}

func pagePolicy(t *testing.T, pages ...string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	raw := make([][]byte, len(pages))
	for i, page := range pages {
		raw[i] = []byte(page)
	}
	r := gin.New()
	r.Use(SecurityHeaders())
	r.Use(WebPageCSP(raw...))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := w.Header().Values("Content-Security-Policy"); len(got) != 1 {
		t.Fatalf("Content-Security-Policy headers = %q, want exactly one", got)
	}
	return w.Header().Get("Content-Security-Policy")
}

// SEC-20: scripts run only from this origin, Turnstile and the hashes of the pages' own inline
// scripts; an injected inline script or event handler has nothing to match.
func TestWebPageCSPScripts(t *testing.T) {
	setAnalyticsCode(t, "")
	index := "<html><head><script>\n  (function () { apply(); })();\n</script>" +
		`<script type="module" src="/assets/index.js"></script></head><body></body></html>`
	home := "<html><body><script>(function(){theme()})()</script><script>drip()</script></body></html>"
	policy := pagePolicy(t, index, home)

	scripts := directive(policy, "script-src")
	want := []string{"'self'", "https://challenges.cloudflare.com",
		scriptHash("\n  (function () { apply(); })();\n"), scriptHash("(function(){theme()})()"), scriptHash("drip()")}
	if strings.Join(scripts, " ") != strings.Join(want, " ") {
		t.Fatalf("script-src = %q, want %q", scripts, want)
	}
	for _, source := range scripts {
		if source == "'unsafe-inline'" || source == "'unsafe-eval'" || source == "*" || source == "https:" {
			t.Fatalf("script-src allows %s: %q", source, policy)
		}
	}
	for name, want := range map[string]string{
		"frame-ancestors": "'none'",
		"object-src":      "'none'",
		"base-uri":        "'self'",
		"default-src":     "'self'",
	} {
		if got := strings.Join(directive(policy, name), " "); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

// The admin's analytics code is added to every page: its script hosts and inline scripts are
// allowed, read again when the code changes.
func TestWebPageCSPAnalyticsCode(t *testing.T) {
	inline := "\n  window.dataLayer = window.dataLayer || [];\n  gtag('config', 'G-TEST');\n"
	setAnalyticsCode(t, `<script async src="https://www.googletagmanager.com/gtag/js?id=G-TEST"></script>`+
		"<script>"+inline+"</script>"+
		`<script defer src="//cloud.umami.is/script.js" data-website-id="x"></script>`+
		`<script src="/local.js"></script>`+
		`<script src="https://evil.example; script-src *"></script>`+
		`<script src="javascript:alert(1)"></script>`)
	scripts := directive(pagePolicy(t), "script-src")
	want := []string{"'self'", "https://challenges.cloudflare.com",
		"https://www.googletagmanager.com", "cloud.umami.is", scriptHash(inline)}
	if strings.Join(scripts, " ") != strings.Join(want, " ") {
		t.Fatalf("script-src = %q, want %q", scripts, want)
	}

	setAnalyticsCode(t, `<script src="https://analytics.example.com:8443/a.js"></script>`)
	scripts = directive(pagePolicy(t), "script-src")
	if !strings.Contains(strings.Join(scripts, " "), "https://analytics.example.com:8443") ||
		strings.Contains(strings.Join(scripts, " "), "googletagmanager") {
		t.Fatalf("script-src after the analytics code changed = %q", scripts)
	}
}

func TestInlineScriptHashesSkipsExternalAndEmpty(t *testing.T) {
	page := `<script src="/a.js">ignored()</script><script></script><script>  </script><p>x</p><script>run()</script>`
	got := inlineScriptHashes([]byte(page))
	if len(got) != 1 || got[0] != scriptHash("run()") {
		t.Fatalf("inlineScriptHashes = %q, want only run()", got)
	}
}
