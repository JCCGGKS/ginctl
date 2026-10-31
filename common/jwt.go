package common

import (
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	jwtIssuer   = "gin-scaffold"     // 签发方标识，解析时必须与 Token 中的 iss 一致
	jwtAudience = "gin-scaffold/api" // 接收方标识，解析时必须与 Token 中的 aud 一致
	jwtLeeway   = 30 * time.Second   // 允许的时钟偏差，避免多实例时间不同步导致误判
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
			Issuer:    jwtIssuer,                     // 签发方：该 Token 由谁签发
			Subject:   username,                      // 主体：该 Token 所属用户
			Audience:  jwt.ClaimStrings{jwtAudience}, // 接收方：该 Token 允许被谁使用
			IssuedAt:  jwt.NewNumericDate(now),       // 签发时间：Token 开始生效的时间
			ExpiresAt: jwt.NewNumericDate(expiresAt), // 过期时间：超过该时间后拒绝该 Token
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
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), // 只接受 HS256，防止签名算法混淆
		jwt.WithIssuer(jwtIssuer),                                    // 校验签发方
		jwt.WithAudience(jwtAudience),                                // 校验接收方
		jwt.WithExpirationRequired(),                                 // 必须包含过期时间
		jwt.WithLeeway(jwtLeeway),                                    // 容忍有限的时钟偏差
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
