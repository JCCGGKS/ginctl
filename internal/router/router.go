// Package router maps URLs to handlers and composes middleware per group.
// This is the gin analogue of go-zero's generated routes.go — here it is
// hand-written and grouped instead of generated from a .api file.
package router

import (
	"github.com/gin-gonic/gin"

	"gin-scaffold/internal/handler"
	"gin-scaffold/internal/middleware"
	"gin-scaffold/internal/svc"
)

// New builds the gin engine with global middleware and all routes.
func New(svcCtx *svc.ServiceContext) *gin.Engine {
	gin.SetMode(svcCtx.Config.Server.Mode)

	engine := gin.New()

	// Global middleware: request id -> access log -> panic guard. Recovery is
	// last so it wraps the handler chain; the others run for every request.
	engine.Use(
		middleware.RequestID(),
		middleware.AccessLog(),
		middleware.Recovery(),
	)

	authHandler := handler.NewAuthHandler(svcCtx)
	resourceHandler := handler.NewResourceHandler(svcCtx)

	// Liveness probe — intentionally outside the unified envelope.
	engine.GET("/healthz", handler.Health)

	api := engine.Group("/api/v1")
	{
		// Public route; also demonstrates the operation-log middleware.
		api.POST("/auth/login",
			middleware.OperationLog("login", nil),
			authHandler.Login,
		)

		// Protected group: everything here requires a valid JWT.
		authed := api.Group("")
		authed.Use(middleware.Auth(svcCtx.TokenManager))
		{
			authed.GET("/resources", resourceHandler.List)
			authed.GET("/resources/:id", resourceHandler.Get)
		}
	}

	return engine
}
