package image

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strconv"
	"testing"

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
