package dao

import "gorm.io/gorm"

// Registry is the single entry point for domain DAOs. Keeping the aggregate
// lets future domains grow without changing the shape of ServiceContext.
type Registry struct {
	User UserDAO
}

// NewRegistry constructs all DAOs from the shared database handle.
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{User: NewUserDAO(db)}
}
