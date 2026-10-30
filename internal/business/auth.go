// Package business holds business orchestration: validate input, call DAOs,
// assemble responses, and return typed errors. Handlers stay thin; business
// decides "when and why". Constructor pattern (ctx, svcCtx) mirrors the
// reference project.
package business

import (
	"context"
	"errors"
	"fmt"

	"gin-scaffold/common/errorcode"
	"gin-scaffold/common/logger"
	"gin-scaffold/internal/dao"
	"gin-scaffold/internal/schema"
	"gin-scaffold/internal/svc"

	"go.uber.org/zap"
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

	user, err := l.svcCtx.DAO.User.FindByUsername(l.ctx, req.Username)
	if err != nil {
		if errors.Is(err, dao.ErrUserNotFound) {
			_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(req.Password))
			logger.App().Warn(l.ctx, "login rejected",
				zap.String("username", req.Username),
				zap.String("reason", "invalid_credentials"),
			)
			return nil, errorcode.ErrInvalidCredentials
		}
		logger.App().Error(l.ctx, "login user lookup failed", zap.String("username", req.Username), zap.Error(err))
		return nil, fmt.Errorf("query user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		logger.App().Warn(l.ctx, "login rejected",
			zap.String("username", req.Username),
			zap.String("reason", "invalid_credentials"),
		)
		return nil, errorcode.ErrInvalidCredentials
	}

	accessToken, expiresAt, err := l.svcCtx.JWTManager.Issue(user.ID, user.Username)
	if err != nil {
		logger.App().Error(l.ctx, "login jwt issue failed", zap.Uint64("user_id", user.ID), zap.Error(err))
		return nil, fmt.Errorf("issue jwt: %w", err)
	}

	logger.App().Info(l.ctx, "login succeeded",
		zap.Uint64("user_id", user.ID),
		zap.String("username", user.Username),
	)

	return &schema.LoginResponse{
		AccessToken:     accessToken,
		AccessExpiresAt: expiresAt.Unix(),
		User: schema.UserDTO{
			ID:       user.ID,
			Username: user.Username,
		},
	}, nil
}
