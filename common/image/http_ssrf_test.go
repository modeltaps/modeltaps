package image

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strconv"
	"sync"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/safefetch"
)

func serverPort(t *testing.T, srv *httptest.Server) uint16 {
	t.Helper()
	u, _ := url.Parse(srv.URL)
	p, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	return uint16(p)
}

func pngServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n"))
	}))
}

func TestRequestFileRejectsPrivateAddress(t *testing.T) {
	srv := pngServer()
	defer srv.Close()

	if _, err := RequestFile(srv.URL, "base64"); err == nil {
		t.Fatal("expected loopback image url to be rejected")
	}
	for _, u := range []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://[::ffff:127.0.0.1]/",
		"file:///etc/passwd",
		"http://user:pass@example.com/",
	} {
		if _, err := RequestFile(u, "base64"); err == nil {
			t.Fatalf("expected %s to be rejected", u)
		}
	}
}

func TestRequestFileAllowsPublicAndBlocksRedirectToPrivate(t *testing.T) {
	internal := pngServer()
	defer internal.Close()
	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, internal.URL, http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n"))
	}))
	defer public.Close()

	// 只把 public 服务器当作「公网」，internal 仍按私网拒绝
	publicPort := serverPort(t, public)
	defer safefetch.SetAllowAddrForTest(func(ap netip.AddrPort) bool { return ap.Port() == publicPort })()

	resp, err := RequestFile(public.URL+"/img.png", "base64")
	if err != nil {
		t.Fatalf("public image should be fetched: %v", err)
	}
	resp.Body.Close()

	if _, err := RequestFile(public.URL+"/redirect", "base64"); err == nil {
		t.Fatal("redirect to a private address should be rejected")
	}
}

// connectOnlyProxy is a minimal http proxy that serves CONNECT only and records the targets it
// was asked for.
func connectOnlyProxy(t *testing.T) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var targets []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		targets = append(targets, r.Host)
		mu.Unlock()
		if r.Method != http.MethodConnect {
			http.Error(w, "connect only", http.StatusMethodNotAllowed)
			return
		}
		upstream, err := net.Dial("tcp", r.Host)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			_ = upstream.Close()
			return
		}
		_, _ = conn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
		go func() { _, _ = io.Copy(conn, upstream); _ = conn.Close() }()
		go func() { _, _ = io.Copy(upstream, conn); _ = upstream.Close() }()
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), targets...)
	}
}

// SEC-65: with ChatImageRequestProxy the gateway resolves and checks the image host itself and
// asks the proxy for the checked IP, so the proxy cannot resolve the name again to a private
// address (DNS rebinding).
func TestRequestFileThroughProxyPinsCheckedIP(t *testing.T) {
	image := pngServer()
	defer image.Close()
	internal := pngServer()
	defer internal.Close()
	imagePort := serverPort(t, image)
	defer safefetch.SetAllowAddrForTest(func(ap netip.AddrPort) bool {
		return ap == netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), imagePort)
	})()
	answers := map[string]string{"img.test": "127.0.0.1", "internal.test": "127.0.0.1", "meta.test": "169.254.169.254"}
	defer safefetch.SetLookupForTest(func(_ context.Context, host string) ([]netip.Addr, error) {
		a, ok := answers[host]
		if !ok {
			return nil, errors.New("no such host")
		}
		return []netip.Addr{netip.MustParseAddr(a)}, nil
	})()
	proxy, seen := connectOnlyProxy(t)
	origProxy, origWorker := config.ChatImageRequestProxy, config.CFWorkerImageUrl
	config.ChatImageRequestProxy, config.CFWorkerImageUrl = proxy.URL, ""
	defer func() { config.ChatImageRequestProxy, config.CFWorkerImageUrl = origProxy, origWorker }()

	resp, err := RequestFile("http://img.test:"+strconv.Itoa(int(imagePort))+"/x.png", "base64")
	if err != nil {
		t.Fatalf("public image through the proxy: %v", err)
	}
	resp.Body.Close()
	want := "127.0.0.1:" + strconv.Itoa(int(imagePort))
	if got := seen(); len(got) != 1 || got[0] != want {
		t.Fatalf("proxy was asked for %v, want [%s]", got, want)
	}

	for _, u := range []string{
		"http://internal.test:" + strconv.Itoa(int(serverPort(t, internal))) + "/",
		"http://meta.test/latest/meta-data/",
		"http://169.254.169.254/latest/meta-data/",
	} {
		if resp, err := RequestFile(u, "base64"); err == nil {
			resp.Body.Close()
			t.Fatalf("%s: expected refusal", u)
		}
	}
	if got := seen(); len(got) != 1 {
		t.Fatalf("proxy was asked for %v after refusals", got)
	}
}
