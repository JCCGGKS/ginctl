// Package middleware holds cross-cutting HTTP concerns: request id, access
// logging, panic recovery, and auth. In gin these are gin.HandlerFunc values
// composed onto the engine or a route group.
package middleware

import (
	"gin-scaffold/infra"

	"github.com/gin-gonic/gin"
)

const ctxClaimsKey = "auth_claims"

// Auth verifies the Bearer token and stores the claims on the gin context.
// Protected handlers read identity via CurrentClaims(c).
func Auth(tm *infra.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := infra.ExtractBearer(c.GetHeader("Authorization"))
		if raw == "" {
			infra.Fail(c, infra.Unauthorized("authorization token is required"))
			c.Abort()
			return
		}

		claims := &infra.AuthClaims{}
		if err := tm.ParseInto(raw, claims); err != nil {
			infra.Fail(c, infra.Unauthorized("authorization token is invalid"))
			c.Abort()
			return
		}

		c.Set(ctxClaimsKey, claims)
		c.Next()
	}
}

// CurrentClaims returns the authenticated JWT claims set by Auth, or nil when
// the route was not protected.
func CurrentClaims(c *gin.Context) *infra.AuthClaims {
	if v, ok := c.Get(ctxClaimsKey); ok {
		if claims, ok := v.(*infra.AuthClaims); ok {
			return claims
		}
	}
	return nil
}
