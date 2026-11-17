package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// setTrustedProxies sets trusted_proxies for one test.
func setTrustedProxies(t *testing.T, entries ...string) {
	t.Helper()
	viper.Set("trusted_proxies", entries)
	t.Cleanup(func() { viper.Set("trusted_proxies", nil) })
}

// newProxiedContext is a request from peer through an engine that trusts trusted (as main.go
// configures it from trusted_proxies), carrying X-Forwarded-For: xff when xff is not empty.
func newProxiedContext(peer string, trusted []string, xff string) *gin.Context {
	w := httptest.NewRecorder()
	c, engine := gin.CreateTestContext(w)
	_ = engine.SetTrustedProxies(trusted)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = peer + ":12345"
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	c.Request = req
	return c
}

func uniqueLoopback() string {
	n := atomic.AddInt64(&rateLimitTestSeq, 1)
	return fmt.Sprintf("127.8.%d.%d", (n>>8)&0xFF, n&0xFF)
}

func TestRateLimitIPKeyGroupsIPv6By64(t *testing.T) {
	cases := map[string]string{
		"203.0.113.7":                  "203.0.113.7",
		"2001:db8:1:2:aaaa:bbbb:cccc:1": "2001:db8:1:2::/64",
		"2001:db8:1:2::ffff":           "2001:db8:1:2::/64",
		"2001:db8:1:3::1":              "2001:db8:1:3::/64",
		"not-an-ip":                    "not-an-ip",
	}
	for in, want := range cases {
		if got := rateLimitIPKey(in); got != want {
			t.Errorf("rateLimitIPKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// SEC-87: rotating addresses inside one IPv6 /64 must not open fresh buckets.
func TestRateLimitIPv6AddressesShareTheir64(t *testing.T) {
	mark := uniqueMark(t)
	n := atomic.AddInt64(&rateLimitTestSeq, 1)
	prefix := fmt.Sprintf("2001:db8:%x:%x", (n>>16)&0xFFFF, n&0xFFFF)
	call := func(host string) *gin.Context {
		c, _ := newRateLimitContext("[" + prefix + host + "]")
		memoryLimitIP(c, 2, 60, mark)
		return c
	}
	if c := call("::1"); c.IsAborted() {
		t.Fatal("first request: expected allow")
	}
	if c := call("::2"); c.IsAborted() {
		t.Fatal("second request: expected allow")
	}
	if c := call(":abcd::3"); !c.IsAborted() {
		t.Fatal("a third address in the same /64 must share the bucket")
	}
	// another /64 has a bucket of its own
	c, _ := newRateLimitContext("[2001:db8:ffff:ffff::1]")
	memoryLimitIP(c, 2, 60, mark)
	if c.IsAborted() {
		t.Fatal("another /64 must not share the bucket")
	}
}

// SEC-87: behind a trusted same-host proxy that passes the client's X-Forwarded-For through, a
// forged 127.0.0.1 must not get the loopback exemption.
func TestRateLimitForgedLoopbackThroughTrustedProxyIsLimited(t *testing.T) {
	if config.RedisEnabled {
		t.Skip("requires memory rate limit path (redis disabled)")
	}
	setTrustedProxies(t, "127.0.0.0/8")
	handler := DownloadRateLimit()
	forged := uniqueLoopback()
	call := func() *gin.Context {
		c := newProxiedContext("127.0.0.1", []string{"127.0.0.0/8"}, forged)
		handler(c)
		return c
	}
	if c := call(); c.ClientIP() != forged {
		t.Fatalf("precondition: ClientIP = %q, want forged %q", c.ClientIP(), forged)
	}
	for i := 1; i < DownloadRateLimitNum; i++ {
		if c := call(); c.IsAborted() {
			t.Fatalf("request %d/%d: expected allow, got aborted", i+1, DownloadRateLimitNum)
		}
	}
	if c := call(); !c.IsAborted() {
		t.Fatal("a loopback client reported by a trusted proxy must be rate limited")
	}
}

// A direct call from the host itself (no forwarding header) keeps its exemption, also with
// trusted_proxies configured.
func TestRateLimitDirectLoopbackStillExempt(t *testing.T) {
	setTrustedProxies(t, "127.0.0.0/8")
	handler := DownloadRateLimit()
	for i := 0; i < DownloadRateLimitNum*3; i++ {
		c := newProxiedContext("127.0.0.1", []string{"127.0.0.0/8"}, "")
		handler(c)
		if c.IsAborted() {
			t.Fatalf("direct loopback request %d: expected exemption", i+1)
		}
	}
}

// A same-host proxy left out of trusted_proxies makes every client look like loopback; the
// requests keep passing (one shared bucket would lock the whole site out) and a warning is logged.
func TestRateLimitUntrustedLocalProxyPassesAndWarns(t *testing.T) {
	setTrustedProxies(t)
	untrustedLocalProxyWarned.Store(0)
	handler := DownloadRateLimit()
	for i := 0; i < DownloadRateLimitNum*3; i++ {
		c := newProxiedContext("127.0.0.1", nil, "198.51.100.9")
		handler(c)
		if c.IsAborted() {
			t.Fatalf("request %d: expected pass", i+1)
		}
	}
	if untrustedLocalProxyWarned.Load() == 0 {
		t.Fatal("expected a warning about the proxy missing from trusted_proxies")
	}
}

func TestFromTrustedProxy(t *testing.T) {
	setTrustedProxies(t, "10.0.0.0/8", "192.0.2.1", "2001:db8::/32")
	for peer, want := range map[string]bool{
		"10.1.2.3":      true,
		"192.0.2.1":     true,
		"192.0.2.2":     false,
		"[2001:db8::5]": true,
		"203.0.113.1":   false,
	} {
		c, _ := newRateLimitContext(peer)
		if got := FromTrustedProxy(c); got != want {
			t.Errorf("FromTrustedProxy(%s) = %v, want %v", peer, got, want)
		}
	}

	// like gin's SetTrustedProxies, one bad entry means no proxy is trusted
	setTrustedProxies(t, "10.0.0.0/8", "bogus")
	c, _ := newRateLimitContext("10.1.2.3")
	if FromTrustedProxy(c) {
		t.Fatal("an invalid trusted_proxies entry must leave no proxy trusted")
	}
}
