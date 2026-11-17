package safefetch

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/net/proxy"
)

// lookupNetIP resolves a host name; tests replace it to simulate DNS answers.
var lookupNetIP = func(ctx context.Context, host string) ([]netip.Addr, error) {
	return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
}

// ResolvePublic resolves host and returns the address to connect to on port. Every address the
// name resolves to must be public: a name whose answers mix public and private addresses is
// refused, whichever of them a later lookup would pick.
func ResolvePublic(ctx context.Context, host string, port uint16) (netip.AddrPort, error) {
	if ip, err := netip.ParseAddr(host); err == nil {
		ap := netip.AddrPortFrom(ip.Unmap(), port)
		if !allowAddr(ap) {
			return netip.AddrPort{}, ErrForbiddenAddr
		}
		return ap, nil
	}
	addrs, err := lookupNetIP(ctx, host)
	if err != nil {
		return netip.AddrPort{}, err
	}
	if len(addrs) == 0 {
		return netip.AddrPort{}, ErrForbiddenAddr
	}
	for _, a := range addrs {
		if !allowAddr(netip.AddrPortFrom(a.Unmap(), port)) {
			return netip.AddrPort{}, ErrForbiddenAddr
		}
	}
	return netip.AddrPortFrom(addrs[0].Unmap(), port), nil
}

// NewGuardedProxyTransport returns a Transport that sends every request through the proxy at
// proxyAddr (http, https, socks5 or socks5h) while keeping the SSRF check of GuardedDialer: it
// resolves the request host itself, refuses non-public addresses and asks the proxy for a tunnel
// to the checked IP. The proxy never resolves the name again, so a name that answers with a
// public address for the check and a private one for the connection (DNS rebinding) cannot reach
// the proxy's network. Redirects dial again and are checked again.
//
// Every target, plain http included, goes through a tunnel (CONNECT or SOCKS) so the request keeps
// its own Host header; an http(s) proxy must therefore allow CONNECT to the target port. The proxy
// address itself comes from the administrator and is not checked. dialTimeout bounds the
// connection to the proxy; set the other timeouts and TLS settings on the returned Transport.
func NewGuardedProxyTransport(proxyAddr string, dialTimeout time.Duration) (*http.Transport, error) {
	proxyURL, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("error parsing proxy address: %w", err)
	}
	base := &net.Dialer{Timeout: dialTimeout, KeepAlive: 30 * time.Second}

	var tunnel func(ctx context.Context, network, target string) (net.Conn, error)
	switch proxyURL.Scheme {
	case "http", "https":
		if proxyURL.Hostname() == "" {
			return nil, fmt.Errorf("proxy address has no host: %s", proxyAddr)
		}
		tunnel = func(ctx context.Context, _, target string) (net.Conn, error) {
			return dialConnectTunnel(ctx, base, proxyURL, target)
		}
	case "socks5", "socks5h":
		socks, err := proxy.FromURL(proxyURL, base)
		if err != nil {
			return nil, fmt.Errorf("error creating proxy dialer: %w", err)
		}
		if cd, ok := socks.(proxy.ContextDialer); ok {
			tunnel = cd.DialContext
		} else {
			tunnel = func(_ context.Context, network, target string) (net.Conn, error) {
				return socks.Dial(network, target)
			}
		}
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s", proxyURL.Scheme)
	}

	return &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, portText, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			port, err := strconv.ParseUint(portText, 10, 16)
			if err != nil {
				return nil, ErrBadURL
			}
			target, err := ResolvePublic(ctx, host, uint16(port))
			if err != nil {
				return nil, err
			}
			return tunnel(ctx, network, target.String())
		},
	}, nil
}

// dialConnectTunnel opens a CONNECT tunnel to target (an IP and port) through an http or https
// proxy.
func dialConnectTunnel(ctx context.Context, base *net.Dialer, proxyURL *url.URL, target string) (net.Conn, error) {
	proxyHost := proxyURL.Host
	if proxyURL.Port() == "" {
		port := "80"
		if proxyURL.Scheme == "https" {
			port = "443"
		}
		proxyHost = net.JoinHostPort(proxyURL.Hostname(), port)
	}
	conn, err := base.DialContext(ctx, "tcp", proxyHost)
	if err != nil {
		return nil, err
	}
	if proxyURL.Scheme == "https" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: proxyURL.Hostname()})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return nil, err
		}
		conn = tlsConn
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Unix(1, 0)) })

	req := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: target},
		Host:   target,
		Header: make(http.Header),
	}
	if user := proxyURL.User; user != nil {
		password, _ := user.Password()
		req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user.Username()+":"+password)))
	}
	br := bufio.NewReader(conn)
	err = req.Write(conn)
	var resp *http.Response
	if err == nil {
		resp, err = http.ReadResponse(br, req)
	}
	if !stop() && err == nil {
		err = ctx.Err()
	}
	if err == nil && resp.StatusCode != http.StatusOK {
		err = errors.New("proxy refused the tunnel: " + resp.Status)
	}
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	if br.Buffered() > 0 {
		return &bufferedConn{Conn: conn, r: br}, nil
	}
	return conn, nil
}

// bufferedConn returns the bytes the proxy sent after its CONNECT response before reading on.
type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.r.Read(p) }

// SetLookupForTest replaces the host name lookup (for tests that simulate DNS answers) and
// returns a function that restores it.
func SetLookupForTest(f func(ctx context.Context, host string) ([]netip.Addr, error)) (restore func()) {
	orig := lookupNetIP
	lookupNetIP = f
	return func() { lookupNetIP = orig }
}
