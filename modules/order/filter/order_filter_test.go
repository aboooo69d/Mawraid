package filter_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/najm-x/nova/database/entities"
	"github.com/najm-x/nova/database/scope"
	orderFilter "github.com/najm-x/nova/modules/order/filter"
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

func TestOrderFilterScopes_SQLGeneration(t *testing.T) {
	db := newDryRunDB(t)

	status := entities.OrderStatusPending
	stockID := "11111111-1111-1111-1111-111111111111"
	consumerType := entities.OrderConsumerHome

	f := orderFilter.OrderFilter{
		Status:       &status,
		StockID:      &stockID,
		ConsumerType: &consumerType,
		Page:         2,
		PageSize:     15,
	}

	sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		stmt := tx.Model(&entities.Order{}).Statement
		for _, sc := range f.FilterScopes() {
			sc(stmt)
		}
		orderFilter.OrderByNewest()(stmt)
		scope.Paginate(f.Page, f.PageSize)(stmt)
		return tx.Find(&[]entities.Order{})
	})

	checks := []string{
		"orders.status = 'pending'",
		"orders.stock_id = '11111111-1111-1111-1111-111111111111'",
		"orders.consumer_type = 'home'",
		"ORDER BY orders.created_at DESC",
		"LIMIT 15 OFFSET 15",
	}

	for _, want := range checks {
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q\ngot: %s", want, sql)
		}
	}
}

func TestOrderFilterScopes_DefaultPagination(t *testing.T) {
	db := newDryRunDB(t)

	sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		stmt := tx.Model(&entities.Order{}).Statement
		orderFilter.OrderByNewest()(stmt)
		scope.Paginate(0, 0)(stmt)
		return tx.Find(&[]entities.Order{})
	})

	if !strings.Contains(sql, "ORDER BY orders.created_at DESC") {
		t.Errorf("expected ORDER BY orders.created_at DESC, got: %s", sql)
	}
	if !strings.Contains(sql, "LIMIT 10") {
		t.Errorf("expected default LIMIT 10 from scope.Paginate, got: %s", sql)
	}
}

func TestOrderFilterScopes_WithGormG(t *testing.T) {
	db := newDryRunDB(t)

	status := entities.OrderStatusAccepted
	f := orderFilter.OrderFilter{
		Status:   &status,
		Page:     1,
		PageSize: 10,
	}

	_, err := gorm.G[entities.Order](db).
		Scopes(f.FilterScopes()...).
		Scopes(
			orderFilter.OrderByNewest(),
			scope.Paginate(f.Page, f.PageSize),
		).
		Preload("Address", nil).
		Find(context.Background())
	if err != nil {
		t.Fatalf("gorm.G dry-run Find failed: %v", err)
	}
}
