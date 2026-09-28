// Package server provides HTTP server initialization and configuration.
package server

import (
	"context"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/websearch"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/web"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"golang.org/x/net/http2"
)

// ProviderSet 提供服务器层的依赖
var ProviderSet = wire.NewSet(
	ProvideRouters,
	ProvideHTTPServers,
)

const frameSrcRefreshTimeout = 5 * time.Second

// Routers 是用户站与管理站各自的路由器。两者路由集合互不相交，分别监听不同端口。
type Routers struct {
	User  *gin.Engine
	Admin *gin.Engine
}

// HTTPServers 是用户站与管理站各自的 HTTP 服务器。
type HTTPServers struct {
	User  *http.Server
	Admin *http.Server
}

// ProvideRouters 提供用户站与管理站的路由器
func ProvideRouters(
	cfg *config.Config,
	handlers *handler.Handlers,
	jwtAuth middleware2.JWTAuthMiddleware,
	optionalJWTAuth middleware2.OptionalJWTAuthMiddleware,
	adminAuth middleware2.AdminAuthMiddleware,
	apiKeyAuth middleware2.APIKeyAuthMiddleware,
	auditLog middleware2.AuditLogMiddleware,
	stepUpAuth middleware2.StepUpAuthMiddleware,
	apiKeyService *service.APIKeyService,
	subscriptionService *service.SubscriptionService,
	opsService *service.OpsService,
	settingService *service.SettingService,
	modelCatalog *service.ModelCatalogService,
	redisClient *redis.Client,
) *Routers {
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Wire up websearch Manager builder so it initializes on startup and rebuilds on config save.
	// 配了 Key 的服务商才进 Manager；一个都没有就不建（Web Search 模拟不生效）。
	settingService.SetWebSearchManagerBuilder(context.Background(), func(cfg *service.WebSearchEmulationConfig) {
		configs := make([]websearch.ProviderConfig, 0, len(cfg.Providers))
		for _, p := range cfg.Providers {
			if p.APIKey == "" {
				continue
			}
			configs = append(configs, websearch.ProviderConfig{Type: p.Type, APIKey: p.APIKey, ExpiresAt: p.ExpiresAt})
		}
		if len(configs) == 0 {
			service.SetWebSearchManager(nil)
			return
		}
		service.SetWebSearchManager(websearch.NewManager(configs))
	})

	middleware2.SetIngressRejectRecorder(opsService)

	// 缓存 iframe 页面的 origin 列表，用于动态注入 CSP frame-src；两个站点共用。
	var cachedFrameOrigins atomic.Pointer[[]string]
	emptyOrigins := []string{}
	cachedFrameOrigins.Store(&emptyOrigins)
	refreshFrameOrigins := func() {
		ctx, cancel := context.WithTimeout(context.Background(), frameSrcRefreshTimeout)
		defer cancel()
		origins, err := settingService.GetFrameSrcOrigins(ctx)
		if err != nil {
			// 获取失败时保留已有缓存，避免 frame-src 被意外清空
			return
		}
		cachedFrameOrigins.Store(&origins)
	}
	refreshFrameOrigins() // 启动时初始化
	frameOrigins := func() []string {
		if p := cachedFrameOrigins.Load(); p != nil {
			return *p
		}
		return nil
	}

	userFrontend, invalidateUser := siteFrontend(web.AppUser, settingService)
	adminFrontend, invalidateAdmin := siteFrontend(web.AppAdmin, settingService)
	// 设置回调是覆盖式注册，只能设一次：两个站点的 HTML 缓存与 frame-src 一起刷新。
	settingService.SetOnUpdateCallback(func() {
		for _, invalidate := range []func(){invalidateUser, invalidateAdmin} {
			if invalidate != nil {
				invalidate()
			}
		}
		refreshFrameOrigins()
	})

	deps := routeDeps{
		handlers:            handlers,
		jwtAuth:             jwtAuth,
		optionalJWTAuth:     optionalJWTAuth,
		adminAuth:           adminAuth,
		apiKeyAuth:          apiKeyAuth,
		auditLog:            auditLog,
		stepUpAuth:          stepUpAuth,
		apiKeyService:       apiKeyService,
		subscriptionService: subscriptionService,
		opsService:          opsService,
		settingService:      settingService,
		modelCatalog:        modelCatalog,
		cfg:                 cfg,
		redisClient:         redisClient,
	}
	return &Routers{
		User:  setupSiteRouter(newEngine(cfg), service.SiteUser, deps, frameOrigins, userFrontend),
		Admin: setupSiteRouter(newEngine(cfg), service.SiteAdmin, deps, frameOrigins, adminFrontend),
	}
}

func newEngine(cfg *config.Config) *gin.Engine {
	r := gin.New()
	r.Use(middleware2.Recovery())
	configureTrustedProxies(r, cfg.Server)
	return r
}

func logFrontendFallback(app web.App, err error) {
	log.Printf("Warning: Failed to create %s frontend server with settings injection: %v, using legacy mode", app, err)
}

func configureTrustedProxies(r *gin.Engine, cfg config.ServerConfig) {
	if cfg.TrustedProxiesConfigured {
		if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
			log.Printf("Failed to set trusted proxies: %v", err)
			_ = r.SetTrustedProxies(nil)
		}
		if len(cfg.TrustedProxies) == 0 && cfg.Mode == "release" {
			log.Printf("Warning: server.trusted_proxies is explicitly empty; forwarded client IP trust is disabled")
		}
	} else {
		if err := r.SetTrustedProxies(nil); err != nil {
			log.Printf("Failed to disable trusted proxies: %v", err)
		}
		if cfg.Mode == "release" {
			log.Printf("Warning: server.trusted_proxies is not configured; disabling the forwarded-IP compatibility switch will use direct peer addresses only")
		}
	}
}

// ProvideHTTPServers 提供用户站与管理站的 HTTP 服务器，分别监听 server.port 与 server.admin_port。
func ProvideHTTPServers(cfg *config.Config, routers *Routers) *HTTPServers {
	return &HTTPServers{
		User:  newHTTPServer(cfg, cfg.Server.Address(), routers.User),
		Admin: newHTTPServer(cfg, cfg.Server.AdminAddress(), routers.Admin),
	}
}

func newHTTPServer(cfg *config.Config, addr string, router *gin.Engine) *http.Server {
	httpHandler := http.Handler(router)
	server := &http.Server{
		Addr:           addr,
		Handler:        httpHandler,
		MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
		// ReadHeaderTimeout: 读取请求头的超时时间，防止慢速请求头攻击
		ReadHeaderTimeout: time.Duration(cfg.Server.ReadHeaderTimeout) * time.Second,
		// IdleTimeout: 空闲连接超时时间，释放不活跃的连接资源
		IdleTimeout: time.Duration(cfg.Server.IdleTimeout) * time.Second,
		// 注意：不设置 WriteTimeout，因为流式响应可能持续十几分钟
		// 不设置 ReadTimeout，因为大请求体可能需要较长时间读取
	}

	globalMaxSize := cfg.Server.MaxRequestBodySize
	if globalMaxSize <= 0 {
		globalMaxSize = cfg.Gateway.MaxBodySize
	}
	if globalMaxSize > 0 {
		httpHandler = http.MaxBytesHandler(httpHandler, globalMaxSize)
		log.Printf("Global max request body size: %d bytes (%.2f MB)", globalMaxSize, float64(globalMaxSize)/(1<<20))
	}

	// 根据配置决定是否启用 H2C
	if cfg.Server.H2C.Enabled {
		h2cConfig := cfg.Server.H2C
		if err := http2.ConfigureServer(server, &http2.Server{
			MaxConcurrentStreams:         h2cConfig.MaxConcurrentStreams,
			IdleTimeout:                  time.Duration(h2cConfig.IdleTimeout) * time.Second,
			MaxReadFrameSize:             uint32(h2cConfig.MaxReadFrameSize),
			MaxUploadBufferPerConnection: int32(h2cConfig.MaxUploadBufferPerConnection),
			MaxUploadBufferPerStream:     int32(h2cConfig.MaxUploadBufferPerStream),
		}); err != nil {
			log.Printf("Failed to configure HTTP/2 Cleartext (h2c): %v", err)
		} else {
			protocols := new(http.Protocols)
			protocols.SetHTTP1(true)
			protocols.SetUnencryptedHTTP2(true)
			server.Protocols = protocols
			log.Printf("HTTP/2 Cleartext (h2c) enabled: max_concurrent_streams=%d, idle_timeout=%ds, max_read_frame_size=%d, max_upload_buffer_per_connection=%d, max_upload_buffer_per_stream=%d",
				h2cConfig.MaxConcurrentStreams,
				h2cConfig.IdleTimeout,
				h2cConfig.MaxReadFrameSize,
				h2cConfig.MaxUploadBufferPerConnection,
				h2cConfig.MaxUploadBufferPerStream,
			)
		}
	}

	server.Handler = httpHandler
	return server
}
