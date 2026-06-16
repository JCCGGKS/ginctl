// Package middleware holds cross-cutting HTTP concerns: request id, access
// logging, panic recovery, and auth. In gin these are gin.HandlerFunc values
// composed onto the engine or a route group.
package middleware

import (
	"gin-scaffold/pkg/response"
	"gin-scaffold/pkg/token"

	"github.com/gin-gonic/gin"
)

const ctxClaimsKey = "auth_claims"

// Auth verifies the Bearer token and stores the claims on the gin context.
// Protected handlers read identity via CurrentClaims(c).
func Auth(tm *token.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := token.ExtractBearer(c.GetHeader("Authorization"))
		if raw == "" {
			response.Fail(c, response.Unauthorized("authorization token is required"))
			c.Abort()
			return
		}

		claims := &token.AuthClaims{}
		if err := tm.ParseInto(raw, claims); err != nil {
			response.Fail(c, response.Unauthorized("authorization token is invalid"))
			c.Abort()
			return
		}

		c.Set(ctxClaimsKey, claims)
		c.Next()
	}
}
