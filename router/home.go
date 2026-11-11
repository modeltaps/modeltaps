package router

import (
	"embed"
	"html"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
)

// The static home page: web/scripts/prerender-home.mjs renders it at build time into
// web/build/home/<lang>.html, plain HTML without the app bundle, so search engines and first-time
// visitors get the content at once. It never depends on the login state.
//
// The tokens below are the placeholders the prerender leaves for values only the server knows; keep
// them in sync with web/src/static-home/tokens.js.
const (
	homeTokenBrand    = "__MT_BRAND__"
	homeTokenWordmark = "__MT_WORDMARK__"
	homeTokenFont     = "__MT_WORDMARK_CLASS__"
	homeTokenOrigin   = "__MT_ORIGIN__"
	homeTokenDocsURL  = "__MT_DOCS_URL__"
	homeDocsStart     = "<!--mt:docs-->"
	homeDocsEnd       = "<!--/mt:docs-->"

	defaultBrand = "Modeltaps"
	// same name as the app's localStorage key; the app writes it whenever the user picks a language
	languageCookie = "appLanguage"
)

// homeLanguages are the app's languages, first is the fallback.
var homeLanguages = []string{"en_US", "zh_CN", "zh_HK", "ja_JP"}

// loadHomePages reads the prerendered pages from the embedded build. A build without them (for
// example a web build from before the static home existed) returns nil, and `/` keeps serving the app.
func loadHomePages(buildFS embed.FS) map[string]string {
	pages := make(map[string]string, len(homeLanguages))
	for _, lng := range homeLanguages {
		data, err := buildFS.ReadFile("web/build/home/" + lng + ".html")
		if err != nil {
			return nil
		}
		pages[lng] = string(data)
	}
	return pages
}

// registerStaticHome serves `/`. When the admin replaced the home page (HomePageContent) or set a
// custom footer, the app renders those as before, so the request goes to fallback.
func registerStaticHome(router *gin.Engine, pages map[string]string, fallback gin.HandlerFunc) {
	router.Match([]string{http.MethodGet, http.MethodHead}, "/", func(c *gin.Context) {
		if pages == nil || config.GlobalOption.Get("HomePageContent") != "" || config.GlobalOption.Get("Footer") != "" {
			fallback(c)
			return
		}

		query := c.Query("lang")
		lng := normalizeLanguage(query)
		if lng != "" {
			// an explicit choice on the page is remembered like a choice made in the app
			c.SetSameSite(http.SameSiteLaxMode)
			c.SetCookie(languageCookie, lng, 365*24*3600, "/", "", false, false)
		} else {
			cookie, _ := c.Cookie(languageCookie)
			lng = normalizeLanguage(cookie)
		}
		if lng == "" {
			lng = negotiateLanguage(c.GetHeader("Accept-Language"))
		}

		c.Header("Cache-Control", "no-cache")
		c.Header("Vary", "Accept-Language, Cookie")
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(fillHomePage(pages[lng], siteOrigin(c))))
	})
}

// fillHomePage puts the configured site name, origin and docs link into a prerendered page.
func fillHomePage(page, origin string) string {
	brand := strings.TrimSpace(config.GlobalOption.Get("SystemName"))
	if brand == "" {
		brand = defaultBrand
	}
	// the default brand is written lowercase next to the mark (utils/brand.js brandWordmark)
	// the bundled wordmark face only has the default brand's letters (components/chrome/Logo.jsx BrandLockup)
	wordmark, font := brand, "font-semibold tracking-[-0.01em]"
	if brand == defaultBrand {
		wordmark, font = strings.ToLower(brand), "brand-wordmark"
	}
	// the docs link sits between markers; without a configured link the whole link goes
	docsURL := strings.TrimSpace(config.GlobalOption.Get("DocsLink"))
	if !strings.HasPrefix(docsURL, "https://") && !strings.HasPrefix(docsURL, "http://") {
		docsURL = ""
		if start := strings.Index(page, homeDocsStart); start >= 0 {
			if end := strings.Index(page[start:], homeDocsEnd); end >= 0 {
				page = page[:start] + page[start+end+len(homeDocsEnd):]
			}
		}
	}
	return strings.NewReplacer(
		homeTokenBrand, html.EscapeString(brand),
		homeTokenWordmark, html.EscapeString(wordmark),
		homeTokenFont, font,
		homeTokenOrigin, html.EscapeString(origin),
		homeTokenDocsURL, html.EscapeString(docsURL),
	).Replace(page)
}

// siteOrigin is the absolute origin for canonical and hreflang links: the configured server
// address, or the request's own origin when none is configured.
func siteOrigin(c *gin.Context) string {
	if addr := strings.TrimRight(strings.TrimSpace(config.GlobalOption.Get("ServerAddress")), "/"); addr != "" {
		return addr
	}
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}

// normalizeLanguage maps an app language code or a browser language tag onto one of homeLanguages,
// following the app's toAppLanguage (web/src/i18n/i18n.js); "" when it is not supported.
func normalizeLanguage(tag string) string {
	lower := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tag), "_", "-"))
	switch {
	case lower == "":
		return ""
	case strings.HasPrefix(lower, "zh"):
		for _, hant := range []string{"hant", "-tw", "-hk", "-mo"} {
			if strings.Contains(lower, hant) {
				return "zh_HK"
			}
		}
		return "zh_CN"
	case strings.HasPrefix(lower, "ja"):
		return "ja_JP"
	case strings.HasPrefix(lower, "en"):
		return "en_US"
	}
	return ""
}

// negotiateLanguage picks the supported language the browser ranks highest in Accept-Language,
// falling back to English.
func negotiateLanguage(header string) string {
	type candidate struct {
		lng string
		q   float64
	}
	var candidates []candidate
	for _, part := range strings.Split(header, ",") {
		fields := strings.Split(part, ";")
		lng := normalizeLanguage(fields[0])
		if lng == "" {
			continue
		}
		q := 1.0
		for _, param := range fields[1:] {
			if v, ok := strings.CutPrefix(strings.TrimSpace(param), "q="); ok {
				if parsed, err := strconv.ParseFloat(v, 64); err == nil {
					q = parsed
				}
			}
		}
		if q > 0 {
			candidates = append(candidates, candidate{lng, q})
		}
	}
	if len(candidates) == 0 {
		return homeLanguages[0]
	}
	sort.SliceStable(candidates, func(a, b int) bool { return candidates[a].q > candidates[b].q })
	return candidates[0].lng
}
