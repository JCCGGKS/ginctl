package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"gin-scaffold/internal/logic"
	"gin-scaffold/internal/schema"
	"gin-scaffold/internal/svc"
	"gin-scaffold/pkg/response"
)

type ResourceHandler struct {
	svcCtx *svc.ServiceContext
}

func NewResourceHandler(svcCtx *svc.ServiceContext) *ResourceHandler {
	return &ResourceHandler{svcCtx: svcCtx}
}

// List binds query params and returns a paginated list.
func (h *ResourceHandler) List(c *gin.Context) {
	var req schema.ListResourcesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.BadRequest(err.Error()))
		return
	}

	resp, err := logic.NewResourceLogic(c.Request.Context(), h.svcCtx).List(&req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.OK(c, resp)
}

// Get binds the :id path param and returns one resource.
func (h *ResourceHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, response.BadRequest("invalid resource id"))
		return
	}

	resp, err := logic.NewResourceLogic(c.Request.Context(), h.svcCtx).Get(id)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.OK(c, resp)
}
