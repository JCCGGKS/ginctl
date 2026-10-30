// Package router maps URLs to handlers and composes middleware per group.
package router

import (
	"github.com/gin-gonic/gin"

	"gin-scaffold/infra"
	"gin-scaffold/infra/logger"
	"gin-scaffold/internal/middleware"
	"gin-scaffold/internal/svc"
)

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
		middleware.Recovery(),
	)

	registerRoutes(engine, svcCtx)

	engine.NoRoute(func(c *gin.Context) {
		infra.Fail(c, infra.NotFound("resource not found"))
	})
	engine.NoMethod(func(c *gin.Context) {
		infra.Fail(c, infra.MethodNotAllowed("method not allowed"))
	})

	return engine
}
