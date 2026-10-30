package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// fixedWindowScript increments a key and sets its TTL atomically. It returns
// the current count and remaining TTL in milliseconds.
var fixedWindowScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
local ttl = redis.call("PTTL", KEYS[1])
if ttl < 0 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
  ttl = tonumber(ARGV[1])
end
return {current, ttl}
`)

// Redis implements a shared fixed-window limiter with atomic Redis counters.
type Redis struct {
	client redis.UniversalClient
	prefix string
	max    int
	window time.Duration
}

// NewRedis builds a Redis-backed limiter. The prefix namespaces keys, for
// example "ratelimit:login".
func NewRedis(client redis.UniversalClient, prefix string, max int, window time.Duration) *Redis {
	max, window = normalize(max, window)
	return &Redis{
		client: client,
		prefix: strings.Trim(prefix, ":"),
		max:    max,
		window: window,
	}
}

// Allow increments the subject's counter and reports whether it is within the
// configured limit. Redis errors are returned to the caller so callers can
// choose an explicit failure policy instead of silently disabling protection.
func (l *Redis) Allow(ctx context.Context, subject string) (Decision, error) {
	if l.client == nil {
		return Decision{}, errors.New("redis rate limiter has no client")
	}

	windowMS := l.window.Milliseconds()
	if windowMS < 1 {
		windowMS = 1
	}

	key := subject
	if l.prefix != "" {
		key = l.prefix + ":" + subject
	}
	values, err := fixedWindowScript.Run(ctx, l.client, []string{key}, windowMS).Slice()
	if err != nil {
		return Decision{}, fmt.Errorf("run redis rate limit script: %w", err)
	}
	if len(values) != 2 {
		return Decision{}, fmt.Errorf("unexpected redis rate limit result length %d", len(values))
	}

	count, ok := values[0].(int64)
	if !ok {
		return Decision{}, fmt.Errorf("unexpected redis rate limit count type %T", values[0])
	}
	ttlMS, ok := values[1].(int64)
	if !ok {
		return Decision{}, fmt.Errorf("unexpected redis rate limit ttl type %T", values[1])
	}

	if count <= int64(l.max) {
		return Decision{Allowed: true}, nil
	}
	return Decision{
		Allowed:    false,
		RetryAfter: time.Duration(ttlMS) * time.Millisecond,
	}, nil
}

var _ Limiter = (*Redis)(nil)
var _ Limiter = (*Memory)(nil)
