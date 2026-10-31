package cache

import "github.com/redis/go-redis/v9"

// Registry is the single entry point for domain caches. Keeping the aggregate
// lets future domains grow without changing the shape of ServiceContext.
type Registry struct {
	User UserCache

	available bool
}

// NewRegistry constructs all domain caches from the shared Redis client. A nil
// client is allowed so Redis remains optional during startup.
func NewRegistry(client *redis.Client) *Registry {
	return &Registry{
		User:      NewUserCache(client),
		available: client != nil,
	}
}

// Available reports whether the registry is backed by a connected Redis client.
func (r *Registry) Available() bool {
	return r != nil && r.available
}
