package brandicon

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/safefetch"

	"golang.org/x/net/html"
)

const (
	faviconTotalTimeout = 15 * time.Second
	faviconMaxCandidate = 6
)

var ErrFaviconNotFound = errors.New("brandicon: no usable favicon")

// fetchFunc 便于测试替换底层抓取。
var fetchFunc = safefetch.Fetch

// FetchFavicon 按可注册域名抓站点图标：先解析首页 <link rel=apple-touch-icon|icon>，
// 再回落 /apple-touch-icon.png 与 /favicon.ico。返回的数据已按文件头确认类型，SVG 已清洗。
func FetchFavicon(ctx context.Context, domain string) ([]byte, string, error) {
	domain, ok := RegistrableDomain(domain)
	if !ok {
		return nil, "", ErrFaviconNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, faviconTotalTimeout)
	defer cancel()

	base := &url.URL{Scheme: "https", Host: domain, Path: "/"}
	var candidates []string
	if res, err := fetchFunc(ctx, base.String(), safefetch.Options{Accept: "text/html"}); err == nil {
		candidates = iconLinks(res.FinalURL, res.Body)
	}
	candidates = append(candidates,
		base.ResolveReference(&url.URL{Path: "/apple-touch-icon.png"}).String(),
		base.ResolveReference(&url.URL{Path: "/favicon.ico"}).String())

	seen := map[string]bool{}
	tried := 0
	for _, c := range candidates {
		if seen[c] || tried >= faviconMaxCandidate || ctx.Err() != nil {
			continue
		}
		seen[c] = true
		tried++
		res, err := fetchFunc(ctx, c, safefetch.Options{Accept: "image/*"})
		if err != nil {
			continue
		}
		mime := safefetch.SniffImage(res.Body)
		switch mime {
		case "":
			continue
		case safefetch.MimeSVG:
			clean, err := safefetch.SanitizeSVG(res.Body)
			if err != nil {
				continue
			}
			return clean, mime, nil
		default:
			return res.Body, mime, nil
		}
	}
	return nil, "", ErrFaviconNotFound
}

// iconLinks 从 HTML 中提取图标链接，apple-touch-icon 优先，其次 icon；只保留 http(s)。
func iconLinks(base *url.URL, body []byte) []string {
	var touch, icons []string
	z := html.NewTokenizer(bytes.NewReader(body))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		name, hasAttr := z.TagName()
		if string(name) == "body" {
			break
		}
		if string(name) != "link" || !hasAttr {
			continue
		}
		var rel, href string
		for {
			k, v, more := z.TagAttr()
			switch string(k) {
			case "rel":
				rel = strings.ToLower(string(v))
			case "href":
				href = strings.TrimSpace(string(v))
			}
			if !more {
				break
			}
		}
		if href == "" {
			continue
		}
		ref, err := url.Parse(href)
		if err != nil {
			continue
		}
		abs := base.ResolveReference(ref)
		if abs.Scheme != "https" && abs.Scheme != "http" {
			continue
		}
		fields := strings.Fields(rel)
		switch {
		case containsAny(fields, "apple-touch-icon", "apple-touch-icon-precomposed"):
			touch = append(touch, abs.String())
		case containsAny(fields, "icon"):
			icons = append(icons, abs.String())
		}
	}
	return append(touch, icons...)
}

func containsAny(fields []string, want ...string) bool {
	for _, f := range fields {
		for _, w := range want {
			if f == w {
				return true
			}
		}
	}
	return false
}
