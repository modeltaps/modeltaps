package middleware

// 仅覆盖内存限流路径（config.RedisEnabled=false）。
// redisRateLimiter 豁免：依赖全局 redis.RDB 真实客户端，仓库无 miniredis/redismock
// 等测试设施，且任务约束不引入新依赖，故不做单测。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/spf13/viper"
)

func init() {
	gin.SetMode(gin.TestMode)
	// 与 rateLimitFactory 相同的初始化，sync.Once 保证幂等
	inMemoryRateLimiter.Init(config.RateLimitKeyExpirationDuration)
}

// inMemoryRateLimiter 是包级全局，key = mark + ClientIP，且 -count=N 多轮共用同一进程；
// 固定 mark/IP 会把限流状态泄漏到下一轮或其它测试，故每次调用生成唯一值。
var rateLimitTestSeq int64

// uniqueMark 生成含 t.Name() 前缀、跨轮次唯一的 mark
func uniqueMark(t *testing.T) string {
	t.Helper()
	return t.Name() + "#" + strconv.FormatInt(atomic.AddInt64(&rateLimitTestSeq, 1), 10) + "-"
}

// uniqueIP 生成唯一测试 IP，供 mark 由生产 wrapper 固定（GW/GA/CT/DW/UP）的用例使用
func uniqueIP() string {
	n := atomic.AddInt64(&rateLimitTestSeq, 1)
	return fmt.Sprintf("10.99.%d.%d", (n>>8)&0xFF, n&0xFF)
}

// memoryLimitIP 以单个 IP 桶调用 memoryRateLimiter，对应未登录 / 敏感端点的分桶口径
func memoryLimitIP(c *gin.Context, maxRequestNum int, duration int64, mark string) {
	memoryRateLimiter(c, duration, mark, rateLimitBuckets(c, c.ClientIP(), maxRequestNum, false))
}

func newRateLimitContext(ip string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ip + ":12345"
	c.Request = req
	return c, w
}

// drainLimit 连续放行 limit 次后，断言第 limit+1 次被 429 拒绝
func drainLimit(t *testing.T, handler func(*gin.Context), ip string, limit int) {
	t.Helper()
	for i := 0; i < limit; i++ {
		c, _ := newRateLimitContext(ip)
		handler(c)
		if c.IsAborted() {
			t.Fatalf("request %d/%d: expected allow, got aborted with status %d", i+1, limit, c.Writer.Status())
		}
	}
	c, _ := newRateLimitContext(ip)
	handler(c)
	if !c.IsAborted() {
		t.Fatalf("request %d: expected rate limited, got allowed", limit+1)
	}
	if got := c.Writer.Status(); got != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, got)
	}
}

func TestMemoryRateLimiterAllowThenReject(t *testing.T) {
	mark := uniqueMark(t)
	drainLimit(t, func(c *gin.Context) {
		memoryLimitIP(c, 3, 60, mark)
	}, "10.1.0.1", 3)
}

func TestMemoryRateLimiterRecoversAfterWindow(t *testing.T) {
	const ip = "10.1.0.2"
	mark := uniqueMark(t)
	call := func() *gin.Context {
		c, _ := newRateLimitContext(ip)
		memoryLimitIP(c, 1, 1, mark)
		return c
	}
	if c := call(); c.IsAborted() {
		t.Fatal("first request: expected allow")
	}
	if c := call(); !c.IsAborted() {
		t.Fatal("second request within window: expected reject")
	}
	// duration=1s，Request 以 Unix 秒比较，sleep 1.1s 必然跨过至少一个秒边界
	time.Sleep(1100 * time.Millisecond)
	if c := call(); c.IsAborted() {
		t.Fatal("request after window expiry: expected allow")
	}
}

func TestMemoryRateLimiterMarkIsolation(t *testing.T) {
	const ip = "10.1.0.3"
	markA, markB := uniqueMark(t), uniqueMark(t)
	callMark := func(mark string) *gin.Context {
		c, _ := newRateLimitContext(ip)
		memoryLimitIP(c, 1, 60, mark)
		return c
	}
	if c := callMark(markA); c.IsAborted() {
		t.Fatal("mark A: first request expected allow")
	}
	if c := callMark(markA); !c.IsAborted() {
		t.Fatal("mark A: second request expected reject")
	}
	if c := callMark(markB); c.IsAborted() {
		t.Fatal("mark B: same IP different mark expected allow")
	}
}

func TestMemoryRateLimiterIPIsolation(t *testing.T) {
	mark := uniqueMark(t)
	callIP := func(ip string) *gin.Context {
		c, _ := newRateLimitContext(ip)
		memoryLimitIP(c, 1, 60, mark)
		return c
	}
	if c := callIP("10.1.0.4"); c.IsAborted() {
		t.Fatal("ip1: first request expected allow")
	}
	if c := callIP("10.1.0.4"); !c.IsAborted() {
		t.Fatal("ip1: second request expected reject")
	}
	if c := callIP("10.1.0.5"); c.IsAborted() {
		t.Fatal("ip2: same mark different IP expected allow")
	}
}

func TestMemoryRateLimiterConcurrent(t *testing.T) {
	const (
		ip       = "10.1.0.6"
		maxNum   = 100
		routines = 20
		perEach  = 25
	)
	mark := uniqueMark(t)
	var allowed int64
	var wg sync.WaitGroup
	for g := 0; g < routines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perEach; i++ {
				c, _ := newRateLimitContext(ip)
				memoryLimitIP(c, maxNum, 60, mark)
				if !c.IsAborted() {
					atomic.AddInt64(&allowed, 1)
				}
			}
		}()
	}
	wg.Wait()
	// duration=60s 内 500 个请求，恰好放行 maxNum 个
	if allowed != maxNum {
		t.Fatalf("expected exactly %d allowed, got %d", maxNum, allowed)
	}
}

func TestRateLimitConstants(t *testing.T) {
	cases := []struct {
		name              string
		num, wantNum      int
		duration, wantDur int64
	}{
		{"GlobalApi", GlobalApiRateLimitNum, 300, GlobalApiRateLimitDuration, 3 * 60},
		{"GlobalWeb", GlobalWebRateLimitNum, 300, GlobalWebRateLimitDuration, 3 * 60},
		{"Upload", UploadRateLimitNum, 10, UploadRateLimitDuration, 60},
		{"Download", DownloadRateLimitNum, 10, DownloadRateLimitDuration, 60},
		{"Critical", CriticalRateLimitNum, 200, CriticalRateLimitDuration, 20 * 60},
	}
	// 见 rate-limit.go 注释：duration 不应超过 RateLimitKeyExpirationDuration
	maxSeconds := int64(config.RateLimitKeyExpirationDuration.Seconds())
	for _, tc := range cases {
		if tc.num != tc.wantNum {
			t.Errorf("%s: num = %d, want %d", tc.name, tc.num, tc.wantNum)
		}
		if tc.duration != tc.wantDur {
			t.Errorf("%s: duration = %d, want %d", tc.name, tc.duration, tc.wantDur)
		}
		if tc.duration > maxSeconds {
			t.Errorf("%s: duration %ds exceeds RateLimitKeyExpirationDuration %ds", tc.name, tc.duration, maxSeconds)
		}
	}
}

func TestGetRateLimitTimeout(t *testing.T) {
	// 与生产逻辑同源计算期望值：viper 未配置（或 <=0）时兜底 2s
	want := time.Duration(viper.GetInt("redis_read_timeout")) * time.Second
	if want <= 0 {
		want = 2 * time.Second
	}
	if got := getRateLimitTimeout(); got != want {
		t.Fatalf("getRateLimitTimeout() = %v, want %v", got, want)
	}
	// sync.Once 缓存，二次调用结果一致
	if got := getRateLimitTimeout(); got != want {
		t.Fatalf("second call = %v, want %v", got, want)
	}
}

// 各入口 wrapper 装配核实：与生产同源计算 maxRequestNum，逐一打满验证限额与 429
func TestRateLimitWrapperAssembly(t *testing.T) {
	if config.RedisEnabled {
		t.Skip("requires memory rate limit path (redis disabled)")
	}
	cases := []struct {
		name    string
		handler func(*gin.Context)
		ip      string
		limit   int
	}{
		{"GlobalWebRateLimit", GlobalWebRateLimit(), uniqueIP(), utils.GetOrDefault("global.web_rate_limit", GlobalWebRateLimitNum)},
		{"GlobalAPIRateLimit", GlobalAPIRateLimit(), uniqueIP(), utils.GetOrDefault("global.api_rate_limit", GlobalApiRateLimitNum)},
		{"CriticalRateLimit", CriticalRateLimit(), uniqueIP(), CriticalRateLimitNum},
		{"DownloadRateLimit", DownloadRateLimit(), uniqueIP(), DownloadRateLimitNum},
		{"UploadRateLimit", UploadRateLimit(), uniqueIP(), UploadRateLimitNum},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			drainLimit(t, tc.handler, tc.ip, tc.limit)
		})
	}
}

// Download/Upload 限额相同但 mark 不同（DW/UP），同一 IP 打满一个不影响另一个
func TestRateLimitWrapperMarkIsolation(t *testing.T) {
	if config.RedisEnabled {
		t.Skip("requires memory rate limit path (redis disabled)")
	}
	ip := uniqueIP()
	drainLimit(t, DownloadRateLimit(), ip, DownloadRateLimitNum)
	drainLimit(t, UploadRateLimit(), ip, UploadRateLimitNum)
}

func TestRateLimitLoopbackBypass(t *testing.T) {
	handler := DownloadRateLimit()
	for i := 0; i < DownloadRateLimitNum*3; i++ {
		c, _ := newRateLimitContext("127.0.0.1")
		handler(c)
		if c.IsAborted() {
			t.Fatalf("loopback request %d: expected bypass, got aborted", i+1)
		}
	}
}

// newSpoofedContext 构造带伪造代理头的上下文。trustAllProxies=true 时引擎沿用 gin 默认
// （信任 0.0.0.0/0），使 ClientIP 采信该头；false 时对齐 trusted_proxies 为空的生产默认。
func newSpoofedContext(ip string, trustAllProxies bool, header, value string) *gin.Context {
	w := httptest.NewRecorder()
	c, engine := gin.CreateTestContext(w)
	if !trustAllProxies {
		_ = engine.SetTrustedProxies(nil)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ip + ":12345"
	req.Header.Set(header, value)
	c.Request = req
	return c
}

// 即使 ClientIP 被伪造成回环地址，豁免也只认 RemoteIP，请求仍应受限流
func TestRateLimitLoopbackBypassIgnoresSpoofedHeader(t *testing.T) {
	if config.RedisEnabled {
		t.Skip("requires memory rate limit path (redis disabled)")
	}
	handler := DownloadRateLimit()
	ip := uniqueIP()
	// 伪造值取唯一回环地址，避免与其它用例共用 DW 键
	n := atomic.AddInt64(&rateLimitTestSeq, 1)
	spoofed := fmt.Sprintf("127.9.%d.%d", (n>>8)&0xFF, n&0xFF)
	call := func() *gin.Context {
		c := newSpoofedContext(ip, true, "X-Forwarded-For", spoofed)
		handler(c)
		return c
	}
	if c := call(); c.ClientIP() != spoofed {
		t.Fatalf("precondition: ClientIP = %q, want spoofed %q", c.ClientIP(), spoofed)
	}
	for i := 1; i < DownloadRateLimitNum; i++ {
		if c := call(); c.IsAborted() {
			t.Fatalf("request %d/%d: expected allow, got aborted", i+1, DownloadRateLimitNum)
		}
	}
	if c := call(); !c.IsAborted() {
		t.Fatal("spoofed loopback header must not bypass rate limit")
	}
}

// trusted_proxies 为空时，限流键取直连对端地址，换伪造头不应换到新的限流桶
func TestRateLimitKeyIgnoresForwardedForWhenNoTrustedProxy(t *testing.T) {
	mark := uniqueMark(t)
	ip := uniqueIP()
	call := func(spoofed string) *gin.Context {
		c := newSpoofedContext(ip, false, "X-Forwarded-For", spoofed)
		memoryLimitIP(c, 1, 60, mark)
		return c
	}
	if c := call("203.0.113.1"); c.IsAborted() {
		t.Fatal("first request: expected allow")
	}
	if c := call("203.0.113.2"); !c.IsAborted() {
		t.Fatal("rate limit key must be the peer address, not X-Forwarded-For")
	}
}

// 被限流时返回 Retry-After / X-RateLimit-* 头与 JSON 错误体
func TestRateLimitTooManyRequestsResponse(t *testing.T) {
	mark := uniqueMark(t)
	ip := uniqueIP()
	c, _ := newRateLimitContext(ip)
	memoryLimitIP(c, 1, 60, mark)
	c, w := newRateLimitContext(ip)
	memoryLimitIP(c, 1, 60, mark)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", w.Code)
	}
	retryAfter, err := strconv.Atoi(w.Header().Get("Retry-After"))
	if err != nil || retryAfter < 1 || retryAfter > 60 {
		t.Fatalf("Retry-After = %q, want 1..60", w.Header().Get("Retry-After"))
	}
	if got := w.Header().Get("X-RateLimit-Limit"); got != "1" {
		t.Fatalf("X-RateLimit-Limit = %q, want 1", got)
	}
	if got := w.Header().Get("X-RateLimit-Remaining"); got != "0" {
		t.Fatalf("X-RateLimit-Remaining = %q, want 0", got)
	}
	if got := w.Header().Get("X-RateLimit-Reset"); got != w.Header().Get("Retry-After") {
		t.Fatalf("X-RateLimit-Reset = %q, want same as Retry-After", got)
	}
	var body struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Type != "system_error" || body.Error.Code != "rate_limit_exceeded" || body.Error.Message == "" {
		t.Fatalf("unexpected error body: %+v", body.Error)
	}
}

// 同机反代：对端恒为 127.0.0.1，但可信反代解析出的 ClientIP 不是回环，不得豁免
func TestRateLimitSameHostProxyNotBypassed(t *testing.T) {
	if config.RedisEnabled {
		t.Skip("requires memory rate limit path (redis disabled)")
	}
	handler := DownloadRateLimit()
	clientIP := uniqueIP()
	call := func() *gin.Context {
		c := newSpoofedContext("127.0.0.1", true, "X-Forwarded-For", clientIP)
		handler(c)
		return c
	}
	for i := 0; i < DownloadRateLimitNum; i++ {
		if c := call(); c.IsAborted() {
			t.Fatalf("request %d/%d: expected allow, got aborted", i+1, DownloadRateLimitNum)
		}
	}
	if c := call(); !c.IsAborted() {
		t.Fatal("same-host proxied request must be rate limited by its client IP")
	}
}

// 登录用户占用「用户配额 + 放宽的 IP 天花板」：同一 IP 下另一用户不受前一用户打满影响
func TestRateLimitPerUserQuota(t *testing.T) {
	if config.RedisEnabled {
		t.Skip("requires memory rate limit path (redis disabled)")
	}
	const limit = 2
	handler := rateLimitFactory(limit, 60, uniqueMark(t), false, true)
	engine := gin.New()
	engine.Use(sessions.Sessions("rl-test", cookie.NewStore([]byte("rate-limit-test-secret"))))
	engine.GET("/:uid", func(c *gin.Context) {
		if uid, _ := strconv.Atoi(c.Param("uid")); uid > 0 {
			sessions.Default(c).Set("id", uid)
		}
		handler(c)
		if !c.IsAborted() {
			c.Status(http.StatusOK)
		}
	})
	ip := uniqueIP()
	do := func(uid int64) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/"+strconv.FormatInt(uid, 10), nil)
		req.RemoteAddr = ip + ":12345"
		engine.ServeHTTP(w, req)
		return w
	}
	userA := atomic.AddInt64(&rateLimitTestSeq, 1)
	userB := atomic.AddInt64(&rateLimitTestSeq, 1)
	for i := 0; i < limit; i++ {
		if w := do(userA); w.Code != http.StatusOK {
			t.Fatalf("user A request %d: status %d, want 200", i+1, w.Code)
		}
	}
	w := do(userA)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("user A over quota: status %d, want 429", w.Code)
	}
	if got := w.Header().Get("X-RateLimit-Limit"); got != strconv.Itoa(limit) {
		t.Fatalf("user bucket X-RateLimit-Limit = %q, want %d", got, limit)
	}
	if w := do(userB); w.Code != http.StatusOK {
		t.Fatalf("user B on same IP: status %d, want 200", w.Code)
	}
	// 匿名请求走独立的 ip: 桶，限额保持原值
	for i := 0; i < limit; i++ {
		if w := do(0); w.Code != http.StatusOK {
			t.Fatalf("anonymous request %d: status %d, want 200", i+1, w.Code)
		}
	}
	if w := do(0); w.Code != http.StatusTooManyRequests {
		t.Fatalf("anonymous over limit: status %d, want 429", w.Code)
	}
}
