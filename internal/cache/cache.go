// Package cache wraps the shared Redis client behind an application-level cache
// dependency. Business code depends on this layer instead of receiving the raw
// Redis client through ServiceContext.
package cache

import "github.com/redis/go-redis/v9"

// Cache is the application cache dependency. It may wrap a nil Redis client
// when Redis is unavailable; callers should check Available before using it.
type Cache struct {
	client *redis.Client
}

// New builds a cache wrapper around the configured Redis client. A nil client is
// allowed so Redis remains optional during startup.
func New(client *redis.Client) *Cache {
	return &Cache{client: client}
}

// Available reports whether the cache is backed by a connected Redis client.
func (c *Cache) Available() bool {
	return c != nil && c.client != nil
}

// Close releases the underlying client when present.
func (c *Cache) Close() error {
	if !c.Available() {
		return nil
	}
	return c.client.Close()
}
