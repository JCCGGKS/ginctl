// Package dao defines data-access contracts and their default implementations.
// Logic depends on the interfaces, not concrete database clients.
package dao

import (
	"context"
	"errors"
	"fmt"

	"gin-scaffold/common/model"
	bizerrors "gin-scaffold/internal/errors"

	"gorm.io/gorm"
)

// UserDAO abstracts user persistence so business and handlers do not depend on
// a concrete database client.
type UserDAO interface {
	FindByUsername(ctx context.Context, username string) (*model.User, error)
}

// GormUserDAO queries users from the configured relational database.
type GormUserDAO struct {
	db *gorm.DB
}

// NewUserDAO builds a GORM-backed user DAO.
func NewUserDAO(db *gorm.DB) *GormUserDAO {
	return &GormUserDAO{db: db}
}

// FindByUsername returns the matching user or internal/errors.ErrUserNotFound.
func (d *GormUserDAO) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := d.db.WithContext(ctx).
		Where("username = ?", username).
		Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, bizerrors.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by username: %w", err)
	}
	return &user, nil
}

var _ UserDAO = (*GormUserDAO)(nil)
