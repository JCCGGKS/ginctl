// Package model holds storage-side structs. API wire DTOs live in schema,
// never here.
package model

import "time"

// User is a storage-side user row. PasswordHash is never serialized to clients.
type User struct {
	ID           uint64    `db:"id" gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Username     string    `db:"username" gorm:"column:username;size:64;not null;uniqueIndex:uk_users_username" json:"username"`
	PasswordHash string    `db:"password_hash" gorm:"column:password_hash;size:255;not null" json:"-"`
	CreatedAt    time.Time `db:"created_at" gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" gorm:"column:updated_at;not null" json:"updated_at"`
}

// TableName keeps the SQL table name stable if the Go type is renamed later.
func (User) TableName() string { return "users" }
