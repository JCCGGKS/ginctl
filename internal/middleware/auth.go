// Package middleware holds cross-cutting HTTP concerns: request id, access
// logging, panic recovery, auth, and operation logging. In gin these are
// gin.HandlerFunc values composed onto the engine or a route group.
package middleware

import (
	"strings"

	"gin-scaffold/pkg/response"
	"gin-scaffold/pkg/token"

	"github.com/gin-gonic/gin"
)

const ctxClaimsKey = "auth_claims"

// Auth verifies the Bearer token and stores the claims on the gin context.
// Protected handlers read identity via CurrentClaims(c).
func Auth(tm *token.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := extractBearer(c.GetHeader("Authorization"))
		if raw == "" {
			response.Fail(c, response.Unauthorized("authorization token is required"))
			c.Abort()
			return
		}

		claims, err := tm.Parse(raw)
		if err != nil {
			response.Fail(c, response.Unauthorized("authorization token is invalid"))
			c.Abort()
			return
		}

		c.Set(ctxClaimsKey, claims)
		c.Next()
	}
}

// CurrentClaims returns the authenticated identity stored by Auth.
func CurrentClaims(c *gin.Context) (*token.Claims, bool) {
	v, ok := c.Get(ctxClaimsKey)
	if !ok {
		return nil, false
	}
	claims, ok := v.(*token.Claims)
	return claims, ok
}

func extractBearer(header string) string {
	header = strings.TrimSpace(header)
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}
