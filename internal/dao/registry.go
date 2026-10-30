package dao

import "gorm.io/gorm"

// Registry is the single entry point for all DAOs. Add one field here for each
// new model and construct it in NewRegistry.
type Registry struct {
	User UserDAO
}

// NewRegistry constructs all DAOs from the shared database handle.
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		User: NewUserDAO(db),
	}
}
