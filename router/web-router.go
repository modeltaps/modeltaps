package router

import (
	"github.com/modeltaps/modeltaps/controller"
	"github.com/modeltaps/modeltaps/middleware"
	"embed"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func SetWebRouter(router *gin.Engine, buildFS embed.FS, indexPage []byte) {
	router.Use(gzip.Gzip(gzip.DefaultCompression))

	embedFS, err := static.EmbedFolder(buildFS, "web/build")
	if err != nil {
		// 处理错误，可以选择记录日志或者 panic
		panic("Failed to create embedded file system: " + err.Error())
	}
	router.Use(skipStaticFiles(embedFS, middleware.GlobalWebRateLimit()))

	// 特别处理favicon.ico请求，设置缓存
	router.GET("/favicon.ico", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=3600") // 1小时缓存
		controller.Favicon(buildFS)(c)
	})

	// llms.txt：面向 AI 工具的机器可读说明，无需认证，需在静态资源中间件之前注册
	router.GET("/llms.txt", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=3600") // 1小时缓存
		controller.LlmsTxt(c)
	})

	router.Use(static.Serve("/", embedFS))

	spaIndex := func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexPage)
	}

	RegisterLoginDirect(router, spaIndex)

	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") {
			controller.RelayNotFound(c)
			return
		}
		// base_url 配错（如缺 /v1）的客户端会打到这里，若明显是 API 调用则回 JSON 404 并提示正确路径，
		// 避免返回 200+HTML 被 OpenAI 兼容客户端解析成"空流"。前端 SPA 刷新仍回 index.html。
		if controller.IsLikelyAPIRequest(c) {
			controller.RelayAPINotFound(c)
			return
		}
		spaIndex(c)
	})
}

// skipStaticFiles runs limit only for requests that static.Serve("/", fs) would not answer from
// the embedded build, so loading the JS/CSS/image bundle does not use up the web rate limit.
func skipStaticFiles(fs static.ServeFileSystem, limit gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if fs.Exists("/", c.Request.URL.Path) {
			return
		}
		limit(c)
	}
}

// RegisterLoginDirect 在 SPA 兜底之前拦下浏览器对 /login 的导航：站点已收敛为单一身份提供方时
// 直接 302 到 IdP 授权页，不再让前端渲染一张过渡页；不满足直达条件时交给 fallback 按原样返回前端。
// 只接 GET / HEAD——其余方法与非浏览器请求仍落到原来的兜底分支。
func RegisterLoginDirect(router *gin.Engine, fallback gin.HandlerFunc) {
	router.Match([]string{http.MethodGet, http.MethodHead}, "/login", func(c *gin.Context) {
		// 旧的管理员逃生口地址 ?local=1 永久跳转到 /login/admin：管理员靠记忆打开，地址要稳定。
		if c.Query("local") == "1" {
			c.Redirect(http.StatusMovedPermanently, controller.AdminLoginPath)
			return
		}
		if controller.LoginDirectRedirect(c) {
			return
		}
		fallback(c)
	})
}
