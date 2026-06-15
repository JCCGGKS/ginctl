package logic

import (
	"context"

	"gin-scaffold/internal/schema"
	"gin-scaffold/internal/svc"
	"gin-scaffold/pkg/pagination"
	"gin-scaffold/pkg/response"
)

type ResourceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewResourceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResourceLogic {
	return &ResourceLogic{ctx: ctx, svcCtx: svcCtx}
}

// List returns a normalized, paginated view over the resource store.
func (l *ResourceLogic) List(req *schema.ListResourcesRequest) (*schema.ListResourcesResponse, error) {
	page := pagination.Normalize(req.Page, req.PageSize)

	total, err := l.svcCtx.ResourceDAO.Count(l.ctx, req.Keyword)
	if err != nil {
		return nil, response.Internal("count resources failed")
	}

	rows, err := l.svcCtx.ResourceDAO.List(l.ctx, req.Keyword, page.Offset(), page.Limit())
	if err != nil {
		return nil, response.Internal("list resources failed")
	}

	items := make([]schema.ResourceItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, schema.ResourceItem{
			ID:        r.ID,
			Name:      r.Name,
			OwnerID:   r.OwnerID,
			CreatedAt: r.CreatedAt.Unix(),
		})
	}

	return &schema.ListResourcesResponse{
		Items:    items,
		Total:    total,
		Page:     page.Page,
		PageSize: page.PageSize,
		HasMore:  page.Offset()+len(items) < total,
	}, nil
}

// Get returns one resource or a 404-typed error.
func (l *ResourceLogic) Get(id uint64) (*schema.ResourceItem, error) {
	row, err := l.svcCtx.ResourceDAO.FindByID(l.ctx, id)
	if err != nil {
		return nil, response.Internal("get resource failed")
	}
	if row == nil {
		return nil, response.NotFound("resource not found")
	}

	return &schema.ResourceItem{
		ID:        row.ID,
		Name:      row.Name,
		OwnerID:   row.OwnerID,
		CreatedAt: row.CreatedAt.Unix(),
	}, nil
}
