package orderService

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	orderDto "github.com/najm-x/nova/modules/order/dto"
	orderFilter "github.com/najm-x/nova/modules/order/filter"
	orderRepo "github.com/najm-x/nova/modules/order/repo"
	stockDto "github.com/najm-x/nova/modules/stock/dto"
	stockFilter "github.com/najm-x/nova/modules/stock/filter"
	"github.com/najm-x/nova/pkg/geo"
)

type mockOrderRepo struct {
	consumer       *entities.Consumer
	consumerErr    error
	createErr      error
	createdOrder   *entities.Order
	createdTx      *entities.Transaction
	createdQty     float64
	acceptErr      error
	rejectErr      error
	cancelErr      error
	completeErr    error
	returnedOrder  *entities.Order
	capturedReason string
}

func (m *mockOrderRepo) GetConsumerByUserId(_ context.Context, _ string) (*entities.Consumer, error) {
	return m.consumer, m.consumerErr
}

func (m *mockOrderRepo) CreateOrder(_ context.Context, order *entities.Order, withdrawTx *entities.Transaction, mainUnitQty float64) error {
	m.createdOrder = order
	m.createdTx = withdrawTx
	m.createdQty = mainUnitQty
	return m.createErr
}

func (m *mockOrderRepo) AcceptOrder(_ context.Context, _ string, _ string) (*entities.Order, error) {
	return m.returnedOrder, m.acceptErr
}

func (m *mockOrderRepo) RejectOrder(_ context.Context, _ string, _ string, reason string, _ *entities.Transaction) (*entities.Order, error) {
	m.capturedReason = reason
	return m.returnedOrder, m.rejectErr
}

func (m *mockOrderRepo) CancelOrder(_ context.Context, _ string, _ string, reason string, _ *entities.Transaction) (*entities.Order, error) {
	m.capturedReason = reason
	return m.returnedOrder, m.cancelErr
}

func (m *mockOrderRepo) CompleteOrder(_ context.Context, _ string, _ string) (*entities.Order, error) {
	return m.returnedOrder, m.completeErr
}

func (m *mockOrderRepo) GetOrderByIdForConsumer(_ context.Context, _ string, _ string) (*entities.Order, error) {
	return m.returnedOrder, nil
}

func (m *mockOrderRepo) GetOrderByIdForProvider(_ context.Context, _ string, _ string) (*entities.Order, error) {
	return m.returnedOrder, nil
}

func (m *mockOrderRepo) ListOrdersByConsumer(_ context.Context, _ string, _ orderFilter.OrderFilter) ([]entities.Order, int64, error) {
	return nil, 0, nil
}

func (m *mockOrderRepo) ListOrdersByProvider(_ context.Context, _ string, _ orderFilter.OrderFilter) ([]entities.Order, int64, error) {
	return nil, 0, nil
}

type mockStockRepo struct {
	stock *entities.Stock
	err   error
}

func (m *mockStockRepo) CreateStock(_ context.Context, _ *entities.Stock) error { return nil }
func (m *mockStockRepo) GetStocks(_ context.Context, _ stockFilter.StockFilter) ([]entities.Stock, int64, error) {
	return nil, 0, nil
}
func (m *mockStockRepo) GetStockById(_ context.Context, _ string) (*entities.Stock, error) {
	return m.stock, m.err
}
func (m *mockStockRepo) GetStocksByUserId(_ context.Context, _ string) ([]entities.Stock, error) {
	return nil, nil
}
func (m *mockStockRepo) GetStockTypeById(_ context.Context, _ uint) (*entities.StockType, error) {
	return nil, nil
}
func (m *mockStockRepo) GetStockTypes(_ context.Context) ([]entities.StockType, error) {
	return nil, nil
}
func (m *mockStockRepo) GetStockTypesByProviderTypeId(_ context.Context, _ uint) ([]entities.StockType, error) {
	return nil, nil
}
func (m *mockStockRepo) GetProviderTypeIdByUserId(_ context.Context, _ string) (uint, error) {
	return 0, nil
}
func (m *mockStockRepo) UpdateStock(_ context.Context, _ string, _ *entities.Stock) error { return nil }
func (m *mockStockRepo) DeleteStock(_ context.Context, _ string) error                   { return nil }

type mockAddressRepo struct {
	addr *entities.Address
	err  error
}

func (m *mockAddressRepo) Create(_ context.Context, _ *entities.Address) error { return nil }
func (m *mockAddressRepo) GetByIdAndUser(_ context.Context, _ string, _ string) (*entities.Address, error) {
	return m.addr, m.err
}
func (m *mockAddressRepo) GetById(_ context.Context, _ string) (*entities.Address, error) {
	return m.addr, m.err
}
func (m *mockAddressRepo) GetDefaultByUser(_ context.Context, _ string) (*entities.Address, error) {
	return m.addr, m.err
}
func (m *mockAddressRepo) ListByUser(_ context.Context, _ string) ([]entities.Address, error) {
	return nil, nil
}

func testFixtures() (uuid.UUID, uuid.UUID, uuid.UUID, *entities.Consumer, *entities.Address, *entities.Stock) {
	consumerID := uuid.New()
	providerID := uuid.New()
	stockID := uuid.New()
	addressID := uuid.New()

	consumer := &entities.Consumer{
		UserID:     consumerID,
		IsHome:     true,
		IsBusiness: false,
		IsDriver:   false,
	}
	addr := &entities.Address{
		ID:          addressID,
		UserID:      consumerID,
		Title:       "Home",
		Description: "Al-Zubairi St",
		Location:    geo.Point{Lat: 15.36, Lng: 44.19},
	}
	stock := &entities.Stock{
		ID:        stockID,
		UserID:    providerID,
		Available: 10,
		Units: []entities.Unit{
			{Name: "Pack of 5", Rate: 5, Price: 36000, IsMain: false},
		},
	}
	stock.Units[0].ID = 2

	return consumerID, stockID, addressID, consumer, addr, stock
}

func TestCreateOrder_Success(t *testing.T) {
	consumerID, stockID, addressID, consumer, addr, stock := testFixtures()
	oRepo := &mockOrderRepo{consumer: consumer}
	svc := NewOrderService(oRepo, &mockStockRepo{stock: stock}, &mockAddressRepo{addr: addr})

	res, err := svc.CreateOrder(context.Background(), consumerID.String(), orderDto.CreateOrderRequest{
		StockID:      stockID.String(),
		UnitID:       2,
		Quantity:     2,
		AddressID:    addressID.String(),
		ConsumerType: "home",
		Notes:        " Call on arrival ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if oRepo.createdQty != 10 {
		t.Fatalf("expected mainUnitQty=10, got %v", oRepo.createdQty)
	}
	if oRepo.createdTx == nil || oRepo.createdTx.IsIn {
		t.Fatalf("expected withdrawal transaction (is_in=false)")
	}
	if res.TotalPrice != 72000 {
		t.Fatalf("expected total_price=72000, got %v", res.TotalPrice)
	}
	if res.Status != entities.OrderStatusPending {
		t.Fatalf("expected pending status, got %s", res.Status)
	}
	if res.PaymentMethod != entities.PaymentMethodCash || res.PaymentStatus != entities.PaymentStatusUnpaid {
		t.Fatalf("unexpected payment defaults: %s / %s", res.PaymentMethod, res.PaymentStatus)
	}
	if res.Address == nil || res.Address.ID != addressID {
		t.Fatalf("expected address in response")
	}
}

func TestCreateOrder_ValidationAndErrors(t *testing.T) {
	consumerID, stockID, addressID, consumer, addr, stock := testFixtures()

	tests := []struct {
		name         string
		consumerType string
		unitID       uint
		repoErr      error
		wantErr      error
	}{
		{
			name:         "invalid consumer type",
			consumerType: "vip",
			unitID:       2,
			wantErr:      orderDto.ErrInvalidConsumerType,
		},
		{
			name:         "consumer type not enabled",
			consumerType: "driver",
			unitID:       2,
			wantErr:      orderDto.ErrConsumerTypeNotAllowed,
		},
		{
			name:         "unit not in stock",
			consumerType: "home",
			unitID:       99,
			wantErr:      stockDto.ErrUnitNotInStock,
		},
		{
			name:         "insufficient stock",
			consumerType: "home",
			unitID:       2,
			repoErr:      orderRepo.ErrInsufficientStock,
			wantErr:      stockDto.ErrInsufficientStock,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oRepo := &mockOrderRepo{consumer: consumer, createErr: tc.repoErr}
			svc := NewOrderService(oRepo, &mockStockRepo{stock: stock}, &mockAddressRepo{addr: addr})

			_, err := svc.CreateOrder(context.Background(), consumerID.String(), orderDto.CreateOrderRequest{
				StockID:      stockID.String(),
				UnitID:       tc.unitID,
				Quantity:     1,
				AddressID:    addressID.String(),
				ConsumerType: tc.consumerType,
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected error %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestOrderTransitions_ErrorMapping(t *testing.T) {
	orderID := uuid.New().String()
	actorID := uuid.New().String()

	oRepo := &mockOrderRepo{
		acceptErr:   orderRepo.ErrInvalidTransition,
		rejectErr:   orderRepo.ErrInvalidTransition,
		cancelErr:   orderRepo.ErrInvalidTransition,
		completeErr: orderRepo.ErrInvalidTransition,
	}
	svc := NewOrderService(oRepo, &mockStockRepo{}, &mockAddressRepo{})

	if _, err := svc.AcceptOrder(context.Background(), actorID, orderID); !errors.Is(err, orderDto.ErrOrderTransitionNotAllowed) {
		t.Fatalf("AcceptOrder: expected ErrOrderTransitionNotAllowed, got %v", err)
	}
	if _, err := svc.RejectOrder(context.Background(), actorID, orderID, orderDto.OrderActionRequest{Reason: "out"}); !errors.Is(err, orderDto.ErrOrderTransitionNotAllowed) {
		t.Fatalf("RejectOrder: expected ErrOrderTransitionNotAllowed, got %v", err)
	}
	if _, err := svc.CancelOrder(context.Background(), actorID, orderID, orderDto.OrderActionRequest{Reason: "changed mind"}); !errors.Is(err, orderDto.ErrOrderTransitionNotAllowed) {
		t.Fatalf("CancelOrder: expected ErrOrderTransitionNotAllowed, got %v", err)
	}
	if _, err := svc.CompleteOrder(context.Background(), actorID, orderID); !errors.Is(err, orderDto.ErrOrderTransitionNotAllowed) {
		t.Fatalf("CompleteOrder: expected ErrOrderTransitionNotAllowed, got %v", err)
	}
}
