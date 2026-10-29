package handler

import (
	"gin-scaffold/internal/middleware"
	"gin-scaffold/internal/schema"
	"gin-scaffold/pkg/response"

	"github.com/gin-gonic/gin"
)

// Me returns the authenticated identity read from the JWT middleware.
func (h *AuthHandler) Me(c *gin.Context) {
	claims := middleware.CurrentClaims(c)
	if claims == nil {
		response.Fail(c, response.Unauthorized("authorization token is invalid"))
		return
	}

	response.OK(c, schema.UserDTO{
		ID:       claims.UserID,
		Username: claims.Username,
	})
}
