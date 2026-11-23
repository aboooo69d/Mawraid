package addressRepo

import (
	"context"

	"github.com/najm-x/nova/database/entities"
	"gorm.io/gorm"
)

type (
	AddressRepo interface {
		// Create saves the address. The first address of a user becomes default;
		// if address.IsDefault is true, other addresses are unset (atomic).
		Create(c context.Context, address *entities.Address) error
		GetByIdAndUser(c context.Context, id string, userId string) (*entities.Address, error)
		GetById(c context.Context, id string) (*entities.Address, error)
		GetDefaultByUser(c context.Context, userId string) (*entities.Address, error)
		ListByUser(c context.Context, userId string) ([]entities.Address, error)
	}

	addressRepo struct {
		db *gorm.DB
	}
)

func NewAddressRepo(db *gorm.DB) AddressRepo {
	return &addressRepo{db: db}
}

func (r *addressRepo) Create(c context.Context, address *entities.Address) error {
	return r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		count, err := gorm.G[entities.Address](tx).Where("user_id = ?", address.UserID).Count(c, "id")
		if err != nil {
			return err
		}
		if count == 0 {
			address.IsDefault = true
		}
		if address.IsDefault && count > 0 {
			if _, err := gorm.G[entities.Address](tx).
				Where("user_id = ? AND is_default = true", address.UserID).
				Update(c, "is_default", false); err != nil {
				return err
			}
		}
		return gorm.G[entities.Address](tx).Create(c, address)
	})
}

func (r *addressRepo) GetByIdAndUser(c context.Context, id string, userId string) (*entities.Address, error) {
	a, err := gorm.G[entities.Address](r.db).Where("id = ? AND user_id = ?", id, userId).First(c)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *addressRepo) GetById(c context.Context, id string) (*entities.Address, error) {
	a, err := gorm.G[entities.Address](r.db).Where("id = ?", id).First(c)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *addressRepo) GetDefaultByUser(c context.Context, userId string) (*entities.Address, error) {
	a, err := gorm.G[entities.Address](r.db).Where("user_id = ? AND is_default = true", userId).First(c)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *addressRepo) ListByUser(c context.Context, userId string) ([]entities.Address, error) {
	return gorm.G[entities.Address](r.db).
		Where("user_id = ?", userId).
		Order("is_default DESC, created_at DESC").
		Find(c)
}
