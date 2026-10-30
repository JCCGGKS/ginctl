// Package router maps URLs to handlers and composes middleware per group.
package router

import (
	"github.com/gin-gonic/gin"

	"gin-scaffold/common"
	"gin-scaffold/common/logger"
	"gin-scaffold/common/middleware"
	apperrors "gin-scaffold/internal/errors"
	"gin-scaffold/internal/handler"
	"gin-scaffold/internal/svc"
)

// noAuthRoutes lists endpoints mounted on the authenticated /api/v1 group that
// stay public. Keys are "METHOD /full/route/pattern".
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

	// Global middleware: request id -> access log -> panic guard.
	// Recovery is last so it wraps the handler chain.
	engine.Use(
		middleware.RequestID(),
		middleware.AccessLog(),
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

// registerRoutes wires every HTTP route. /api/v1 authenticates by default;
// public endpoints are explicit entries in noAuthRoutes.
func registerRoutes(engine *gin.Engine, svcCtx *svc.ServiceContext) {
	engine.GET("/healthz", handler.Health)

	authHandler := handler.NewAuthHandler(svcCtx)
	v1 := engine.Group("/api/v1", middleware.Auth(svcCtx.JWTManager, apperrors.Unauthorized, noAuthRoutes))
	v1.POST("/login", authHandler.Login)
	v1.GET("/me", authHandler.Me)
}
