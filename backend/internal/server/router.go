package server

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/server/routes"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/web"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const routerSettingsRefreshTimeout = 5 * time.Second

type routerSettings struct {
	frameOrigins   []string
	ingressDomains map[string]struct{}
}

func newRouterSettings(settings *service.PublicSettings) *routerSettings {
	endpoints := dto.ParseCustomEndpoints(settings.CustomEndpoints)
	urls := make([]string, 0, len(endpoints)+1)
	urls = append(urls, settings.APIBaseURL)
	for _, endpoint := range endpoints {
		urls = append(urls, endpoint.Endpoint)
	}
	return &routerSettings{
		frameOrigins:   settings.FrameSrcOrigins(),
		ingressDomains: middleware2.IngressDomainsFromURLs(urls),
	}
}

// SetupRouter 配置路由器中间件和路由
func SetupRouter(
	r *gin.Engine,
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
	compositeResolver *service.CompositeRouteResolver,
	cfg *config.Config,
	redisClient *redis.Client,
) *gin.Engine {
	middleware2.SetIngressRejectRecorder(opsService)
	// Load CSP origins and API domains together, outside the request path.
	var cachedSettings atomic.Pointer[routerSettings]
	cachedSettings.Store(&routerSettings{})
	var refreshMu sync.Mutex
	refreshRouterSettings := func() {
		refreshMu.Lock()
		defer refreshMu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), routerSettingsRefreshTimeout)
		defer cancel()
		settings, err := settingService.GetPublicSettings(ctx)
		if err != nil {
			return // Keep the previous snapshot on a transient read failure.
		}
		cachedSettings.Store(newRouterSettings(settings))
	}
	refreshRouterSettings()

	// 应用中间件
	r.Use(middleware2.IngressDomain(cfg.Server.TrustedProxies, func() map[string]struct{} {
		return cachedSettings.Load().ingressDomains
	}))
	r.Use(middleware2.RequestLogger())
	// 将客户端 IP + UA 注入 request context，供 token 签发/会话绑定/审计日志统一读取。
	// 解析模式按请求快照：兼容开关开启时信任原始转发头，关闭时使用 server.trusted_proxies。
	r.Use(middleware2.SessionBindingContext(cfg))
	r.Use(middleware2.Logger())
	r.Use(middleware2.CORS(cfg.CORS))
	r.Use(middleware2.SecurityHeaders(cfg.Security.CSP, func() []string {
		return cachedSettings.Load().frameOrigins
	}))
	r.Use(middleware2.ServerTiming(cfg.Server.EnableServerTiming))

	onSettingsUpdate := refreshRouterSettings
	// Serve embedded frontend with settings injection if available
	if web.HasEmbeddedFrontend() {
		frontendServer, err := web.NewFrontendServer(settingService) //nolint:staticcheck // SA4023: the !embed stub always errors; embed builds can return nil
		if err != nil {                                              //nolint:staticcheck // SA4023: see above
			log.Printf("Warning: Failed to create frontend server with settings injection: %v, using legacy mode", err)
			r.Use(web.ServeEmbeddedFrontend())
		} else {
			onSettingsUpdate = func() {
				frontendServer.InvalidateCache()
				refreshRouterSettings()
			}
			r.Use(frontendServer.Middleware())
		}
	}
	settingService.SetOnUpdateCallback(onSettingsUpdate)

	// 注册路由
	registerRoutes(r, handlers, jwtAuth, optionalJWTAuth, adminAuth, apiKeyAuth, auditLog, stepUpAuth, apiKeyService, subscriptionService, opsService, settingService, compositeResolver, cfg, redisClient)

	return r
}

// registerRoutes 注册所有 HTTP 路由
func registerRoutes(
	r *gin.Engine,
	h *handler.Handlers,
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
	compositeResolver *service.CompositeRouteResolver,
	cfg *config.Config,
	redisClient *redis.Client,
) {
	// 通用路由（健康检查、状态等）
	routes.RegisterCommonRoutes(r)

	// API v1
	v1 := r.Group("/api/v1")

	// 面板 API 限流器：认证接口按用户 ID、公开接口按安全客户端 IP，
	// 防止高频刷管理面接口打爆数据库（阈值可在系统设置中调整）。
	panelRateLimiter := middleware2.NewPanelRateLimiter(redisClient, settingService)

	// 注册各模块路由
	routes.RegisterAuthRoutes(v1, h, jwtAuth, auditLog, redisClient, settingService, panelRateLimiter)
	routes.RegisterUserRoutes(v1, h, jwtAuth, auditLog, settingService, panelRateLimiter)
	routes.RegisterModelPlazaRoutes(v1, h, optionalJWTAuth, settingService, panelRateLimiter)
	routes.RegisterAdminRoutes(v1, h, adminAuth, auditLog, stepUpAuth, settingService, panelRateLimiter)
	routes.RegisterMCPRoutes(v1, h, auditLog, stepUpAuth, settingService, panelRateLimiter, cfg.CORS.AllowedOrigins, redisClient)
	routes.RegisterMCPOAuthRoutes(r, v1, h, redisClient)
	routes.RegisterGatewayRoutes(r, h, apiKeyAuth, apiKeyService, subscriptionService, opsService, settingService, compositeResolver, cfg)
	routes.RegisterPaymentRoutes(v1, h.Payment, h.PaymentWebhook, h.Admin.Payment, jwtAuth, adminAuth, auditLog, settingService, panelRateLimiter, redisClient)

	handler.RegisterPageRoutes(v1, cfg.Pricing.DataDir, gin.HandlerFunc(jwtAuth), gin.HandlerFunc(adminAuth), settingService)
}
