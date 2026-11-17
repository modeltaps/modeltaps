package router

import (
	"github.com/modeltaps/modeltaps/common/agentclient"
	"github.com/modeltaps/modeltaps/middleware"
	"github.com/modeltaps/modeltaps/relay"
	"github.com/modeltaps/modeltaps/relay/midjourney"
	"github.com/modeltaps/modeltaps/relay/task"
	"github.com/modeltaps/modeltaps/relay/task/kling"
	"github.com/modeltaps/modeltaps/relay/task/suno"

	"github.com/gin-gonic/gin"
)

func SetRelayRouter(router *gin.Engine) {
	router.Use(middleware.CORS())
	// 之后注册的中继路由都限制请求体大小（engine.Use 只影响之后注册的路由，不影响管理端 API）
	router.Use(middleware.RelayBodyLimit())
	// https://platform.openai.com/docs/api-reference/introduction
	setOpenAIRouter(router)
	setMJRouter(router)
	setSunoRouter(router)
	setClaudeRouter(router)
	setGeminiRouter(router)
	setRecraftRouter(router)
	setKlingRouter(router)
	setAgentClientRouter(router)
}

// openAIRoutes describes one mount of the OpenAI-shaped relay routes. The bare mount is /v1;
// the coding-agent client endpoints mount the same routes under their own prefix.
type openAIRoutes struct {
	prefix string
	// first runs before everything else on every route (the client endpoint marker).
	first []gin.HandlerFunc
	// listModels serves GET {prefix}/models.
	listModels gin.HandlerFunc
	// modelsAuth authenticates GET {prefix}/models and {prefix}/models/*model.
	modelsAuth gin.HandlerFunc
}

func setOpenAIRouter(router *gin.Engine) {
	registerOpenAIRoutes(router, openAIRoutes{prefix: "/v1", listModels: relay.ListModelsByToken, modelsAuth: middleware.OpenaiAuth()})
}

func registerOpenAIRoutes(router *gin.Engine, routes openAIRoutes) {
	modelsRouter := router.Group(routes.prefix + "/models")
	modelsRouter.Use(routes.first...)
	modelsRouter.Use(routes.modelsAuth, middleware.ContextUserId(), middleware.Distribute())
	{
		modelsRouter.GET("", routes.listModels)
		// 通配而非 :model：OpenRouter 风格别名（google/gemini-2.5-pro）含斜杠，单段参数会在路由层
		// 就 404。同名 DELETE 路由在另一棵方法树上，行为不受影响。
		modelsRouter.GET("/*model", relay.RetrieveModel)
	}
	relayV1Router := router.Group(routes.prefix)
	relayV1Router.Use(routes.first...)
	relayV1Router.Use(middleware.RelayPanicRecover(), middleware.OpenaiAuth(), middleware.ContextUserId(), middleware.Distribute(), middleware.DynamicRedisRateLimiter())
	{
		relayV1Router.POST("/completions", relay.Relay)
		relayV1Router.POST("/chat/completions", relay.Relay)
		relayV1Router.POST("/responses", relay.Relay)
		relayV1Router.POST("/responses/compact", relay.Relay)
		// relayV1Router.POST("/edits", controller.Relay)
		relayV1Router.POST("/images/generations", relay.Relay)
		relayV1Router.POST("/images/edits", relay.Relay)
		relayV1Router.POST("/images/variations", relay.Relay)
		relayV1Router.POST("/embeddings", relay.Relay)
		// relayV1Router.POST("/engines/:model/embeddings", controller.RelayEmbeddings)
		relayV1Router.POST("/audio/transcriptions", relay.Relay)
		relayV1Router.POST("/audio/translations", relay.Relay)
		relayV1Router.POST("/audio/speech", relay.Relay)
		relayV1Router.POST("/moderations", relay.Relay)
		relayV1Router.POST("/rerank", relay.RelayRerank)
		relayV1Router.POST("/decisions", relay.Relay)
		relayV1Router.GET("/realtime", relay.ChatRealtime)

		relayV1Router.Use(middleware.SpecifiedChannel())
		{
			relayV1Router.Any("/files", relay.RelayOnly)
			relayV1Router.Any("/files/*any", relay.RelayOnly)
			relayV1Router.Any("/fine_tuning/*any", relay.RelayOnly)
			relayV1Router.Any("/assistants", relay.RelayOnly)
			relayV1Router.Any("/assistants/*any", relay.RelayOnly)
			relayV1Router.Any("/threads", relay.RelayOnly)
			relayV1Router.Any("/threads/*any", relay.RelayOnly)
			relayV1Router.Any("/batches/*any", relay.RelayOnly)
			relayV1Router.Any("/vector_stores/*any", relay.RelayOnly)
			relayV1Router.DELETE("/models/:model", relay.RelayOnly)
		}
	}
}

func setMJRouter(router *gin.Engine) {
	relayMjRouter := router.Group("/mj")
	registerMjRouterGroup(relayMjRouter)

	relayMjModeRouter := router.Group("/:mode/mj")
	registerMjRouterGroup(relayMjModeRouter)
}

// Path: router/relay-router.go
func registerMjRouterGroup(relayMjRouter *gin.RouterGroup) {
	// Image links are opened without a key (img tags, shared links), so the route is limited per IP.
	relayMjRouter.GET("/image/:id", middleware.MJImageRateLimit(), midjourney.RelayMidjourneyImage)
	relayMjRouter.Use(middleware.RelayMJPanicRecover(), middleware.MjAuth(), middleware.ContextUserId(), middleware.Distribute(), middleware.DynamicRedisRateLimiter())
	{
		relayMjRouter.POST("/submit/action", midjourney.RelayMidjourney)
		relayMjRouter.POST("/submit/shorten", midjourney.RelayMidjourney)
		relayMjRouter.POST("/submit/modal", midjourney.RelayMidjourney)
		relayMjRouter.POST("/submit/imagine", midjourney.RelayMidjourney)
		relayMjRouter.POST("/submit/change", midjourney.RelayMidjourney)
		relayMjRouter.POST("/submit/simple-change", midjourney.RelayMidjourney)
		relayMjRouter.POST("/submit/describe", midjourney.RelayMidjourney)
		relayMjRouter.POST("/submit/blend", midjourney.RelayMidjourney)
		relayMjRouter.POST("/notify", midjourney.RelayMidjourney)
		relayMjRouter.GET("/task/:id/fetch", midjourney.RelayMidjourney)
		relayMjRouter.GET("/task/:id/image-seed", midjourney.RelayMidjourney)
		relayMjRouter.POST("/task/list-by-condition", midjourney.RelayMidjourney)
		relayMjRouter.POST("/insight-face/swap", midjourney.RelayMidjourney)
		relayMjRouter.POST("/submit/upload-discord-images", midjourney.RelayMidjourney)
	}
}

func setSunoRouter(router *gin.Engine) {
	relaySunoRouter := router.Group("/suno")
	relaySunoRouter.Use(middleware.RelaySunoPanicRecover(), middleware.OpenaiAuth(), middleware.ContextUserId(), middleware.Distribute(), middleware.DynamicRedisRateLimiter())
	{
		relaySunoRouter.POST("/submit/:action", task.RelayTaskSubmit)
		relaySunoRouter.POST("/fetch", suno.GetFetch)
		relaySunoRouter.GET("/fetch/:id", suno.GetFetchByID)
	}
}

// claudeRoutes describes one mount of the Anthropic Messages relay routes. The bare mount is
// /claude/v1; the coding-agent client endpoints mount the same routes under their own prefix.
type claudeRoutes struct {
	prefix string
	// first runs before everything else on every route (the client endpoint marker).
	first []gin.HandlerFunc
	// afterAuth runs after authentication, before the handler (client request normalization).
	afterAuth []gin.HandlerFunc
	// listModels serves GET {prefix}/models; nil leaves the path to another mount.
	listModels gin.HandlerFunc
}

func setClaudeRouter(router *gin.Engine) {
	registerClaudeRoutes(router, claudeRoutes{prefix: "/claude/v1", listModels: relay.ListClaudeModelsByToken})
}

func registerClaudeRoutes(router *gin.Engine, routes claudeRoutes) {
	relayV1Router := router.Group(routes.prefix)
	relayV1Router.Use(routes.first...)
	relayV1Router.Use(middleware.APIEnabled("claude"), middleware.RelayCluadePanicRecover(), middleware.ClaudeAuth(), middleware.ContextUserId(), middleware.Distribute(), middleware.DynamicRedisRateLimiter())
	relayV1Router.Use(routes.afterAuth...)
	{
		relayV1Router.POST("/messages", relay.Relay)
		relayV1Router.POST("/messages/count_tokens", relay.CountClaudeTokens)
		if routes.listModels != nil {
			relayV1Router.GET("/models", routes.listModels)
		}
	}
}

// setAgentClientRouter mounts the coding-agent client endpoints. The path names the calling
// client, not the protocol: the sub-path still picks the protocol, and every route is the bare
// route's handler chain with the client marker in front (middleware.AgentClientEndpoint), so
// auth, routing, billing and errors are those of the bare path. Only the model lists differ,
// in the shape each client's model picker parses.
//
//	/claude-code/v1/messages[/count_tokens], /models  Anthropic Messages, for Claude Code
//	/codex/v1/*                                        OpenAI (Responses and the rest), for Codex
//	/coding-agent/v1/*                                 OpenAI, any coding agent (traffic marker)
//	/coding-agent/v1/messages[/count_tokens]           Anthropic Messages, any coding agent
func setAgentClientRouter(router *gin.Engine) {
	registerClaudeRoutes(router, claudeRoutes{
		prefix:     "/claude-code/v1",
		first:      []gin.HandlerFunc{middleware.AgentClientEndpoint(agentclient.ClaudeCode, "/claude-code/v1", "/claude/v1")},
		afterAuth:  []gin.HandlerFunc{middleware.ClaudeCodeModelID()},
		listModels: relay.ListClaudeCodeModelsByToken,
	})

	registerOpenAIRoutes(router, openAIRoutes{
		prefix:     "/codex/v1",
		first:      []gin.HandlerFunc{middleware.AgentClientEndpoint(agentclient.Codex, "/codex/v1", "/v1")},
		listModels: relay.ListCodexModelsByToken,
		modelsAuth: middleware.OpenaiAuth(),
	})

	// /coding-agent/v1/models is shared by both protocols: it accepts x-api-key as well as a
	// bearer token, and answers an Anthropic client with the Anthropic list.
	registerOpenAIRoutes(router, openAIRoutes{
		prefix:     "/coding-agent/v1",
		first:      []gin.HandlerFunc{middleware.AgentClientEndpoint(agentclient.CodingAgent, "/coding-agent/v1", "/v1")},
		listModels: relay.ListCodingAgentModelsByToken,
		modelsAuth: middleware.ClaudeAuth(),
	})
	registerClaudeRoutes(router, claudeRoutes{
		prefix: "/coding-agent/v1",
		first:  []gin.HandlerFunc{middleware.AgentClientEndpoint(agentclient.CodingAgent, "/coding-agent/v1", "/claude/v1")},
	})
}

func setGeminiRouter(router *gin.Engine) {
	relayGeminiRouter := router.Group("/gemini")
	relayGeminiRouter.Use(middleware.APIEnabled("gemini"), middleware.RelayGeminiPanicRecover(), middleware.GeminiAuth(), middleware.ContextUserId(), middleware.Distribute(), middleware.DynamicRedisRateLimiter())
	{
		relayGeminiRouter.POST("/:version/models/*action", relay.Relay)
		relayGeminiRouter.GET("/:version/models", relay.ListGeminiModelsByToken)
	}
}

func setRecraftRouter(router *gin.Engine) {
	relayRecraftRouter := router.Group("/recraftAI/v1")
	relayRecraftRouter.Use(middleware.RelayPanicRecover(), middleware.OpenaiAuth(), middleware.ContextUserId(), middleware.Distribute(), middleware.DynamicRedisRateLimiter())
	{
		relayRecraftRouter.POST("/images/generations", relay.Relay)
		relayRecraftRouter.POST("/images/vectorize", relay.RelayRecraftAI)
		relayRecraftRouter.POST("/images/removeBackground", relay.RelayRecraftAI)
		relayRecraftRouter.POST("/images/clarityUpscale", relay.RelayRecraftAI)
		relayRecraftRouter.POST("/images/generativeUpscale", relay.RelayRecraftAI)
		relayRecraftRouter.POST("/styles", relay.RelayRecraftAI)
	}
}

func setKlingRouter(router *gin.Engine) {
	relayKlingRouter := router.Group("/kling")
	relayKlingRouter.Use(middleware.RelayKlingPanicRecover(), middleware.OpenaiAuth(), middleware.ContextUserId(), middleware.Distribute())
	relayKlingRouter.GET("/v1/videos/text2video/:id", kling.GetFetchByID)
	relayKlingRouter.GET("/v1/videos/image2video/:id", kling.GetFetchByID)

	relayKlingRouter.Use(middleware.DynamicRedisRateLimiter())
	{
		relayKlingRouter.POST("/v1/:class/:action", task.RelayTaskSubmit)
	}
}
