package common

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	jwtIssuer         = "gin-scaffold"     // 签发方标识，解析时必须与 Token 中的 iss 一致
	jwtAudience       = "gin-scaffold/api" // 接收方标识，解析时必须与 Token 中的 aud 一致
	jwtLeeway         = 30 * time.Second   // 允许的时钟偏差，避免多实例时间不同步导致误判
	defaultAccessTTL  = 15 * time.Minute
	defaultRefreshTTL = 7 * 24 * time.Hour

	TokenUseAccess  = "access"
	TokenUseRefresh = "refresh"
)

// AuthClaims is the JWT payload. Extend with roles/tenant/scope as needed.
type AuthClaims struct {
	UserID   uint64 `json:"uid"`       // 用户 ID，必须与 sub 保持一致
	Username string `json:"username"`  // 用户名，仅用于展示，鉴权以 UserID 为准
	TokenUse string `json:"token_use"` // Token 用途：access（访问）或 refresh（刷新）
	jwt.RegisteredClaims
}

// JWTManager signs and parses JWTs with a single HS256 secret.
type JWTManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewJWTManager builds a JWT manager. Non-positive TTLs fall back to defaults.
func NewJWTManager(secret string, accessTTL, refreshTTL time.Duration) *JWTManager {
	if accessTTL <= 0 {
		accessTTL = defaultAccessTTL
	}
	if refreshTTL <= 0 {
		refreshTTL = defaultRefreshTTL
	}
	return &JWTManager{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// Issue creates a signed access token and returns it with its expiry.
func (m *JWTManager) Issue(userID uint64, username string) (string, time.Time, error) {
	return m.issue(userID, username, TokenUseAccess, m.accessTTL)
}

// IssueRefresh creates a signed refresh token and returns it with its expiry.
func (m *JWTManager) IssueRefresh(userID uint64, username string) (string, time.Time, error) {
	return m.issue(userID, username, TokenUseRefresh, m.refreshTTL)
}

func (m *JWTManager) issue(userID uint64, username, tokenUse string, ttl time.Duration) (string, time.Time, error) {
	if len(m.secret) == 0 {
		return "", time.Time{}, errors.New("jwt secret is not configured")
	}
	if userID == 0 {
		return "", time.Time{}, errors.New("user id is required")
	}
	if strings.TrimSpace(username) == "" {
		return "", time.Time{}, errors.New("username is required")
	}

	now := time.Now()
	expiresAt := now.Add(ttl)
	claims := AuthClaims{
		UserID:   userID,
		Username: username,
		TokenUse: tokenUse,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    jwtIssuer,                      // 签发方：该 Token 由谁签发
			Subject:   strconv.FormatUint(userID, 10), // 主体：稳定的用户 ID
			Audience:  jwt.ClaimStrings{jwtAudience},  // 接收方：该 Token 允许被谁使用
			IssuedAt:  jwt.NewNumericDate(now),        // 签发时间：Token 开始生效的时间
			ExpiresAt: jwt.NewNumericDate(expiresAt),  // 过期时间：超过该时间后拒绝该 Token
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
	return m.parse(jwtString, TokenUseAccess)
}

// ParseRefresh validates a refresh JWT and rejects access tokens.
func (m *JWTManager) ParseRefresh(jwtString string) (*AuthClaims, error) {
	return m.parse(jwtString, TokenUseRefresh)
}

func (m *JWTManager) parse(jwtString, expectedUse string) (*AuthClaims, error) {
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
	if claims.Subject != strconv.FormatUint(claims.UserID, 10) {
		return nil, errors.New("jwt subject does not match user id")
	}
	if claims.TokenUse != expectedUse {
		return nil, errors.New("jwt token use is invalid")
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
