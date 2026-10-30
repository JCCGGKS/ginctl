package router

import (
	"gin-scaffold/internal/handler"
	"gin-scaffold/internal/middleware"
	"gin-scaffold/internal/svc"

	"github.com/gin-gonic/gin"
)

// registerRoutes wires every HTTP route. Public and protected groups are
// declared explicitly so the wiring is easy to read, override, and test.
func registerRoutes(engine *gin.Engine, svcCtx *svc.ServiceContext) {
	engine.GET("/healthz", handler.Health)

	authHandler := handler.NewAuthHandler(svcCtx)
	v1 := engine.Group("/api/v1")
	v1.POST("/login", authHandler.Login)

	protected := engine.Group("/api/v1", middleware.Auth(svcCtx.JWTManager))
	protected.GET("/me", authHandler.Me)
}
