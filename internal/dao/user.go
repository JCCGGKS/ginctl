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

// UserDAO abstracts user persistence so business and handlers do not depend on
// a concrete database client.
type UserDAO interface {
	FindByUsername(ctx context.Context, username string) (*model.User, error)
}
