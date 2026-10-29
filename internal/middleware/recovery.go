package middleware

import (
	"gin-scaffold/infra/logger"
	"gin-scaffold/infra/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Recovery converts a panic into a unified 500 response instead of crashing
// the connection, keeping the error contract consistent.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("panic recovered",
					zap.Any("error", r),
					zap.String("path", c.Request.URL.Path),
					zap.String("request_id", response.RequestID(c)),
				)
				response.Fail(c, response.Internal("internal server error"))
				c.Abort()
			}
		}()
		c.Next()
	}
}
