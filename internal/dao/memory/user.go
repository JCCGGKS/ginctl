// Package memory provides an in-memory UserDAO so the skeleton boots without
// any external infrastructure.
package memory

import (
	"context"
	"sync"
	"time"

	"gin-scaffold/internal/dao"
	"gin-scaffold/internal/model"

	"golang.org/x/crypto/bcrypt"
)

// UserDAO is a thread-safe in-memory user store seeded with a demo account.
type UserDAO struct {
	mu    sync.RWMutex
	users map[string]*model.User
}

// NewUserDAO builds the store and seeds the demo user admin/admin123.
func NewUserDAO() *UserDAO {
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		panic(err) // static seed; cannot fail in practice
	}
	now := time.Now()
	return &UserDAO{
		users: map[string]*model.User{
			"admin": {
				ID:           1,
				Username:     "admin",
				PasswordHash: string(hash),
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		},
	}
}

// FindByUsername returns a copy of the matching user or dao.ErrUserNotFound.
func (d *UserDAO) FindByUsername(_ context.Context, username string) (*model.User, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	u, ok := d.users[username]
	if !ok {
		return nil, dao.ErrUserNotFound
	}
	cp := *u
	return &cp, nil
}

var _ dao.UserDAO = (*UserDAO)(nil)
