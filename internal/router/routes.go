package router

import (
	"time"

	"gin-scaffold/infra/ratelimit"
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
	loginLimiter := ratelimit.NewRedis(
		svcCtx.Redis,
		"ratelimit:login",
		svcCtx.Config.Auth.LoginRateLimitPerMinute,
		time.Minute,
	)
	v1 := engine.Group("/api/v1")
	v1.POST("/login",
		middleware.RateLimit(loginLimiter),
		authHandler.Login,
	)

	protected := engine.Group("/api/v1", middleware.Auth(svcCtx.TokenManager))
	protected.GET("/me", authHandler.Me)
}
