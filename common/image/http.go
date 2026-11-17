package image

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/safefetch"
	"github.com/modeltaps/modeltaps/common/utils"
)

const (
	imageFetchTimeout      = 15 * time.Second
	imageFetchMaxRedirects = 3
)

// ImageHttpClients 直连下载用户请求里的图片 URL。URL 由任意 API key 持有者提供，
// 拨号时拒绝私网/回环/链路本地（含云元数据）地址，重定向逐跳复检，防 SSRF。
var ImageHttpClients = &http.Client{
	Transport: &http.Transport{
		Proxy:                 nil,
		DialContext:           safefetch.GuardedDialer(5 * time.Second).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: imageFetchTimeout,
	},
	Timeout:       imageFetchTimeout,
	CheckRedirect: safefetch.CheckRedirect(imageFetchMaxRedirects),
}

// imageProxyBaseClient 是请求经 ChatImageRequestProxy 发往管理员配置的 CF Worker 时的基础 client：
// 拨号目标是管理员配置的代理和 Worker（可能在内网），不套用拨号期校验；用户图片 URL 由 Worker 在公网侧抓取。
var imageProxyBaseClient = &http.Client{
	Transport: &http.Transport{
		DialContext:           utils.NewDialer().DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: imageFetchTimeout,
	},
	Timeout: imageFetchTimeout,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if err := safefetch.CheckRedirect(imageFetchMaxRedirects)(req, via); err != nil {
			return err
		}
		return safefetch.CheckHost(req.Context(), req.URL)
	},
}

// imageProxyClients 按 ChatImageRequestProxy 隔离连接池；该配置可在运行时修改，旧代理 client 闲置后回收。
var imageProxyClients = utils.NewProxyClientPool(func() *http.Client { return imageProxyBaseClient }, 10*time.Minute)

// guardedImageProxy downloads user image URLs through ChatImageRequestProxy. The proxy resolves
// nothing itself: each connection, redirects included, is resolved and checked here and the proxy
// is asked for a tunnel to the checked IP (see safefetch.NewGuardedProxyTransport), so a host
// that resolves to a public address for a check and to a private one afterwards (DNS rebinding)
// cannot reach the proxy's network.
var guardedImageProxy guardedProxyClient

// guardedProxyClient keeps the client of the current proxy address; the setting can change at
// run time, and the client of an old address is dropped.
type guardedProxyClient struct {
	mu     sync.Mutex
	addr   string
	client *http.Client
}

func (g *guardedProxyClient) get(proxyAddr string) *http.Client {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.client != nil && g.addr == proxyAddr {
		return g.client
	}
	if g.client != nil {
		g.client.CloseIdleConnections()
	}
	g.addr, g.client = proxyAddr, newGuardedProxyClient(proxyAddr)
	return g.client
}

func newGuardedProxyClient(proxyAddr string) *http.Client {
	transport, err := safefetch.NewGuardedProxyTransport(proxyAddr, 5*time.Second)
	if err != nil {
		// Never fall back to a direct connection: that would silently bypass the configured proxy.
		return &http.Client{Transport: failingTransport{err: fmt.Errorf("invalid image request proxy: %w", err)}}
	}
	transport.TLSHandshakeTimeout = 5 * time.Second
	transport.ResponseHeaderTimeout = imageFetchTimeout
	return &http.Client{
		Transport:     transport,
		Timeout:       imageFetchTimeout,
		CheckRedirect: safefetch.CheckRedirect(imageFetchMaxRedirects),
	}
}

type failingTransport struct{ err error }

func (f failingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		_ = req.Body.Close()
	}
	return nil, f.err
}

var maxFileSize int64 = 20 * 1024 * 1024 // 20MB

type CFRequest struct {
	Action string `json:"action"`
	APIKey string `json:"api_key"`
	URL    string `json:"url"`
}

type CFResponse struct {
	Status   bool   `json:"status"`
	Message  string `json:"message,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

func RequestFile(url, action string) (*http.Response, error) {
	if _, err := safefetch.CheckURL(url); err != nil {
		return nil, err
	}

	reqUrl := url
	method := http.MethodGet
	var requestBody any
	client := ImageHttpClients

	if config.CFWorkerImageUrl != "" {
		requestBody = &CFRequest{
			Action: action,
			APIKey: config.CFWorkerImageKey,
			URL:    url,
		}
		reqUrl = config.CFWorkerImageUrl
		method = http.MethodPost
		// CF Worker 由管理员配置，请求发往 Worker 本身，目标地址由 Worker 在公网侧抓取
		client = imageProxyClients.Get(config.ChatImageRequestProxy)
	} else if config.ChatImageRequestProxy != "" {
		client = guardedImageProxy.get(config.ChatImageRequestProxy)
	}

	res, err := utils.RequestBuilder(context.Background(), method, reqUrl, requestBody, nil)

	if err != nil {
		return nil, err
	}

	response, err := client.Do(res)
	if err != nil {
		return nil, err
	}

	response.Body = http.MaxBytesReader(nil, response.Body, maxFileSize)

	if response.StatusCode != http.StatusOK && config.CFWorkerImageUrl != "" {
		var cfResp *CFResponse
		err = json.NewDecoder(response.Body).Decode(&cfResp)
		if err != nil {
			return nil, err
		}
		return nil, errors.New(cfResp.Message)
	}

	return response, err
}
