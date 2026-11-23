package stockRepo

import (
	"context"
	"errors"

	"github.com/najm-x/nova/database/entities"
	"github.com/najm-x/nova/database/scope"
	"gorm.io/gorm"
)

var (
	// Returned by ApplyTransaction when a withdrawal is bigger than Stock.Available.
	ErrInsufficientStock = errors.New("insufficient stock")
)

type (
	TransactionFilter struct {
		IsIn     *bool // nil = all, true = only in, false = only out
		Page     int
		PageSize int
	}

	TransactionRepo interface {
		ApplyTransaction(c context.Context, t *entities.Transaction, qty float64) error
		GetTransactionById(c context.Context, stockId string, id string) (*entities.Transaction, error)
		GetTransactionsByStockId(c context.Context, stockId string, f TransactionFilter) ([]entities.Transaction, int64, error)
	}

	transactionRepo struct {
		db *gorm.DB
	}
)

func NewTransactionRepo(db *gorm.DB) TransactionRepo {
	return &transactionRepo{db: db}
}

// ApplyTransaction changes Stock.Available by qty (already converted to the main unit
// by the service) and records t, all in one DB transaction (both succeed or both are rolled back).
//
//	t.IsIn = true  -> Available += qty
//	t.IsIn = false -> Available -= qty (fails with ErrInsufficientStock if not enough)
func (r *transactionRepo) ApplyTransaction(c context.Context, t *entities.Transaction, qty float64) error {
	return r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		// Update the balance atomically inside the DB (safe with concurrent requests).
		q := tx.Model(&entities.Stock{}).Where("id = ?", t.StockID)
		var res *gorm.DB
		if t.IsIn {
			res = q.Update("available", gorm.Expr("available + ?", qty))
		} else {
			// The condition makes the update a no-op if there is not enough stock.
			res = q.Where("available >= ?", qty).
				Update("available", gorm.Expr("available - ?", qty))
		}
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			if t.IsIn {
				return gorm.ErrRecordNotFound // stock does not exist
			}
			return ErrInsufficientStock
		}

		// Record the transaction.
		return tx.Create(t).Error
	})
}

func (r *transactionRepo) GetTransactionById(c context.Context, stockId string, id string) (*entities.Transaction, error) {
	var t entities.Transaction
	err := r.db.WithContext(c).
		Preload("Unit", withDeleted).
		Where("id = ? AND stock_id = ?", id, stockId).
		First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// GetTransactionsByStockId returns one page of the stock's history (newest first)
// and the total number of matching transactions (for pagination).
func (r *transactionRepo) GetTransactionsByStockId(c context.Context, stockId string, f TransactionFilter) ([]entities.Transaction, int64, error) {
	q := gorm.G[entities.Transaction](r.db).Where("stock_id = ?", stockId)
	if f.IsIn != nil {
		q = q.Where("is_in = ?", *f.IsIn)
	}

	total, err := q.Count(c, "id")
	if err != nil {
		return nil, 0, err
	}

	list, err := q.
		Scopes(scope.Paginate(f.Page, f.PageSize)).
		Preload("Unit", nil).
		Order("created_at DESC").
		Find(c)
	if err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// withDeleted loads soft-deleted units too, so old transactions still show their unit.
func withDeleted(db *gorm.DB) *gorm.DB {
	return db.Unscoped()
}
