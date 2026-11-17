package safefetch

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"sync"
	"testing"
	"time"
)

// connectProxy is a minimal http proxy that only serves CONNECT and records the targets and
// Proxy-Authorization headers it was asked for.
type connectProxy struct {
	*httptest.Server
	mu      sync.Mutex
	targets []string
	auth    []string
}

func newConnectProxy(t *testing.T) *connectProxy {
	t.Helper()
	p := &connectProxy{}
	p.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		p.targets = append(p.targets, r.Host)
		p.auth = append(p.auth, r.Header.Get("Proxy-Authorization"))
		p.mu.Unlock()
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
		pipe(conn, upstream)
	}))
	t.Cleanup(p.Close)
	return p
}

func (p *connectProxy) seen() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.targets...)
}

func pipe(a, b net.Conn) {
	go func() { _, _ = io.Copy(a, b); _ = a.Close() }()
	go func() { _, _ = io.Copy(b, a); _ = b.Close() }()
}

// socks5Proxy is a minimal no-auth SOCKS5 server that records the CONNECT targets as the client
// sent them (IP or domain name).
type socks5Proxy struct {
	ln      net.Listener
	mu      sync.Mutex
	targets []string
}

func newSocks5Proxy(t *testing.T) *socks5Proxy {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &socks5Proxy{ln: ln}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go p.serve(conn)
		}
	}()
	return p
}

func (p *socks5Proxy) serve(conn net.Conn) {
	buf := make([]byte, 262)
	// greeting: version, method count, methods
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		_ = conn.Close()
		return
	}
	if _, err := io.ReadFull(conn, buf[:buf[1]]); err != nil {
		_ = conn.Close()
		return
	}
	_, _ = conn.Write([]byte{5, 0})
	// request: version, cmd, reserved, address type
	if _, err := io.ReadFull(conn, buf[:4]); err != nil {
		_ = conn.Close()
		return
	}
	var host string
	switch buf[3] {
	case 1:
		_, _ = io.ReadFull(conn, buf[:4])
		host = netip.AddrFrom4([4]byte(buf[:4])).String()
	case 4:
		_, _ = io.ReadFull(conn, buf[:16])
		host = netip.AddrFrom16([16]byte(buf[:16])).String()
	case 3:
		_, _ = io.ReadFull(conn, buf[:1])
		n := int(buf[0])
		_, _ = io.ReadFull(conn, buf[:n])
		host = "name:" + string(buf[:n])
	}
	_, _ = io.ReadFull(conn, buf[:2])
	port := binary.BigEndian.Uint16(buf[:2])
	target := net.JoinHostPort(host, strconv.Itoa(int(port)))
	p.mu.Lock()
	p.targets = append(p.targets, target)
	p.mu.Unlock()
	upstream, err := net.Dial("tcp", target)
	if err != nil {
		_, _ = conn.Write([]byte{5, 4, 0, 1, 0, 0, 0, 0, 0, 0})
		_ = conn.Close()
		return
	}
	_, _ = conn.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
	pipe(conn, upstream)
}

func (p *socks5Proxy) seen() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.targets...)
}

// fakeDNS answers lookups from a table; a host with several entries returns them in turn, one
// per lookup, to simulate a name that rebinds between lookups.
type fakeDNS struct {
	mu      sync.Mutex
	answers map[string][][]netip.Addr
	calls   map[string]int
}

func useFakeDNS(t *testing.T, answers map[string][][]netip.Addr) *fakeDNS {
	t.Helper()
	d := &fakeDNS{answers: answers, calls: map[string]int{}}
	t.Cleanup(SetLookupForTest(func(_ context.Context, host string) ([]netip.Addr, error) {
		d.mu.Lock()
		defer d.mu.Unlock()
		list, ok := d.answers[host]
		if !ok {
			return nil, errors.New("no such host")
		}
		i := d.calls[host]
		d.calls[host]++
		return list[min(i, len(list)-1)], nil
	}))
	return d
}

func addrs(s ...string) []netip.Addr {
	out := make([]netip.Addr, len(s))
	for i, a := range s {
		out[i] = netip.MustParseAddr(a)
	}
	return out
}

// allowLoopbackPort treats only 127.0.0.1 on port as public, so the test servers count as public
// and every other address (other private ranges, other ports) stays refused.
func allowLoopbackPort(t *testing.T, port uint16) {
	t.Helper()
	t.Cleanup(SetAllowAddrForTest(func(ap netip.AddrPort) bool {
		return ap == netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), port)
	}))
}

func guardedClient(t *testing.T, proxyAddr string) *http.Client {
	t.Helper()
	tr, err := NewGuardedProxyTransport(proxyAddr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tr.CloseIdleConnections)
	return &http.Client{Transport: tr, Timeout: 5 * time.Second, CheckRedirect: CheckRedirect(3)}
}

// The proxy is asked for the IP the gateway checked, never for the name, so it cannot resolve
// the name again to a private address (SEC-65).
func TestGuardedProxyTunnelsToCheckedIP(t *testing.T) {
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("host=" + r.Host))
	}))
	defer target.Close()
	port := serverPort(t, target)
	allowLoopbackPort(t, port)
	useFakeDNS(t, map[string][][]netip.Addr{"example.com": {addrs("127.0.0.1")}})
	proxy := newConnectProxy(t)

	client := guardedClient(t, "http://user:secret@"+proxy.Listener.Addr().String())
	tr := client.Transport.(*http.Transport)
	tr.TLSClientConfig = target.Client().Transport.(*http.Transport).TLSClientConfig.Clone()

	resp, err := client.Get("https://example.com:" + strconv.Itoa(int(port)) + "/img.png")
	if err != nil {
		t.Fatalf("fetch through the proxy: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if want := "host=example.com:" + strconv.Itoa(int(port)); string(body) != want {
		t.Fatalf("target saw %q, want %q", body, want)
	}
	want := "127.0.0.1:" + strconv.Itoa(int(port))
	if got := proxy.seen(); len(got) != 1 || got[0] != want {
		t.Fatalf("proxy was asked for %v, want [%s]", got, want)
	}
	if proxy.auth[0] != "Basic dXNlcjpzZWNyZXQ=" {
		t.Fatalf("proxy authorization %q", proxy.auth[0])
	}
}

func TestGuardedProxyRejectsPrivateAnswers(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("secret")) }))
	defer target.Close()
	port := serverPort(t, target)
	allowLoopbackPort(t, port)
	proxy := newConnectProxy(t)
	client := guardedClient(t, proxy.URL)
	p := ":" + strconv.Itoa(int(port))

	useFakeDNS(t, map[string][][]netip.Addr{
		"private.test": {addrs("127.0.0.1")},
		// public for the first lookup, private for any later one
		"rebind.test": {addrs("127.0.0.1"), addrs("10.0.0.1")},
		"mixed.test":  {addrs("127.0.0.1", "169.254.169.254")},
	})
	// "private.test" resolves to the target, but only on its port the test treats as public; use
	// another port so the address counts as private.
	for _, u := range []string{"http://private.test:1/", "http://mixed.test" + p + "/", "http://169.254.169.254/", "http://localhost/"} {
		if resp, err := client.Get(u); err == nil {
			resp.Body.Close()
			t.Fatalf("%s: expected refusal", u)
		} else if !errors.Is(err, ErrForbiddenAddr) && u != "http://localhost/" {
			t.Fatalf("%s: want ErrForbiddenAddr, got %v", u, err)
		}
	}
	if got := proxy.seen(); len(got) != 0 {
		t.Fatalf("proxy was asked for %v after refusals", got)
	}

	// A rebinding name is resolved once per connection, and the connection goes to that answer.
	resp, err := client.Get("http://rebind.test" + p + "/")
	if err != nil {
		t.Fatalf("rebind first answer: %v", err)
	}
	resp.Body.Close()
	if got := proxy.seen(); len(got) != 1 || got[0] != "127.0.0.1"+p {
		t.Fatalf("proxy was asked for %v", got)
	}
}

func TestGuardedProxyChecksRedirects(t *testing.T) {
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("secret")) }))
	defer internal.Close()
	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://internal.test:"+strconv.Itoa(int(serverPort(t, internal)))+"/", http.StatusFound)
	}))
	defer public.Close()
	allowLoopbackPort(t, serverPort(t, public))
	useFakeDNS(t, map[string][][]netip.Addr{"public.test": {addrs("127.0.0.1")}, "internal.test": {addrs("127.0.0.1")}})
	proxy := newConnectProxy(t)

	resp, err := guardedClient(t, proxy.URL).Get("http://public.test:" + strconv.Itoa(int(serverPort(t, public))) + "/")
	if err == nil {
		resp.Body.Close()
		t.Fatal("redirect to a private address should be refused")
	}
	if !errors.Is(err, ErrForbiddenAddr) {
		t.Fatalf("want ErrForbiddenAddr, got %v", err)
	}
	if got := proxy.seen(); len(got) != 1 {
		t.Fatalf("proxy was asked for %v, want only the first hop", got)
	}
}

func TestGuardedProxySocks5UsesCheckedIP(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer target.Close()
	port := serverPort(t, target)
	allowLoopbackPort(t, port)
	useFakeDNS(t, map[string][][]netip.Addr{"img.test": {addrs("127.0.0.1")}, "private.test": {addrs("10.0.0.1")}})

	for _, scheme := range []string{"socks5", "socks5h"} {
		proxy := newSocks5Proxy(t)
		client := guardedClient(t, scheme+"://"+proxy.ln.Addr().String())
		resp, err := client.Get("http://img.test:" + strconv.Itoa(int(port)) + "/")
		if err != nil {
			t.Fatalf("%s: %v", scheme, err)
		}
		resp.Body.Close()
		if resp, err := client.Get("http://private.test:" + strconv.Itoa(int(port)) + "/"); err == nil {
			resp.Body.Close()
			t.Fatalf("%s: private answer accepted", scheme)
		}
		want := "127.0.0.1:" + strconv.Itoa(int(port))
		if got := proxy.seen(); len(got) != 1 || got[0] != want {
			t.Fatalf("%s: proxy was asked for %v, want [%s]", scheme, got, want)
		}
	}
}

func TestGuardedProxyRefusedTunnel(t *testing.T) {
	refusing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "denied", http.StatusForbidden)
	}))
	defer refusing.Close()
	allowLoopbackPort(t, 443)
	useFakeDNS(t, map[string][][]netip.Addr{"img.test": {addrs("127.0.0.1")}})
	if resp, err := guardedClient(t, refusing.URL).Get("http://img.test:443/"); err == nil {
		resp.Body.Close()
		t.Fatal("expected an error when the proxy refuses the tunnel")
	}
	if _, err := NewGuardedProxyTransport("ftp://proxy.example", time.Second); err == nil {
		t.Fatal("unsupported proxy scheme accepted")
	}
}
