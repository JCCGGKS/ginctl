// Package gormdao implements DAO contracts with GORM.
package gormdao

import (
	"context"
	"errors"
	"fmt"

	"gin-scaffold/common/model"
	"gin-scaffold/internal/dao"

	"gorm.io/gorm"
)

// UserDAO queries users from the configured relational database.
type UserDAO struct {
	db *gorm.DB
}

// NewUserDAO builds a GORM-backed user DAO.
func NewUserDAO(db *gorm.DB) *UserDAO {
	return &UserDAO{db: db}
}

// FindByUsername returns the matching user or dao.ErrUserNotFound.
func (d *UserDAO) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := d.db.WithContext(ctx).
		Where("username = ?", username).
		Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, dao.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by username: %w", err)
	}
	return &user, nil
}

var _ dao.UserDAO = (*UserDAO)(nil)
