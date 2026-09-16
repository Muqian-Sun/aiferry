package server

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/server/routes"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/web"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// routeDeps 汇总路由注册需要的依赖，两个站点共用同一份。
type routeDeps struct {
	handlers            *handler.Handlers
	jwtAuth             middleware2.JWTAuthMiddleware
	optionalJWTAuth     middleware2.OptionalJWTAuthMiddleware
	adminAuth           middleware2.AdminAuthMiddleware
	apiKeyAuth          middleware2.APIKeyAuthMiddleware
	auditLog            middleware2.AuditLogMiddleware
	stepUpAuth          middleware2.StepUpAuthMiddleware
	apiKeyService       *service.APIKeyService
	subscriptionService *service.SubscriptionService
	opsService          *service.OpsService
	settingService      *service.SettingService
	compositeResolver   *service.CompositeRouteResolver
	cfg                 *config.Config
	redisClient         *redis.Client
}

// setupSiteRouter 配置单个站点的中间件与路由。
//
// 站点中间件放在最前，后续认证中间件与 token 签发都依赖它做站点角色校验。
// frontendServer 为 nil 表示未内嵌前端（开发构建），只提供 API。
func setupSiteRouter(r *gin.Engine, site service.Site, deps routeDeps, frameOrigins func() []string, frontend gin.HandlerFunc) *gin.Engine {
	cfg := deps.cfg
	r.Use(middleware2.SiteContext(site))
	r.Use(middleware2.RequestLogger())
	// 将客户端 IP + UA 注入 request context，供 token 签发/会话绑定/审计日志统一读取。
	// 解析模式按请求快照：兼容开关开启时信任原始转发头，关闭时使用 server.trusted_proxies。
	r.Use(middleware2.SessionBindingContext(cfg))
	r.Use(middleware2.Logger())
	r.Use(middleware2.CORS(cfg.CORS))
	r.Use(middleware2.SecurityHeaders(cfg.Security.CSP, frameOrigins))
	r.Use(middleware2.ServerTiming(cfg.Server.EnableServerTiming))

	if frontend != nil {
		r.Use(frontend)
	}

	switch site {
	case service.SiteUser:
		registerUserSiteRoutes(r, deps)
	case service.SiteAdmin:
		registerAdminSiteRoutes(r, deps)
	default:
		panic("unknown site: " + string(site))
	}
	return r
}

// registerUserSiteRoutes 注册用户站路由：用户面板 API、模型网关、支付与回调。管理 API 不在此注册。
func registerUserSiteRoutes(r *gin.Engine, d routeDeps) {
	h := d.handlers
	routes.RegisterCommonRoutes(r)

	v1 := r.Group("/api/v1")
	// 面板 API 限流器：认证接口按用户 ID、公开接口按安全客户端 IP，
	// 防止高频刷管理面接口打爆数据库（阈值可在系统设置中调整）。
	panelRateLimiter := middleware2.NewPanelRateLimiter(d.redisClient, d.settingService)

	routes.RegisterAuthRoutes(v1, service.SiteUser, h, d.jwtAuth, d.auditLog, d.redisClient, d.settingService, panelRateLimiter)
	routes.RegisterAccountSecurityRoutes(v1, h, d.jwtAuth, d.auditLog, d.settingService, panelRateLimiter)
	routes.RegisterUserRoutes(v1, h, d.jwtAuth, d.auditLog, d.settingService, panelRateLimiter)
	routes.RegisterModelPlazaRoutes(v1, h, d.optionalJWTAuth, d.settingService, panelRateLimiter)
	routes.RegisterGatewayRoutes(r, h, d.apiKeyAuth, d.apiKeyService, d.subscriptionService, d.opsService, d.settingService, d.compositeResolver, d.cfg)
	routes.RegisterPaymentRoutes(v1, h.Payment, h.PaymentWebhook, d.jwtAuth, d.settingService, panelRateLimiter)

	handler.RegisterPageRoutes(v1, service.SiteUser, d.cfg.Pricing.DataDir, gin.HandlerFunc(d.jwtAuth), gin.HandlerFunc(d.adminAuth), d.settingService)
}

// registerAdminSiteRoutes 注册管理站路由：登录与账号安全、管理 API、支付管理、页面管理。
// 模型网关与用户面板 API 不在此注册。
func registerAdminSiteRoutes(r *gin.Engine, d routeDeps) {
	h := d.handlers
	routes.RegisterCommonRoutes(r)

	v1 := r.Group("/api/v1")
	panelRateLimiter := middleware2.NewPanelRateLimiter(d.redisClient, d.settingService)

	routes.RegisterAuthRoutes(v1, service.SiteAdmin, h, d.jwtAuth, d.auditLog, d.redisClient, d.settingService, panelRateLimiter)
	routes.RegisterAccountSecurityRoutes(v1, h, d.jwtAuth, d.auditLog, d.settingService, panelRateLimiter)
	routes.RegisterAdminRoutes(v1, h, d.adminAuth, d.auditLog, d.stepUpAuth, d.settingService, panelRateLimiter)
	routes.RegisterAdminPaymentRoutes(v1, h.Admin.Payment, d.adminAuth, d.auditLog, d.settingService)

	handler.RegisterPageRoutes(v1, service.SiteAdmin, d.cfg.Pricing.DataDir, gin.HandlerFunc(d.jwtAuth), gin.HandlerFunc(d.adminAuth), d.settingService)
}

// siteFrontend 返回站点的前端托管中间件与缓存失效函数；未内嵌前端时两者皆为 nil。
func siteFrontend(app web.App, settingService *service.SettingService) (gin.HandlerFunc, func()) {
	if !web.HasEmbeddedFrontend(app) {
		return nil, nil
	}
	frontendServer, err := web.NewFrontendServer(settingService, app) //nolint:staticcheck // SA4023: the !embed stub always errors; embed builds can return nil
	if err != nil {                                                   //nolint:staticcheck // SA4023: see above
		logFrontendFallback(app, err)
		return web.ServeEmbeddedFrontend(app), nil
	}
	return frontendServer.Middleware(), frontendServer.InvalidateCache
}
