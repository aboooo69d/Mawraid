package repo

import (
	"context"

	"github.com/najm-x/nova/database/entities"
	"gorm.io/gorm"
)

// Names of the built-in roles (must be seeded in the database).
const (
	RoleConsumer = "consumer"
	RoleProvider = "provider"
)

type RoleRepo interface {
	GetRoleByName(c context.Context, name string) (*entities.Role, error)
}

type roleRepo struct {
	db *gorm.DB
}

func NewRoleRepo(db *gorm.DB) RoleRepo {
	return &roleRepo{db: db}
}

func (r *roleRepo) GetRoleByName(c context.Context, name string) (*entities.Role, error) {
	role, err := gorm.G[entities.Role](r.db).Where("name = ?", name).First(c)
	if err != nil {
		return nil, err
	}
	return &role, nil
}
