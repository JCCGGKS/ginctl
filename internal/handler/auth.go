// Package handler adapts HTTP to the business layer: bind input, call business,
// write a unified response. No business branching lives here.
package handler

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"

	"gin-scaffold/common"
	apperrors "gin-scaffold/common/errors"
	"gin-scaffold/internal/business"
	"gin-scaffold/internal/schema"
	"gin-scaffold/internal/svc"
)

// AuthHandler holds shared dependencies for auth endpoints.
type AuthHandler struct {
	svcCtx *svc.ServiceContext
}

// NewAuthHandler builds an auth handler set.
func NewAuthHandler(svcCtx *svc.ServiceContext) *AuthHandler {
	return &AuthHandler{svcCtx: svcCtx}
}

// Login follows the standard flow: bind -> business -> unified response.
func (h *AuthHandler) Login(c *gin.Context) {
	var req schema.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, apperrors.BadRequest("invalid request body"))
		return
	}

	resp, err := business.NewAuthBusiness(c.Request.Context(), h.svcCtx).Login(&req)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrInvalidCredentials):
			common.Fail(c, apperrors.Unauthorized("username or password is invalid"))
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			common.Fail(c, apperrors.Timeout("request cancelled"))
		default:
			common.Fail(c, err)
		}
		return
	}

	common.OK(c, resp)
}
