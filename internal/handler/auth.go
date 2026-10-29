// Package handler adapts HTTP to the logic layer: bind input, call logic,
// write a unified response. No business branching lives here.
package handler

import (
	"github.com/gin-gonic/gin"

	"gin-scaffold/infra/response"
	"gin-scaffold/internal/logic"
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

// Login follows the standard flow: bind -> logic -> unified response.
func (h *AuthHandler) Login(c *gin.Context) {
	var req schema.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.BadRequest(err.Error()))
		return
	}

	resp, err := logic.NewAuthLogic(c.Request.Context(), h.svcCtx).Login(&req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.OK(c, resp)
}
