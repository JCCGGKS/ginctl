// Package logic holds business orchestration: validate input, call DAOs,
// assemble responses, and return typed errors. Handlers stay thin; logic
// decides "when and why". Constructor pattern (ctx, svcCtx) mirrors the
// reference project.
package logic

import (
	"context"
	"errors"

	"gin-scaffold/infra/response"
	"gin-scaffold/internal/dao"
	"gin-scaffold/internal/schema"
	"gin-scaffold/internal/svc"

	"golang.org/x/crypto/bcrypt"
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

// Login validates credentials and issues an access token.
func (l *AuthLogic) Login(req *schema.LoginRequest) (*schema.LoginResponse, error) {
	select {
	case <-l.ctx.Done():
		return nil, response.Internal("request cancelled")
	default:
	}

	user, err := l.svcCtx.UserDAO.FindByUsername(l.ctx, req.Username)
	if err != nil {
		if errors.Is(err, dao.ErrUserNotFound) {
			return nil, response.Unauthorized("username or password is invalid")
		}
		return nil, response.Internal("query user failed")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, response.Unauthorized("username or password is invalid")
	}

	accessToken, expiresAt, err := l.svcCtx.TokenManager.Issue(user.ID, user.Username)
	if err != nil {
		return nil, response.Internal("issue token failed")
	}

	return &schema.LoginResponse{
		AccessToken:     accessToken,
		AccessExpiresAt: expiresAt.Unix(),
		User: schema.UserDTO{
			ID:       user.ID,
			Username: user.Username,
		},
	}, nil
}
