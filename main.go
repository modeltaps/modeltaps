package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"github.com/modeltaps/modeltaps/cli"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/brandicon"
	"github.com/modeltaps/modeltaps/common/cache"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/modeltaps/modeltaps/common/redis"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/common/search"
	"github.com/modeltaps/modeltaps/common/storage"
	"github.com/modeltaps/modeltaps/common/telegram"
	"github.com/modeltaps/modeltaps/controller"
	"github.com/modeltaps/modeltaps/cron"
	"github.com/modeltaps/modeltaps/middleware"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/relay/task"
	"github.com/modeltaps/modeltaps/router"
	"github.com/modeltaps/modeltaps/safty"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// all: keeps files whose names start with "_" or ".", which a plain directory
// embed skips. Vite names shared chunks after their module, so a module such as
// catalog/_helpers.js becomes assets/_helpers-<hash>.js and must be served.
//
//go:embed all:web/build
var buildFS embed.FS

//go:embed web/build/index.html
var indexPage []byte

func main() {
	if tz := os.Getenv("TZ"); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			time.Local = loc
		}
	} else {
		time.Local = time.UTC
	}

	cli.InitCli()
	config.InitConf()
	if viper.GetString("log_level") == "debug" {
		config.Debug = true
	}

	logger.SetupLogger()
	logger.SysLog("Modeltaps " + config.Version + " started")

	if !config.SessionSecretExplicit {
		logger.LogWarn(nil, "[SYS] | SESSION_SECRET is not set, using a random session secret for this run: all login sessions are lost on restart and sessions are not shared across replicas. Set SESSION_SECRET explicitly in production")
	}

	// Initialize user token
	err := common.InitUserToken()
	if err != nil {
		logger.FatalLog("failed to initialize user token: " + err.Error())
	}

	// Initialize SQL Database
	model.SetupDB()
	defer model.CloseDB()
	// Initialize Redis
	redis.InitRedisClient()
	cache.InitCacheManager()
	// Initialize invite code lock system
	model.InitInviteCodeLock()
	// Initialize options
	model.InitOptionMap()
	model.NewPricing()
	model.HandleOldTokenMaxId()

	initMemoryCache()
	initSync()

	common.InitTokenEncoders()
	requester.InitHttpClient()
	initMemoryMonitor()
	// Initialize Telegram bot
	telegram.InitTelegramBot()

	controller.InitMidjourneyTask()
	task.InitTask()
	notify.InitNotifier()
	cron.InitCron()
	// 品牌图标同步层：每个节点各自维护本地 data 目录，故不限主节点
	brandicon.StartSyncLoop(viper.GetString("brand_icon_dir"), 24*time.Hour,
		func() bool { return config.BrandIconSyncEnabled },
		func() string { return config.BrandIconRegistry })
	controller.BrandIconSync = func(ctx context.Context) error {
		_, _, err := brandicon.Sync(ctx, config.BrandIconRegistry, viper.GetString("brand_icon_dir"))
		return err
	}
	// Bifrost 式急切同步：启动时立即拉一次「自带价格」渠道(OpenRouter)的真实价格，
	// 不必等每日 cron。受 channel_pricing.auto_sync 开关控制(默认开)。
	if viper.GetBool("channel_pricing.auto_sync") {
		go func() {
			n, err := controller.SyncAllOpenRouterChannelsPricing()
			if err != nil {
				logger.SysError("startup OpenRouter pricing sync failed: " + err.Error())
				return
			}
			if n > 0 {
				logger.SysLog(fmt.Sprintf("startup OpenRouter pricing sync done: %d models", n))
			}
		}()
	}
	storage.InitStorage()
	search.InitSearcher()
	// 初始化安全检查器
	safty.InitSaftyTools()
	// 初始化账单数据
	if config.UserInvoiceMonth {
		logger.SysLog("Enable User Invoice Monthly Data")
		go model.InsertStatisticsMonth()
	}
	initHttpServer()
}

func initMemoryCache() {
	if viper.GetBool("memory_cache_enabled") {
		config.MemoryCacheEnabled = true
	}

	if !config.MemoryCacheEnabled {
		return
	}

	syncFrequency := viper.GetInt("sync_frequency")
	model.TokenCacheSeconds = syncFrequency

	logger.SysLog("memory cache enabled")
	logger.SysLog(fmt.Sprintf("sync frequency: %d seconds", syncFrequency))
	go model.SyncOptions(syncFrequency)
	go SyncChannelCache(syncFrequency)
}

func initSync() {
	// go controller.AutomaticallyUpdateChannels(viper.GetInt("channel.update_frequency"))
	go controller.AutomaticallyTestChannels(viper.GetInt("channel.test_frequency"))
}

func initHttpServer() {
	if viper.GetString("gin_mode") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	server := gin.New()
	server.Use(gin.Recovery())
	server.Use(middleware.RequestId())
	middleware.SetUpLogger(server)

	// gin 默认信任所有代理（0.0.0.0/0），任何直连客户端都能用 X-Forwarded-For 伪造
	// 客户端 IP，从而绕过按 IP 计的限流。改为只信任显式配置的反代；未配置时
	// SetTrustedProxies(nil) 让 ClientIP 退化为直连对端地址。
	trustedProxies := config.TrustedProxies()
	if err := server.SetTrustedProxies(trustedProxies); err != nil {
		logger.SysError("invalid trusted_proxies, trusting no proxy: " + err.Error())
		trustedProxies = nil
		_ = server.SetTrustedProxies(nil)
	}

	// trusted_header 用 RemoteIPHeaders 而非 TrustedPlatform：后者会无条件采信请求头，
	// 前者只在直连对端落在 trusted_proxies 内时才生效。
	trustedHeader := viper.GetString("trusted_header")
	if trustedHeader != "" {
		server.RemoteIPHeaders = []string{trustedHeader}
		if len(trustedProxies) == 0 {
			logger.SysError("trusted_header is set but trusted_proxies is empty, the header is ignored and client IP falls back to the peer address")
		}
	}

	store := cookie.NewStore([]byte(config.SessionSecret))

	// 检测是否在 HTTPS 环境下运行
	isHTTPS := viper.GetBool("https") || viper.GetString("trusted_header") == "CF-Connecting-IP"

	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   int(config.SessionMaxDuration.Seconds()), // cookie 上限与会话绝对有效期一致；空闲超时由会话行控制
		HttpOnly: true,
		Secure:   isHTTPS,                 // 在 HTTPS 环境下启用 Secure
		SameSite: http.SameSiteStrictMode, // Strict：OIDC 回调页是同源 XHR 再带 cookie，跨站首跳不需要它
	})

	// cookie 名带站点前缀，避免与同域名部署的身份提供方（Authgear 等也叫 session）互相覆盖；
	// HTTPS 下再加 __Host- 前缀：浏览器强制 Secure + Path=/ + 不可设 Domain，子域也覆盖不了它。
	sessionCookieName := "modeltaps_session"
	if isHTTPS {
		sessionCookieName = "__Host-modeltaps_session"
	}
	server.Use(sessions.Sessions(sessionCookieName, store))

	router.SetRouter(server, buildFS, indexPage)
	port := viper.GetString("port")

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: server,
	}

	serverErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case err := <-serverErr:
		if err != nil {
			logger.FatalLog("failed to start HTTP server: " + err.Error())
		}
	case sig := <-quit:
		logger.SysLog(fmt.Sprintf("received signal %s, shutting down...", sig))
		// 注意：srv.Shutdown 不会等已 hijack 的 WebSocket 连接（net/http 不 track），
		// 长流式 / WS 会话由后面的 WaitTrackedGoroutines 通过 timeout 兜底
		// 默认 30s 以容纳长流式响应（LLM 流式补全单次常 30-120s）
		// 部署侧需配套 docker-compose stop_grace_period >= shutdown_timeout + flush 余量
		shutdownTimeout := viper.GetInt("shutdown_timeout")
		if shutdownTimeout <= 0 {
			shutdownTimeout = 30
		}
		deadline := time.Now().Add(time.Duration(shutdownTimeout) * time.Second)

		// 1) 停止接受新请求并等待 in-flight HTTP 请求完成
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		if err := srv.Shutdown(ctx); err != nil {
			logger.SysError("HTTP server shutdown error: " + err.Error())
		}
		cancel()

		// 2) 等待 handler 派生的 tracked goroutine（realtime / task 的 Consume）跑完，
		//    否则它们的 RecordConsumeLog 会塞进 batch 队列后无人 flush
		remaining := time.Until(deadline)
		// 即使 srv.Shutdown 把预算耗光，也至少给 tracked goroutine 1s 收尾，
		// 否则刚跑完 Shutdown 就立即超时退出，必丢这部分数据。
		// 极端情况（shutdown_timeout=1）下，真实等待会比配置多 ~1s
		if remaining < time.Second {
			remaining = time.Second
		}
		if !common.WaitTrackedGoroutines(remaining) {
			logger.SysError(fmt.Sprintf("tracked goroutines did not finish within %s, data may be lost", remaining))
		}

		// 3) 停 batch updater 后台 ticker，避免它和主线程同时 batchUpdate
		//    引发"swap 后未写完"窗口数据丢失
		// 4) flush batch 队列：必须在前三步完成之后，确保没有新数据再进队列
		//    注意：StopBatchUpdater + FlushAllBatches 内部是同步 DB 调用，不受
		//    shutdown_timeout 约束；DB 慢时这一步可能超出总退出时长，由 docker
		//    stop_grace_period 兜底
		if config.BatchUpdateEnabled {
			logger.SysLog("stopping batch updater before flush")
			model.StopBatchUpdater()
			logger.SysLog("flushing batch updates before exit")
			model.FlushAllBatches()
		}
		logger.SysLog("shutdown complete")
	}
}

func SyncChannelCache(frequency int) {
	// 只有 从 服务器端获取数据的时候才会用到
	if config.IsMasterNode {
		logger.SysLog("master node does't synchronize the channel")
		return
	}
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		logger.SysLog("syncing channels from database")
		model.ChannelGroup.Load()
		model.PricingInstance.Init()
		model.ModelOwnedBysInstance.Load()
		model.GlobalUserGroupRatio.Load()
	}
}

// initMemoryMonitor 初始化内存监控，定期记录内存使用情况
func initMemoryMonitor() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			heapMB := memStats.HeapAlloc / 1024 / 1024
			sysMB := memStats.Sys / 1024 / 1024
			numGoroutines := runtime.NumGoroutine()

			logger.SysLog(fmt.Sprintf("Memory: Heap=%dMB, Sys=%dMB, Goroutines=%d", heapMB, sysMB, numGoroutines))
		}
	}()
}
