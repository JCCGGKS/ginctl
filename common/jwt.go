package common

import (
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	jwtIssuer   = "gin-scaffold"
	jwtAudience = "gin-scaffold/api"
	jwtLeeway   = 30 * time.Second
)

// AuthClaims is the JWT payload. Extend with roles/tenant/scope as needed.
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

	now := time.Now()
	expiresAt := now.Add(m.accessTTL)
	claims := AuthClaims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   username,
			Audience:  jwt.ClaimStrings{jwtAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

// Parse validates a JWT string and returns its typed claims.
func (m *JWTManager) Parse(jwtString string) (*AuthClaims, error) {
	if len(m.secret) == 0 {
		return nil, errors.New("jwt secret is not configured")
	}

	claims := &AuthClaims{}
	parsed, err := jwt.ParseWithClaims(
		jwtString,
		claims,
		func(_ *jwt.Token) (any, error) {
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(jwtIssuer),
		jwt.WithAudience(jwtAudience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(jwtLeeway),
	)
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, errors.New("jwt is invalid")
	}
	if claims.UserID == 0 || strings.TrimSpace(claims.Username) == "" {
		return nil, errors.New("jwt claims are incomplete")
	}
	return claims, nil
}

// ExtractBearer returns the bearer JWT from an Authorization header.
// It returns "" when the header is not in the expected "Bearer <jwt>" form.
func ExtractBearer(header string) string {
	scheme, token, ok := strings.Cut(strings.TrimSpace(header), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}
