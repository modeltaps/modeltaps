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
	"sync"
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
				{name: "ipc:" + clientIP, limit: maxRequestNum * rateLimitIPToleranceFactor},
			}
		}
	}
	return []rateLimitBucket{{name: "ip:" + clientIP, limit: maxRequestNum}}
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
			"code": "rate_limit_exceeded",
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
		// 回环豁免要求 RemoteIP 与 ClientIP 同为回环：只看 ClientIP，信任全部代理时伪造头可白嫖豁免；
		// 只看 RemoteIP，同机反代（对端恒为 127.0.0.1）会让所有流量都被豁免。
		if isLoopbackIP(c.RemoteIP()) && isLoopbackIP(clientIP) {
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
