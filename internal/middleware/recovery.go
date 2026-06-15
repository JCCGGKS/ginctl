package middleware

import (
	"log/slog"

	"gin-scaffold/pkg/response"

	"github.com/gin-gonic/gin"
)

// Recovery converts a panic into a unified 500 response instead of crashing
// the connection, keeping the error contract consistent.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic recovered",
					"error", r,
					"path", c.Request.URL.Path,
					"request_id", response.RequestID(c),
				)
				response.Fail(c, response.Internal("internal server error"))
				c.Abort()
			}
		}()
		c.Next()
	}
}
