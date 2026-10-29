// Package business holds business orchestration: validate input, call DAOs,
// assemble responses, and return typed errors. Handlers stay thin; business
// decides "when and why". Constructor pattern (ctx, svcCtx) mirrors the
// reference project.
package business

import (
	"context"
	"errors"
	"fmt"

	"gin-scaffold/internal/dao"
	"gin-scaffold/internal/schema"
	"gin-scaffold/internal/svc"

	"golang.org/x/crypto/bcrypt"
)

// dummyPasswordHash keeps the unknown-user path as expensive as a real bcrypt
// comparison, preventing username enumeration by response timing.
var dummyPasswordHash = []byte("$2a$10$6OGoa2u1yiNPAencu1s1TeFtX1ZcLZNTaRINGjJ2U/nhTrriqDqb2")

// AuthBusiness handles authentication business flow.
type AuthBusiness struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewAuthBusiness builds AuthBusiness with request-scoped context and shared deps.
func NewAuthBusiness(ctx context.Context, svcCtx *svc.ServiceContext) *AuthBusiness {
	return &AuthBusiness{ctx: ctx, svcCtx: svcCtx}
}

// Login validates credentials and issues an access token.
func (l *AuthBusiness) Login(req *schema.LoginRequest) (*schema.LoginResponse, error) {
	if err := l.ctx.Err(); err != nil {
		return nil, err
	}

	user, err := l.svcCtx.UserDAO.FindByUsername(l.ctx, req.Username)
	if err != nil {
		if errors.Is(err, dao.ErrUserNotFound) {
			_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(req.Password))
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("query user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	accessToken, expiresAt, err := l.svcCtx.TokenManager.Issue(user.ID, user.Username)
	if err != nil {
		return nil, fmt.Errorf("issue token: %w", err)
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
