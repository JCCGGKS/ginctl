// Package pagination centralizes page/size normalization so every list
// endpoint shares the same defaults and upper bound (from the reference
// project's utils/pagination.go).
package pagination

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
func Normalize(page, pageSize int) Pagination {
	if page <= 0 {
		page = DefaultPage
	}
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	return Pagination{Page: page, PageSize: pageSize}
}

func (p Pagination) Offset() int { return (p.Page - 1) * p.PageSize }
func (p Pagination) Limit() int  { return p.PageSize }
