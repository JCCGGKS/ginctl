package common

import (
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the JWT payload. Extend with roles/tenant/scope as needed.
type AuthClaims struct {
	UserID   uint64 `json:"uid"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// JWTManager signs and parses JWTs with a single HS256 secret.
type JWTManager struct {
	secret    []byte
	accessTTL time.Duration
}

// NewJWTManager builds a JWT manager. A non-positive TTL falls back to 15m.
func NewJWTManager(secret string, accessTTL time.Duration) *JWTManager {
	if accessTTL <= 0 {
		accessTTL = 15 * time.Minute
	}
	return &JWTManager{secret: []byte(secret), accessTTL: accessTTL}
}

// Issue creates a signed access token and returns it with its expiry.
func (m *JWTManager) Issue(userID uint64, username string) (string, time.Time, error) {
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

// ParseInto validates a JWT string and decodes it into the provided claims.
func (m *JWTManager) ParseInto(jwtString string, claims jwt.Claims) error {
	if len(m.secret) == 0 {
		return errors.New("jwt secret is not configured")
	}

	parsed, err := jwt.ParseWithClaims(
		jwtString,
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
	if !parsed.Valid {
		return errors.New("jwt is invalid")
	}
	return nil
}

// ExtractBearer returns the bearer JWT from an Authorization header.
// It returns "" when the header is not in the expected "Bearer <jwt>" form.
func ExtractBearer(header string) string {
	header = strings.TrimSpace(header)
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
