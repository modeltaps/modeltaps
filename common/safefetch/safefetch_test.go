package safefetch

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
)

func serverPort(t *testing.T, s *httptest.Server) uint16 {
	t.Helper()
	u, _ := url.Parse(s.URL)
	return portOf(u)
}

// allowOnlyPort 仅放行指定回环端口，模拟「该测试服务器是公网地址」。
func allowOnlyPort(t *testing.T, port uint16) {
	t.Helper()
	orig := allowAddr
	allowAddr = func(ap netip.AddrPort) bool { return ap.Port() == port }
	t.Cleanup(func() { allowAddr = orig })
}

func TestIsPublicAddr(t *testing.T) {
	blocked := []string{"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.1", "169.254.169.254",
		"100.64.0.1", "0.0.0.0", "::1", "::", "fe80::1", "fc00::1", "::ffff:127.0.0.1", "::ffff:10.0.0.1",
		"64:ff9b::a00:1", "224.0.0.1", "255.255.255.255", "::127.0.0.1", "fec0::1", "2001::1"}
	for _, s := range blocked {
		if IsPublicAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s should be blocked", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if !IsPublicAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s should be public", s)
		}
	}
}

func TestFetchRejectsPrivateAddress(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("x")) }))
	defer srv.Close()
	_, err := Fetch(context.Background(), srv.URL, Options{})
	if !errors.Is(err, ErrForbiddenAddr) {
		t.Fatalf("want ErrForbiddenAddr, got %v", err)
	}
	if _, err := Fetch(context.Background(), "http://localhost:1/", Options{}); !errors.Is(err, ErrForbiddenAddr) {
		t.Fatalf("localhost: want ErrForbiddenAddr, got %v", err)
	}
}

func TestFetchRejectsBadURL(t *testing.T) {
	for _, u := range []string{"file:///etc/passwd", "gopher://x/", "/relative", "http://user:pw@example.com/"} {
		if _, err := Fetch(context.Background(), u, Options{}); !errors.Is(err, ErrBadURL) {
			t.Errorf("%s: want ErrBadURL, got %v", u, err)
		}
	}
}

func TestFetchRejectsRedirectToPrivate(t *testing.T) {
	private := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("secret")) }))
	defer private.Close()
	for _, target := range []string{private.URL + "/", strings.Replace(private.URL, "127.0.0.1", "localhost", 1) + "/", "http://169.254.169.254/latest/meta-data"} {
		public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target, http.StatusFound)
		}))
		allowOnlyPort(t, serverPort(t, public))
		_, err := Fetch(context.Background(), public.URL, Options{})
		public.Close()
		if !errors.Is(err, ErrForbiddenAddr) {
			t.Errorf("redirect to %s: want ErrForbiddenAddr, got %v", target, err)
		}
	}
}

func TestFetchRedirectLimitAndSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/loop" {
			http.Redirect(w, r, "/loop", http.StatusFound)
			return
		}
		if r.URL.Path == "/hop" {
			http.Redirect(w, r, "/ok", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nrest"))
	}))
	defer srv.Close()
	allowOnlyPort(t, serverPort(t, srv))
	if _, err := Fetch(context.Background(), srv.URL+"/loop", Options{}); !errors.Is(err, ErrTooManyRedirect) {
		t.Fatalf("want ErrTooManyRedirect, got %v", err)
	}
	res, err := Fetch(context.Background(), srv.URL+"/hop", Options{})
	if err != nil || res.FinalURL.Path != "/ok" || SniffImage(res.Body) != MimePNG {
		t.Fatalf("unexpected result %+v err=%v", res, err)
	}
}

func TestFetchRejectsOversizedBody(t *testing.T) {
	big := bytes.Repeat([]byte("a"), int(DefaultMaxBytes)+1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chunked" {
			w.(http.Flusher).Flush()
		}
		_, _ = w.Write(big)
	}))
	defer srv.Close()
	allowOnlyPort(t, serverPort(t, srv))
	for _, p := range []string{"/declared", "/chunked"} {
		if _, err := Fetch(context.Background(), srv.URL+p, Options{}); !errors.Is(err, ErrTooLarge) {
			t.Errorf("%s: want ErrTooLarge, got %v", p, err)
		}
	}
}

func TestFetchRejectsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	allowOnlyPort(t, serverPort(t, srv))
	if _, err := Fetch(context.Background(), srv.URL, Options{}); err == nil {
		t.Fatal("want error on 404")
	}
}
