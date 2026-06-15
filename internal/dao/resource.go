// Package dao isolates data access behind interfaces. Logic depends on the
// interface, not the concrete store, so you can swap the in-memory demo for
// MySQL/Redis without touching logic (mirrors the reference dao boundary).
package dao

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"gin-scaffold/internal/model"
)

// ResourceDAO is the data-access contract for resources.
type ResourceDAO interface {
	Count(ctx context.Context, keyword string) (int, error)
	List(ctx context.Context, keyword string, offset, limit int) ([]model.Resource, error)
	FindByID(ctx context.Context, id uint64) (*model.Resource, error)
}

// inMemoryResourceDAO is a dependency-free implementation so the skeleton runs
// without a database. Replace with a SQL-backed DAO in real projects.
type inMemoryResourceDAO struct {
	mu   sync.RWMutex
	rows []model.Resource
}

// NewInMemoryResourceDAO returns a demo DAO seeded with sample rows.
func NewInMemoryResourceDAO() ResourceDAO {
	now := time.Now()
	rows := make([]model.Resource, 0, 25)
	for i := 1; i <= 25; i++ {
		rows = append(rows, model.Resource{
			ID:        uint64(i),
			Name:      "resource-" + strconv.Itoa(i),
			OwnerID:   1,
			CreatedAt: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	return &inMemoryResourceDAO{rows: rows}
}

func (d *inMemoryResourceDAO) filter(keyword string) []model.Resource {
	if keyword == "" {
		return d.rows
	}
	out := make([]model.Resource, 0, len(d.rows))
	for _, r := range d.rows {
		if strings.Contains(r.Name, keyword) {
			out = append(out, r)
		}
	}
	return out
}

func (d *inMemoryResourceDAO) Count(_ context.Context, keyword string) (int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.filter(keyword)), nil
}

func (d *inMemoryResourceDAO) List(_ context.Context, keyword string, offset, limit int) ([]model.Resource, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows := d.filter(keyword)
	if offset >= len(rows) {
		return []model.Resource{}, nil
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	out := make([]model.Resource, end-offset)
	copy(out, rows[offset:end])
	return out, nil
}

func (d *inMemoryResourceDAO) FindByID(_ context.Context, id uint64) (*model.Resource, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for i := range d.rows {
		if d.rows[i].ID == id {
			r := d.rows[i]
			return &r, nil
		}
	}
	return nil, nil // not found -> logic translates to 404
}
