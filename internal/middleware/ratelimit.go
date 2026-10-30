package middleware

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"gin-scaffold/infra/ratelimit"
	"gin-scaffold/infra/response"

	"github.com/gin-gonic/gin"
)

// RateLimit rejects a client IP when the shared limiter denies the request.
// Limiter failures fail closed so a Redis outage cannot silently disable
// brute-force protection on sensitive endpoints.
func RateLimit(limiter ratelimit.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()
		if key == "" {
			key = c.Request.RemoteAddr
		}

		decision, err := limiter.Allow(c.Request.Context(), key)
		if err != nil {
			response.Fail(c, fmt.Errorf("rate limit check failed: %w",
				errors.Join(response.ServiceUnavailable("service temporarily unavailable"), err)))
			return
		}
		if !decision.Allowed {
			retrySeconds := int(math.Ceil(decision.RetryAfter.Seconds()))
			if retrySeconds < 1 {
				retrySeconds = 1
			}
			c.Header("Retry-After", strconv.Itoa(retrySeconds))
			response.Fail(c, response.TooManyRequests("too many requests, please try again later"))
			return
		}

		c.Next()
	}
}
