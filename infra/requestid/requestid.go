// Package requestid carries the per-request correlation id through
// context.Context so infrastructure packages can log it without importing the
// HTTP layer.
package requestid

import "context"

type contextKey struct{}

// WithContext returns a child context carrying id. An empty id leaves ctx
// unchanged.
func WithContext(ctx context.Context, id string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, id)
}

// FromContext returns the request id stored in ctx, or "" when absent.
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(contextKey{}).(string)
	return id
}
