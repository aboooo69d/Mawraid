package filter

import (
	"strings"

	"github.com/najm-x/nova/database/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OrderFilter holds filtering and pagination parameters for querying orders.
type OrderFilter struct {
	Status       *entities.OrderStatus
	StockID      *string
	ConsumerType *entities.OrderConsumerType
	Page         int
	PageSize     int
}

// FilterScopes returns the WHERE filter scopes (safe to share between Count and Find).
func (f OrderFilter) FilterScopes() []func(*gorm.Statement) {
	return []func(*gorm.Statement){
		ByStatus(f.Status),
		ByStockID(f.StockID),
		ByConsumerType(f.ConsumerType),
	}
}

// ByStatus filters orders by exact status.
func ByStatus(status *entities.OrderStatus) func(*gorm.Statement) {
	return func(stmt *gorm.Statement) {
		if status == nil {
			return
		}
		trimmed := strings.TrimSpace(string(*status))
		if trimmed == "" {
			return
		}
		stmt.AddClause(clause.Where{
			Exprs: []clause.Expression{
				clause.Expr{
					SQL:  "orders.status = ?",
					Vars: []any{trimmed},
				},
			},
		})
	}
}

// ByStockID filters orders belonging to a specific stock UUID.
func ByStockID(stockID *string) func(*gorm.Statement) {
	return func(stmt *gorm.Statement) {
		if stockID == nil {
			return
		}
		trimmed := strings.TrimSpace(*stockID)
		if trimmed == "" {
			return
		}
		stmt.AddClause(clause.Where{
			Exprs: []clause.Expression{
				clause.Expr{
					SQL:  "orders.stock_id = ?",
					Vars: []any{trimmed},
				},
			},
		})
	}
}

// ByConsumerType filters orders by consumer booking type (home, business, driver).
func ByConsumerType(consumerType *entities.OrderConsumerType) func(*gorm.Statement) {
	return func(stmt *gorm.Statement) {
		if consumerType == nil {
			return
		}
		trimmed := strings.TrimSpace(string(*consumerType))
		if trimmed == "" {
			return
		}
		stmt.AddClause(clause.Where{
			Exprs: []clause.Expression{
				clause.Expr{
					SQL:  "orders.consumer_type = ?",
					Vars: []any{trimmed},
				},
			},
		})
	}
}

// OrderByNewest orders results from newest to oldest.
func OrderByNewest() func(*gorm.Statement) {
	return func(stmt *gorm.Statement) {
		stmt.AddClause(clause.OrderBy{
			Expression: clause.Expr{SQL: "orders.created_at DESC"},
		})
	}
}
