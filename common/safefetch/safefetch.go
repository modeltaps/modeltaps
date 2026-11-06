// Package safefetch 为「按管理员配置的地址去外网取资源」提供带 SSRF 防护的 HTTP GET：
// 只允许 http(s)、拒绝私网/回环/链路本地等非公网地址（在拨号时按实际解析出的 IP 校验，
// 可防 DNS rebinding）、每一跳重定向都重新校验、限制响应大小与总超时、不走环境代理。
package safefetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

const (
	DefaultMaxBytes     int64 = 256 << 10
	DefaultTimeout            = 3 * time.Second
	DefaultMaxRedirects       = 3
	userAgent                 = "Mozilla/5.0 (compatible; ModeltapsIconFetcher/1.0)"
)

var (
	ErrBadURL          = errors.New("safefetch: only absolute http(s) urls are allowed")
	ErrForbiddenAddr   = errors.New("safefetch: destination address is not public")
	ErrTooLarge        = errors.New("safefetch: response body exceeds size limit")
	ErrTooManyRedirect = errors.New("safefetch: too many redirects")
)

// Options 控制单次抓取；零值字段取默认值。
type Options struct {
	MaxBytes     int64
	Timeout      time.Duration
	MaxRedirects int
	Accept       string
}

// Result 是一次成功抓取（2xx）的结果。
type Result struct {
	FinalURL    *url.URL
	ContentType string
	Body        []byte
}

// allowAddr 判断拨号目标是否放行；测试可替换以放行 httptest 的回环地址。
var allowAddr = func(addr netip.AddrPort) bool { return IsPublicAddr(addr.Addr()) }

// Fetch 以 GET 抓取 rawURL，非 2xx 状态返回错误。
func Fetch(ctx context.Context, rawURL string, opts Options) (*Result, error) {
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = DefaultMaxBytes
	}
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultTimeout
	}
	if opts.MaxRedirects <= 0 {
		opts.MaxRedirects = DefaultMaxRedirects
	}
	u, err := checkURL(rawURL)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	if opts.Accept != "" {
		req.Header.Set("Accept", opts.Accept)
	}
	resp, err := newClient(opts).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("safefetch: unexpected status %d", resp.StatusCode)
	}
	if resp.ContentLength > opts.MaxBytes {
		return nil, ErrTooLarge
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, opts.MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > opts.MaxBytes {
		return nil, ErrTooLarge
	}
	return &Result{FinalURL: resp.Request.URL, ContentType: resp.Header.Get("Content-Type"), Body: body}, nil
}

func checkURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return nil, ErrBadURL
	}
	return u, nil
}

func portOf(u *url.URL) uint16 {
	if p := u.Port(); p != "" {
		var n uint16
		if _, err := fmt.Sscanf(p, "%d", &n); err == nil {
			return n
		}
	}
	if u.Scheme == "https" {
		return 443
	}
	return 80
}

func newClient(opts Options) *http.Client {
	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            GuardedDialer(opts.Timeout).DialContext,
		TLSHandshakeTimeout:    opts.Timeout,
		ResponseHeaderTimeout:  opts.Timeout,
		DisableKeepAlives:      true,
		MaxResponseHeaderBytes: 32 << 10,
	}
	return &http.Client{
		Transport:     transport,
		Timeout:       opts.Timeout,
		CheckRedirect: CheckRedirect(opts.MaxRedirects),
	}
}

// CheckRedirect 返回一个 http.Client.CheckRedirect：限制跳数，且每一跳只允许 http(s)、
// 拒绝字面量为非公网 IP 的目标（域名由 GuardedDialer 或 CheckHost 在拨号/解析时再校验）。
func CheckRedirect(maxRedirects int) func(*http.Request, []*http.Request) error {
	if maxRedirects <= 0 {
		maxRedirects = DefaultMaxRedirects
	}
	return func(req *http.Request, via []*http.Request) error {
		if len(via) > maxRedirects {
			return ErrTooManyRedirect
		}
		if _, err := checkURL(req.URL.String()); err != nil {
			return err
		}
		if ip, err := netip.ParseAddr(req.URL.Hostname()); err == nil && !allowAddr(netip.AddrPortFrom(ip.Unmap(), portOf(req.URL))) {
			return ErrForbiddenAddr
		}
		return nil
	}
}

// CheckURL 校验 rawURL 是绝对 http(s) 地址且不含用户信息。
func CheckURL(rawURL string) (*url.URL, error) { return checkURL(rawURL) }

// CheckHost 解析 u 的主机名，任一解析结果不是公网地址即拒绝。
// 用于请求经代理发出、拨号器看不到真实目标的场景；它无法防御 DNS rebinding，
// 能直连时应优先用 GuardedDialer。
func CheckHost(ctx context.Context, u *url.URL) error {
	host := u.Hostname()
	port := portOf(u)
	if ip, err := netip.ParseAddr(host); err == nil {
		if !allowAddr(netip.AddrPortFrom(ip.Unmap(), port)) {
			return ErrForbiddenAddr
		}
		return nil
	}
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return err
	}
	if len(addrs) == 0 {
		return ErrForbiddenAddr
	}
	for _, a := range addrs {
		if !allowAddr(netip.AddrPortFrom(a.Unmap(), port)) {
			return ErrForbiddenAddr
		}
	}
	return nil
}

// GuardedDialer 返回在拨号时按实际解析出的 IP 拒绝非公网地址的拨号器，供需要自建 Transport
// （流式下载、长超时等）的调用方复用 safefetch 的 SSRF 防护；可防 DNS rebinding。
func GuardedDialer(timeout time.Duration) *net.Dialer {
	return &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			ap, err := netip.ParseAddrPort(address)
			if err != nil || !allowAddr(netip.AddrPortFrom(ap.Addr().Unmap(), ap.Port())) {
				return ErrForbiddenAddr
			}
			return nil
		},
	}
}

// SetAllowAddrForTest 替换地址放行规则（测试用，放行 httptest 的回环地址），返回恢复函数。
func SetAllowAddrForTest(f func(netip.AddrPort) bool) (restore func()) {
	orig := allowAddr
	allowAddr = f
	return func() { allowAddr = orig }
}
