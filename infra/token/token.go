// Package token issues and verifies stateless JWT access tokens.
// Extracted from the reference project's utils/auth.go (HS256 + custom claims).
package token

import (
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// Claims is the JWT payload. Extend with roles/tenant/scope as needed.
type AuthClaims struct {
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
	claims := AuthClaims{
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

// ParseInto validates a token string and decodes it into the provided claims.
func (m *Manager) ParseInto(tokenString string, claims jwt.Claims) error {
	if len(m.secret) == 0 {
		return errors.New("jwt secret is not configured")
	}

	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(t *jwt.Token) (any, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("unexpected signing method")
			}
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		return err
	}
	if !token.Valid {
		return errors.New("token is invalid")
	}
	return nil
}

// ExtractBearer returns the bearer token part from an Authorization header.
// It returns "" when the header is not in the expected "Bearer <token>" form.
func ExtractBearer(header string) string {
	header = strings.TrimSpace(header)
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
