package middleware

import (
	"gin-scaffold/common"
	"gin-scaffold/common/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Recovery converts a panic into a unified 500 response instead of crashing
// the connection, keeping the error contract consistent.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.App().Error(c.Request.Context(), "panic recovered",
					zap.Any("error", r),
					zap.String("path", c.Request.URL.Path),
				)
				common.Fail(c, common.Internal("internal server error"))
				c.Abort()
			}
		}()
		c.Next()
	}
}
