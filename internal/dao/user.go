// Package dao defines data-access contracts. Logic depends on these interfaces,
// never on concrete storage implementations.
package dao

import (
	"context"
	"errors"

	"gin-scaffold/internal/model"
)

// ErrUserNotFound is the sentinel returned when a lookup has no match.
var ErrUserNotFound = errors.New("user not found")

// UserDAO abstracts user persistence. Swap the in-memory implementation for
// MySQL/gorm without touching business or handlers.
type UserDAO interface {
	FindByUsername(ctx context.Context, username string) (*model.User, error)
}
