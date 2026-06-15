// Package token issues and verifies stateless JWT access tokens.
// Extracted from the reference project's utils/auth.go (HS256 + custom claims).
package token

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// Claims is the JWT payload. Extend with roles/tenant/scope as needed.
type Claims struct {
	UserID   uint64 `json:"uid"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Manager signs and parses tokens with a single HS256 secret.
type Manager struct {
	secret    []byte
	accessTTL time.Duration
}

// NewManager builds a token manager. A non-positive TTL falls back to 15m.
func NewManager(secret string, accessTTL time.Duration) *Manager {
	if accessTTL <= 0 {
		accessTTL = 15 * time.Minute
	}
	return &Manager{secret: []byte(secret), accessTTL: accessTTL}
}

// Issue creates a signed access token and returns it with its expiry.
func (m *Manager) Issue(userID uint64, username string) (string, time.Time, error) {
	if len(m.secret) == 0 {
		return "", time.Time{}, errors.New("jwt secret is not configured")
	}

	expiresAt := time.Now().Add(m.accessTTL)
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

// Parse validates a token string and returns its claims.
func (m *Manager) Parse(tokenString string) (*Claims, error) {
	if len(m.secret) == 0 {
		return nil, errors.New("jwt secret is not configured")
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("token is invalid")
	}
	return claims, nil
}
