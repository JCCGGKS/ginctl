package middleware

import (
	"log/slog"
	"time"

	"gin-scaffold/pkg/response"

	"github.com/gin-gonic/gin"
)

// AccessLog emits one structured line per request after it completes.
// Uses the stdlib log/slog instead of go-zero's logx.
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		slog.Info("access",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
			"request_id", response.RequestID(c),
		)
	}
}
