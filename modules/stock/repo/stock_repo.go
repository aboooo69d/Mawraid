package stockRepo

import (
	"context"

	"github.com/najm-x/nova/database/entities"
	"github.com/najm-x/nova/database/scope"
	stockFilter "github.com/najm-x/nova/modules/stock/filter"
	"gorm.io/gorm"
)

type (
	StockRepo interface {
		CreateStock(c context.Context, stock *entities.Stock) error
		GetStocks(c context.Context, f stockFilter.StockFilter) ([]entities.Stock, int64, error)
		GetStockById(c context.Context, id string) (*entities.Stock, error)
		GetStocksByUserId(c context.Context, userId string) ([]entities.Stock, error)
		GetStockTypeById(c context.Context, id uint) (*entities.StockType, error)
		GetStockTypes(c context.Context) ([]entities.StockType, error)
		GetStockTypesByProviderTypeId(c context.Context, providerTypeId uint) ([]entities.StockType, error)
		GetProviderTypeIdByUserId(c context.Context, userId string) (uint, error)
		UpdateStock(c context.Context, id string, stock *entities.Stock) error
		DeleteStock(c context.Context, id string) error
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
		if u.Price < 0 {
			return ErrInvalidUnitPrice
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

func (r *stockRepo) GetStocks(c context.Context, f stockFilter.StockFilter) ([]entities.Stock, int64, error) {
	q := gorm.G[entities.Stock](r.db).Scopes(f.FilterScopes()...)

	total, err := q.Count(c, "id")
	if err != nil {
		return nil, 0, err
	}

	stocks, err := q.
		Scopes(
			stockFilter.OrderByNearest(f.Lat, f.Lng),
			scope.Paginate(f.Page, f.PageSize),
		).
		Preload("StockType", nil).
		Preload("Units", func(db gorm.PreloadBuilder) error {
			db.Order("is_main DESC, rate ASC")
			return nil
		}).
		Find(c)
	if err != nil {
		return nil, 0, err
	}

	return stocks, total, nil
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

func (r *stockRepo) GetStocksByUserId(c context.Context, userId string) ([]entities.Stock, error) {
	return gorm.G[entities.Stock](r.db).
		Preload("StockType", nil).
		Preload("Units", func(db gorm.PreloadBuilder) error {
			db.Order("is_main DESC, rate ASC")
			return nil
		}).
		Where("user_id = ?", userId).
		Order("created_at DESC").
		Find(c)
}

func (r *stockRepo) GetStockTypeById(c context.Context, id uint) (*entities.StockType, error) {
	st, err := gorm.G[entities.StockType](r.db).
		Where("id = ?", id).
		First(c)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (r *stockRepo) GetStockTypes(c context.Context) ([]entities.StockType, error) {
	return gorm.G[entities.StockType](r.db).Order("id ASC").Find(c)
}

func (r *stockRepo) GetStockTypesByProviderTypeId(c context.Context, providerTypeId uint) ([]entities.StockType, error) {
	return gorm.G[entities.StockType](r.db).
		Where("provider_type_id = ?", providerTypeId).
		Order("id ASC").
		Find(c)
}

func (r *stockRepo) GetProviderTypeIdByUserId(c context.Context, userId string) (uint, error) {
	provider, err := gorm.G[entities.Provider](r.db).
		Where("user_id = ?", userId).
		First(c)
	if err != nil {
		return 0, err
	}
	return provider.ProviderTypeID, nil
}

func (r *stockRepo) UpdateStock(c context.Context, id string, stock *entities.Stock) error {
	// Select(...) is required so zero values are written too.
	res := r.db.WithContext(c).
		Model(&entities.Stock{}).
		Where("id = ?", id).
		Select("StockTypeID", "Address", "Location", "StockDetails").
		Updates(&entities.Stock{
			StockTypeID:  stock.StockTypeID,
			Address:      stock.Address,
			Location:     stock.Location,
			StockDetails: stock.StockDetails,
		})
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
