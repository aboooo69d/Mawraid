package orderService

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	addressDto "github.com/najm-x/nova/modules/address/dto"
	addressRepo "github.com/najm-x/nova/modules/address/repo"
	orderDto "github.com/najm-x/nova/modules/order/dto"
	orderFilter "github.com/najm-x/nova/modules/order/filter"
	orderRepo "github.com/najm-x/nova/modules/order/repo"
	stockDto "github.com/najm-x/nova/modules/stock/dto"
	stockRepo "github.com/najm-x/nova/modules/stock/repo"
	"github.com/najm-x/nova/pkg/respons"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	defaultPageSize = 10
	maxPageSize     = 100
)

type OrderService interface {
	// Consumer
	CreateOrder(c context.Context, consumerId string, req orderDto.CreateOrderRequest) (*orderDto.OrderResponse, error)
	ListConsumerOrders(c context.Context, consumerId string, q orderDto.OrderQueryFilter) ([]orderDto.OrderResponse, *respons.Meta, error)
	GetConsumerOrder(c context.Context, consumerId string, orderId string) (*orderDto.OrderResponse, error)
	CancelOrder(c context.Context, consumerId string, orderId string, req orderDto.OrderActionRequest) (*orderDto.OrderResponse, error)

	// Provider
	ListProviderOrders(c context.Context, providerId string, q orderDto.OrderQueryFilter) ([]orderDto.OrderResponse, *respons.Meta, error)
	GetProviderOrder(c context.Context, providerId string, orderId string) (*orderDto.OrderResponse, error)
	AcceptOrder(c context.Context, providerId string, orderId string) (*orderDto.OrderResponse, error)
	RejectOrder(c context.Context, providerId string, orderId string, req orderDto.OrderActionRequest) (*orderDto.OrderResponse, error)
	CompleteOrder(c context.Context, providerId string, orderId string) (*orderDto.OrderResponse, error)
}

type orderService struct {
	orderRepo   orderRepo.OrderRepo
	stockRepo   stockRepo.StockRepo
	addressRepo addressRepo.AddressRepo
}

func NewOrderService(
	orderRepo orderRepo.OrderRepo,
	stockRepo stockRepo.StockRepo,
	addressRepo addressRepo.AddressRepo,
) OrderService {
	return &orderService{
		orderRepo:   orderRepo,
		stockRepo:   stockRepo,
		addressRepo: addressRepo,
	}
}

// CreateOrder validates consumer eligibility, address ownership, stock & unit,
// and atomically reserves stock while creating the pending order.
func (s *orderService) CreateOrder(c context.Context, consumerId string, req orderDto.CreateOrderRequest) (*orderDto.OrderResponse, error) {
	consumerUUID, err := uuid.Parse(consumerId)
	if err != nil {
		return nil, respons.ErrBadRequest
	}
	if _, err := uuid.Parse(req.StockID); err != nil {
		return nil, stockDto.ErrStockNotFound
	}
	if _, err := uuid.Parse(req.AddressID); err != nil {
		return nil, addressDto.ErrAddressNotFound
	}
	if req.Quantity <= 0 {
		return nil, stockDto.ErrInvalidAmount
	}

	consumerType, err := parseConsumerType(req.ConsumerType)
	if err != nil {
		return nil, err
	}

	consumer, err := s.orderRepo.GetConsumerByUserId(c, consumerId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, orderDto.ErrConsumerRequired
		}
		return nil, err
	}
	if !isConsumerTypeAllowed(consumer, consumerType) {
		return nil, orderDto.ErrConsumerTypeNotAllowed
	}

	addr, err := s.addressRepo.GetByIdAndUser(c, req.AddressID, consumerId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, addressDto.ErrAddressNotFound
		}
		return nil, err
	}

	stock, err := s.stockRepo.GetStockById(c, req.StockID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, stockDto.ErrStockNotFound
		}
		return nil, err
	}

	unit := findStockUnit(stock.Units, req.UnitID)
	if unit == nil {
		return nil, stockDto.ErrUnitNotInStock
	}

	mainUnitQty := float64(req.Quantity) * float64(unit.Rate)
	totalPrice := round2(float64(req.Quantity) * unit.Price)
	orderNumber := generateOrderNumber()

	var orderDetails datatypes.JSON
	if len(stock.StockDetails) > 0 {
		orderDetails = append(datatypes.JSON(nil), stock.StockDetails...)
	}

	order := &entities.Order{
		ID:            uuid.New(),
		OrderNumber:   orderNumber,
		ConsumerID:    consumerUUID,
		ProviderID:    stock.UserID,
		StockID:       stock.ID,
		AddressID:     addr.ID,
		ConsumerType:  consumerType,
		UnitID:        unit.ID,
		UnitName:      unit.Name,
		UnitRate:      unit.Rate,
		UnitPrice:     unit.Price,
		Quantity:      req.Quantity,
		TotalPrice:    totalPrice,
		Status:        entities.OrderStatusPending,
		PaymentMethod: entities.PaymentMethodCash,
		PaymentStatus: entities.PaymentStatusUnpaid,
		Notes:         strings.TrimSpace(req.Notes),
		OrderDetails:  orderDetails,
	}

	withdrawTx := &entities.Transaction{
		ID:          uuid.New(),
		Amount:      req.Quantity,
		UnitID:      unit.ID,
		StockID:     stock.ID,
		IsIn:        false,
		Description: fmt.Sprintf("Order reservation %s", orderNumber),
		UserID:      consumerUUID,
	}

	if err := s.orderRepo.CreateOrder(c, order, withdrawTx, mainUnitQty); err != nil {
		return nil, mapOrderErr(err)
	}

	order.Address = *addr
	res := orderDto.ToOrderResponse(order)
	return &res, nil
}

func (s *orderService) ListConsumerOrders(c context.Context, consumerId string, q orderDto.OrderQueryFilter) ([]orderDto.OrderResponse, *respons.Meta, error) {
	if _, err := uuid.Parse(consumerId); err != nil {
		return nil, nil, respons.ErrBadRequest
	}
	filter, err := buildOrderFilter(q)
	if err != nil {
		return nil, nil, err
	}

	orders, total, err := s.orderRepo.ListOrdersByConsumer(c, consumerId, filter)
	if err != nil {
		return nil, nil, err
	}
	return toOrderResponses(orders), buildMeta(filter.Page, filter.PageSize, total), nil
}

func (s *orderService) GetConsumerOrder(c context.Context, consumerId string, orderId string) (*orderDto.OrderResponse, error) {
	if _, err := uuid.Parse(orderId); err != nil {
		return nil, orderDto.ErrOrderNotFound
	}
	if _, err := uuid.Parse(consumerId); err != nil {
		return nil, orderDto.ErrOrderNotFound
	}

	order, err := s.orderRepo.GetOrderByIdForConsumer(c, orderId, consumerId)
	if err != nil {
		return nil, mapOrderErr(err)
	}
	res := orderDto.ToOrderResponse(order)
	return &res, nil
}

func (s *orderService) CancelOrder(c context.Context, consumerId string, orderId string, req orderDto.OrderActionRequest) (*orderDto.OrderResponse, error) {
	if _, err := uuid.Parse(orderId); err != nil {
		return nil, orderDto.ErrOrderNotFound
	}
	if _, err := uuid.Parse(consumerId); err != nil {
		return nil, orderDto.ErrOrderNotFound
	}

	refundTx := &entities.Transaction{
		ID:          uuid.New(),
		IsIn:        true,
		Description: fmt.Sprintf("Order cancelled %s", orderId),
	}
	order, err := s.orderRepo.CancelOrder(c, orderId, consumerId, strings.TrimSpace(req.Reason), refundTx)
	if err != nil {
		return nil, mapOrderErr(err)
	}
	res := orderDto.ToOrderResponse(order)
	return &res, nil
}

func (s *orderService) ListProviderOrders(c context.Context, providerId string, q orderDto.OrderQueryFilter) ([]orderDto.OrderResponse, *respons.Meta, error) {
	if _, err := uuid.Parse(providerId); err != nil {
		return nil, nil, respons.ErrBadRequest
	}
	filter, err := buildOrderFilter(q)
	if err != nil {
		return nil, nil, err
	}

	orders, total, err := s.orderRepo.ListOrdersByProvider(c, providerId, filter)
	if err != nil {
		return nil, nil, err
	}
	return toOrderResponses(orders), buildMeta(filter.Page, filter.PageSize, total), nil
}

func (s *orderService) GetProviderOrder(c context.Context, providerId string, orderId string) (*orderDto.OrderResponse, error) {
	if _, err := uuid.Parse(orderId); err != nil {
		return nil, orderDto.ErrOrderNotFound
	}
	if _, err := uuid.Parse(providerId); err != nil {
		return nil, orderDto.ErrOrderNotFound
	}

	order, err := s.orderRepo.GetOrderByIdForProvider(c, orderId, providerId)
	if err != nil {
		return nil, mapOrderErr(err)
	}
	res := orderDto.ToOrderResponse(order)
	return &res, nil
}

func (s *orderService) AcceptOrder(c context.Context, providerId string, orderId string) (*orderDto.OrderResponse, error) {
	if _, err := uuid.Parse(orderId); err != nil {
		return nil, orderDto.ErrOrderNotFound
	}
	if _, err := uuid.Parse(providerId); err != nil {
		return nil, orderDto.ErrOrderNotFound
	}

	order, err := s.orderRepo.AcceptOrder(c, orderId, providerId)
	if err != nil {
		return nil, mapOrderErr(err)
	}
	res := orderDto.ToOrderResponse(order)
	return &res, nil
}

func (s *orderService) RejectOrder(c context.Context, providerId string, orderId string, req orderDto.OrderActionRequest) (*orderDto.OrderResponse, error) {
	if _, err := uuid.Parse(orderId); err != nil {
		return nil, orderDto.ErrOrderNotFound
	}
	if _, err := uuid.Parse(providerId); err != nil {
		return nil, orderDto.ErrOrderNotFound
	}

	refundTx := &entities.Transaction{
		ID:          uuid.New(),
		IsIn:        true,
		Description: fmt.Sprintf("Order rejected %s", orderId),
	}
	order, err := s.orderRepo.RejectOrder(c, orderId, providerId, strings.TrimSpace(req.Reason), refundTx)
	if err != nil {
		return nil, mapOrderErr(err)
	}
	res := orderDto.ToOrderResponse(order)
	return &res, nil
}

func (s *orderService) CompleteOrder(c context.Context, providerId string, orderId string) (*orderDto.OrderResponse, error) {
	if _, err := uuid.Parse(orderId); err != nil {
		return nil, orderDto.ErrOrderNotFound
	}
	if _, err := uuid.Parse(providerId); err != nil {
		return nil, orderDto.ErrOrderNotFound
	}

	order, err := s.orderRepo.CompleteOrder(c, orderId, providerId)
	if err != nil {
		return nil, mapOrderErr(err)
	}
	res := orderDto.ToOrderResponse(order)
	return &res, nil
}

// ==================== Helpers ====================

func parseConsumerType(raw string) (entities.OrderConsumerType, error) {
	ct := entities.OrderConsumerType(strings.ToLower(strings.TrimSpace(raw)))
	switch ct {
	case entities.OrderConsumerHome, entities.OrderConsumerBusiness, entities.OrderConsumerDriver:
		return ct, nil
	default:
		return "", orderDto.ErrInvalidConsumerType
	}
}

func isConsumerTypeAllowed(c *entities.Consumer, ct entities.OrderConsumerType) bool {
	switch ct {
	case entities.OrderConsumerHome:
		return c.IsHome
	case entities.OrderConsumerBusiness:
		return c.IsBusiness
	case entities.OrderConsumerDriver:
		return c.IsDriver
	default:
		return false
	}
}

func parseOrderStatus(raw string) (entities.OrderStatus, error) {
	st := entities.OrderStatus(strings.ToLower(strings.TrimSpace(raw)))
	switch st {
	case entities.OrderStatusPending,
		entities.OrderStatusAccepted,
		entities.OrderStatusCompleted,
		entities.OrderStatusRejected,
		entities.OrderStatusCancelled:
		return st, nil
	default:
		return "", orderDto.ErrInvalidOrderStatus
	}
}

func buildOrderFilter(q orderDto.OrderQueryFilter) (orderFilter.OrderFilter, error) {
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	f := orderFilter.OrderFilter{
		Page:     page,
		PageSize: pageSize,
	}
	if q.Status != nil && strings.TrimSpace(*q.Status) != "" {
		st, err := parseOrderStatus(*q.Status)
		if err != nil {
			return orderFilter.OrderFilter{}, err
		}
		f.Status = &st
	}
	if q.StockID != nil && strings.TrimSpace(*q.StockID) != "" {
		sid := strings.TrimSpace(*q.StockID)
		if _, err := uuid.Parse(sid); err != nil {
			return orderFilter.OrderFilter{}, respons.ErrBadRequest
		}
		f.StockID = &sid
	}
	if q.ConsumerType != nil && strings.TrimSpace(*q.ConsumerType) != "" {
		ct, err := parseConsumerType(*q.ConsumerType)
		if err != nil {
			return orderFilter.OrderFilter{}, err
		}
		f.ConsumerType = &ct
	}
	return f, nil
}

func findStockUnit(units []entities.Unit, id uint) *entities.Unit {
	for i := range units {
		if units[i].ID == id {
			return &units[i]
		}
	}
	return nil
}

func toOrderResponses(orders []entities.Order) []orderDto.OrderResponse {
	res := make([]orderDto.OrderResponse, 0, len(orders))
	for i := range orders {
		res = append(res, orderDto.ToOrderResponse(&orders[i]))
	}
	return res
}

func mapOrderErr(err error) error {
	switch {
	case errors.Is(err, orderRepo.ErrInsufficientStock):
		return stockDto.ErrInsufficientStock
	case errors.Is(err, orderRepo.ErrInvalidTransition):
		return orderDto.ErrOrderTransitionNotAllowed
	case errors.Is(err, gorm.ErrRecordNotFound):
		return orderDto.ErrOrderNotFound
	}
	return err
}

func generateOrderNumber() string {
	short := strings.ToUpper(strings.ReplaceAll(uuid.New().String(), "-", "")[:8])
	return fmt.Sprintf("ORD-%s-%s", time.Now().UTC().Format("20060102"), short)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func buildMeta(page int, pageSize int, total int64) *respons.Meta {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return &respons.Meta{
		Page:       page,
		PerPage:    pageSize,
		Total:      int(total),
		TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize)),
	}
}
