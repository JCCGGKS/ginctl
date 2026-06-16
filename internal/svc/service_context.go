// Package svc wires shared dependencies once at startup and hands them to the
// handler/logic layers. This is the gin equivalent of go-zero's ServiceContext.
package svc

import (
	"time"

	"gin-scaffold/internal/config"
	"gin-scaffold/pkg/token"
)

// ServiceContext is the dependency-injection container. Construct heavy deps
// (DB, Redis, clients, DAOs) once here and share the pointers everywhere.
type ServiceContext struct {
	Config       *config.Config
	TokenManager *token.Manager
}

// NewServiceContext assembles all dependencies from config.
func NewServiceContext(c *config.Config) *ServiceContext {
	return &ServiceContext{
		Config:       c,
		TokenManager: token.NewManager(c.Auth.JWTSecret, time.Duration(c.Auth.AccessExpireSeconds)*time.Second),
	}
}
