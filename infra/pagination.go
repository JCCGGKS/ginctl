package infra

const (
	DefaultPage     = 1
	DefaultPageSize = 10
	MaxPageSize     = 100
)

type Pagination struct {
	Page     int
	PageSize int
}

// Normalize clamps page/pageSize into safe bounds.
func Normalize(page, pageSize int) *Pagination {
	if page <= 0 {
		page = DefaultPage
	}
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	return &Pagination{Page: page, PageSize: pageSize}
}

func (p *Pagination) Offset() int { return (p.Page - 1) * p.PageSize }
func (p *Pagination) Limit() int  { return p.PageSize }

// CursorPagination represents keyset pagination based on an increasing id.
// Callers typically translate it to: where id > last_id order by id asc limit n.
type CursorPagination struct {
	LastID uint64
	LimitN int
}

// NormalizeIDCursor clamps the page size into safe bounds for id-based cursor
// pagination.
func NormalizeIDCursor(lastID uint64, limit int) *CursorPagination {
	if limit <= 0 {
		limit = DefaultPageSize
	}
	if limit > MaxPageSize {
		limit = MaxPageSize
	}
	return &CursorPagination{
		LastID: lastID,
		LimitN: limit,
	}
}

// Limit returns the normalized fetch size for id-based cursor pagination.
func (p *CursorPagination) Limit() int { return p.LimitN }

// HasCursor reports whether the request carries a non-zero last id.
func (p *CursorPagination) HasCursor() bool { return p.LastID > 0 }
