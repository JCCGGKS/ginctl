package middleware

import (
	"math"
	"strconv"
	"sync"
	"time"

	"gin-scaffold/infra/response"

	"github.com/gin-gonic/gin"
)

const defaultRateLimitPerWindow = 10

type rateLimitEntry struct {
	count int
	reset time.Time
}

type fixedWindowLimiter struct {
	mu      sync.Mutex
	entries map[string]rateLimitEntry
	max     int
	window  time.Duration
	calls   int
}

// RateLimit allows max requests per window for each client IP. It is an
// in-process limiter: use a shared store such as Redis when running multiple
// instances. max <= 0 falls back to 10; window <= 0 falls back to one minute.
func RateLimit(max int, window time.Duration) gin.HandlerFunc {
	if max <= 0 {
		max = defaultRateLimitPerWindow
	}
	if window <= 0 {
		window = time.Minute
	}
	limiter := &fixedWindowLimiter{
		entries: make(map[string]rateLimitEntry),
		max:     max,
		window:  window,
	}

	return func(c *gin.Context) {
		key := c.ClientIP()
		if key == "" {
			key = c.Request.RemoteAddr
		}

		allowed, retryAfter := limiter.allow(key, time.Now())
		if !allowed {
			retrySeconds := int(math.Ceil(retryAfter.Seconds()))
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

func (l *fixedWindowLimiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.calls++
	if l.calls%1024 == 0 {
		l.cleanup(now)
	}

	entry, ok := l.entries[key]
	if !ok || !now.Before(entry.reset) {
		l.entries[key] = rateLimitEntry{count: 1, reset: now.Add(l.window)}
		return true, 0
	}
	if entry.count >= l.max {
		return false, entry.reset.Sub(now)
	}

	entry.count++
	l.entries[key] = entry
	return true, 0
}

func (l *fixedWindowLimiter) cleanup(now time.Time) {
	for key, entry := range l.entries {
		if !now.Before(entry.reset) {
			delete(l.entries, key)
		}
	}
}
