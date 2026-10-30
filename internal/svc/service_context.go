// Package svc wires shared dependencies once at startup and hands them to the
// handler/business layers. This is the gin equivalent of go-zero's ServiceContext.
package svc

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	"gin-scaffold/common"
	"gin-scaffold/common/logger"
	"gin-scaffold/internal/config"
	"gin-scaffold/internal/dao"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ServiceContext is the dependency-injection container. Construct heavy deps
// (DB, Redis, clients, DAOs) once here and share the pointers everywhere.
type ServiceContext struct {
	Config     *config.Config
	JWTManager *common.JWTManager
	DAO        *dao.Registry
	DB         *gorm.DB
	// Redis is optional and may be nil when the configured instance is
	// unavailable at startup.
	Redis *redis.Client

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
		logger.App().Warn(context.Background(), "redis unavailable; continuing without redis",
			zap.String("addr", c.Redis.Addr()),
			zap.Int("db", c.Redis.DB),
			zap.Error(err),
		)
		redisClient = nil
	}

	return &ServiceContext{
		Config:     c,
		JWTManager: common.NewJWTManager(secret, time.Duration(c.Auth.AccessExpireSeconds)*time.Second),
		DAO:        dao.NewRegistry(db),
		DB:         db,
		Redis:      redisClient,
		sqlDB:      sqlDB,
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
