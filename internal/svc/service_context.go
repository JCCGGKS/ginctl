// Package svc wires shared dependencies once at startup and hands them to the
// handler/business layers. This is the gin equivalent of go-zero's ServiceContext.
package svc

import (
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	"gin-scaffold/infra/token"
	"gin-scaffold/internal/config"
	"gin-scaffold/internal/dao"
	gormdao "gin-scaffold/internal/dao/gorm"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// ServiceContext is the dependency-injection container. Construct heavy deps
// (DB, Redis, clients, DAOs) once here and share the pointers everywhere.
type ServiceContext struct {
	Config       *config.Config
	TokenManager *token.Manager
	UserDAO      dao.UserDAO
	DB           *gorm.DB
	Redis        *redis.Client

	sqlDB     *sql.DB
	closeOnce sync.Once
	closeErr  error
}

// NewServiceContext assembles all dependencies from config. It returns an error
// instead of starting a process with missing critical configuration.
func NewServiceContext(c *config.Config) (*ServiceContext, error) {
	if c == nil {
		return nil, errors.New("config is nil")
	}
	secret := strings.TrimSpace(c.Auth.JWTSecret)
	if secret == "" {
		return nil, errors.New("auth.jwtSecret is required")
	}

	db, sqlDB, err := openDatabase(c.MySQL)
	if err != nil {
		return nil, err
	}
	redisClient, err := openRedis(c.Redis)
	if err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	return &ServiceContext{
		Config:       c,
		TokenManager: token.NewManager(secret, time.Duration(c.Auth.AccessExpireSeconds)*time.Second),
		UserDAO:      gormdao.NewUserDAO(db),
		DB:           db,
		Redis:        redisClient,
		sqlDB:        sqlDB,
	}, nil
}

// Close releases the database connection pool. It is safe to call more than
// once and is intended for process shutdown and failed-startup cleanup.
func (s *ServiceContext) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		var errs []error
		if s.sqlDB != nil {
			errs = append(errs, s.sqlDB.Close())
		}
		if s.Redis != nil {
			errs = append(errs, s.Redis.Close())
		}
		s.closeErr = errors.Join(errs...)
	})
	return s.closeErr
}
