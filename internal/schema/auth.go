// Package schema holds API wire structs (request/response DTOs).
// Rule: no DB models here; no API DTOs in the model package.
package schema

// LoginRequest is the login payload. binding tags drive gin's validation.
type LoginRequest struct {
	Username string `json:"username" binding:"required,max=64"`
	Password string `json:"password" binding:"required,max=72"`
}

// LoginResponse is the login response payload.
type LoginResponse struct {
	AccessToken      string  `json:"access_token"`
	AccessExpiresAt  int64   `json:"access_expires_at"`
	RefreshToken     string  `json:"refresh_token"`
	RefreshExpiresAt int64   `json:"refresh_expires_at"`
	User             UserDTO `json:"user"`
}

// RefreshRequest exchanges a refresh token for a new access token.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required,max=4096"`
}

// RefreshResponse returns only a fresh access token; the refresh token is not rotated.
type RefreshResponse struct {
	AccessToken     string  `json:"access_token"`
	AccessExpiresAt int64   `json:"access_expires_at"`
	User            UserDTO `json:"user"`
}

// UserDTO is the API-facing user payload.
type UserDTO struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
}
