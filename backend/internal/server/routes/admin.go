// Package routes provides HTTP route registration and handlers.
package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterAdminRoutes 注册管理员路由
func RegisterAdminRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	adminAuth middleware.AdminAuthMiddleware,
	auditLog middleware.AuditLogMiddleware,
	stepUpAuth middleware.StepUpAuthMiddleware,
	settingService *service.SettingService,
	panelRateLimiter *middleware.PanelRateLimiter,
) {

	admin := v1.Group("/admin")
	admin.Use(gin.HandlerFunc(adminAuth))
	// 面板全局按用户限流（默认管理员豁免，可在系统设置中关闭豁免）
	admin.Use(panelRateLimiter.Global())
	// 审计中间件挂在认证之后：所有管理面变更类操作 + 敏感读取入审计日志
	admin.Use(gin.HandlerFunc(auditLog))
	{
		// 仪表盘
		registerDashboardRoutes(admin, h)

		// 用户管理
		registerUserManagementRoutes(admin, h)

		// 分组管理
		registerGroupRoutes(admin, h)

		// 账号管理
		registerAccountRoutes(admin, h, stepUpAuth)

		// 公告管理
		registerAnnouncementRoutes(admin, h)

		// OpenAI OAuth
		registerOpenAIOAuthRoutes(admin, h)

		// Gemini OAuth
		registerGeminiOAuthRoutes(admin, h)

		// Antigravity OAuth
		registerAntigravityOAuthRoutes(admin, h)

		// Grok OAuth
		registerGrokOAuthRoutes(admin, h)

		// 国产供应商（kimi/zhipu/deepseek）额度与余额
		registerCNProviderRoutes(admin, h)

		// 代理管理
		registerProxyRoutes(admin, h, stepUpAuth)

		// 卡密管理
		registerRedeemCodeRoutes(admin, h)

		// 系统设置
		registerSettingsRoutes(admin, h)

		// 异步生图对象存储
		registerImageStorageRoutes(admin, h, stepUpAuth)

		// 运维监控（Ops）
		registerOpsRoutes(admin, h)

		// 系统管理
		registerSystemRoutes(admin, h)

		// 订阅管理
		registerSubscriptionRoutes(admin, h)

		// 使用记录管理
		registerUsageRoutes(admin, h)

		// 用户属性管理
		registerUserAttributeRoutes(admin, h)

		// 错误透传规则管理
		registerErrorPassthroughRoutes(admin, h)

		// TLS 指纹模板管理
		registerTLSFingerprintProfileRoutes(admin, h)

		// API Key 管理
		registerAdminAPIKeyRoutes(admin, h)

		// 定时测试计划
		registerScheduledTestRoutes(admin, h)

		// 模型目录
		registerModelCatalogRoutes(admin, h)

		// 渠道监控
		registerChannelMonitorRoutes(admin, h, settingService)
		// 渠道状态（按渠道看可用率 / 首字延迟 / 缓存命中率，数据与用户站服务状态同源）
		admin.GET("/channel-status", h.ChannelMonitorV2.AdminChannels)

		// 风控中心
		registerContentModerationRoutes(admin, h)

		// 独立提示词输入审计
		registerPromptAuditRoutes(admin, h)

		// 邀请返利（专属用户管理）
		registerAffiliateRoutes(admin, h)
	}
}

func registerPromptAuditRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	promptAudit := admin.Group("/prompt-audit")
	{
		promptAudit.GET("/config", h.Admin.PromptAudit.GetConfig)
		promptAudit.PUT("/config", h.Admin.PromptAudit.UpdateConfig)
		promptAudit.POST("/endpoints/probe", h.Admin.PromptAudit.ProbeEndpoint)
		promptAudit.GET("/runtime", h.Admin.PromptAudit.GetRuntime)
		promptAudit.GET("/events", h.Admin.PromptAudit.ListEvents)
		promptAudit.GET("/events/:id", h.Admin.PromptAudit.GetEvent)
		promptAudit.DELETE("/events/:id", h.Admin.PromptAudit.DeleteEvent)
		promptAudit.POST("/events/batch-delete", h.Admin.PromptAudit.BatchDelete)
		promptAudit.POST("/events/delete-preview", h.Admin.PromptAudit.DeletePreview)
		promptAudit.POST("/events/delete-by-filter", h.Admin.PromptAudit.DeleteByFilter)
	}
}

func registerContentModerationRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	risk := admin.Group("/risk-control")
	{
		risk.GET("/config", h.Admin.ContentModeration.GetConfig)
		risk.PUT("/config", h.Admin.ContentModeration.UpdateConfig)
		risk.POST("/api-keys/test", h.Admin.ContentModeration.TestAPIKeys)
		risk.GET("/status", h.Admin.ContentModeration.GetStatus)
		risk.GET("/logs", h.Admin.ContentModeration.ListLogs)
		risk.POST("/users/:user_id/unban", h.Admin.ContentModeration.UnbanUser)
		risk.DELETE("/hashes", h.Admin.ContentModeration.DeleteFlaggedHash)
		risk.DELETE("/hashes/all", h.Admin.ContentModeration.ClearFlaggedHashes)
	}
}

func registerAdminAPIKeyRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	apiKeys := admin.Group("/api-keys")
	{
		apiKeys.PUT("/:id", h.Admin.APIKey.UpdateKey)
	}
}

func registerOpsRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	ops := admin.Group("/ops")
	{
		// Realtime ops signals
		ops.GET("/concurrency", h.Admin.Ops.GetConcurrencyStats)
		ops.GET("/user-concurrency", h.Admin.Ops.GetUserConcurrencyStats)
		ops.GET("/account-availability", h.Admin.Ops.GetAccountAvailability)
		ops.GET("/realtime-traffic", h.Admin.Ops.GetRealtimeTrafficSummary)

		// Alerts (rules + events)
		ops.GET("/alert-rules", h.Admin.Ops.ListAlertRules)
		ops.POST("/alert-rules", h.Admin.Ops.CreateAlertRule)
		ops.PUT("/alert-rules/:id", h.Admin.Ops.UpdateAlertRule)
		ops.DELETE("/alert-rules/:id", h.Admin.Ops.DeleteAlertRule)
		ops.GET("/alert-events", h.Admin.Ops.ListAlertEvents)
		ops.GET("/alert-events/:id", h.Admin.Ops.GetAlertEvent)
		ops.PUT("/alert-events/:id/status", h.Admin.Ops.UpdateAlertEventStatus)
		ops.POST("/alert-silences", h.Admin.Ops.CreateAlertSilence)

		// Email notification config (DB-backed)
		ops.GET("/email-notification/config", h.Admin.Ops.GetEmailNotificationConfig)
		ops.PUT("/email-notification/config", h.Admin.Ops.UpdateEmailNotificationConfig)

		// Runtime settings (DB-backed)
		runtime := ops.Group("/runtime")
		{
			runtime.GET("/alert", h.Admin.Ops.GetAlertRuntimeSettings)
			runtime.PUT("/alert", h.Admin.Ops.UpdateAlertRuntimeSettings)
			runtime.GET("/logging", h.Admin.Ops.GetRuntimeLogConfig)
			runtime.PUT("/logging", h.Admin.Ops.UpdateRuntimeLogConfig)
			runtime.POST("/logging/reset", h.Admin.Ops.ResetRuntimeLogConfig)
		}

		// Advanced settings (DB-backed)
		ops.GET("/advanced-settings", h.Admin.Ops.GetAdvancedSettings)
		ops.PUT("/advanced-settings", h.Admin.Ops.UpdateAdvancedSettings)

		// Settings group (DB-backed)
		settings := ops.Group("/settings")
		{
			settings.GET("/metric-thresholds", h.Admin.Ops.GetMetricThresholds)
			settings.PUT("/metric-thresholds", h.Admin.Ops.UpdateMetricThresholds)
		}

		// WebSocket realtime (QPS/TPS)
		ws := ops.Group("/ws")
		{
			ws.GET("/qps", h.Admin.Ops.QPSWSHandler)
		}

		// Error logs (legacy)
		ops.GET("/errors", h.Admin.Ops.GetErrorLogs)
		ops.GET("/errors/:id", h.Admin.Ops.GetErrorLogByID)
		ops.PUT("/errors/:id/resolve", h.Admin.Ops.UpdateErrorResolution)

		// Request errors (client-visible failures)
		ops.GET("/request-errors", h.Admin.Ops.ListRequestErrors)
		ops.GET("/request-errors/:id", h.Admin.Ops.GetRequestError)
		ops.GET("/request-errors/:id/upstream-errors", h.Admin.Ops.ListRequestErrorUpstreamErrors)
		ops.PUT("/request-errors/:id/resolve", h.Admin.Ops.ResolveRequestError)

		// Bounded ingress-admission rejection aggregates.
		ops.GET("/ingress-rejections", h.Admin.Ops.ListIngressRejects)
		ops.GET("/ingress-rejections/health", h.Admin.Ops.GetIngressRejectHealth)
		ops.GET("/auth-cache-invalidation/health", h.Admin.Ops.GetAuthCacheInvalidationHealth)

		// Upstream errors (independent upstream failures)
		ops.GET("/upstream-errors", h.Admin.Ops.ListUpstreamErrors)
		ops.GET("/upstream-errors/:id", h.Admin.Ops.GetUpstreamError)
		ops.PUT("/upstream-errors/:id/resolve", h.Admin.Ops.ResolveUpstreamError)

		// Request drilldown (success + error)
		ops.GET("/requests", h.Admin.Ops.ListRequestDetails)

		// Indexed system logs
		ops.GET("/system-logs", h.Admin.Ops.ListSystemLogs)
		ops.POST("/system-logs/cleanup", h.Admin.Ops.CleanupSystemLogs)
		ops.GET("/system-logs/health", h.Admin.Ops.GetSystemLogIngestionHealth)

		// Dashboard (vNext - raw path for MVP)
		ops.GET("/dashboard/snapshot-v2", h.Admin.Ops.GetDashboardSnapshotV2)
		ops.GET("/dashboard/overview", h.Admin.Ops.GetDashboardOverview)
		ops.GET("/dashboard/throughput-trend", h.Admin.Ops.GetDashboardThroughputTrend)
		ops.GET("/dashboard/latency-histogram", h.Admin.Ops.GetDashboardLatencyHistogram)
		ops.GET("/dashboard/error-trend", h.Admin.Ops.GetDashboardErrorTrend)
		ops.GET("/dashboard/error-distribution", h.Admin.Ops.GetDashboardErrorDistribution)
		ops.GET("/dashboard/openai-token-stats", h.Admin.Ops.GetDashboardOpenAITokenStats)
	}
}

func registerDashboardRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	dashboard := admin.Group("/dashboard")
	{
		dashboard.GET("/snapshot-v2", h.Admin.Dashboard.GetSnapshotV2)
		dashboard.GET("/stats", h.Admin.Dashboard.GetStats)
		dashboard.GET("/realtime", h.Admin.Dashboard.GetRealtimeMetrics)
		dashboard.GET("/trend", h.Admin.Dashboard.GetUsageTrend)
		dashboard.GET("/models", h.Admin.Dashboard.GetModelStats)
		dashboard.GET("/api-keys-trend", h.Admin.Dashboard.GetAPIKeyUsageTrend)
		dashboard.POST("/users-usage", h.Admin.Dashboard.GetBatchUsersUsage)
		dashboard.POST("/api-keys-usage", h.Admin.Dashboard.GetBatchAPIKeysUsage)
		dashboard.GET("/user-breakdown", h.Admin.Dashboard.GetUserBreakdown)
		dashboard.POST("/aggregation/backfill", h.Admin.Dashboard.BackfillAggregation)
	}
}

func registerUserManagementRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	users := admin.Group("/users")
	{
		users.GET("", h.Admin.User.List)
		users.GET("/defaults", h.Admin.User.GetNewUserDefaults)
		users.GET("/:id", h.Admin.User.GetByID)
		users.POST("/:id/auth-identities", h.Admin.User.BindAuthIdentity)
		users.POST("", h.Admin.User.Create)
		users.PUT("/:id", h.Admin.User.Update)
		users.DELETE("/:id", h.Admin.User.Delete)
		users.POST("/:id/balance", h.Admin.User.UpdateBalance)
		users.GET("/:id/api-keys", h.Admin.User.GetUserAPIKeys)
		users.GET("/:id/usage", h.Admin.User.GetUserUsage)
		users.GET("/:id/balance-history", h.Admin.User.GetBalanceHistory)
		users.GET("/:id/rpm-status", h.Admin.User.GetUserRPMStatus)
		users.POST("/batch-concurrency", h.Admin.User.BatchUpdateConcurrency)
		users.POST("/batch-limits", h.Admin.User.BatchUpdateLimits)

		// User attribute values
		users.GET("/:id/attributes", h.Admin.UserAttribute.GetUserAttributes)
		users.PUT("/:id/attributes", h.Admin.UserAttribute.UpdateUserAttributes)
	}
}

func registerGroupRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
}

func registerAccountRoutes(admin *gin.RouterGroup, h *handler.Handlers, stepUpAuth middleware.StepUpAuthMiddleware) {
	accounts := admin.Group("/accounts")
	{
		accounts.GET("", h.Admin.Account.List)
		accounts.GET("/protocol-defaults", h.Admin.Account.GetProtocolDefaults)
		accounts.GET("/:id", h.Admin.Account.GetByID)
		accounts.POST("", h.Admin.Account.Create)
		accounts.POST("/:id/duplicate", h.Admin.Account.Duplicate)
		accounts.POST("/import/codex-session", h.Admin.Account.ImportCodexSession)
		accounts.PUT("/:id", h.Admin.Account.Update)

		accounts.GET("/:id/ollama-cloud-usage", h.Admin.Account.GetOllamaCloudUsage)
		accounts.PUT("/:id/ollama-cloud-usage/session", h.Admin.Account.SaveOllamaCloudUsageSession)
		accounts.DELETE("/:id/ollama-cloud-usage/session", h.Admin.Account.DeleteOllamaCloudUsageSession)
		accounts.POST("/:id/ollama-cloud-usage/refresh", h.Admin.Account.RefreshOllamaCloudUsage)
		accounts.DELETE("/:id", h.Admin.Account.Delete)
		accounts.POST("/:id/test", h.Admin.Account.Test)
		accounts.POST("/:id/recover-state", h.Admin.Account.RecoverState)
		accounts.POST("/:id/refresh", h.Admin.Account.Refresh)
		accounts.POST("/:id/apply-oauth-credentials", h.Admin.Account.ApplyOAuthCredentials)
		accounts.POST("/:id/set-privacy", h.Admin.Account.SetPrivacy)
		accounts.POST("/:id/refresh-tier", h.Admin.Account.RefreshTier)
		accounts.GET("/:id/stats", h.Admin.Account.GetStats)
		accounts.POST("/:id/clear-error", h.Admin.Account.ClearError)
		accounts.POST("/:id/revert-proxy-fallback", h.Admin.Account.RevertProxyFallback)
		accounts.GET("/:id/usage", h.Admin.Account.GetUsage)
		accounts.GET("/:id/today-stats", h.Admin.Account.GetTodayStats)
		accounts.POST("/usage/batch", h.Admin.Account.GetBatchUsage)
		accounts.POST("/today-stats/batch", h.Admin.Account.GetBatchTodayStats)
		accounts.POST("/:id/clear-rate-limit", h.Admin.Account.ClearRateLimit)
		accounts.POST("/:id/reset-quota", h.Admin.Account.ResetQuota)
		accounts.GET("/:id/temp-unschedulable", h.Admin.Account.GetTempUnschedulable)
		accounts.DELETE("/:id/temp-unschedulable", h.Admin.Account.ClearTempUnschedulable)
		accounts.POST("/:id/schedulable", h.Admin.Account.SetSchedulable)
		accounts.POST("/models/sync-upstream-preview", h.Admin.Account.SyncUpstreamModelsPreview)
		accounts.POST("/models/probe", h.Admin.Account.ProbeUpstreamModels)
		accounts.POST("/protocols/probe", h.Admin.Account.ProbeUpstreamProtocols)
		accounts.GET("/:id/models", h.Admin.Account.GetAvailableModels)
		accounts.POST("/:id/models/sync-upstream", h.Admin.Account.SyncUpstreamModels)
		accounts.POST("/batch", h.Admin.Account.BatchCreate)
		// 账号导出泄露上游凭证原文——要求 step-up 2FA
		accounts.GET("/data", gin.HandlerFunc(stepUpAuth), h.Admin.Account.ExportData)
		accounts.POST("/data", h.Admin.Account.ImportData)
		accounts.POST("/batch-update-credentials", h.Admin.Account.BatchUpdateCredentials)
		accounts.POST("/batch-refresh-tier", h.Admin.Account.BatchRefreshTier)
		accounts.POST("/bulk-update", h.Admin.Account.BulkUpdate)
		accounts.POST("/batch-delete", h.Admin.Account.BatchDelete)
		accounts.POST("/batch-clear-error", h.Admin.Account.BatchClearError)
		accounts.POST("/batch-refresh", h.Admin.Account.BatchRefresh)

		// Antigravity 默认模型映射
		accounts.GET("/antigravity/default-model-mapping", h.Admin.Account.GetAntigravityDefaultModelMapping)

		// Spark 影子账号
		accounts.POST("/:id/shadow", h.Admin.OpenAIOAuth.CreateShadow)

		// Claude OAuth routes
		accounts.POST("/generate-auth-url", h.Admin.OAuth.GenerateAuthURL)
		accounts.POST("/generate-setup-token-url", h.Admin.OAuth.GenerateSetupTokenURL)
		accounts.POST("/exchange-code", h.Admin.OAuth.ExchangeCode)
		accounts.POST("/exchange-setup-token-code", h.Admin.OAuth.ExchangeSetupTokenCode)
		accounts.POST("/cookie-auth", h.Admin.OAuth.CookieAuth)
		accounts.POST("/setup-token-cookie-auth", h.Admin.OAuth.SetupTokenCookieAuth)
	}
}

func registerAnnouncementRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	announcements := admin.Group("/announcements")
	{
		announcements.GET("", h.Admin.Announcement.List)
		announcements.POST("", h.Admin.Announcement.Create)
		announcements.GET("/:id", h.Admin.Announcement.GetByID)
		announcements.PUT("/:id", h.Admin.Announcement.Update)
		announcements.DELETE("/:id", h.Admin.Announcement.Delete)
		announcements.GET("/:id/read-status", h.Admin.Announcement.ListReadStatus)
	}
}

func registerOpenAIOAuthRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	openai := admin.Group("/openai")
	{
		openai.POST("/generate-auth-url", h.Admin.OpenAIOAuth.GenerateAuthURL)
		openai.POST("/exchange-code", h.Admin.OpenAIOAuth.ExchangeCode)
		openai.POST("/refresh-token", h.Admin.OpenAIOAuth.RefreshToken)
		openai.POST("/accounts/:id/refresh", h.Admin.OpenAIOAuth.RefreshAccountToken)
		openai.POST("/create-from-oauth", h.Admin.OpenAIOAuth.CreateAccountFromOAuth)
		openai.POST("/create-from-codex-pat", h.Admin.OpenAIOAuth.CreateAccountFromCodexPAT)
		openai.GET("/accounts/:id/quota", h.Admin.OpenAIOAuth.QueryQuota)
		openai.POST("/accounts/:id/quota/refresh", h.Admin.OpenAIOAuth.RefreshQuota)
		openai.POST("/accounts/:id/reset-quota", h.Admin.OpenAIOAuth.ResetQuota)
	}
}

func registerGeminiOAuthRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	gemini := admin.Group("/gemini")
	{
		gemini.POST("/oauth/auth-url", h.Admin.GeminiOAuth.GenerateAuthURL)
		gemini.POST("/oauth/exchange-code", h.Admin.GeminiOAuth.ExchangeCode)
		gemini.GET("/oauth/capabilities", h.Admin.GeminiOAuth.GetCapabilities)
	}
}

func registerAntigravityOAuthRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	antigravity := admin.Group("/antigravity")
	{
		antigravity.POST("/oauth/auth-url", h.Admin.AntigravityOAuth.GenerateAuthURL)
		antigravity.POST("/oauth/exchange-code", h.Admin.AntigravityOAuth.ExchangeCode)
		antigravity.POST("/oauth/refresh-token", h.Admin.AntigravityOAuth.RefreshToken)
	}
}

func registerGrokOAuthRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	grok := admin.Group("/grok")
	{
		grok.GET("/oauth/capabilities", h.Admin.GrokOAuth.GetCapabilities)
		grok.POST("/oauth/auth-url", h.Admin.GrokOAuth.GenerateAuthURL)
		grok.POST("/oauth/exchange-code", h.Admin.GrokOAuth.ExchangeCode)
		grok.POST("/oauth/refresh-token", h.Admin.GrokOAuth.RefreshToken)
		grok.POST("/oauth/sso-token", h.Admin.GrokOAuth.ValidateSSOToken)
		grok.POST("/oauth/password", h.Admin.GrokOAuth.AuthorizePassword)
		grok.POST("/oauth/create-from-oauth", h.Admin.GrokOAuth.CreateAccountFromOAuth)
		grok.POST("/sso-to-oauth", h.Admin.GrokOAuth.CreateAccountsFromSSO)
		grok.POST("/oauth/reconcile", h.Admin.GrokOAuth.ReconcileOAuthAccounts)
		grok.POST("/accounts/:id/refresh", h.Admin.GrokOAuth.RefreshAccountToken)
		grok.GET("/accounts/:id/quota", h.Admin.GrokOAuth.QueryQuota)
		grok.POST("/accounts/:id/reset-quota", h.Admin.GrokOAuth.ResetQuota)
		grok.GET("/runtime-sanity", h.Admin.GrokOAuth.RuntimeSanity)
	}
}

// registerCNProviderRoutes 注册国产供应商（kimi/zhipu/deepseek）的额度与余额查询端点。
func registerCNProviderRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	cn := admin.Group("/cn-providers")
	{
		// Coding Plan 滚动窗口用量（kimi/zhipu coding 账号）。
		cn.GET("/accounts/:id/quota", h.Admin.CNProvider.QueryQuota)
		// payg 账号余额（kimi/deepseek；zhipu 无余额端点）。
		cn.GET("/accounts/:id/balance", h.Admin.CNProvider.QueryBalance)
	}
}

func registerProxyRoutes(admin *gin.RouterGroup, h *handler.Handlers, stepUpAuth middleware.StepUpAuthMiddleware) {
	proxies := admin.Group("/proxies")
	{
		proxies.GET("", h.Admin.Proxy.List)
		proxies.GET("/all", h.Admin.Proxy.GetAll)
		// 代理导出泄露账号密码原文——要求 step-up 2FA
		proxies.GET("/data", gin.HandlerFunc(stepUpAuth), h.Admin.Proxy.ExportData)
		proxies.POST("/data", h.Admin.Proxy.ImportData)
		proxies.GET("/:id", h.Admin.Proxy.GetByID)
		proxies.POST("", h.Admin.Proxy.Create)
		proxies.PUT("/:id", h.Admin.Proxy.Update)
		proxies.DELETE("/:id", h.Admin.Proxy.Delete)
		proxies.POST("/:id/test", h.Admin.Proxy.Test)
		proxies.POST("/:id/quality-check", h.Admin.Proxy.CheckQuality)
		proxies.GET("/:id/stats", h.Admin.Proxy.GetStats)
		proxies.GET("/:id/accounts", h.Admin.Proxy.GetProxyAccounts)
		proxies.POST("/batch-delete", h.Admin.Proxy.BatchDelete)
		proxies.POST("/batch", h.Admin.Proxy.BatchCreate)
	}
}

func registerRedeemCodeRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	codes := admin.Group("/redeem-codes")
	{
		codes.GET("", h.Admin.Redeem.List)
		codes.GET("/stats", h.Admin.Redeem.GetStats)
		codes.GET("/export", h.Admin.Redeem.Export)
		codes.GET("/:id", h.Admin.Redeem.GetByID)
		codes.POST("/create-and-redeem", h.Admin.Redeem.CreateAndRedeem)
		codes.POST("/generate", h.Admin.Redeem.Generate)
		codes.DELETE("/:id", h.Admin.Redeem.Delete)
		codes.POST("/batch-delete", h.Admin.Redeem.BatchDelete)
		codes.POST("/batch-update", h.Admin.Redeem.BatchUpdate)
		codes.POST("/:id/expire", h.Admin.Redeem.Expire)
	}
}

func registerSettingsRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	adminSettings := admin.Group("/settings")
	{
		adminSettings.GET("", h.Admin.Setting.GetSettings)
		adminSettings.PUT("", h.Admin.Setting.UpdateSettings)
		adminSettings.GET("/email-templates", h.Admin.Setting.ListEmailTemplates)
		adminSettings.POST("/email-template-preview", h.Admin.Setting.PreviewEmailTemplate)
		adminSettings.GET("/email-templates/:event/:locale", h.Admin.Setting.GetEmailTemplate)
		adminSettings.PUT("/email-templates/:event/:locale", h.Admin.Setting.UpdateEmailTemplate)
		adminSettings.POST("/email-templates/:event/:locale/restore-official", h.Admin.Setting.RestoreOfficialEmailTemplate)
		// Admin API Key 管理
		adminSettings.GET("/admin-api-key", h.Admin.Setting.GetAdminAPIKey)
		adminSettings.POST("/admin-api-key/regenerate", h.Admin.Setting.RegenerateAdminAPIKey)
		adminSettings.DELETE("/admin-api-key", h.Admin.Setting.DeleteAdminAPIKey)
	}
}

// registerImageStorageRoutes 异步生图结果对象存储（S3 兼容）的后台配置。
func registerImageStorageRoutes(admin *gin.RouterGroup, h *handler.Handlers, stepUpAuth middleware.StepUpAuthMiddleware) {
	imageStorage := admin.Group("/image-storage")
	{
		imageStorage.GET("", h.Admin.ImageStorage.GetConfig)
		// 改写对象存储目标可将生成内容导向外部账号——要求 step-up 2FA
		imageStorage.PUT("", gin.HandlerFunc(stepUpAuth), h.Admin.ImageStorage.UpdateConfig)
		imageStorage.POST("/test", h.Admin.ImageStorage.TestConnection)
	}
}

func registerSystemRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	system := admin.Group("/system")
	{
		system.GET("/version", h.Admin.System.GetVersion)
	}
}

func registerSubscriptionRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	subscriptions := admin.Group("/subscriptions")
	{
		subscriptions.GET("", h.Admin.Subscription.List)
		subscriptions.GET("/:id", h.Admin.Subscription.GetByID)
		subscriptions.GET("/:id/progress", h.Admin.Subscription.GetProgress)
		subscriptions.POST("/assign", h.Admin.Subscription.Assign)
		subscriptions.POST("/bulk-assign", h.Admin.Subscription.BulkAssign)
		subscriptions.POST("/bulk-action", h.Admin.Subscription.BulkAction)
		subscriptions.POST("/:id/extend", h.Admin.Subscription.Extend)
		subscriptions.POST("/:id/reset-quota", h.Admin.Subscription.ResetQuota)
		subscriptions.POST("/:id/revoke", h.Admin.Subscription.Revoke)
		subscriptions.POST("/:id/restore", h.Admin.Subscription.Restore)
		subscriptions.DELETE("/:id", h.Admin.Subscription.Revoke)
	}

	// 用户下的订阅列表
	admin.GET("/users/:id/subscriptions", h.Admin.Subscription.ListByUser)
}

func registerUsageRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	usage := admin.Group("/usage")
	{
		usage.GET("", h.Admin.Usage.List)
		usage.GET("/stats", h.Admin.Usage.Stats)
		usage.GET("/search-users", h.Admin.Usage.SearchUsers)
		usage.GET("/search-api-keys", h.Admin.Usage.SearchAPIKeys)
		usage.GET("/cleanup-tasks", h.Admin.Usage.ListCleanupTasks)
		usage.POST("/cleanup-tasks", h.Admin.Usage.CreateCleanupTask)
		usage.POST("/cleanup-tasks/preview", h.Admin.Usage.PreviewCleanupTask)
		usage.POST("/cleanup-tasks/:id/cancel", h.Admin.Usage.CancelCleanupTask)
	}
}

func registerUserAttributeRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	attrs := admin.Group("/user-attributes")
	{
		attrs.GET("", h.Admin.UserAttribute.ListDefinitions)
		attrs.POST("", h.Admin.UserAttribute.CreateDefinition)
		attrs.POST("/batch", h.Admin.UserAttribute.GetBatchUserAttributes)
		attrs.PUT("/reorder", h.Admin.UserAttribute.ReorderDefinitions)
		attrs.PUT("/:id", h.Admin.UserAttribute.UpdateDefinition)
		attrs.DELETE("/:id", h.Admin.UserAttribute.DeleteDefinition)
	}
}

func registerScheduledTestRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	plans := admin.Group("/scheduled-test-plans")
	{
		plans.POST("", h.Admin.ScheduledTest.Create)
		plans.PUT("/:id", h.Admin.ScheduledTest.Update)
		plans.DELETE("/:id", h.Admin.ScheduledTest.Delete)
		plans.GET("/:id/results", h.Admin.ScheduledTest.ListResults)
	}
	// Nested under accounts
	admin.GET("/accounts/:id/scheduled-test-plans", h.Admin.ScheduledTest.ListByAccount)
}

func registerErrorPassthroughRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	rules := admin.Group("/error-passthrough-rules")
	{
		rules.GET("", h.Admin.ErrorPassthrough.List)
		rules.GET("/:id", h.Admin.ErrorPassthrough.GetByID)
		rules.POST("", h.Admin.ErrorPassthrough.Create)
		rules.PUT("/:id", h.Admin.ErrorPassthrough.Update)
		rules.DELETE("/:id", h.Admin.ErrorPassthrough.Delete)
	}
}

func registerTLSFingerprintProfileRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	profiles := admin.Group("/tls-fingerprint-profiles")
	{
		profiles.GET("", h.Admin.TLSFingerprintProfile.List)
		profiles.GET("/:id", h.Admin.TLSFingerprintProfile.GetByID)
		profiles.POST("", h.Admin.TLSFingerprintProfile.Create)
		profiles.PUT("/:id", h.Admin.TLSFingerprintProfile.Update)
		profiles.DELETE("/:id", h.Admin.TLSFingerprintProfile.Delete)
	}
}

// registerModelCatalogRoutes 注册模型目录的管理端 CRUD、承接关系查询与诊断。
// 目录是「有哪些模型 + 基准价 + 承接的渠道（带上游价）」的权威表。
func registerModelCatalogRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	catalog := admin.Group("/model-catalog")
	{
		catalog.GET("/entries", h.Admin.ModelCatalog.ListEntries)
		catalog.GET("/entries/:id", h.Admin.ModelCatalog.GetEntry)
		catalog.POST("/entries", h.Admin.ModelCatalog.CreateEntry)
		catalog.POST("/entries/match", h.Admin.ModelCatalog.MatchUpstreamModels)
		catalog.POST("/entries/import", h.Admin.ModelCatalog.ImportUpstreamModels)
		catalog.PUT("/entries/:id", h.Admin.ModelCatalog.UpdateEntry)
		catalog.DELETE("/entries/:id", h.Admin.ModelCatalog.DeleteEntry)
		catalog.GET("/entries/:id/bindings", h.Admin.ModelCatalog.ListBindings)
		catalog.GET("/entries/:id/diagnosis", h.Admin.ModelCatalog.Diagnose)

		catalog.POST("/aliases", h.Admin.ModelCatalog.CreateAlias)
		catalog.PUT("/aliases/:id", h.Admin.ModelCatalog.UpdateAlias)
		catalog.DELETE("/aliases/:id", h.Admin.ModelCatalog.DeleteAlias)

		catalog.POST("/seed", h.Admin.ModelCatalog.Seed)
		catalog.GET("/price-lookup", h.Admin.ModelCatalog.PriceLookup)
		catalog.POST("/official-lookup", h.Admin.ModelCatalog.OfficialLookup)
	}

	// 价格页：官方价与上游价，按模型 / 按渠道整块保存。
	pricing := admin.Group("/pricing")
	{
		pricing.GET("", h.Admin.ModelCatalog.PricingOverview)
		pricing.PUT("/models/:id", h.Admin.ModelCatalog.SavePricingModel)
		pricing.PUT("/channels/:id", h.Admin.ModelCatalog.SavePricingChannel)
	}
}

func registerChannelMonitorRoutes(admin *gin.RouterGroup, h *handler.Handlers, settingService *service.SettingService) {
	guard := channelMonitorAdminFeatureGuard(settingService)
	monitors := admin.Group("/channel-monitors")
	monitors.Use(guard)
	{
		monitors.GET("", h.Admin.ChannelMonitor.List)
		monitors.POST("", h.Admin.ChannelMonitor.Create)
		monitors.GET("/:id", h.Admin.ChannelMonitor.Get)
		monitors.POST("/:id/duplicate", h.Admin.ChannelMonitor.Duplicate)
		monitors.PUT("/:id", h.Admin.ChannelMonitor.Update)
		monitors.DELETE("/:id", h.Admin.ChannelMonitor.Delete)
		monitors.POST("/:id/run", h.Admin.ChannelMonitor.Run)
		monitors.GET("/:id/history", h.Admin.ChannelMonitor.History)
	}

	templates := admin.Group("/channel-monitor-templates")
	templates.Use(guard)
	{
		templates.GET("", h.Admin.ChannelMonitorTemplate.List)
		templates.POST("", h.Admin.ChannelMonitorTemplate.Create)
		templates.GET("/:id", h.Admin.ChannelMonitorTemplate.Get)
		templates.PUT("/:id", h.Admin.ChannelMonitorTemplate.Update)
		templates.DELETE("/:id", h.Admin.ChannelMonitorTemplate.Delete)
		templates.GET("/:id/monitors", h.Admin.ChannelMonitorTemplate.AssociatedMonitors)
		templates.POST("/:id/apply", h.Admin.ChannelMonitorTemplate.Apply)
	}
}

// registerAffiliateRoutes 注册邀请返利的管理端路由（专属用户配置）
func registerAffiliateRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	affiliates := admin.Group("/affiliates")
	{
		affiliates.GET("/invites", h.Admin.Affiliate.ListInviteRecords)
		affiliates.GET("/rebates", h.Admin.Affiliate.ListRebateRecords)
		affiliates.GET("/transfers", h.Admin.Affiliate.ListTransferRecords)

		users := affiliates.Group("/users")
		{
			users.GET("", h.Admin.Affiliate.ListUsers)
			users.GET("/lookup", h.Admin.Affiliate.LookupUsers)
			users.POST("/batch-rate", h.Admin.Affiliate.BatchSetRate)
			users.GET("/:user_id/overview", h.Admin.Affiliate.GetUserOverview)
			users.PUT("/:user_id", h.Admin.Affiliate.UpdateUserSettings)
			users.DELETE("/:user_id", h.Admin.Affiliate.ClearUserSettings)
		}
	}
}

func channelMonitorAdminFeatureGuard(settingService *service.SettingService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if settingService != nil && settingService.GetChannelMonitorRuntime(c.Request.Context()).Enabled {
			c.Next()
			return
		}
		response.ErrorFrom(c, service.ErrChannelMonitorDisabled)
		c.Abort()
	}
}
