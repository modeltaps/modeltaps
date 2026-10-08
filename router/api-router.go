package router

import (
	"github.com/modeltaps/modeltaps/controller"
	"github.com/modeltaps/modeltaps/middleware"
	"github.com/modeltaps/modeltaps/relay"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func SetApiRouter(router *gin.Engine) {
	apiRouter := router.Group("/api")
	// 先设置通用中间件
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.SecurityHeaders()) // 添加安全头部
	apiRouter.Use(middleware.NoCache())         // 确保所有API接口都不缓存

	// metrics接口单独处理，不需要NoCache
	apiRouter.GET("/metrics", middleware.MetricsWithBasicAuth(), gin.WrapH(promhttp.Handler()))

	// 内嵌品牌图标：静态资源、带强缓存，不计入全局 API 限流
	apiRouter.GET("/brand-icon/manifest", controller.GetBrandIconManifest)
	apiRouter.HEAD("/brand-icon/manifest", controller.GetBrandIconManifest)
	apiRouter.GET("/brand-icon/:key", controller.GetBrandIcon)
	apiRouter.HEAD("/brand-icon/:key", controller.GetBrandIcon)

	systemInfo := apiRouter.Group("/system_info")
	systemInfo.Use(middleware.RootAuth())
	{
		systemInfo.POST("/log", controller.SystemLog)
		systemInfo.POST("/log/query", controller.SystemLogQuery)
	}

	apiRouter.POST("/telegram/:token", middleware.Telegram(), controller.TelegramBotWebHook)
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	{
		apiRouter.GET("/image/:id", controller.CheckImg)
		apiRouter.GET("/brand-icon/domain/:domain", controller.GetBrandIconByDomain)
		apiRouter.HEAD("/brand-icon/domain/:domain", controller.GetBrandIconByDomain)
		apiRouter.GET("/brand-icon/upload/:assetId", controller.GetBrandIconUpload)
		apiRouter.HEAD("/brand-icon/upload/:assetId", controller.GetBrandIconUpload)
		apiRouter.GET("/status", controller.GetStatus)
		apiRouter.GET("/notice", controller.GetNotice)
		apiRouter.GET("/about", controller.GetAbout)
		apiRouter.GET("/prices", middleware.PricesAuth(), middleware.CORS(), controller.GetPricesList)
		apiRouter.GET("/ownedby", relay.GetModelOwnedBy)
		apiRouter.GET("/available_model", middleware.CORS(), middleware.TrySetUserBySession(), relay.AvailableModel)
		apiRouter.GET("/user_group_map", middleware.TrySetUserBySession(), controller.GetUserGroupRatio)
		apiRouter.GET("/home_page_content", controller.GetHomePageContent)
		apiRouter.GET("/verification", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.SendEmailVerification)
		apiRouter.GET("/reset_password", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.SendPasswordResetEmail)
		apiRouter.POST("/user/reset", middleware.CriticalRateLimit(), controller.ResetPassword)
		apiRouter.GET("/oauth/github", middleware.CriticalRateLimit(), middleware.SessionSecurity(), controller.GitHubOAuth)
		apiRouter.GET("/oauth/lark", middleware.CriticalRateLimit(), middleware.SessionSecurity(), controller.LarkOAuth)
		apiRouter.GET("/oauth/state", middleware.CriticalRateLimit(), middleware.SessionSecurity(), controller.GenerateOAuthCode)
		apiRouter.POST("/oauth/invite_code", middleware.CriticalRateLimit(), middleware.SessionSecurity(), controller.SetOAuthInviteCode)
		apiRouter.GET("/oauth/wechat", middleware.CriticalRateLimit(), middleware.SessionSecurity(), controller.WeChatAuth)
		apiRouter.GET("/oauth/wechat/bind", middleware.CriticalRateLimit(), middleware.SessionSecurity(), middleware.UserAuth(), controller.WeChatBind)
		apiRouter.GET("/oauth/email/bind", middleware.CriticalRateLimit(), middleware.SessionSecurity(), middleware.UserAuth(), controller.EmailBind)

		// 无 slug 的两条保留为 slug=oidc 的别名：存量部署在 IdP 侧登记的回调地址就是它。
		apiRouter.GET("/oauth/endpoint", middleware.CriticalRateLimit(), middleware.SessionSecurity(), controller.OIDCEndpoint)
		apiRouter.GET("/oauth/endpoint/:slug", middleware.CriticalRateLimit(), middleware.SessionSecurity(), controller.OIDCEndpoint)
		apiRouter.GET("/oauth/oidc", middleware.CriticalRateLimit(), middleware.SessionSecurity(), controller.OIDCAuth)
		apiRouter.GET("/oauth/oidc/:slug", middleware.CriticalRateLimit(), middleware.SessionSecurity(), controller.OIDCAuth)

		apiRouter.GET("/oauth/linuxdo", middleware.CriticalRateLimit(), middleware.SessionSecurity(), controller.LinuxDoOAuth)
		apiRouter.GET("/oauth/linuxdo/bind", middleware.CriticalRateLimit(), middleware.SessionSecurity(), middleware.UserAuth(), controller.LinuxDoBind)

		webauthnGroup := apiRouter.Group("/webauthn")
		{
			// 注册相关
			webauthnGroup.POST("/registration/begin", middleware.UserAuth(), controller.WebauthnBeginRegistration)
			webauthnGroup.POST("/registration/finish", middleware.UserAuth(), controller.WebauthnFinishRegistration)

			// 登录相关
			webauthnGroup.POST("/login/begin", middleware.CriticalRateLimit(), controller.WebauthnBeginLogin)
			webauthnGroup.POST("/login/finish", middleware.CriticalRateLimit(), controller.WebauthnFinishLogin)

			// 凭据管理
			webauthnGroup.GET("/credentials", middleware.UserAuth(), controller.GetUserWebAuthnCredentials)
			webauthnGroup.DELETE("/credentials/:id", middleware.UserAuth(), controller.DeleteWebAuthnCredential)
		}

		apiRouter.Any("/payment/notify/:uuid", controller.PaymentCallback)

		userRoute := apiRouter.Group("/user")
		{
			userRoute.GET("/epay/notify", controller.EpayCallback)
			userRoute.POST("/register", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.Register)
			userRoute.POST("/login", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), middleware.SessionSecurity(), controller.Login)
			// 邮箱验证码登录：发码走人机验证与关键限流，验码只走限流（失败计数与密码登录共用账号锁）
			userRoute.POST("/login_code", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), middleware.SessionSecurity(), controller.SendLoginCode)
			userRoute.POST("/login/code", middleware.CriticalRateLimit(), middleware.SessionSecurity(), controller.LoginWithCode)
			userRoute.GET("/logout", middleware.SessionSecurity(), controller.Logout)

			selfRoute := userRoute.Group("/")
			selfRoute.Use(middleware.UserAuth())
			selfRoute.Use(middleware.SessionSecurity()) // 为所有用户相关接口添加会话安全
			{
				selfRoute.GET("/dashboard", controller.GetUserDashboard)
				selfRoute.GET("/dashboard/rate", controller.GetRateRealtime)
				selfRoute.GET("/dashboard/uptimekuma/status-page", controller.UptimeKumaStatusPage)
				selfRoute.GET("/dashboard/uptimekuma/status-page/heartbeat", controller.UptimeKumaStatusPageHeartbeat)
				selfRoute.GET("/invoice", controller.GetUserInvoice)
				selfRoute.GET("/invoice/detail", controller.GetUserInvoiceDetail)
				selfRoute.GET("/self", controller.GetSelf)
				selfRoute.PUT("/self", controller.UpdateSelf)
				selfRoute.GET("/self/analytics", controller.GetUserUsageAnalytics)
				selfRoute.GET("/setting", controller.GetUserSetting)
				selfRoute.PUT("/setting", controller.UpdateUserSetting)
				selfRoute.POST("/unbind", controller.Unbind)
				selfRoute.GET("/sessions", controller.GetUserSessions)
				selfRoute.DELETE("/sessions/:id", controller.RevokeUserSession)
				selfRoute.POST("/sessions/logout_others", controller.LogoutOtherSessions)
				// selfRoute.DELETE("/self", controller.DeleteSelf)
				selfRoute.GET("/token", controller.GenerateAccessToken)
				selfRoute.GET("/aff", controller.GetAffCode)
				selfRoute.POST("/topup", controller.TopUp)
				selfRoute.GET("/payment", controller.GetUserPaymentList)
				selfRoute.POST("/order", controller.CreateOrder)
				selfRoute.GET("/order", controller.GetUserOrderList)
				selfRoute.GET("/order/status", controller.CheckOrderStatus)
			}

			adminRoute := userRoute.Group("/")
			adminRoute.Use(middleware.AdminAuth())
			{
				adminRoute.GET("/", controller.GetUsersList)
				adminRoute.GET("/oidc_coverage", controller.GetOidcCoverage)
				adminRoute.GET("/stats", controller.GetUserStats)
				adminRoute.GET("/:id", controller.GetUser)
				adminRoute.POST("/", controller.CreateUser)
				adminRoute.POST("/manage", controller.ManageUser)
				adminRoute.POST("/quota/:id", controller.ChangeUserQuota)
				adminRoute.PUT("/", controller.UpdateUser)
				adminRoute.DELETE("/:id", controller.DeleteUser)
			}
		}
		optionRoute := apiRouter.Group("/option")
		optionRoute.Use(middleware.RootAuth())
		{
			optionRoute.GET("/", controller.GetOptions)
			optionRoute.PUT("/", controller.UpdateOption)
			optionRoute.GET("/telegram", controller.GetTelegramMenuList)
			optionRoute.POST("/telegram", controller.AddOrUpdateTelegramMenu)
			optionRoute.GET("/telegram/status", controller.GetTelegramBotStatus)
			optionRoute.PUT("/telegram/reload", controller.ReloadTelegramBot)
			optionRoute.GET("/telegram/:id", controller.GetTelegramMenu)
			optionRoute.DELETE("/telegram/:id", controller.DeleteTelegramMenu)
			optionRoute.GET("/safe_tools", controller.GetSafeTools)
			optionRoute.POST("/invoice/gen/:time", controller.GenInvoice)
			optionRoute.POST("/invoice/update/:time", controller.UpdateInvoice)
			optionRoute.POST("/system_info/log", controller.SystemLog)
		}

		inviteCodeRoute := apiRouter.Group("/invite-code")
		inviteCodeRoute.Use(middleware.AdminAuth())
		{
			inviteCodeRoute.GET("/", controller.GetInviteCodesList)
			inviteCodeRoute.GET("/generate", controller.GenerateRandomInviteCode)
			inviteCodeRoute.GET("/:id", controller.GetInviteCode)
			inviteCodeRoute.POST("/", controller.CreateInviteCode)
			inviteCodeRoute.PUT("/:id", controller.UpdateInviteCode)
			inviteCodeRoute.DELETE("/:id", controller.DeleteInviteCode)
			inviteCodeRoute.POST("/batch-delete", controller.BatchDeleteInviteCodes)
		}

		oidcProviderRoute := apiRouter.Group("/oidc_provider")
		oidcProviderRoute.Use(middleware.AdminAuth())
		{
			oidcProviderRoute.GET("/", controller.GetOidcProvidersList)
			oidcProviderRoute.GET("/:id", controller.GetOidcProvider)
			oidcProviderRoute.POST("/", controller.CreateOidcProvider)
			oidcProviderRoute.PUT("/:id", controller.UpdateOidcProvider)
			oidcProviderRoute.PUT("/:id/status", controller.UpdateOidcProviderStatus)
			oidcProviderRoute.POST("/:id/test", controller.TestOidcProvider)
			oidcProviderRoute.DELETE("/:id", controller.DeleteOidcProvider)
		}

		modelOwnedByRoute := apiRouter.Group("/model_ownedby")
		modelOwnedByRoute.GET("/", controller.GetAllModelOwnedBy)
		modelOwnedByRoute.Use(middleware.AdminAuth())
		{
			modelOwnedByRoute.GET("/:id", controller.GetModelOwnedBy)
			modelOwnedByRoute.POST("/", controller.CreateModelOwnedBy)
			modelOwnedByRoute.PUT("/", controller.UpdateModelOwnedBy)
			modelOwnedByRoute.DELETE("/:id", controller.DeleteModelOwnedBy)
		}

		brandIconAdminRoute := apiRouter.Group("/brand-icon")
		brandIconAdminRoute.Use(middleware.AdminAuth())
		{
			brandIconAdminRoute.POST("/refresh", controller.RefreshBrandIcon)
			brandIconAdminRoute.POST("/upload", controller.UploadBrandIcon)
		}

		modelInfoRoute := apiRouter.Group("/model_info")
		// 公开价格页（匿名 /price）与用户面板按模型名取元信息，故列表保持公开；
		// 处理器无条件只返回有渠道可路由、未隐藏的主名，且不含管理字段
		modelInfoRoute.GET("/", controller.GetAllModelInfo)
		modelInfoRoute.Use(middleware.AdminAuth())
		{
			modelInfoRoute.GET("/catalog", controller.GetModelInfoCatalog)
			modelInfoRoute.POST("/check_consistency", controller.CheckModelInfoConsistency)
			modelInfoRoute.POST("/hide", controller.HideModelInfo)
			modelInfoRoute.POST("/apply_seed", controller.ApplyModelInfoSeed)
			modelInfoRoute.GET("/:id", controller.GetModelInfo)
			modelInfoRoute.POST("/", controller.CreateModelInfo)
			modelInfoRoute.PUT("/", controller.UpdateModelInfo)
			modelInfoRoute.DELETE("/:id", controller.DeleteModelInfo)
		}

		userGroup := apiRouter.Group("/user_group")
		userGroup.Use(middleware.AdminAuth())
		{
			userGroup.GET("/", controller.GetUserGroups)
			userGroup.GET("/:id", controller.GetUserGroupById)
			userGroup.POST("/", controller.AddUserGroup)
			userGroup.PUT("/enable/:id", controller.ChangeUserGroupEnable)
			userGroup.PUT("/", controller.UpdateUserGroup)
			userGroup.DELETE("/:id", controller.DeleteUserGroup)

		}
		channelRoute := apiRouter.Group("/channel")
		channelRoute.Use(middleware.AdminAuth())
		{
			channelRoute.GET("/", controller.GetChannelsList)
			channelRoute.GET("/models", relay.ListModelsForAdmin)
			channelRoute.POST("/provider_models_list", controller.GetModelList)
			channelRoute.POST("/check_drift", controller.CheckModelDrift)
			channelRoute.POST("/dismiss_new_models", controller.DismissNewModels)
			channelRoute.POST("/sync_pricing", controller.SyncChannelPricing)
			channelRoute.GET("/:id", controller.GetChannel)
			channelRoute.GET("/test", controller.TestAllChannels)
			channelRoute.GET("/test/:id", controller.TestChannel)
			channelRoute.GET("/update_balance", controller.UpdateAllChannelsBalance)
			channelRoute.GET("/update_balance/:id", controller.UpdateChannelBalance)
			channelRoute.POST("/", controller.AddChannel)
			channelRoute.PUT("/", controller.UpdateChannel)
			channelRoute.PUT("/batch/azure_api", controller.BatchUpdateChannelsAzureApi)
			channelRoute.PUT("/batch/del_model", controller.BatchDelModelChannels)
			channelRoute.PUT("/batch/add_model", controller.BatchAddModelToChannels)
			channelRoute.PUT("/batch/add_user_group", controller.BatchAddUserGroupToChannels)
		}

		// GeminiCli OAuth routes (no auth required for callback)
		geminiCliRoute := apiRouter.Group("/geminicli")
		{
			geminiCliRoute.POST("/oauth/start", middleware.AdminAuth(), controller.StartGeminiCliOAuth)
			geminiCliRoute.GET("/oauth/callback", controller.GeminiCliOAuthCallback)
			geminiCliRoute.GET("/oauth/status/:state", middleware.AdminAuth(), controller.GetGeminiCliOAuthStatus)
			channelRoute.DELETE("/disabled", controller.DeleteDisabledChannel)
			channelRoute.DELETE("/:id/tag", controller.DeleteChannelTag)
			channelRoute.DELETE("/:id", controller.DeleteChannel)
			channelRoute.DELETE("/batch", controller.BatchDeleteChannel)
		}

		// ClaudeCode OAuth routes
		claudeCodeRoute := apiRouter.Group("/claudecode")
		claudeCodeRoute.Use(middleware.AdminAuth())
		{
			claudeCodeRoute.POST("/oauth/start", controller.StartClaudeCodeOAuth)
			claudeCodeRoute.POST("/oauth/exchange-code", controller.ClaudeCodeOAuthCallback)
		}

		// Codex OAuth routes
		codexRoute := apiRouter.Group("/codex")
		codexRoute.Use(middleware.AdminAuth())
		{
			codexRoute.POST("/oauth/start", controller.StartCodexOAuth)
			codexRoute.POST("/oauth/exchange-code", controller.CodexOAuthCallback)
		}

		// Antigravity OAuth routes
		antigravityRoute := apiRouter.Group("/antigravity")
		{
			antigravityRoute.POST("/oauth/start", middleware.AdminAuth(), controller.StartAntigravityOAuth)
			antigravityRoute.GET("/oauth/callback", controller.AntigravityOAuthCallback)
			antigravityRoute.GET("/oauth/status/:state", middleware.AdminAuth(), controller.GetAntigravityOAuthStatus)
		}

		channelTagRoute := apiRouter.Group("/channel_tag")
		channelTagRoute.Use(middleware.AdminAuth())
		{
			channelTagRoute.GET("/_all", controller.GetChannelsTagAllList)
			channelTagRoute.GET("/:tag/list", controller.GetChannelsTagList)
			channelTagRoute.GET("/:tag", controller.GetChannelsTag)
			channelTagRoute.PUT("/:tag", controller.UpdateChannelsTag)
			channelTagRoute.DELETE("/:tag", controller.DeleteChannelsTag)
			channelTagRoute.DELETE("/:tag/disabled", controller.DeleteDisabledChannelsTag)
			channelTagRoute.PUT("/:tag/priority", controller.UpdateChannelsTagPriority)
			channelTagRoute.PUT("/:tag/status/:status", controller.ChangeChannelsTagStatus)

		}

		tokenRoute := apiRouter.Group("/token")
		tokenRoute.Use(middleware.UserAuth())
		{
			tokenRoute.GET("/playground", controller.GetPlaygroundToken)
			tokenRoute.GET("/", controller.GetUserTokensList)
			tokenRoute.GET("/:id", controller.GetToken)
			tokenRoute.POST("/", controller.AddToken)
			tokenRoute.PUT("/", controller.UpdateToken)
			tokenRoute.DELETE("/:id", controller.DeleteToken)
		}
		tokenAdminRoute := apiRouter.Group("/token")
		tokenAdminRoute.Use(middleware.AdminAuth())
		{
			tokenAdminRoute.GET("/admin/search", controller.GetTokensListByAdmin)
			tokenAdminRoute.PUT("/admin", controller.UpdateTokenByAdmin)
			tokenAdminRoute.DELETE("/admin/:id", controller.DeleteTokenByAdmin)
		}
		redemptionRoute := apiRouter.Group("/redemption")
		redemptionRoute.Use(middleware.AdminAuth())
		{
			redemptionRoute.GET("/", controller.GetRedemptionsList)
			redemptionRoute.GET("/:id", controller.GetRedemption)
			redemptionRoute.POST("/", controller.AddRedemption)
			redemptionRoute.PUT("/", controller.UpdateRedemption)
			redemptionRoute.DELETE("/:id", controller.DeleteRedemption)
		}
		logRoute := apiRouter.Group("/log")
		{
			logRoute.GET("/", middleware.AdminAuth(), controller.GetLogsList)
			logRoute.GET("/export", middleware.AdminAuth(), controller.ExportLogsList)
			logRoute.DELETE("/", middleware.AdminAuth(), controller.DeleteHistoryLogs)
			logRoute.GET("/stat", middleware.AdminAuth(), controller.GetLogsStat)
			logRoute.GET("/self/stat", middleware.UserAuth(), controller.GetLogsSelfStat)
			logRoute.GET("/histogram", middleware.AdminAuth(), controller.GetLogsHistogram)
			logRoute.GET("/self/histogram", middleware.UserAuth(), controller.GetLogsSelfHistogram)
			// logRoute.GET("/search", middleware.AdminAuth(), controller.SearchAllLogs)
			logRoute.GET("/self", middleware.UserAuth(), controller.GetUserLogsList)
			logRoute.GET("/self/export", middleware.UserAuth(), controller.ExportUserLogsList)
			// 完整请求/响应明细(T50f):以 CanViewTokenLogIO 越权拒绝;/self 在组织上下文经 orgScope 改写到 /api/org/:id/logs/detail
			logRoute.GET("/detail/:id", middleware.AdminAuth(), controller.GetLogDetail)
			logRoute.GET("/self/detail/:id", middleware.UserAuth(), controller.GetLogDetail)
			// logRoute.GET("/self/search", middleware.UserAuth(), controller.SearchUserLogs)
		}
		groupRoute := apiRouter.Group("/group")
		groupRoute.Use(middleware.AdminAuth())
		{
			groupRoute.GET("/", controller.GetGroups)
		}

		analyticsRoute := apiRouter.Group("/analytics")
		analyticsRoute.Use(middleware.AdminAuth())
		{
			analyticsRoute.GET("/statistics", controller.GetStatisticsDetail)
			analyticsRoute.GET("/period", controller.GetStatisticsByPeriod)
			analyticsRoute.GET("/multi_user_stats", controller.GetMultiUserStatistics)
			analyticsRoute.GET("/multi_user_stats/export", controller.ExportMultiUserStatisticsCSV)
			analyticsRoute.GET("/recharge", controller.GetRechargeStatisticsByTimeRange)
		}
		pricesRoute := apiRouter.Group("/prices")
		pricesRoute.Use(middleware.AdminAuth())
		{
			pricesRoute.GET("/model_list", controller.GetAllModelList)
			pricesRoute.POST("/single", controller.AddPrice)
			pricesRoute.PUT("/single/*model", controller.UpdatePrice)
			pricesRoute.DELETE("/single/*model", controller.DeletePrice)
			pricesRoute.POST("/multiple", controller.BatchSetPrices)
			pricesRoute.PUT("/multiple/delete", controller.BatchDeletePrices)
			pricesRoute.POST("/sync", controller.SyncPricing)
			pricesRoute.POST("/sync_catalog", controller.SyncCatalogPricing)
			pricesRoute.GET("/updateService", controller.GetUpdatePriceService)
			pricesRoute.GET("/modelsdev", controller.GetPricesFromModelsDev)

		}

		paymentRoute := apiRouter.Group("/payment")
		paymentRoute.Use(middleware.AdminAuth())
		{
			paymentRoute.GET("/order", controller.GetOrderList)
			paymentRoute.GET("/", controller.GetPaymentList)
			paymentRoute.GET("/:id", controller.GetPayment)
			paymentRoute.POST("/", controller.AddPayment)
			paymentRoute.PUT("/", controller.UpdatePayment)
			paymentRoute.DELETE("/:id", controller.DeletePayment)
		}

		mjRoute := apiRouter.Group("/mj")
		mjRoute.GET("/self", middleware.UserAuth(), controller.GetUserMidjourney)
		mjRoute.GET("/", middleware.AdminAuth(), controller.GetAllMidjourney)

		taskRoute := apiRouter.Group("/task")
		taskRoute.GET("/self", middleware.UserAuth(), controller.GetUserAllTask)
		taskRoute.GET("/", middleware.AdminAuth(), controller.GetAllTask)

		// 组织功能路由(见 org-router.go,T3/T4 在其中追加)
		setOrgRouter(apiRouter)

		// 站点级组织管理后台(T7,仅 root)
		organizationAdminRoute := apiRouter.Group("/organization")
		organizationAdminRoute.Use(middleware.RootAuth())
		{
			organizationAdminRoute.GET("/", controller.AdminGetOrganizationsList)
			organizationAdminRoute.GET("/:id", controller.AdminGetOrganization)
			organizationAdminRoute.PUT("/:id/status", controller.AdminSetOrganizationStatus)
			organizationAdminRoute.PUT("/:id/quota", controller.AdminChangeOrganizationQuota)
			organizationAdminRoute.DELETE("/:id", controller.AdminDissolveOrganization)
		}
	}

	sseRouter := router.Group("/api/sse")
	sseRouter.Use(middleware.GlobalAPIRateLimit())
	{
		sseRouter.POST("/channel/check", middleware.AdminAuth(), controller.CheckChannel)
	}

}
