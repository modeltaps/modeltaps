package brandicon

import (
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// ChannelTypeKey 返回渠道类型映射到的内置图标 key。
func ChannelTypeKey(channelType int) (string, bool) {
	target, ok := manifest.ChannelTypes[strconv.Itoa(channelType)]
	if !ok {
		return "", false
	}
	return Resolve(target)
}

// DomainKey 按最具体的域名优先匹配内置图标（api.deepseek.com → deepseek.com）。
func DomainKey(host string) (string, bool) {
	labels := strings.Split(strings.ToLower(strings.TrimSuffix(host, ".")), ".")
	for i := 0; i < len(labels)-1; i++ {
		if target, ok := manifest.Domains[strings.Join(labels[i:], ".")]; ok {
			return Resolve(target)
		}
	}
	return "", false
}

// HostFromURL 从 base_url 等配置中取出小写主机名；容忍缺少 scheme 的写法。
func HostFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + strings.TrimLeft(raw, "/")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
}

// RegistrableDomain 把主机名归一成可注册域名（eTLD+1，如 api.foo.co.uk → foo.co.uk）。
// IP 字面量、单标签主机、非法字符、公共后缀本身都返回 false。
func RegistrableDomain(host string) (string, bool) {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" || len(host) > 253 || !strings.Contains(host, ".") {
		return "", false
	}
	if _, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		return "", false
	}
	for _, r := range host {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '.') {
			return "", false
		}
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return "", false
		}
	}
	domain, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return "", false
	}
	if _, icann := publicsuffix.PublicSuffix(domain[strings.LastIndex(domain, ".")+1:]); !icann {
		return "", false
	}
	return domain, true
}
