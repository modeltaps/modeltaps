package middleware

import (
	"context"
	"errors"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/redis"
	"github.com/modeltaps/modeltaps/common/utils"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func isLoopbackIP(remoteIP string) bool {
	ip := net.ParseIP(remoteIP)
	return ip != nil && ip.IsLoopback()
}

// rateLimitIPKey is the part of a bucket key that names the client address. IPv6 clients are
// grouped by /64, the smallest block a provider hands to one subscriber: counting every address
// on its own would let anyone holding a /64 open a fresh bucket per request.
func rateLimitIPKey(clientIP string) string {
	ip := net.ParseIP(clientIP)
	if ip == nil || ip.To4() != nil {
		return clientIP
	}
	return ip.Mask(net.CIDRMask(64, 128)).String() + "/64"
}

// trustedProxySet is trusted_proxies parsed the way gin's SetTrustedProxies reads it: one bad
// entry makes main.go trust no proxy at all, so it yields no network here too. It is kept with
// the raw list it came from and parsed again only when that list changes.
type trustedProxySet struct {
	raw  string
	nets []*net.IPNet
}

var trustedProxyCache atomic.Pointer[trustedProxySet]

func trustedProxyNets() []*net.IPNet {
	entries := config.TrustedProxies()
	raw := strings.Join(entries, ",")
	if set := trustedProxyCache.Load(); set != nil && set.raw == raw {
		return set.nets
	}
	set := &trustedProxySet{raw: raw, nets: parseTrustedProxyNets(entries)}
	trustedProxyCache.Store(set)
	return set.nets
}

func parseTrustedProxyNets(entries []string) []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(entries))
	for _, entry := range entries {
		if !strings.Contains(entry, "/") {
			ip := net.ParseIP(entry)
			if ip == nil {
				return nil
			}
			bits := 128
			if ip.To4() != nil {
				bits = 32
			}
			nets = append(nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		_, ipNet, err := net.ParseCIDR(entry)
		if err != nil {
			return nil
		}
		nets = append(nets, ipNet)
	}
	return nets
}

// FromTrustedProxy reports whether the request's TCP peer is one of trusted_proxies, the only
// peers whose forwarding headers (X-Forwarded-For, X-Forwarded-Proto, ...) may be believed.
func FromTrustedProxy(c *gin.Context) bool {
	ip := net.ParseIP(c.RemoteIP())
	if ip == nil {
		return false
	}
	for _, ipNet := range trustedProxyNets() {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

// hasForwardingHeader reports whether the request carries one of the headers gin reads the
// client address from: a request that came through a proxy, not a direct local call.
func hasForwardingHeader(c *gin.Context) bool {
	if c.GetHeader("X-Forwarded-For") != "" || c.GetHeader("X-Real-IP") != "" {
		return true
	}
	if h := viper.GetString("trusted_header"); h != "" && c.GetHeader(h) != "" {
		return true
	}
	return false
}

var untrustedLocalProxyWarned atomic.Int64 // last warning, Unix seconds

// loopbackExempt reports whether the request is a call from the same host (a script or a probe)
// that skips the limit. Only a direct call qualifies:
//   - a loopback client reported by a trusted proxy is either a forged X-Forwarded-For passed on
//     by the proxy or a local call through the proxy, and is limited like any other client;
//   - a proxied request from a loopback peer that is not in trusted_proxies means a same-host
//     proxy was left out of trusted_proxies, so every client looks like 127.0.0.1. Limiting would
//     put the whole site into one bucket, so the request still passes, and the log says why.
func loopbackExempt(c *gin.Context, clientIP string) bool {
	if !isLoopbackIP(c.RemoteIP()) || !isLoopbackIP(clientIP) {
		return false
	}
	if !hasForwardingHeader(c) {
		return true
	}
	if FromTrustedProxy(c) {
		return false
	}
	now := time.Now().Unix()
	if last := untrustedLocalProxyWarned.Load(); now-last >= 3600 && untrustedLocalProxyWarned.CompareAndSwap(last, now) {
		logger.SysError("rate limits are off for proxied requests: they come from a proxy on this host that is not in trusted_proxies, " +
			"so every client looks like the loopback address; add the proxy to trusted_proxies")
	}
	return true
}

// rateLimitWhitelist 是免限流的来源，取自 global.rate_limit_whitelist，
// 面向管理脚本 / 监控探针这类调用量天然高于人类用户的可信来源。
//
// 只豁免 global.* 总量限流，不豁免 CriticalRateLimit：后者护着登录、注册、改密、
// OAuth 回调等端点，是口令爆破的节流点；Upload/Download 同理。
var (
	rateLimitWhitelist     []*net.IPNet
	rateLimitWhitelistOnce sync.Once
)

func isRateLimitWhitelisted(ip net.IP) bool {
	rateLimitWhitelistOnce.Do(func() {
		rateLimitWhitelist = config.ParseCIDRList(viper.GetStringSlice("global.rate_limit_whitelist"), "global.rate_limit_whitelist")
	})
	if ip == nil {
		return false
	}
	for _, cidr := range rateLimitWhitelist {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// rateLimitBucket 是本次请求要占用的一格配额，limit 是这个桶自己的上限。
// name 决定阈值归属，不同 limit 的计数必须落在不同 name 上（见 rateLimitBuckets）。
type rateLimitBucket struct {
	name  string
	limit int
}

// sessionUserID 取出会话里已登录的用户 id。
// 不用 sessions.Default：它内部是 MustGet，未挂 session 中间件的 Engine（RelayOnly）上会 panic。
// 只认服务端签名的会话，不采纳请求头里的 access token：此中间件跑在鉴权之前，
// 未经校验的凭据一旦用于分桶，不断更换 token 值就能无限开桶。
func sessionUserID(c *gin.Context) (int, bool) {
	raw, exists := c.Get(sessions.DefaultKey)
	if !exists {
		return 0, false
	}
	session, ok := raw.(sessions.Session)
	if !ok {
		return 0, false
	}
	id, ok := session.Get("id").(int)
	if !ok || id <= 0 {
		return 0, false
	}
	return id, true
}

// rateLimitBuckets 返回本次请求要占用的桶，全部都要过、任一满即拒。
//
// 识别到登录用户时是「用户配额 + 放宽的 IP 天花板」两个桶：只按用户计，单机靠多注册账号
// 就能线性放大配额；只按 IP 计，同一 NAT / 公司出口后面的用户互相挤占。
// 识别不到用户时只有 IP 一个桶，限额保持 maxRequestNum 原值、不放宽：此时它是唯一的总量闸门。
//
// 两种情形的 IP 计数用不同的 key（ip: 与 ipc:），滑动窗口的阈值必须与计数器一一对应。
// perUserQuota 为 false 时强制只按 IP：敏感端点若认用户桶，攻击者带自己的有效 session
// 去打别人的账号就换到了一个空桶。
func rateLimitBuckets(c *gin.Context, clientIP string, maxRequestNum int, perUserQuota bool) []rateLimitBucket {
	if perUserQuota {
		if id, ok := sessionUserID(c); ok {
			return []rateLimitBucket{
				{name: "u:" + strconv.Itoa(id), limit: maxRequestNum},
				{name: "ipc:" + rateLimitIPKey(clientIP), limit: maxRequestNum * rateLimitIPToleranceFactor},
			}
		}
	}
	return []rateLimitBucket{{name: "ip:" + rateLimitIPKey(clientIP), limit: maxRequestNum}}
}

// abortWithTooManyRequests 返回 429，带 Retry-After（秒）、X-RateLimit-* 与 JSON 错误体。
// 不复用 abortWithMessage：429 是高频出口，逐条写 error 日志会在被刷时放大写入并冲掉其他诊断记录，
// 状态码本身已进访问日志；request id 仍按同样格式带上，便于用户报障时定位。
func abortWithTooManyRequests(c *gin.Context, bucket rateLimitBucket, retryAfterSeconds int64) {
	if retryAfterSeconds < 1 {
		retryAfterSeconds = 1
	}
	retryAfter := strconv.FormatInt(retryAfterSeconds, 10)
	c.Header("Retry-After", retryAfter)
	// Remaining 恒为 0：能走到这里就是这个桶已经满了；Reset 给的是还需等待的秒数，与 Retry-After 同义。
	c.Header("X-RateLimit-Limit", strconv.Itoa(bucket.limit))
	c.Header("X-RateLimit-Remaining", "0")
	c.Header("X-RateLimit-Reset", retryAfter)
	c.JSON(http.StatusTooManyRequests, gin.H{
		"error": gin.H{
			"message": utils.MessageWithRequestId(
				"Too many requests, please retry after "+retryAfter+" seconds.",
				c.GetString(logger.RequestIdKey),
			),
			"type": "system_error",
			"code": common.APICodeRateLimitExceeded,
			// The web console builds its translated notice from params (see common/api_error.go).
			"params": gin.H{"retry_after": retryAfterSeconds},
		},
	})
	c.Abort()
}

var inMemoryRateLimiter common.InMemoryRateLimiter

// rate-limit 的 Redis 操作超时跟随 redis_read_timeout 配置，但热路径上每请求调一次
// viper.GetInt 走 RWMutex.RLock+reflect 不划算，init-once 缓存（与 redis.go 的
// stickySessionOpTimeout 同思路）。
var (
	rateLimitTimeout     time.Duration
	rateLimitTimeoutOnce sync.Once
)

func getRateLimitTimeout() time.Duration {
	rateLimitTimeoutOnce.Do(func() {
		rateLimitTimeout = time.Duration(viper.GetInt("redis_read_timeout")) * time.Second
		if rateLimitTimeout <= 0 {
			rateLimitTimeout = 2 * time.Second
		}
	})
	return rateLimitTimeout
}

// degradeAllow 把 rate-limit 路径上的 Redis 错误降级为放行，并过滤 context.Canceled 不打日志。
// 限流只是辅助，鉴权/配额在后续 middleware；把瞬时 Redis 故障翻译成 500 会让上游渠道
// 误以为模型挂了并触发重试，反而放大问题。
// context.Canceled 是客户端中途断开（父 ctx 取消），不是 Redis 故障，过滤掉避免误告警；
// context.DeadlineExceeded 保留为真 Redis 慢的信号。
func degradeAllow(where string, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	logger.SysError("rate limit degraded, allowing request (" + where + "): " + err.Error())
}

// rateLimitIPToleranceFactor 是 IP 天花板相对单用户配额的倍率，
// 仅在已识别出登录用户、IP 桶退居第二道防线时生效（见 rateLimitBuckets）。
const rateLimitIPToleranceFactor = 4

// All duration's unit is seconds
// Shouldn't larger then RateLimitKeyExpirationDuration
var (
	GlobalApiRateLimitNum            = 300
	GlobalApiRateLimitDuration int64 = 3 * 60

	GlobalWebRateLimitNum            = 300
	GlobalWebRateLimitDuration int64 = 3 * 60

	UploadRateLimitNum            = 10
	UploadRateLimitDuration int64 = 60

	DownloadRateLimitNum            = 10
	DownloadRateLimitDuration int64 = 60

	CriticalRateLimitNum            = 200
	CriticalRateLimitDuration int64 = 20 * 60
)

// rateLimitScript 在 Redis 侧原子完成「清理过期 + 判定 + 记账」，替换原先的 LLen/LIndex/LPush/Expire 多次往返：
//   - 原子性：原实现读 LLen 与写 LPush 之间无互斥，并发请求会一起判定为「未满」而击穿配额；
//   - 往返次数：每请求 3-4 次 RTT 降到 1 次；
//   - 时间基准：原实现以本地时区 Format、按 UTC Parse 的字符串存时间，非 UTC 部署下 elapsed
//     偏掉整个时区差；这里统一传 Unix 秒，旧格式记录 tonumber 失败时一并丢弃（自愈）。
//
// 队列顺序 [新 --> 旧]（LPUSH 进、尾部最旧）。返回 {allowed, retryAfterSeconds}。
var rateLimitScript = goredis.NewScript(`
local key      = KEYS[1]
local maxReq   = tonumber(ARGV[1])
local duration = tonumber(ARGV[2])
local now      = tonumber(ARGV[3])
local ttl      = tonumber(ARGV[4])

while true do
  local oldest = redis.call('LINDEX', key, -1)
  if not oldest then break end
  local ts = tonumber(oldest)
  if ts == nil or now - ts >= duration then
    redis.call('RPOP', key)
  else
    break
  end
end

if redis.call('LLEN', key) < maxReq then
  redis.call('LPUSH', key, now)
  redis.call('EXPIRE', key, ttl)
  return {1, 0}
end

local oldest = tonumber(redis.call('LINDEX', key, -1))
local retry = math.ceil(oldest + duration - now)
if retry < 1 then retry = 1 end
if retry > duration then retry = duration end
redis.call('EXPIRE', key, ttl)
return {0, retry}
`)

// redisRateLimiter 依次扣减每个桶的配额，任一桶满即 429。
// 桶之间不是原子事务：先扣的桶成功、后扣的桶失败时不回滚，多扣的一格只占同一来源的配额，可接受。
func redisRateLimiter(c *gin.Context, duration int64, mark string, buckets []rateLimitBucket) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), getRateLimitTimeout())
	defer cancel()

	ttl := int64(config.RateLimitKeyExpirationDuration.Seconds())
	now := time.Now().Unix()
	for _, bucket := range buckets {
		key := "rateLimit:" + mark + bucket.name
		res, err := rateLimitScript.Run(ctx, redis.RDB, []string{key}, bucket.limit, duration, now, ttl).Int64Slice()
		if err != nil {
			degradeAllow("eval", err)
			return
		}
		if len(res) != 2 {
			degradeAllow("eval", errors.New("unexpected script result"))
			return
		}
		if res[0] == 0 {
			abortWithTooManyRequests(c, bucket, res[1])
			return
		}
	}
}

func memoryRateLimiter(c *gin.Context, duration int64, mark string, buckets []rateLimitBucket) {
	for _, bucket := range buckets {
		allowed, retryAfter := inMemoryRateLimiter.Request(mark+bucket.name, bucket.limit, duration)
		if !allowed {
			abortWithTooManyRequests(c, bucket, retryAfter)
			return
		}
	}
}

// rateLimitFactory 构造限流中间件。
// allowWhitelist：是否让 global.rate_limit_whitelist 豁免本限流器。
// perUserQuota：已登录请求是否额外享有独立的用户配额，同时保留放宽的 IP 天花板。
// 两者都只对 global.* 总量限流开启，敏感端点一律按 IP 照常计数。
func rateLimitFactory(maxRequestNum int, duration int64, mark string, allowWhitelist, perUserQuota bool) func(c *gin.Context) {
	var limiter func(c *gin.Context, buckets []rateLimitBucket)
	if config.RedisEnabled {
		limiter = func(c *gin.Context, buckets []rateLimitBucket) {
			redisRateLimiter(c, duration, mark, buckets)
		}
	} else {
		// It's safe to call multi times.
		inMemoryRateLimiter.Init(config.RateLimitKeyExpirationDuration)
		limiter = func(c *gin.Context, buckets []rateLimitBucket) {
			memoryRateLimiter(c, duration, mark, buckets)
		}
	}
	return func(c *gin.Context) {
		// 分桶与白名单用 ClientIP：其可信度由 main.go 的 SetTrustedProxies 保证，
		// 来自不可信对端的转发头会被 gin 忽略。
		clientIP := c.ClientIP()
		if loopbackExempt(c, clientIP) {
			return
		}
		if allowWhitelist && isRateLimitWhitelisted(net.ParseIP(clientIP)) {
			return
		}
		limiter(c, rateLimitBuckets(c, clientIP, maxRequestNum, perUserQuota))
	}
}

func GlobalWebRateLimit() func(c *gin.Context) {
	return rateLimitFactory(utils.GetOrDefault("global.web_rate_limit", GlobalWebRateLimitNum), GlobalWebRateLimitDuration, "GW", true, true)
}

func GlobalAPIRateLimit() func(c *gin.Context) {
	return rateLimitFactory(utils.GetOrDefault("global.api_rate_limit", GlobalApiRateLimitNum), GlobalApiRateLimitDuration, "GA", true, true)
}

func CriticalRateLimit() func(c *gin.Context) {
	return rateLimitFactory(CriticalRateLimitNum, CriticalRateLimitDuration, "CT", false, false)
}

func DownloadRateLimit() func(c *gin.Context) {
	return rateLimitFactory(DownloadRateLimitNum, DownloadRateLimitDuration, "DW", false, false)
}

func UploadRateLimit() func(c *gin.Context) {
	return rateLimitFactory(UploadRateLimitNum, UploadRateLimitDuration, "UP", false, false)
}

// MJImageRateLimit limits /mj/image, which anyone may call with a task ID and which makes the
// server fetch the image from upstream. A page of results loads a few dozen images; this leaves
// room for that and for a few users behind one address.
var (
	MJImageRateLimitNum            = 300
	MJImageRateLimitDuration int64 = 3 * 60
)

func MJImageRateLimit() func(c *gin.Context) {
	return rateLimitFactory(MJImageRateLimitNum, MJImageRateLimitDuration, "MI", true, false)
}

// Agent handover (AGENT-7). The endpoints an agent calls without signing in are limited per IP;
// per code, a pairing code is void after model.AgentPairingMaxFailures wrong secrets and a device
// code answers slow_down when polled faster than its interval.
var (
	AgentPairRateLimitNum            = 20
	AgentPairRateLimitDuration int64 = 10 * 60

	AgentDeviceStartRateLimitNum            = 20
	AgentDeviceStartRateLimitDuration int64 = 10 * 60

	// A poll every 5 seconds for the 15 minutes a device code lives is 180 requests; this leaves
	// room for a few agents behind one address.
	AgentDevicePollRateLimitNum            = 600
	AgentDevicePollRateLimitDuration int64 = 15 * 60
)

func AgentPairRateLimit() func(c *gin.Context) {
	return rateLimitFactory(AgentPairRateLimitNum, AgentPairRateLimitDuration, "AP", false, false)
}

func AgentDeviceStartRateLimit() func(c *gin.Context) {
	return rateLimitFactory(AgentDeviceStartRateLimitNum, AgentDeviceStartRateLimitDuration, "AS", false, false)
}

func AgentDevicePollRateLimit() func(c *gin.Context) {
	return rateLimitFactory(AgentDevicePollRateLimitNum, AgentDevicePollRateLimitDuration, "AT", false, false)
}
