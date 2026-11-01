// Package middleware holds cross-cutting HTTP concerns: request id, access
// logging, panic recovery, and auth. In gin these are gin.HandlerFunc values
// composed onto the engine or a route group.
package middleware

import (
	"gin-scaffold/common"

	"github.com/gin-gonic/gin"
)

const ctxClaimsKey = "auth_claims"

// ErrorBuilder creates the service-specific application error used by the
// middleware while keeping error types under each service's internal/errors.
type ErrorBuilder func(string) error

// NoAuthRoutes lists routes that bypass JWT auth even when mounted on an
// authenticated group. Keys use the exact "METHOD /full/route/pattern" form.
// Keep this map next to route registration so it is reviewed with the routes.
type NoAuthRoutes map[string]struct{}

// Skip reports whether the current request matches a no-auth route. Matching is
// exact and fails closed when the route template is unknown.
func (r NoAuthRoutes) Skip(c *gin.Context) bool {
	route := c.FullPath()
	if route == "" {
		route = c.Request.URL.Path
	}
	_, ok := r[c.Request.Method+" "+route]
	return ok
}

// Auth verifies the Bearer JWT and stores the claims on the gin context.
// Protected handlers read identity via CurrentClaims(c).
func Auth(tm *common.JWTManager, unauthorized ErrorBuilder, noAuth NoAuthRoutes) gin.HandlerFunc {
	return func(c *gin.Context) {
		if noAuth.Skip(c) {
			c.Next()
			return
		}

		// Authorization：客户端携带的认证凭据，格式为 "Bearer <JWT>"。
		raw := common.ExtractBearer(c.GetHeader("Authorization"))
		if raw == "" {
			common.Fail(c, unauthorized("authorization JWT is required"))
			return
		}

		claims, err := tm.Parse(raw)
		if err != nil {
			common.Fail(c, unauthorized("authorization JWT is invalid"))
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
