package pagination

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name       string
		page       int
		pageSize   int
		wantPage   int
		wantSize   int
		wantOffset int
		wantLimit  int
	}{
		{"defaults", 0, 0, DefaultPage, DefaultPageSize, 0, DefaultPageSize},
		{"page two", 2, 20, 2, 20, 20, 20},
		{"clamps negative", -1, -5, DefaultPage, DefaultPageSize, 0, DefaultPageSize},
		{"clamps max size", 3, 999, 3, MaxPageSize, 2 * MaxPageSize, MaxPageSize},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Normalize(tt.page, tt.pageSize)
			if p.Page != tt.wantPage || p.PageSize != tt.wantSize || p.Offset() != tt.wantOffset || p.Limit() != tt.wantLimit {
				t.Fatalf("Normalize(%d,%d)=%+v offset=%d limit=%d, want page=%d size=%d offset=%d limit=%d",
					tt.page, tt.pageSize, p, p.Offset(), p.Limit(), tt.wantPage, tt.wantSize, tt.wantOffset, tt.wantLimit)
			}
		})
	}
}

func TestNormalizeIDCursor(t *testing.T) {
	tests := []struct {
		name    string
		lastID  uint64
		limit   int
		wantN   int
		wantCur bool
	}{
		{"default limit", 0, 0, DefaultPageSize, false},
		{"explicit limit", 42, 25, 25, true},
		{"clamps max", 7, MaxPageSize + 1, MaxPageSize, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NormalizeIDCursor(tt.lastID, tt.limit)
			if p.Limit() != tt.wantN || p.HasCursor() != tt.wantCur {
				t.Fatalf("NormalizeIDCursor(%d,%d)=%+v limit=%d hasCursor=%v, want limit=%d hasCursor=%v",
					tt.lastID, tt.limit, p, p.Limit(), p.HasCursor(), tt.wantN, tt.wantCur)
			}
		})
	}
}
