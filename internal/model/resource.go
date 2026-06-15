// Package model holds storage-oriented structs (one struct == one row shape).
// In a real project these carry db tags and map 1:1 to tables. Keep API
// response structs out of here (those live in schema).
package model

import "time"

// Resource is a demo domain/storage model.
type Resource struct {
	ID        uint64    `db:"id"`
	Name      string    `db:"name"`
	OwnerID   uint64    `db:"owner_id"`
	CreatedAt time.Time `db:"created_at"`
}
