package filter

import (
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StockFilter holds filtering, geo-sorting, and pagination parameters for querying stocks.
type StockFilter struct {
	Name        *string
	StockType   *string
	StockTypeID *uint
	InStock     *bool
	Lat         *float64
	Lng         *float64
	MaxDistance *float64 // meters
	Page        int
	PageSize    int
}

// FilterScopes returns the WHERE filter scopes (safe to share between Count and Find).
func (f StockFilter) FilterScopes() []func(*gorm.Statement) {
	return []func(*gorm.Statement){
		ByName(f.Name),
		ByStockType(f.StockType),
		ByStockTypeID(f.StockTypeID),
		ByInStock(f.InStock),
		ByMaxDistance(f.Lat, f.Lng, f.MaxDistance),
	}
}

// ByName filters stocks whose address or owning provider's name matches the search term (case-insensitive).
func ByName(name *string) func(*gorm.Statement) {
	return func(stmt *gorm.Statement) {
		if name == nil {
			return
		}
		trimmed := strings.TrimSpace(*name)
		if trimmed == "" {
			return
		}
		pattern := "%" + trimmed + "%"
		stmt.AddClause(clause.Where{
			Exprs: []clause.Expression{
				clause.Expr{
					SQL: "(stocks.address ILIKE ? OR EXISTS (SELECT 1 FROM providers WHERE providers.user_id = stocks.user_id AND providers.deleted_at IS NULL AND providers.name ILIKE ?))",
					Vars: []any{
						pattern,
						pattern,
					},
				},
			},
		})
	}
}

// ByStockType filters stocks by stock type slug or name (case-insensitive).
func ByStockType(stockType *string) func(*gorm.Statement) {
	return func(stmt *gorm.Statement) {
		if stockType == nil {
			return
		}
		trimmed := strings.TrimSpace(*stockType)
		if trimmed == "" {
			return
		}
		stmt.AddClause(clause.Where{
			Exprs: []clause.Expression{
				clause.Expr{
					SQL: "EXISTS (SELECT 1 FROM stock_types WHERE stock_types.id = stocks.stock_type_id AND stock_types.deleted_at IS NULL AND (LOWER(stock_types.slug) = LOWER(?) OR LOWER(stock_types.name) = LOWER(?)))",
					Vars: []any{
						trimmed,
						trimmed,
					},
				},
			},
		})
	}
}

// ByStockTypeID filters stocks by an exact stock_type_id.
func ByStockTypeID(stockTypeID *uint) func(*gorm.Statement) {
	return func(stmt *gorm.Statement) {
		if stockTypeID == nil || *stockTypeID == 0 {
			return
		}
		stmt.AddClause(clause.Where{
			Exprs: []clause.Expression{
				clause.Expr{
					SQL:  "stocks.stock_type_id = ?",
					Vars: []any{*stockTypeID},
				},
			},
		})
	}
}

// ByInStock filters stocks by availability (true: available > 0, false: available <= 0).
func ByInStock(inStock *bool) func(*gorm.Statement) {
	return func(stmt *gorm.Statement) {
		if inStock == nil {
			return
		}
		sql := "stocks.available <= 0"
		if *inStock {
			sql = "stocks.available > 0"
		}
		stmt.AddClause(clause.Where{
			Exprs: []clause.Expression{
				clause.Expr{SQL: sql},
			},
		})
	}
}

// ByMaxDistance restricts results to stocks within maxDistance meters of (lat, lng).
func ByMaxDistance(lat, lng, maxDistance *float64) func(*gorm.Statement) {
	return func(stmt *gorm.Statement) {
		if lat == nil || lng == nil || maxDistance == nil || *maxDistance <= 0 {
			return
		}
		stmt.AddClause(clause.Where{
			Exprs: []clause.Expression{
				clause.Expr{
					SQL:  "ST_DWithin(stocks.location, ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography, ?)",
					Vars: []any{*lng, *lat, *maxDistance},
				},
			},
		})
	}
}

// OrderByNearest orders stocks by distance from (lat, lng) ascending using PostGIS geography distance.
// When coordinates are not provided, it falls back to newest stocks first.
func OrderByNearest(lat, lng *float64) func(*gorm.Statement) {
	return func(stmt *gorm.Statement) {
		if lat == nil || lng == nil {
			stmt.AddClause(clause.OrderBy{
				Expression: clause.Expr{SQL: "stocks.created_at DESC"},
			})
			return
		}
		stmt.AddClause(clause.OrderBy{
			Expression: clause.Expr{
				SQL:  "ST_Distance(stocks.location, ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography) ASC, stocks.created_at DESC",
				Vars: []any{*lng, *lat},
			},
		})
	}
}
