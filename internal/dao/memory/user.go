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

// NewUserDAO builds an empty thread-safe user store.
func NewUserDAO() *UserDAO {
	return &UserDAO{users: make(map[string]*model.User)}
}

// NewDemoUserDAO builds a store seeded with the demo user admin/admin123.
// It is intended for dev/test only; production must use a real DAO.
func NewDemoUserDAO() *UserDAO {
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		panic(err) // static seed; cannot fail in practice
	}
	now := time.Now()
	d := NewUserDAO()
	d.users["admin"] = &model.User{
		ID:           1,
		Username:     "admin",
		PasswordHash: string(hash),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return d
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
