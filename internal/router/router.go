// Package router maps URLs to handlers and composes middleware per group.
package router

import (
	"time"

	"github.com/gin-gonic/gin"

	"gin-scaffold/common"
	"gin-scaffold/common/logger"
	"gin-scaffold/common/middleware"
	"gin-scaffold/internal/business"
	apperrors "gin-scaffold/internal/errors"
	"gin-scaffold/internal/handler"
	"gin-scaffold/internal/svc"
)

const maxRequestBodyBytes = 1 << 20

// noAuthRoutes lists endpoints mounted on the authenticated /api/v1 group that
// stay public. Keys must exactly match "METHOD /full/route/pattern".
var noAuthRoutes = middleware.NoAuthRoutes{
	"POST /api/v1/login": {},
}

// New builds the gin engine with global middleware and all routes.
func New(svcCtx *svc.ServiceContext) *gin.Engine {
	gin.SetMode(svcCtx.Config.GinMode())

	// Gin framework output belongs to the access module, not business logs.
	gin.DefaultWriter = logger.AccessWriter()
	gin.DefaultErrorWriter = logger.AccessErrorWriter()

	engine := gin.New()
	engine.HandleMethodNotAllowed = true

	// Never trust X-Forwarded-*; ClientIP uses the remote socket address.
	_ = engine.SetTrustedProxies(nil)

	// Global middleware: request id -> security headers -> access log ->
	// body limit -> panic guard. Recovery is last so it wraps the handlers.
	engine.Use(
		middleware.RequestID(),
		middleware.SecurityHeaders(),
		middleware.AccessLog(),
		middleware.BodyLimit(maxRequestBodyBytes),
		middleware.Recovery(apperrors.Internal),
	)

	registerRoutes(engine, svcCtx)

	engine.NoRoute(func(c *gin.Context) {
		common.Fail(c, apperrors.NotFound("resource not found"))
	})
	engine.NoMethod(func(c *gin.Context) {
		common.Fail(c, apperrors.MethodNotAllowed("method not allowed"))
	})

	return engine
}

// registerRoutes wires every HTTP route. /api/v1 authenticates by default; the
// explicit noAuthRoutes map is the only way to mount a public endpoint there.
func registerRoutes(engine *gin.Engine, svcCtx *svc.ServiceContext) {
	engine.GET("/healthz", handler.Health)

	authService := business.NewAuthService(svcCtx)
	authHandler := handler.NewAuthHandler(authService)
	v1 := engine.Group("/api/v1", middleware.Auth(svcCtx.JWTManager, apperrors.Unauthorized, noAuthRoutes))

	loginLimiter := middleware.NewFixedWindowLimiter(svcCtx.Config.Auth.LoginRatePerMinute, time.Minute)
	v1.POST("/login", middleware.RateLimit(loginLimiter, apperrors.TooManyRequests), authHandler.Login)
	v1.GET("/me", authHandler.Me)
}
