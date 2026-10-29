// Package model holds storage-side structs. API wire DTOs live in schema,
// never here.
package model

import "time"

// User is a storage-side user row. PasswordHash is never serialized to clients.
type User struct {
	ID           uint64    `db:"id" json:"id"`
	Username     string    `db:"username" json:"username"`
	PasswordHash string    `db:"password_hash" json:"-"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}
