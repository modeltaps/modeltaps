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
