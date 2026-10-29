// Package svc wires shared dependencies once at startup and hands them to the
// handler/business layers. This is the gin equivalent of go-zero's ServiceContext.
package svc

import (
	"errors"
	"strings"
	"time"

	"gin-scaffold/infra/logger"
	"gin-scaffold/infra/token"
	"gin-scaffold/internal/config"
	"gin-scaffold/internal/dao"
	"gin-scaffold/internal/dao/memory"
)

// ServiceContext is the dependency-injection container. Construct heavy deps
// (DB, Redis, clients, DAOs) once here and share the pointers everywhere.
type ServiceContext struct {
	Config       *config.Config
	TokenManager *token.Manager
	UserDAO      dao.UserDAO
}

// NewServiceContext assembles all dependencies from config. It returns an error
// instead of starting a process with missing critical configuration.
func NewServiceContext(c *config.Config, env string) (*ServiceContext, error) {
	if c == nil {
		return nil, errors.New("config is nil")
	}
	secret := strings.TrimSpace(c.Auth.JWTSecret)
	if secret == "" {
		return nil, errors.New("auth.jwtSecret is required")
	}

	env = strings.ToLower(strings.TrimSpace(env))
	if env == "" {
		env = config.DefaultEnv
	}

	var userDAO dao.UserDAO
	if env == "prod" {
		logger.Warn("using in-memory user DAO in production; no demo user is seeded; wire a real DAO before deployment")
		userDAO = memory.NewUserDAO()
	} else {
		userDAO = memory.NewDemoUserDAO()
	}

	return &ServiceContext{
		Config:       c,
		TokenManager: token.NewManager(secret, time.Duration(c.Auth.AccessExpireSeconds)*time.Second),
		UserDAO:      userDAO,
	}, nil
}
