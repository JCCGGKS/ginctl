package schema

// ListResourcesRequest binds from the query string (?page=&page_size=&keyword=).
type ListResourcesRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Keyword  string `form:"keyword"`
}

type ResourceItem struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	OwnerID   uint64 `json:"owner_id"`
	CreatedAt int64  `json:"created_at"`
}

type ListResourcesResponse struct {
	Items    []ResourceItem `json:"items"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	HasMore  bool           `json:"has_more"`
}
