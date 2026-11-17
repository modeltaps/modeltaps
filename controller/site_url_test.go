package controller

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// trustRequestPeer makes the request's TCP peer one of trusted_proxies for the test.
func trustRequestPeer(t *testing.T, req *http.Request) {
	t.Helper()
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		t.Fatal(err)
	}
	viper.Set("trusted_proxies", []string{host})
	t.Cleanup(func() { viper.Set("trusted_proxies", nil) })
}

func siteURLFor(t *testing.T, req *http.Request) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	return publicSiteURL(c)
}

// SEC-88: without a configured address, X-Forwarded-Proto is read only from a trusted proxy and
// only as http or https, so it cannot put another origin into the documents' base URL.
func TestPublicSiteURLForwardedProto(t *testing.T) {
	oldAddr := config.ServerAddress
	config.ServerAddress = ""
	t.Cleanup(func() { config.ServerAddress = oldAddr })

	newReq := func(proto string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/llms.txt", nil)
		req.Host = "gateway.example.org"
		req.Header.Set("X-Forwarded-Proto", proto)
		return req
	}

	// untrusted peer: the header is ignored
	if got := siteURLFor(t, newReq("https")); got != "http://gateway.example.org" {
		t.Fatalf("untrusted peer: got %q, want the header ignored", got)
	}

	req := newReq("https://evil.example/p?x=")
	trustRequestPeer(t, req)
	if got := siteURLFor(t, req); got != "http://gateway.example.org" {
		t.Fatalf("injected scheme: got %q, want http://gateway.example.org", got)
	}
	for proto, want := range map[string]string{
		"https":       "https://gateway.example.org",
		"HTTPS, http": "https://gateway.example.org",
		"http":        "http://gateway.example.org",
		"javascript":  "http://gateway.example.org",
	} {
		if got := siteURLFor(t, newReq(proto)); got != want {
			t.Errorf("trusted peer, X-Forwarded-Proto %q: got %q, want %q", proto, got, want)
		}
	}
}

func TestSiteURLConfigured(t *testing.T) {
	oldAddr := config.ServerAddress
	t.Cleanup(func() { config.ServerAddress = oldAddr })
	for addr, want := range map[string]bool{
		"":                         false,
		defaultServerAddress:       false,
		defaultServerAddress + "/": false,
		"https://api.example.com":  true,
	} {
		config.ServerAddress = addr
		if got := SiteURLConfigured(); got != want {
			t.Errorf("SiteURLConfigured(%q) = %v, want %v", addr, got, want)
		}
	}
}
