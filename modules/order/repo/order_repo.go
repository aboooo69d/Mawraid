package orderRepo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	"github.com/najm-x/nova/database/scope"
	orderFilter "github.com/najm-x/nova/modules/order/filter"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrInvalidTransition = errors.New("invalid order status transition")
)

type OrderRepo interface {
	GetConsumerByUserId(c context.Context, userId string) (*entities.Consumer, error)
	CreateOrder(c context.Context, order *entities.Order, withdrawTx *entities.Transaction, mainUnitQty float64) error
	AcceptOrder(c context.Context, orderId string, providerId string) (*entities.Order, error)
	RejectOrder(c context.Context, orderId string, providerId string, reason string, refundTx *entities.Transaction) (*entities.Order, error)
	CancelOrder(c context.Context, orderId string, consumerId string, reason string, refundTx *entities.Transaction) (*entities.Order, error)
	CompleteOrder(c context.Context, orderId string, providerId string) (*entities.Order, error)
	GetOrderByIdForConsumer(c context.Context, orderId string, consumerId string) (*entities.Order, error)
	GetOrderByIdForProvider(c context.Context, orderId string, providerId string) (*entities.Order, error)
	ListOrdersByConsumer(c context.Context, consumerId string, f orderFilter.OrderFilter) ([]entities.Order, int64, error)
	ListOrdersByProvider(c context.Context, providerId string, f orderFilter.OrderFilter) ([]entities.Order, int64, error)
}

type orderRepo struct {
	db *gorm.DB
}

func NewOrderRepo(db *gorm.DB) OrderRepo {
	return &orderRepo{db: db}
}

func (r *orderRepo) GetConsumerByUserId(c context.Context, userId string) (*entities.Consumer, error) {
	consumer, err := gorm.G[entities.Consumer](r.db).Where("user_id = ?", userId).First(c)
	if err != nil {
		return nil, err
	}
	return &consumer, nil
}

// CreateOrder atomically deducts mainUnitQty from Stock.Available, increments PendingOrders,
// inserts the withdrawal transaction, links TransactionID, and creates the order.
func (r *orderRepo) CreateOrder(c context.Context, order *entities.Order, withdrawTx *entities.Transaction, mainUnitQty float64) error {
	return r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&entities.Stock{}).
			Where("id = ? AND available >= ?", order.StockID, mainUnitQty).
			Updates(map[string]any{
				"available":      gorm.Expr("available - ?", mainUnitQty),
				"pending_orders": gorm.Expr("pending_orders + 1"),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			var count int64
			if err := tx.Model(&entities.Stock{}).Where("id = ?", order.StockID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return gorm.ErrRecordNotFound
			}
			return ErrInsufficientStock
		}

		if err := tx.Create(withdrawTx).Error; err != nil {
			return err
		}
		order.TransactionID = &withdrawTx.ID
		return tx.Create(order).Error
	})
}

// AcceptOrder transitions a pending order to accepted and decrements Stock.PendingOrders.
func (r *orderRepo) AcceptOrder(c context.Context, orderId string, providerId string) (*entities.Order, error) {
	var result *entities.Order
	err := r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		order, err := lockOrderForActor(tx, orderId, "provider_id", providerId)
		if err != nil {
			return err
		}
		if order.Status != entities.OrderStatusPending {
			return ErrInvalidTransition
		}

		if err := tx.Model(&entities.Order{}).
			Where("id = ?", order.ID).
			Update("status", entities.OrderStatusAccepted).Error; err != nil {
			return err
		}

		if err := tx.Model(&entities.Stock{}).
			Where("id = ?", order.StockID).
			Update("pending_orders", gorm.Expr("GREATEST(pending_orders - 1, 0)")).Error; err != nil {
			return err
		}

		result, err = reloadOrderWithAddress(tx, order.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// RejectOrder transitions a pending order to rejected, refunds stock, decrements PendingOrders, and logs a refund transaction.
func (r *orderRepo) RejectOrder(c context.Context, orderId string, providerId string, reason string, refundTx *entities.Transaction) (*entities.Order, error) {
	return r.reversePendingOrder(c, orderId, "provider_id", providerId, entities.OrderStatusRejected, reason, nil, refundTx)
}

// CancelOrder transitions a pending order to cancelled, refunds stock, decrements PendingOrders, and logs a refund transaction.
func (r *orderRepo) CancelOrder(c context.Context, orderId string, consumerId string, reason string, refundTx *entities.Transaction) (*entities.Order, error) {
	uid, err := uuid.Parse(consumerId)
	if err != nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.reversePendingOrder(c, orderId, "consumer_id", consumerId, entities.OrderStatusCancelled, reason, &uid, refundTx)
}

func (r *orderRepo) reversePendingOrder(
	c context.Context,
	orderId string,
	actorCol string,
	actorId string,
	targetStatus entities.OrderStatus,
	reason string,
	cancelledBy *uuid.UUID,
	refundTx *entities.Transaction,
) (*entities.Order, error) {
	var result *entities.Order
	err := r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		order, err := lockOrderForActor(tx, orderId, actorCol, actorId)
		if err != nil {
			return err
		}
		if order.Status != entities.OrderStatusPending {
			return ErrInvalidTransition
		}

		updates := map[string]any{
			"status": targetStatus,
			"reason": reason,
		}
		if cancelledBy != nil {
			updates["cancelled_by"] = *cancelledBy
		}
		if err := tx.Model(&entities.Order{}).Where("id = ?", order.ID).Updates(updates).Error; err != nil {
			return err
		}

		mainUnitQty := float64(order.Quantity) * float64(order.UnitRate)
		if err := tx.Model(&entities.Stock{}).
			Where("id = ?", order.StockID).
			Updates(map[string]any{
				"available":      gorm.Expr("available + ?", mainUnitQty),
				"pending_orders": gorm.Expr("GREATEST(pending_orders - 1, 0)"),
			}).Error; err != nil {
			return err
		}

		refundTx.Amount = order.Quantity
		refundTx.UnitID = order.UnitID
		refundTx.StockID = order.StockID
		refundTx.IsIn = true
		if cancelledBy != nil {
			refundTx.UserID = order.ConsumerID
		} else {
			refundTx.UserID = order.ProviderID
		}
		if err := tx.Create(refundTx).Error; err != nil {
			return err
		}

		result, err = reloadOrderWithAddress(tx, order.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// CompleteOrder transitions an accepted order to completed, marks cash payments paid, and increments Stock.CompletedOrders.
func (r *orderRepo) CompleteOrder(c context.Context, orderId string, providerId string) (*entities.Order, error) {
	var result *entities.Order
	err := r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		order, err := lockOrderForActor(tx, orderId, "provider_id", providerId)
		if err != nil {
			return err
		}
		if order.Status != entities.OrderStatusAccepted {
			return ErrInvalidTransition
		}

		updates := map[string]any{
			"status": entities.OrderStatusCompleted,
		}
		if order.PaymentMethod == entities.PaymentMethodCash {
			updates["payment_status"] = entities.PaymentStatusPaid
		}
		if err := tx.Model(&entities.Order{}).Where("id = ?", order.ID).Updates(updates).Error; err != nil {
			return err
		}

		if err := tx.Model(&entities.Stock{}).
			Where("id = ?", order.StockID).
			Update("completed_orders", gorm.Expr("completed_orders + 1")).Error; err != nil {
			return err
		}

		result, err = reloadOrderWithAddress(tx, order.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *orderRepo) GetOrderByIdForConsumer(c context.Context, orderId string, consumerId string) (*entities.Order, error) {
	return r.getOrderByActor(c, orderId, "consumer_id", consumerId)
}

func (r *orderRepo) GetOrderByIdForProvider(c context.Context, orderId string, providerId string) (*entities.Order, error) {
	return r.getOrderByActor(c, orderId, "provider_id", providerId)
}

func (r *orderRepo) ListOrdersByConsumer(c context.Context, consumerId string, f orderFilter.OrderFilter) ([]entities.Order, int64, error) {
	return r.listOrders(c, "consumer_id", consumerId, f)
}

func (r *orderRepo) ListOrdersByProvider(c context.Context, providerId string, f orderFilter.OrderFilter) ([]entities.Order, int64, error) {
	return r.listOrders(c, "provider_id", providerId, f)
}

func (r *orderRepo) getOrderByActor(c context.Context, orderId string, actorCol string, actorId string) (*entities.Order, error) {
	var order entities.Order
	err := r.db.WithContext(c).
		Preload("Address", withDeleted).
		Where("id = ? AND "+actorCol+" = ?", orderId, actorId).
		First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *orderRepo) listOrders(c context.Context, actorCol string, actorId string, f orderFilter.OrderFilter) ([]entities.Order, int64, error) {
	q := gorm.G[entities.Order](r.db).
		Where(actorCol+" = ?", actorId).
		Scopes(f.FilterScopes()...)

	total, err := q.Count(c, "id")
	if err != nil {
		return nil, 0, err
	}

	orders, err := q.
		Scopes(
			orderFilter.OrderByNewest(),
			scope.Paginate(f.Page, f.PageSize),
		).
		Preload("Address", nil).
		Find(c)
	if err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func lockOrderForActor(tx *gorm.DB, orderId string, actorCol string, actorId string) (*entities.Order, error) {
	var order entities.Order
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND "+actorCol+" = ?", orderId, actorId).
		First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func reloadOrderWithAddress(tx *gorm.DB, orderId uuid.UUID) (*entities.Order, error) {
	var order entities.Order
	if err := tx.Preload("Address", withDeleted).Where("id = ?", orderId).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func withDeleted(db *gorm.DB) *gorm.DB {
	return db.Unscoped()
}
