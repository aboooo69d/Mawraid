package stockRepo

import (
	"context"
	"errors"

	"github.com/najm-x/nova/database/entities"
	"gorm.io/gorm"
)

var (
	ErrStockAlreadyExists = errors.New("user already has a stock")
)

type (
	StockRepo interface {
		CreateStock(c context.Context, stock *entities.Stock) error
		GetStockById(c context.Context, id string) (*entities.Stock, error)
		GetStockByUserId(c context.Context, userId string) (entities.Stock, error)
		UserHasStock(c context.Context, userId string) (bool, error)
		StockTypeExists(c context.Context, id uint) (bool, error)
		UpdateStock(c context.Context, id string, stock *entities.Stock) error
		DeleteStock(c context.Context, id string) error
		GetStockTypes(c context.Context) ([]entities.StockType, error)
	}

	stockRepo struct {
		db *gorm.DB
	}
)

func NewStockRepo(db *gorm.DB) StockRepo {
	return &stockRepo{db: db}
}

// CreateStock inserts the stock together with stock.Units.
// stock.Units must contain exactly one main unit (IsMain = true, Rate = 1);
// it is the base unit that Stock.Available is stored in.
// GORM saves the associations in the same DB transaction and sets each unit's StockID.
func (r *stockRepo) CreateStock(c context.Context, stock *entities.Stock) error {
	if err := validateStockUnits(stock.Units); err != nil {
		return err
	}
	return gorm.G[entities.Stock](r.db).Create(c, stock)
}

func validateStockUnits(units []entities.Unit) error {
	mainCount := 0
	for _, u := range units {
		if u.Rate < 1 {
			return ErrInvalidUnitRate
		}
		if u.IsMain {
			mainCount++
			if u.Rate != 1 {
				return ErrMainUnitRate
			}
		}
	}
	if mainCount != 1 {
		return ErrMainUnitRequired
	}
	return nil
}

func (r *stockRepo) GetStockById(c context.Context, id string) (*entities.Stock, error) {
	stock, err := gorm.G[entities.Stock](r.db).
		Preload("StockType", nil).
		Preload("Units", func(db gorm.PreloadBuilder) error {
			db.Order("is_main DESC, rate ASC")
			return nil
		}).
		Where("id = ?", id).
		First(c)
	if err != nil {
		return nil, err
	}
	return &stock, nil
}

func (r *stockRepo) GetStockByUserId(c context.Context, userId string) (entities.Stock, error) {
	return gorm.G[entities.Stock](r.db).Preload("StockType", nil).Preload("Units", func(db gorm.PreloadBuilder) error {
		db.Order("is_main DESC, rate ASC")
		return nil
	}).Where("user_id = ?", userId).First(c)
}

func (r *stockRepo) UserHasStock(c context.Context, userId string) (bool, error) {
	var count int64
	err := r.db.WithContext(c).
		Model(&entities.Stock{}).
		Where("user_id = ?", userId).
		Count(&count).Error
	return count > 0, err
}

func (r *stockRepo) StockTypeExists(c context.Context, id uint) (bool, error) {
	var count int64
	err := r.db.WithContext(c).
		Model(&entities.StockType{}).
		Where("id = ?", id).
		Count(&count).Error
	return count > 0, err
}

func (r *stockRepo) UpdateStock(c context.Context, id string, stock *entities.Stock) error {
	// Select(...) is required so zero values are written too.
	res := r.db.WithContext(c).
		Model(&entities.Stock{}).
		Where("id = ?", id).
		Select("StockTypeID").
		Updates(&entities.Stock{StockTypeID: stock.StockTypeID})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteStock soft-deletes the stock and its units.
// (The DB-level ON DELETE CASCADE only fires on hard deletes, so units are deleted explicitly.)
func (r *stockRepo) DeleteStock(c context.Context, id string) error {
	return r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ?", id).Delete(&entities.Stock{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Where("stock_id = ?", id).Delete(&entities.Unit{}).Error
	})
}

func (r *stockRepo) GetStockTypes(c context.Context) ([]entities.StockType, error) {
	return gorm.G[entities.StockType](r.db).Find(c)
}
