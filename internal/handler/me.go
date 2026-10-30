package handler

import (
	"gin-scaffold/common"
	apperrors "gin-scaffold/common/errors"
	"gin-scaffold/common/middleware"
	"gin-scaffold/internal/schema"

	"github.com/gin-gonic/gin"
)

// Me returns the authenticated identity read from the JWT middleware.
func (h *AuthHandler) Me(c *gin.Context) {
	claims := middleware.CurrentClaims(c)
	if claims == nil {
		common.Fail(c, apperrors.Unauthorized("authorization JWT is invalid"))
		return
	}

	common.OK(c, schema.UserDTO{
		ID:       claims.UserID,
		Username: claims.Username,
	})
}
