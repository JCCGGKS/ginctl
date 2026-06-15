// Package schema holds API wire structs (request/response DTOs).
// Rule: no DB models here; no API DTOs in the model package.
package schema

// LoginRequest is the login payload. binding tags drive gin's validation.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse is the login response payload.
type LoginResponse struct {
	AccessToken     string  `json:"access_token"`
	AccessExpiresAt int64   `json:"access_expires_at"`
	User            UserDTO `json:"user"`
}

// UserDTO is the API-facing user payload.
type UserDTO struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
}
