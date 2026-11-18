package router

import (
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/media"
	"github.com/modeltaps/modeltaps/middleware"
	"embed"
	"fmt"
	"net/http"
	"net/http/pprof"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func SetRouter(router *gin.Engine, buildFS embed.FS, indexPage []byte) {
	// Generated media the gateway keeps for an hour (common/media); public, served on every node.
	router.GET("/media/:name", media.Serve)
	SetApiRouter(router)
	SetDashboardRouter(router)
	SetRelayRouter(router)
	// 初始化MCP服务器与Gin集成
	if config.MCP_ENABLE {
		logger.SysLog("Enable MCP Server")
		SetMcpRouter(router)
	}
	SetAdminMcpRouter(router)
	// 启用 pprof 调试端点
	if viper.GetBool("pprof_enabled") {
		logger.SysLog("Enable pprof debug endpoints at /debug/pprof/")
		SetPprofRouter(router)
	}
	frontendBaseUrl := viper.GetString("frontend_base_url")
	if config.IsMasterNode && frontendBaseUrl != "" {
		frontendBaseUrl = ""
		logger.SysLog("FRONTEND_BASE_URL is ignored on master node")
	}
	if frontendBaseUrl == "" {
		SetWebRouter(router, buildFS, indexPage)
	} else {
		frontendBaseUrl = strings.TrimSuffix(frontendBaseUrl, "/")
		toFrontend := func(c *gin.Context) {
			c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("%s%s", frontendBaseUrl, c.Request.RequestURI))
		}
		// 外置前端同样在兜底之前处理 /login 直达，不满足条件才转给前端
		RegisterLoginDirect(router, toFrontend)
		router.NoRoute(toFrontend)
	}
}

// SetPprofRouter registers the pprof debug routes. Profiles expose memory contents, command lines and
// goroutine stacks, so only the root user may read them: a root console session, or root's legacy
// access token in the Authorization header (management keys are refused).
func SetPprofRouter(router *gin.Engine) {
	pprofGroup := router.Group("/debug/pprof")
	pprofGroup.Use(middleware.NoCache(), middleware.RootAuth())
	{
		pprofGroup.GET("/", gin.WrapF(pprof.Index))
		pprofGroup.GET("/cmdline", gin.WrapF(pprof.Cmdline))
		pprofGroup.GET("/profile", gin.WrapF(pprof.Profile))
		pprofGroup.GET("/symbol", gin.WrapF(pprof.Symbol))
		pprofGroup.POST("/symbol", gin.WrapF(pprof.Symbol))
		pprofGroup.GET("/trace", gin.WrapF(pprof.Trace))
		pprofGroup.GET("/allocs", gin.WrapH(pprof.Handler("allocs")))
		pprofGroup.GET("/block", gin.WrapH(pprof.Handler("block")))
		pprofGroup.GET("/goroutine", gin.WrapH(pprof.Handler("goroutine")))
		pprofGroup.GET("/heap", gin.WrapH(pprof.Handler("heap")))
		pprofGroup.GET("/mutex", gin.WrapH(pprof.Handler("mutex")))
		pprofGroup.GET("/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
	}
}
