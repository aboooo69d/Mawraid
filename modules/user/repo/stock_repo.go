package repo

import (
	"context"

	"github.com/najm-x/nova/database/entities"
	"gorm.io/gorm"
)

type (
	UserStockRepo interface {
		GetStockByUserId(c context.Context, userId string) (*entities.Stock, error)
	}

	userStockRepo struct {
		db *gorm.DB
	}
)

func NewUserStockRepo(db *gorm.DB) UserStockRepo {
	return &userStockRepo{db: db}
}

func (r *userStockRepo) GetStockByUserId(c context.Context, userId string) (*entities.Stock, error) {
	stock, err := gorm.G[entities.Stock](r.db).
		Preload("StockType", nil).
		Preload("Units", func(db gorm.PreloadBuilder) error {
			db.Order("is_main DESC, rate ASC")
			return nil
		}).
		Where("user_id = ?", userId).
		First(c)
	if err != nil {
		return nil, err
	}
	return &stock, nil
}
