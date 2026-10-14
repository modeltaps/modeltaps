package controller

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

// oauthErrorDescriptionMaxRunes is how much of the callback's error_description is kept.
const oauthErrorDescriptionMaxRunes = 200

// generateNonce returns a random nonce for the callback page CSP. It returns "" on failure, which
// callers must treat as a hard error rather than fall back to a predictable value.
func generateNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

// truncateErrorDescription only truncates; HTML escaping happens once, when the page is rendered.
func truncateErrorDescription(input string) string {
	runes := []rune(input)
	if len(runes) > oauthErrorDescriptionMaxRunes {
		return string(runes[:oauthErrorDescriptionMaxRunes])
	}
	return input
}

// generateRandomProjectID returns a placeholder project ID for when detection fails,
// formatted as projects/random-{hex8}/locations/global.
func generateRandomProjectID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// Fall back to 4 bytes of the timestamp.
		n := uint32(time.Now().UnixNano() & 0xFFFFFFFF)
		b = []byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
	}
	return fmt.Sprintf("projects/random-%x/locations/global", b)
}

// oauthOpenerOrigin returns the console origin that started the authorization (the start
// request's Origin, or its Referer when Origin is missing). The callback page posts the
// credentials only to this origin. Without a valid http(s) origin it returns "", the page does
// not postMessage and the console polls the status endpoint instead.
func oauthOpenerOrigin(c *gin.Context) string {
	for _, raw := range []string{c.GetHeader("Origin"), c.GetHeader("Referer")} {
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			continue
		}
		return u.Scheme + "://" + u.Host
	}
	return ""
}

// jsStringLiteral encodes any string as a JS literal that is safe inside <script>: json.Marshal
// adds the quotes and escapes < > & by default, which also covers </script>.
func jsStringLiteral(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}

// writeOAuthResultPage writes the callback page with a CSP that only allows the nonce script,
// plus anti-framing headers.
func writeOAuthResultPage(c *gin.Context, nonce, page string) {
	c.Header("Content-Security-Policy", fmt.Sprintf("default-src 'none'; script-src 'nonce-%s'; style-src 'unsafe-inline'; img-src 'self'", nonce))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Frame-Options", "DENY")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, page)
}
