package handler

import (
	"gin-scaffold/infra"
	"gin-scaffold/internal/middleware"
	"gin-scaffold/internal/schema"

	"github.com/gin-gonic/gin"
)

// Me returns the authenticated identity read from the JWT middleware.
func (h *AuthHandler) Me(c *gin.Context) {
	claims := middleware.CurrentClaims(c)
	if claims == nil {
		infra.Fail(c, infra.Unauthorized("authorization token is invalid"))
		return
	}

	infra.OK(c, schema.UserDTO{
		ID:       claims.UserID,
		Username: claims.Username,
	})
}
