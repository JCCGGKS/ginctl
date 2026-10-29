// Package router maps URLs to handlers and composes middleware per group.
package router

import (
	"github.com/gin-gonic/gin"

	"gin-scaffold/internal/middleware"
	"gin-scaffold/internal/svc"
	"gin-scaffold/pkg/logger"
)

// New builds the gin engine with global middleware and all routes.
func New(svcCtx *svc.ServiceContext) *gin.Engine {
	gin.SetMode(svcCtx.Config.Server.Mode)

	// Route gin's own debug/warning output through zap too.
	gin.DefaultWriter = logger.Writer()
	gin.DefaultErrorWriter = logger.Writer()

	engine := gin.New()

	// Global middleware: request id -> access log -> panic guard. Recovery is
	// last so it wraps the handler chain; the others run for every request.
	engine.Use(
		middleware.RequestID(),
		middleware.AccessLog(),
		middleware.Recovery(),
	)

	registerRoutes(engine, svcCtx)

	return engine
}
