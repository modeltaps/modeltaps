package router

import (
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/mcp"
	"github.com/modeltaps/modeltaps/middleware"
)

func SetMcpRouter(router *gin.Engine) {
	mcpServer := mcp.NewMcpServer()
	mcpServer.RegisterTools()

	mcpRouter := router.Group("/mcp")
	// MCP is registered after the relay routes, so RelayBodyLimit runs too; the smaller cap wins.
	mcpRouter.Use(middleware.BodyLimit(apiBodyLimit()), middleware.BodyReadTimeout(apiBodyReadTimeout()))
	mcpRouter.Use(middleware.CORS())
	mcpRouter.Use(middleware.GlobalAPIRateLimit())
	mcpRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	// CORS lets any origin send credentials; a console session must still come from this site.
	mcpRouter.Use(middleware.CrossSiteRequestGuard())
	mcpRouter.Use(middleware.UserAuth())
	mcpRouter.Use(middleware.ContextUserId())
	{
		// Clients authenticate with an Authorization: Bearer <access token> header. The old
		// /mcp/<token> forms are gone: a token in the path lands in proxy and access logs.
		mcpRouter.POST("", mcpServer.HandleStreamable)
		mcpRouter.GET("/sse", mcpServer.HandleSSE)
		mcpRouter.POST("/message", mcpServer.HandleMessage)
	}

	go func() {
		err := mcpServer.SSEServer.Run()
		if err != nil {
			logger.SysError("mcp sse server error: " + err.Error())
			return
		}
		err = mcpServer.StreamableServer.Run()
		if err != nil {
			logger.SysError("mcp streamable server error: " + err.Error())
			return
		}
	}()

}

// SetAdminMcpRouter serves the admin MCP endpoint: the settings tools, for root only (the same
// permission as the platform settings). A read management key of a root user may call it. It is
// served whether or not the user MCP endpoint (MCP_ENABLE) is on: it only reads, and only root
// gets in.
func SetAdminMcpRouter(router *gin.Engine) {
	adminServer := mcp.NewAdminMcpServer()
	if adminServer == nil {
		return
	}
	adminRouter := router.Group("/mcp/admin")
	adminRouter.Use(middleware.BodyLimit(apiBodyLimit()), middleware.BodyReadTimeout(apiBodyReadTimeout()))
	adminRouter.Use(middleware.CORS())
	adminRouter.Use(middleware.GlobalAPIRateLimit())
	adminRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	adminRouter.Use(middleware.CrossSiteRequestGuard())
	adminRouter.Use(middleware.RootAuth())
	adminRouter.Use(middleware.ContextUserId())
	{
		adminRouter.POST("", adminServer.HandleStreamable)
		adminRouter.GET("/sse", adminServer.HandleSSE)
		adminRouter.POST("/message", adminServer.HandleMessage)
	}
	go func() {
		if err := adminServer.SSEServer.Run(); err != nil {
			logger.SysError("admin mcp sse server error: " + err.Error())
			return
		}
		if err := adminServer.StreamableServer.Run(); err != nil {
			logger.SysError("admin mcp streamable server error: " + err.Error())
		}
	}()
}
