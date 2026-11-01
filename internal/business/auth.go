// Package business holds business orchestration: validate input, call DAOs,
// assemble responses, and return typed errors. Handlers stay thin; business
// decides "when and why".
package business

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gin-scaffold/common/logger"
	apperrors "gin-scaffold/internal/errors"
	"gin-scaffold/internal/schema"
	"gin-scaffold/internal/svc"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// dummyPasswordHash keeps the unknown-user path as expensive as a real bcrypt
// comparison, preventing username enumeration by response timing.
var dummyPasswordHash = []byte("$2a$10$6OGoa2u1yiNPAencu1s1TeFtX1ZcLZNTaRINGjJ2U/nhTrriqDqb2")

// AuthService owns authentication use cases. Construct it once at startup.
type AuthService struct {
	svcCtx *svc.ServiceContext
}

// NewAuthService builds the authentication service from the shared context.
func NewAuthService(svcCtx *svc.ServiceContext) *AuthService {
	return &AuthService{svcCtx: svcCtx}
}

// Login validates credentials and issues an access/refresh token pair.
func (s *AuthService) Login(ctx context.Context, req *schema.LoginRequest) (*schema.LoginResponse, error) {
	if ctx.Err() != nil {
		return nil, apperrors.Timeout("request cancelled")
	}

	username := strings.TrimSpace(req.Username)
	user, err := s.svcCtx.DAO.User.FindByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, apperrors.ErrUserNotFound) {
			_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(req.Password))
			return nil, invalidCredentials(ctx, username)
		}
		if ctx.Err() != nil {
			return nil, apperrors.Timeout("request cancelled")
		}
		logger.App().Error(ctx, "login user lookup failed", zap.String("username", username), zap.Error(err))
		return nil, fmt.Errorf("query user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, invalidCredentials(ctx, username)
	}

	accessToken, expiresAt, err := s.svcCtx.JWTManager.Issue(user.ID, user.Username)
	if err != nil {
		logger.App().Error(ctx, "login jwt issue failed", zap.Uint64("user_id", user.ID), zap.Error(err))
		return nil, fmt.Errorf("issue jwt: %w", err)
	}
	refreshToken, refreshExpiresAt, err := s.svcCtx.JWTManager.IssueRefresh(user.ID, user.Username)
	if err != nil {
		logger.App().Error(ctx, "login refresh jwt issue failed", zap.Uint64("user_id", user.ID), zap.Error(err))
		return nil, fmt.Errorf("issue refresh jwt: %w", err)
	}

	logger.App().Info(ctx, "login succeeded",
		zap.Uint64("user_id", user.ID),
		zap.String("username", user.Username),
	)

	return &schema.LoginResponse{
		AccessToken:      accessToken,
		AccessExpiresAt:  expiresAt.Unix(),
		RefreshToken:     refreshToken,
		RefreshExpiresAt: refreshExpiresAt.Unix(),
		User: schema.UserDTO{
			ID:       user.ID,
			Username: user.Username,
		},
	}, nil
}

// Refresh validates a refresh token and issues a new access token.
func (s *AuthService) Refresh(ctx context.Context, req *schema.RefreshRequest) (*schema.RefreshResponse, error) {
	if ctx.Err() != nil {
		return nil, apperrors.Timeout("request cancelled")
	}

	claims, err := s.svcCtx.JWTManager.ParseRefresh(strings.TrimSpace(req.RefreshToken))
	if err != nil {
		return nil, rejectRefresh(ctx)
	}

	user, err := s.svcCtx.DAO.User.FindByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, apperrors.ErrUserNotFound) {
			return nil, rejectRefresh(ctx)
		}
		if ctx.Err() != nil {
			return nil, apperrors.Timeout("request cancelled")
		}
		logger.App().Error(ctx, "refresh user lookup failed", zap.Uint64("user_id", claims.UserID), zap.Error(err))
		return nil, fmt.Errorf("query user: %w", err)
	}

	accessToken, expiresAt, err := s.svcCtx.JWTManager.Issue(user.ID, user.Username)
	if err != nil {
		logger.App().Error(ctx, "refresh jwt issue failed", zap.Uint64("user_id", user.ID), zap.Error(err))
		return nil, fmt.Errorf("issue jwt: %w", err)
	}

	logger.App().Info(ctx, "token refreshed",
		zap.Uint64("user_id", user.ID),
		zap.String("username", user.Username),
	)

	return &schema.RefreshResponse{
		AccessToken:     accessToken,
		AccessExpiresAt: expiresAt.Unix(),
		User: schema.UserDTO{
			ID:       user.ID,
			Username: user.Username,
		},
	}, nil
}

func rejectRefresh(ctx context.Context) error {
	logger.App().Warn(ctx, "refresh rejected", zap.String("reason", "invalid_refresh_token"))
	return apperrors.ErrInvalidRefreshToken
}

func invalidCredentials(ctx context.Context, username string) error {
	logger.App().Warn(ctx, "login rejected",
		zap.String("username", username),
		zap.String("reason", "invalid_credentials"),
	)
	return apperrors.ErrInvalidCredentials
}
