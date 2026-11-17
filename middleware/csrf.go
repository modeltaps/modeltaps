package middleware

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/telegram"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// csrfExemptRoutes are the routes (gin's FullPath) that take unsafe requests from other sites by
// design: payment gateway notifications and the Telegram webhook. They authenticate by their own
// signature or secret, never by the console session. OAuth callbacks need no entry: providers
// redirect the browser with a GET, which is never checked.
var csrfExemptRoutes = map[string]bool{
	"/api/payment/notify/:uuid": true,
	"/api/user/epay/notify":     true,
	telegram.WebhookPath:        true,
}

// CrossSiteRequestGuard refuses state-changing requests (POST, PUT, PATCH, DELETE) that are signed
// in by the console session cookie but were sent by a page of another site (CSRF). The session
// cookie is SameSite=Strict already; this check also covers browsers that ignore SameSite and
// sibling subdomains, which count as the same site.
//
// A request passes when the browser marks it Sec-Fetch-Site: same-origin, or when its Origin (the
// Referer's origin when Origin is missing) is one of this site's origins: the configured server
// address, frontend_base_url, the request's Host, the X-Forwarded-Host set by a proxy in
// trusted_proxies, or a loopback origin on a loopback connection (the Vite dev server rewrites
// Host when it proxies to the backend). Requests without a signed-in session (sign-in, API and
// management keys in the Authorization header) are not checked: a page of another site cannot
// set that header, and they carry no ambient credential.
func CrossSiteRequestGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !csrfUnsafeMethod(c.Request.Method) || csrfExemptRoutes[c.FullPath()] || !csrfSessionSignedIn(c) {
			c.Next()
			return
		}
		// Browsers set Sec-Fetch-Site themselves and pages cannot override it. "none" means the
		// user started the request (typed or bookmarked), not a page.
		switch c.GetHeader("Sec-Fetch-Site") {
		case "same-origin", "none":
			c.Next()
			return
		}
		origin := requestOrigin(c.Request)
		if origin != "" && csrfAllowedOrigin(c, origin) {
			c.Next()
			return
		}
		message := "Cross-site request refused: the Origin header does not match this site"
		if origin == "" {
			message = "Cross-site request refused: the request has no Origin or Referer header"
		}
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("refused cross-site %s %s from origin %q (host %q)",
			c.Request.Method, c.FullPath(), truncateForLog(origin, 200), truncateForLog(c.Request.Host, 200)))
		common.APIRespondWithCode(c, http.StatusForbidden, common.APICodePermissionDenied, message, nil)
		c.Abort()
	}
}

func csrfUnsafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	}
	return true
}

// csrfSessionSignedIn reports whether the request's cookie session signs a user in, the session
// authHelper prefers over any Authorization header.
func csrfSessionSignedIn(c *gin.Context) bool {
	if _, ok := c.Get(sessions.DefaultKey); !ok {
		return false
	}
	session := sessions.Default(c)
	return session.Get("id") != nil || session.Get("username") != nil
}

// requestOrigin returns the Origin header, or the origin of the Referer when Origin is missing.
// "null" (sandboxed frames, opaque origins) is returned as is and never matches.
func requestOrigin(r *http.Request) string {
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		return origin
	}
	referer := strings.TrimSpace(r.Header.Get("Referer"))
	if referer == "" {
		return ""
	}
	u, err := url.Parse(referer)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "null"
	}
	return u.Scheme + "://" + u.Host
}

func csrfAllowedOrigin(c *gin.Context, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return false
	}
	if (u.Path != "" && u.Path != "/") || u.User != nil || u.RawQuery != "" {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	host := csrfHostKey(u.Host, scheme)

	// The site's configured addresses: scheme and host must both match.
	for _, addr := range []string{config.ServerAddress, viper.GetString("frontend_base_url")} {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		site, err := url.Parse(addr)
		if err != nil || site.Host == "" {
			continue
		}
		siteScheme := strings.ToLower(site.Scheme)
		if siteScheme == scheme && csrfHostKey(site.Host, siteScheme) == host {
			return true
		}
	}

	// The host the request was sent to. The scheme is not compared: behind a proxy that ends TLS
	// the backend cannot tell it.
	if reqHost := strings.TrimSpace(c.Request.Host); reqHost != "" && csrfHostKey(reqHost, scheme) == host {
		return true
	}
	if FromTrustedProxy(c) {
		if fwd := strings.TrimSpace(strings.Split(c.GetHeader("X-Forwarded-Host"), ",")[0]); fwd != "" && csrfHostKey(fwd, scheme) == host {
			return true
		}
	}

	// Development: the Vite dev server forwards the console's requests with Host rewritten to the
	// backend address. A loopback origin only reaches a loopback connection from this machine.
	if csrfIsLoopbackHost(u.Hostname()) {
		if peer := net.ParseIP(c.RemoteIP()); peer != nil && peer.IsLoopback() {
			return true
		}
	}
	return false
}

// csrfHostKey lowercases host and drops the default port of scheme, so "Example.com:443" and
// "example.com" compare equal for https.
func csrfHostKey(host, scheme string) string {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if h, port, err := net.SplitHostPort(host); err == nil {
		if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
			if strings.Contains(h, ":") {
				return "[" + h + "]"
			}
			return h
		}
	}
	return host
}

func csrfIsLoopbackHost(hostname string) bool {
	hostname = strings.ToLower(hostname)
	if hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") {
		return true
	}
	ip := net.ParseIP(hostname)
	return ip != nil && ip.IsLoopback()
}

func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
