package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"golang.org/x/net/html"
)

// SecurityHeaders 添加通用的安全头部
//
// Mounted on /api and on every page of the embedded web app. No page of the site is meant to be
// shown inside a frame (the console frames other sites, never itself), so framing is refused
// outright: X-Frame-Options for older browsers, CSP frame-ancestors for current ones. This keeps
// pages such as /panel/device out of clickjacking frames, including ones on a same-site subdomain.
// Handlers that set a Content-Security-Policy of their own replace this one, as WebPageCSP does
// for the web app's pages; X-Frame-Options still applies to them.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 添加通用的安全头部
		c.Header("X-Frame-Options", "DENY")
		c.Header("Content-Security-Policy", "frame-ancestors 'none'")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")

		// 仅在 HTTPS 环境下注入 HSTS，判定条件与 Cookie Secure 保持一致
		if viper.GetBool("https") || viper.GetString("trusted_header") == "CF-Connecting-IP" {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		c.Next()
	}
}

// SessionSecurity 为会话相关的接口添加额外的安全头部
func SessionSecurity() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 添加会话安全相关的头部
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		// 设置强制防缓存策略
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate, private, max-age=0")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")

		// 检查是否使用 Cloudflare，设置相应的 Vary 头部
		trustedHeader := viper.GetString("trusted_header")
		if trustedHeader == "CF-Connecting-IP" {
			// 对于 Cloudflare，添加额外的 Vary 头部确保不同用户不会共享缓存
			c.Header("Vary", "Cookie, Authorization, X-Requested-With, User-Agent, CF-Connecting-IP")
		} else {
			c.Header("Vary", "Cookie, Authorization, X-Requested-With, User-Agent")
		}

		c.Next()
	}
}

// The Content-Security-Policy of the web app's HTML pages (SEC-20), a second line of defence
// should an XSS ever get through: scripts run only from this origin, from Cloudflare Turnstile and
// from the hosts of the admin's analytics code, and inline scripts only when the server knows
// their hash, so an injected <script> or event handler does not run.
//
// The other directives stay open on purpose. Operators point images, home and about pages,
// payment forms and media at hosts of their choosing, the playground fetches upstream results,
// and an image already reaches any host, so narrowing connect-src would not stop a leak.
// Styles keep 'unsafe-inline': the UI library and toast components insert <style> elements, and
// sanitized operator HTML keeps its style attributes.
const (
	turnstileOrigin = "https://challenges.cloudflare.com"
	webPagePolicy   = "default-src 'self'; " +
		"script-src %s; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src * data: blob:; " +
		"font-src 'self' data:; " +
		"connect-src * data: blob:; " +
		"media-src * data: blob:; " +
		"frame-src *; " +
		"worker-src 'self' blob:; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"form-action *; " +
		"frame-ancestors 'none'"
)

// WebPageCSP sets the Content-Security-Policy of the web app, replacing the frame-ancestors-only
// policy of SecurityHeaders. pages are the HTML pages the server sends as they are (the app's
// index.html and the prerendered home pages); the hashes of their inline scripts are allowed. A
// page must not have placeholders the server fills in inside an inline script, or its hash no
// longer matches.
//
// The analytics code (AnalyticsCode), which the app adds to every page, is read on each request,
// so a saved change applies on the next page load: the origins of its <script src> and the hashes
// of its inline scripts are allowed. Scripts the analytics service loads from other hosts, or
// inline scripts it writes itself (some tag managers do), are blocked and show up as CSP
// violations in the browser console.
func WebPageCSP(pages ...[]byte) gin.HandlerFunc {
	base := []string{"'self'", turnstileOrigin}
	for _, page := range pages {
		base = appendUnique(base, inlineScriptHashes(page)...)
	}
	var cache atomic.Pointer[cspCacheEntry]
	policyFor := func(analyticsCode string) string {
		if entry := cache.Load(); entry != nil && entry.analyticsCode == analyticsCode {
			return entry.policy
		}
		sources := append([]string(nil), base...)
		origins, hashes := analyticsScriptSources(analyticsCode)
		sources = appendUnique(sources, origins...)
		sources = appendUnique(sources, hashes...)
		policy := fmt.Sprintf(webPagePolicy, strings.Join(sources, " "))
		cache.Store(&cspCacheEntry{analyticsCode: analyticsCode, policy: policy})
		return policy
	}
	return func(c *gin.Context) {
		c.Header("Content-Security-Policy", policyFor(config.GlobalOption.Get("AnalyticsCode")))
		c.Next()
	}
}

type cspCacheEntry struct {
	analyticsCode string
	policy        string
}

// inlineScriptHashes returns the CSP hash source of every inline <script> in page, computed over
// the script text as the browser reads it.
func inlineScriptHashes(page []byte) []string {
	var hashes []string
	forEachScript(page, func(src string, text []byte) {
		if src == "" && len(bytes.TrimSpace(text)) > 0 {
			sum := sha256.Sum256(text)
			hashes = appendUnique(hashes, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
		}
	})
	return hashes
}

// cspHost is what a script origin taken from the analytics code may look like; anything else
// (spaces, quotes, semicolons) could change the policy and is dropped.
var cspHost = regexp.MustCompile(`^[A-Za-z0-9.-]+(:[0-9]{1,5})?$`)

// analyticsScriptSources returns the origins of the external scripts in the analytics code and
// the hashes of its inline scripts. A relative or same-origin src needs nothing: 'self' covers it.
func analyticsScriptSources(code string) (origins, hashes []string) {
	if strings.TrimSpace(code) == "" {
		return nil, nil
	}
	forEachScript([]byte(code), func(src string, _ []byte) {
		src = strings.TrimSpace(src)
		if src == "" {
			return
		}
		u, err := url.Parse(src)
		if err != nil || u.Host == "" || !cspHost.MatchString(u.Host) {
			return
		}
		switch strings.ToLower(u.Scheme) {
		case "http", "https":
			origins = appendUnique(origins, strings.ToLower(u.Scheme)+"://"+strings.ToLower(u.Host))
		case "":
			// protocol-relative: a scheme-less source follows the page's scheme
			origins = appendUnique(origins, strings.ToLower(u.Host))
		}
	})
	return origins, inlineScriptHashes([]byte(code))
}

// forEachScript calls fn for every <script> element in page with its src attribute and its text.
func forEachScript(page []byte, fn func(src string, text []byte)) {
	z := html.NewTokenizer(bytes.NewReader(page))
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			if string(name) != "script" {
				continue
			}
			src := ""
			for hasAttr {
				var key, val []byte
				key, val, hasAttr = z.TagAttr()
				if string(key) == "src" {
					src = string(val)
				}
			}
			// A browser reads <script/> as an opening tag and the markup after it as script text;
			// that text is not hashed, so it does not run unless the tag has a src.
			var text []byte
			if tt == html.StartTagToken && z.Next() == html.TextToken {
				text = append(text, z.Text()...)
			}
			fn(src, text)
		}
	}
}

func appendUnique(list []string, items ...string) []string {
	for _, item := range items {
		if !slices.Contains(list, item) {
			list = append(list, item)
		}
	}
	return list
}
