package middleware

import (
	"time"

	"gin-scaffold/pkg/logger"
	"gin-scaffold/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AccessLog emits one structured line per request after it completes.
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Int64("latency_ms", time.Since(start).Milliseconds()),
			zap.String("client_ip", c.ClientIP()),
			zap.String("request_id", response.RequestID(c)),
		}
		if claims := CurrentClaims(c); claims != nil {
			fields = append(fields, zap.Uint64("user_id", claims.UserID))
		}

		logger.Info("access", fields...)
	}
}
