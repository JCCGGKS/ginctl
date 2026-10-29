// Package router maps URLs to handlers and composes middleware per group.
package router

import (
	"context"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"gin-scaffold/infra/logger"
	"gin-scaffold/infra/response"
	"gin-scaffold/internal/middleware"
	"gin-scaffold/internal/svc"
)

// New builds the gin engine with global middleware and all routes.
func New(svcCtx *svc.ServiceContext) *gin.Engine {
	gin.SetMode(svcCtx.Config.Server.Mode)

	// Route gin's own debug/warning output through zap too.
	gin.DefaultWriter = logger.Writer()
	gin.DefaultErrorWriter = logger.Writer()

	engine := gin.New()
	engine.HandleMethodNotAllowed = true

	// Never trust X-Forwarded-* by default. Configure CIDRs for a known load
	// balancer; an empty list makes ClientIP() use the remote socket address.
	trustedProxies := svcCtx.Config.Server.TrustedProxies
	if len(trustedProxies) == 0 {
		trustedProxies = nil
	}
	if err := engine.SetTrustedProxies(trustedProxies); err != nil {
		logger.App().Error(context.Background(), "invalid trusted proxies; disabling proxy trust", zap.Error(err))
		_ = engine.SetTrustedProxies(nil)
	}

	maxBodyBytes := svcCtx.Config.Server.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = 1 << 20
	}

	// Global middleware: request id -> body limit -> access log -> panic guard.
	// Recovery is last so it wraps the handler chain.
	engine.Use(
		middleware.RequestID(),
		middleware.BodyLimit(maxBodyBytes),
		middleware.AccessLog(),
		middleware.Recovery(),
	)

	registerRoutes(engine, svcCtx)

	engine.NoRoute(func(c *gin.Context) {
		response.Fail(c, response.NotFound("resource not found"))
	})
	engine.NoMethod(func(c *gin.Context) {
		response.Fail(c, response.MethodNotAllowed("method not allowed"))
	})

	return engine
}
