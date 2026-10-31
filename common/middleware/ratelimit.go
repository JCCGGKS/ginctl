package middleware

import (
	"strconv"
	"sync"
	"time"

	"gin-scaffold/common"

	"github.com/gin-gonic/gin"
)

// FixedWindowLimiter is a small in-process guard for abuse-prone endpoints.
// Multi-instance deployments should still enforce limits at the gateway or in
// a shared store.
type FixedWindowLimiter struct {
	mu          sync.Mutex
	limit       int
	window      time.Duration
	entries     map[string]rateWindow
	lastCleanup time.Time
}

type rateWindow struct {
	start time.Time
	count int
}

// NewFixedWindowLimiter builds a limiter. Non-positive inputs fall back to safe
// defaults rather than disabling protection accidentally.
func NewFixedWindowLimiter(limit int, window time.Duration) *FixedWindowLimiter {
	if limit <= 0 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &FixedWindowLimiter{
		limit:   limit,
		window:  window,
		entries: make(map[string]rateWindow),
	}
}

// Allow reports whether key may perform another request in the current window.
func (l *FixedWindowLimiter) Allow(key string) bool {
	if l == nil {
		return true
	}
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastCleanup) >= l.window {
		for candidate, state := range l.entries {
			if now.Sub(state.start) >= l.window {
				delete(l.entries, candidate)
			}
		}
		l.lastCleanup = now
	}

	state, ok := l.entries[key]
	if !ok || now.Sub(state.start) >= l.window {
		l.entries[key] = rateWindow{start: now, count: 1}
		return true
	}
	if state.count >= l.limit {
		return false
	}
	state.count++
	l.entries[key] = state
	return true
}

// RateLimit rejects requests over the limiter's budget. The caller supplies the
// service-specific 429 error so middleware stays independent of internal/errors.
func RateLimit(limiter *FixedWindowLimiter, tooMany ErrorBuilder) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limiter == nil {
			c.Next()
			return
		}

		key := c.ClientIP()
		if key == "" {
			key = "unknown"
		}
		if limiter.Allow(key) {
			c.Next()
			return
		}

		retryAfter := int(limiter.window / time.Second)
		if retryAfter < 1 {
			retryAfter = 1
		}
		c.Header("Retry-After", strconv.Itoa(retryAfter))
		common.Fail(c, tooMany("too many requests"))
	}
}
