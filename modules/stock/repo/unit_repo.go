package stockRepo

import (
	"context"
	"errors"

	"github.com/najm-x/nova/database/entities"
	"gorm.io/gorm"
)

var (
	// The main unit is the base that Stock.Available is stored in,
	// so it must keep Rate = 1 and cannot be deleted.
	ErrMainUnitRate         = errors.New("main unit rate must be 1")
	ErrMainUnitRequired     = errors.New("stock must have exactly one main unit")
	ErrInvalidUnitRate      = errors.New("unit rate must be at least 1")
	ErrCannotDeleteMainUnit = errors.New("main unit cannot be deleted")
	// Changing the rate of a used unit would corrupt the meaning of its past transactions.
	ErrUnitHasTransactions = errors.New("cannot change rate of a unit that has transactions")
)

type (
	UnitRepo interface {
		CreateUnit(c context.Context, unit *entities.Unit) error
		GetUnitById(c context.Context, stockId string, unitId uint) (*entities.Unit, error)
		GetUnitsByStockId(c context.Context, stockId string) ([]entities.Unit, error)
		GetMainUnit(c context.Context, stockId string) (*entities.Unit, error)
		UpdateUnit(c context.Context, stockId string, unitId uint, unit *entities.Unit) error
		DeleteUnit(c context.Context, stockId string, unitId uint) error
		UnitHasTransactions(c context.Context, unitId uint) (bool, error)
	}

	unitRepo struct {
		db *gorm.DB
	}
)

func NewUnitRepo(db *gorm.DB) UnitRepo {
	return &unitRepo{db: db}
}

// CreateUnit adds an extra (non-main) unit to an existing stock.
// The main unit is created together with the stock (see StockRepo.CreateStock).
func (u *unitRepo) CreateUnit(c context.Context, unit *entities.Unit) error {
	if unit.Rate < 1 {
		return ErrInvalidUnitRate
	}
	unit.IsMain = false
	return gorm.G[entities.Unit](u.db).Create(c, unit)
}

// GetUnitById is scoped by stockId so a unit of another stock can never be returned.
func (u *unitRepo) GetUnitById(c context.Context, stockId string, unitId uint) (*entities.Unit, error) {
	unit, err := gorm.G[entities.Unit](u.db).
		Where("id = ? AND stock_id = ?", unitId, stockId).
		First(c)
	if err != nil {
		return nil, err
	}
	return &unit, nil
}

func (u *unitRepo) GetUnitsByStockId(c context.Context, stockId string) ([]entities.Unit, error) {
	return gorm.G[entities.Unit](u.db).
		Where("stock_id = ?", stockId).
		Order("is_main DESC, rate ASC").
		Find(c)
}

func (u *unitRepo) GetMainUnit(c context.Context, stockId string) (*entities.Unit, error) {
	unit, err := gorm.G[entities.Unit](u.db).
		Where("stock_id = ? AND is_main = ?", stockId, true).
		First(c)
	if err != nil {
		return nil, err
	}
	return &unit, nil
}

// UpdateUnit updates Name and Rate. IsMain is never changed here.
func (u *unitRepo) UpdateUnit(c context.Context, stockId string, unitId uint, unit *entities.Unit) error {
	return u.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		var current entities.Unit
		if err := tx.Where("id = ? AND stock_id = ?", unitId, stockId).
			First(&current).Error; err != nil {
			return err
		}

		if current.IsMain && unit.Rate != 1 {
			return ErrMainUnitRate
		}
		if unit.Rate < 1 {
			return ErrInvalidUnitRate
		}

		if unit.Rate != current.Rate {
			var count int64
			if err := tx.Model(&entities.Transaction{}).
				Where("unit_id = ?", unitId).
				Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return ErrUnitHasTransactions
			}
		}

		// Select(...) is required so zero values are written too.
		return tx.Model(&current).
			Select("Name", "Rate").
			Updates(&entities.Unit{Name: unit.Name, Rate: unit.Rate}).Error
	})
}

// DeleteUnit soft-deletes a non-main unit. Past transactions keep pointing to it.
func (u *unitRepo) DeleteUnit(c context.Context, stockId string, unitId uint) error {
	return u.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		var current entities.Unit
		if err := tx.Where("id = ? AND stock_id = ?", unitId, stockId).
			First(&current).Error; err != nil {
			return err
		}
		if current.IsMain {
			return ErrCannotDeleteMainUnit
		}
		return tx.Delete(&current).Error
	})
}

func (u *unitRepo) UnitHasTransactions(c context.Context, unitId uint) (bool, error) {
	var count int64
	err := u.db.WithContext(c).
		Model(&entities.Transaction{}).
		Where("unit_id = ?", unitId).
		Limit(1).
		Count(&count).Error
	return count > 0, err
}
