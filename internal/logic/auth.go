// Package logic holds business orchestration: validate input, call DAOs,
// assemble responses, and return typed errors. Handlers stay thin; logic
// decides "when and why". Constructor pattern (ctx, svcCtx) mirrors the
// reference project.
package logic

import (
	"context"

	"gin-scaffold/internal/schema"
	"gin-scaffold/internal/svc"
	"gin-scaffold/pkg/response"
)

// AuthLogic handles authentication-related business logic.
type AuthLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewAuthLogic builds AuthLogic with request-scoped context and shared deps.
func NewAuthLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthLogic {
	return &AuthLogic{ctx: ctx, svcCtx: svcCtx}
}

// Demo credentials so the skeleton has a working login without a user table.
// Replace with a UserDAO lookup + bcrypt compare in a real service.
const (
	demoUsername = "admin"
	demoPassword = "admin123"
	demoUserID   = 1
)

// Login validates credentials and issues an access token.
func (l *AuthLogic) Login(req *schema.LoginRequest) (*schema.LoginResponse, error) {
	select {
	case <-l.ctx.Done():
		return nil, response.Internal(l.ctx.Err().Error())
	default:
	}

	if req.Username != demoUsername || req.Password != demoPassword {
		return nil, response.Unauthorized("username or password is invalid")
	}

	accessToken, expiresAt, err := l.svcCtx.TokenManager.Issue(demoUserID, req.Username)
	if err != nil {
		return nil, response.Internal("issue token failed")
	}

	return &schema.LoginResponse{
		AccessToken:     accessToken,
		AccessExpiresAt: expiresAt.Unix(),
		User: schema.UserDTO{
			ID:       demoUserID,
			Username: req.Username,
		},
	}, nil
}
