package handler

import (
	"github.com/gin-gonic/gin"

	"gin-scaffold/internal/logic"
	"gin-scaffold/internal/schema"
	"gin-scaffold/internal/svc"
	"gin-scaffold/pkg/response"
)

type AuthHandler struct {
	svcCtx *svc.ServiceContext
}

func NewAuthHandler(svcCtx *svc.ServiceContext) *AuthHandler {
	return &AuthHandler{svcCtx: svcCtx}
}

// Login: bind -> logic -> unified response. The four-step shape is identical
// for every handler (compare reference loginhandler.go).
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
