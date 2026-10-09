package config

import (
	"net"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"

	"github.com/spf13/viper"
)

func InitConf() {
	defaultConfig()
	setEnv()
	IsMasterNode = viper.GetString("node_type") != "slave"
	RequestInterval = time.Duration(viper.GetInt("polling_interval")) * time.Second
	SessionSecretExplicit = viper.GetString("session_secret") != ""
	SessionSecret = utils.GetOrDefault("session_secret", SessionSecret)
	UserInvoiceMonth = viper.GetBool("user_invoice_month")
	UnpricedModelPolicy = viper.GetString("unpriced_model_policy")
	UnpricedModelDefaultRatio = viper.GetFloat64("unpriced_model_default_ratio")
	GitHubProxy = viper.GetString("github_proxy")
	MCP_ENABLE = viper.GetBool("mcp.enable") != false
	UPTIMEKUMA_ENABLE = viper.GetBool("uptime_kuma.enable") != false
	UPTIMEKUMA_DOMAIN = viper.GetString("uptime_kuma.domain")
	UPTIMEKUMA_STATUS_PAGE_NAME = viper.GetString("uptime_kuma.status_page_name")
	if v := viper.GetInt("login.max_failures"); v > 0 {
		LoginMaxFailures = v
	}
	if v := viper.GetInt("login.lockout_minutes"); v > 0 {
		LoginLockoutDuration = time.Duration(v) * time.Minute
	}
	if v := viper.GetInt("turnstile_session_minutes"); v > 0 {
		TurnstileSessionTTL = time.Duration(v) * time.Minute
	}
	if v := viper.GetInt("session.idle_days"); v > 0 {
		SessionIdleDuration = time.Duration(v) * 24 * time.Hour
	}
	if v := viper.GetInt("session.max_days"); v > 0 {
		SessionMaxDuration = time.Duration(v) * 24 * time.Hour
	}
}

func setEnv() {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}

// TrustedProxies 读取 trusted_proxies（反向代理的 IP/CIDR 列表）。
// 返回 nil 表示不信任任何代理，此时客户端 IP 退化为直连对端地址。
func TrustedProxies() []string {
	return ParseTrustedProxies(viper.GetStringSlice("trusted_proxies"))
}

// ParseTrustedProxies 归一化 trusted_proxies：兼容 YAML 列表与逗号分隔的环境变量写法，
// 去除空白与空项；结果为空时返回 nil。
func ParseTrustedProxies(raw []string) []string {
	var proxies []string
	for _, item := range raw {
		for _, part := range strings.Split(item, ",") {
			if part = strings.TrimSpace(part); part != "" {
				proxies = append(proxies, part)
			}
		}
	}
	return proxies
}

// ParseCIDRList 把 IP / CIDR 混合列表（YAML 列表或逗号分隔）预编译成网段，裸 IP 按单机处理（/32、/128）。
// 非法条目跳过并告警：宁可少放行一个来源（表现为被限流，可见），也不要让运维误以为已生效。
func ParseCIDRList(raw []string, optionName string) []*net.IPNet {
	items := ParseTrustedProxies(raw)
	out := make([]*net.IPNet, 0, len(items))
	for _, item := range items {
		if !strings.Contains(item, "/") {
			ip := net.ParseIP(item)
			if ip == nil {
				logger.SysError(optionName + ": invalid entry ignored: " + item)
				continue
			}
			bits := 128
			if ip.To4() != nil {
				bits = 32
			}
			out = append(out, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		if _, ipNet, err := net.ParseCIDR(item); err == nil {
			out = append(out, ipNet)
		} else {
			logger.SysError(optionName + ": invalid entry ignored: " + item)
		}
	}
	return out
}

func defaultConfig() {
	viper.SetDefault("port", "3000")
	viper.SetDefault("gin_mode", "release")
	viper.SetDefault("log_dir", "./logs")
	viper.SetDefault("log_io_max_body_kb", 1024)
	// sqlite_path 默认 modeltaps.db，可通过配置(SQLITE_PATH 等)指向其它库文件。
	viper.SetDefault("sqlite_path", "modeltaps.db")
	viper.SetDefault("sqlite_busy_timeout", 3000)
	viper.SetDefault("brand_icon_dir", "data/brand-icons")
	viper.SetDefault("sync_frequency", 600)
	viper.SetDefault("batch_update_interval", 5)
	viper.SetDefault("shutdown_timeout", 30)
	viper.SetDefault("redis_pool_size", 100)
	viper.SetDefault("redis_min_idle_conns", 10)
	viper.SetDefault("redis_pool_timeout", 5)
	viper.SetDefault("redis_read_timeout", 2)
	viper.SetDefault("redis_write_timeout", 2)
	viper.SetDefault("global.api_rate_limit", 300)
	viper.SetDefault("global.web_rate_limit", 300)
	// 免限流来源（IP 或 CIDR），供管理脚本 / 监控探针等高频可信调用方使用。
	viper.SetDefault("global.rate_limit_whitelist", []string{})
	viper.SetDefault("connect_timeout", 5)
	viper.SetDefault("auto_price_updates", false)
	viper.SetDefault("auto_price_updates_mode", "system")
	viper.SetDefault("auto_price_updates_interval", 1440)
	viper.SetDefault("update_price_service", "")
	viper.SetDefault("catalog_pricing.url", "")
	viper.SetDefault("catalog_pricing.auto_sync", true)
	viper.SetDefault("channel_pricing.auto_sync", true)
	viper.SetDefault("model_drift.auto_check", false)
	viper.SetDefault("model_drift.check_interval", "")
	viper.SetDefault("model_info_consistency.auto_check", false)
	viper.SetDefault("routing.model_info_mode", false)
	viper.SetDefault("unpriced_model_policy", "block")
	viper.SetDefault("unpriced_model_default_ratio", 30.0)
	viper.SetDefault("favicon", "")
	viper.SetDefault("user_invoice_month", false)
	viper.SetDefault("mcp.enable", false)
	viper.SetDefault("uptime_kuma.enable", false)
	viper.SetDefault("uptime_kuma.domain", "")
	viper.SetDefault("uptime_kuma.status_page_name", "")
	viper.SetDefault("login.max_failures", 5)
	viper.SetDefault("login.lockout_minutes", 15)
	viper.SetDefault("turnstile_session_minutes", 10)
	viper.SetDefault("session.idle_days", 7)
	viper.SetDefault("session.max_days", 30)
}
