// Package handler adapts HTTP to the business layer: bind input, call business,
// write a unified response. No business branching lives here.
package handler

import (
	"github.com/gin-gonic/gin"

	"gin-scaffold/common"
	"gin-scaffold/internal/business"
	apperrors "gin-scaffold/internal/errors"
	"gin-scaffold/internal/schema"
)

// AuthHandler adapts authentication use cases to HTTP.
type AuthHandler struct {
	auth *business.AuthService
}

// NewAuthHandler builds an auth handler set.
func NewAuthHandler(auth *business.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

// Login follows the standard flow: bind -> business -> unified response.
func (h *AuthHandler) Login(c *gin.Context) {
	// Content-Type：请求体媒体类型；登录接口只接受 JSON，避免表单/文本解析歧义。
	if c.ContentType() != "application/json" {
		common.Fail(c, apperrors.BadRequest("content type must be application/json"))
		return
	}

	var req schema.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, apperrors.BadRequest("invalid request body"))
		return
	}

	resp, err := h.auth.Login(c.Request.Context(), &req)
	if err != nil {
		common.Fail(c, err)
		return
	}

	common.OK(c, resp)
}

// Refresh exchanges a refresh token for a new access token.
func (h *AuthHandler) Refresh(c *gin.Context) {
	// Content-Type：请求体媒体类型；刷新接口只接受 JSON。
	if c.ContentType() != "application/json" {
		common.Fail(c, apperrors.BadRequest("content type must be application/json"))
		return
	}

	var req schema.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, apperrors.BadRequest("invalid request body"))
		return
	}

	resp, err := h.auth.Refresh(c.Request.Context(), &req)
	if err != nil {
		common.Fail(c, err)
		return
	}

	common.OK(c, resp)
}
