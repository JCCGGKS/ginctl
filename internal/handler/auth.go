// Package handler adapts HTTP to the business layer: bind input, call business,
// write a unified response. No business branching lives here.
package handler

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"

	"gin-scaffold/infra"
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
		infra.Fail(c, infra.BadRequest("invalid request body"))
		return
	}

	resp, err := business.NewAuthBusiness(c.Request.Context(), h.svcCtx).Login(&req)
	if err != nil {
		switch {
		case errors.Is(err, business.ErrInvalidCredentials):
			infra.Fail(c, infra.Unauthorized("username or password is invalid"))
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			infra.Fail(c, infra.Timeout("request cancelled"))
		default:
			infra.Fail(c, err)
		}
		return
	}

	infra.OK(c, resp)
}
