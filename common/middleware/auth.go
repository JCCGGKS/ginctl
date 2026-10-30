// Package middleware holds cross-cutting HTTP concerns: request id, access
// logging, panic recovery, and auth. In gin these are gin.HandlerFunc values
// composed onto the engine or a route group.
package middleware

import (
	"gin-scaffold/common"
	apperrors "gin-scaffold/common/errors"

	"github.com/gin-gonic/gin"
)

const ctxClaimsKey = "auth_claims"

// Auth verifies the Bearer JWT and stores the claims on the gin context.
// Protected handlers read identity via CurrentClaims(c).
func Auth(tm *common.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := common.ExtractBearer(c.GetHeader("Authorization"))
		if raw == "" {
			common.Fail(c, apperrors.Unauthorized("authorization JWT is required"))
			c.Abort()
			return
		}

		claims := &common.AuthClaims{}
		if err := tm.ParseInto(raw, claims); err != nil {
			common.Fail(c, apperrors.Unauthorized("authorization JWT is invalid"))
			c.Abort()
			return
		}

		c.Set(ctxClaimsKey, claims)
		c.Next()
	}
}

// CurrentClaims returns the authenticated JWT claims set by Auth, or nil when
// the route was not protected.
func CurrentClaims(c *gin.Context) *common.AuthClaims {
	if v, ok := c.Get(ctxClaimsKey); ok {
		if claims, ok := v.(*common.AuthClaims); ok {
			return claims
		}
	}
	return nil
}
