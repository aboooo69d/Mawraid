package filter_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/najm-x/nova/database/entities"
	"github.com/najm-x/nova/database/scope"
	stockFilter "github.com/najm-x/nova/modules/stock/filter"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type noopConnPool struct{}

func (noopConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) { return nil, nil }
func (noopConnPool) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, nil
}
func (noopConnPool) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, nil
}
func (noopConnPool) QueryRowContext(context.Context, string, ...any) *sql.Row { return nil }

func newDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 noopConnPool{},
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("failed to open dry-run gorm db: %v", err)
	}
	return db
}

func TestStockFilterScopes_SQLGeneration(t *testing.T) {
	db := newDryRunDB(t)

	name := "gaz station"
	stockType := "gaz"
	stockTypeID := uint(2)
	inStock := true
	lat := 31.5017
	lng := 34.4668
	maxDist := 5000.0

	f := stockFilter.StockFilter{
		Name:        &name,
		StockType:   &stockType,
		StockTypeID: &stockTypeID,
		InStock:     &inStock,
		Lat:         &lat,
		Lng:         &lng,
		MaxDistance: &maxDist,
		Page:        2,
		PageSize:    15,
	}

	sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		stmt := tx.Model(&entities.Stock{}).Statement
		for _, sc := range f.FilterScopes() {
			sc(stmt)
		}
		stockFilter.OrderByNearest(f.Lat, f.Lng)(stmt)
		scope.Paginate(f.Page, f.PageSize)(stmt)
		return tx.Find(&[]entities.Stock{})
	})

	checks := []string{
		"stocks.address ILIKE",
		"providers.name ILIKE",
		"stock_types.slug",
		"stocks.stock_type_id = 2",
		"stocks.available > 0",
		"ST_DWithin(stocks.location, ST_SetSRID(ST_MakePoint(34.4668, 31.5017), 4326)::geography, 5000)",
		"ORDER BY ST_Distance(stocks.location, ST_SetSRID(ST_MakePoint(34.4668, 31.5017), 4326)::geography) ASC, stocks.created_at DESC",
		"LIMIT 15 OFFSET 15",
	}

	for _, want := range checks {
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q\ngot: %s", want, sql)
		}
	}
}

func TestStockFilterScopes_DefaultOrderWithoutLocation(t *testing.T) {
	db := newDryRunDB(t)

	sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		stmt := tx.Model(&entities.Stock{}).Statement
		stockFilter.OrderByNearest(nil, nil)(stmt)
		scope.Paginate(0, 0)(stmt)
		return tx.Find(&[]entities.Stock{})
	})

	if !strings.Contains(sql, "ORDER BY stocks.created_at DESC") {
		t.Errorf("expected fallback ORDER BY stocks.created_at DESC, got: %s", sql)
	}
	if !strings.Contains(sql, "LIMIT 10") {
		t.Errorf("expected default LIMIT 10 from scope.Paginate, got: %s", sql)
	}
}

func TestStockFilterScopes_WithGormG(t *testing.T) {
	db := newDryRunDB(t)

	lat := 31.5
	lng := 34.4
	f := stockFilter.StockFilter{
		Lat:      &lat,
		Lng:      &lng,
		Page:     1,
		PageSize: 10,
	}

	_, err := gorm.G[entities.Stock](db).
		Scopes(f.FilterScopes()...).
		Scopes(
			stockFilter.OrderByNearest(f.Lat, f.Lng),
			scope.Paginate(f.Page, f.PageSize),
		).
		Find(context.Background())
	if err != nil {
		t.Fatalf("gorm.G dry-run Find failed: %v", err)
	}
}
