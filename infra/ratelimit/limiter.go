// Package ratelimit defines shared rate-limiting backends.
package ratelimit

import (
	"context"
	"time"
)

// Decision is the outcome of one rate-limit check.
type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

// Limiter checks whether a subject may proceed within its configured window.
type Limiter interface {
	Allow(ctx context.Context, subject string) (Decision, error)
}
