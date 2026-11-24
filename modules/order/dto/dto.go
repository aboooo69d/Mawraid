package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	errsCode "github.com/najm-x/nova/pkg/constants/errs_code"
	"github.com/najm-x/nova/pkg/respons"
)

var (
	ErrOrderNotFound             = &respons.AppError{Status: 404, Code: errsCode.ErrOrderNotFound, Message: "order not found"}
	ErrInvalidOrderStatus        = &respons.AppError{Status: 400, Code: errsCode.ErrInvalidOrderStatus, Message: "invalid order status"}
	ErrInvalidConsumerType       = &respons.AppError{Status: 400, Code: errsCode.ErrInvalidConsumerType, Message: "invalid consumer type"}
	ErrConsumerTypeNotAllowed    = &respons.AppError{Status: 403, Code: errsCode.ErrConsumerTypeNotAllowed, Message: "consumer type is not enabled for your account"}
	ErrConsumerRequired          = &respons.AppError{Status: 403, Code: errsCode.ErrConsumerRequired, Message: "consumer account is required"}
	ErrOrderTransitionNotAllowed = &respons.AppError{Status: 409, Code: errsCode.ErrOrderTransitionNotAllowed, Message: "order status transition is not allowed"}
)

type CreateOrderRequest struct {
	StockID      string `json:"stock_id" binding:"required"`
	UnitID       uint   `json:"unit_id" binding:"required,gt=0"`
	Quantity     int    `json:"quantity" binding:"required,gt=0"`
	AddressID    string `json:"address_id" binding:"required"`
	ConsumerType string `json:"consumer_type" binding:"required"`
	Notes        string `json:"notes" binding:"omitempty,max=500"`
}

type OrderActionRequest struct {
	Reason string `json:"reason" binding:"omitempty,max=500"`
}

type OrderQueryFilter struct {
	Status       *string `form:"status" binding:"omitempty"`
	StockID      *string `form:"stock_id" binding:"omitempty"`
	ConsumerType *string `form:"consumer_type" binding:"omitempty"`
	Page         int     `form:"page" binding:"omitempty,min=1"`
	PageSize     int     `form:"page_size" binding:"omitempty,min=1,max=100"`
}

type OrderAddressResponse struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Lat         float64   `json:"lat"`
	Lng         float64   `json:"lng"`
}

type OrderResponse struct {
	ID            uuid.UUID                  `json:"id"`
	OrderNumber   string                     `json:"order_number"`
	ConsumerID    uuid.UUID                  `json:"consumer_id"`
	ProviderID    uuid.UUID                  `json:"provider_id"`
	StockID       uuid.UUID                  `json:"stock_id"`
	AddressID     uuid.UUID                  `json:"address_id"`
	Address       *OrderAddressResponse      `json:"address,omitempty"`
	ConsumerType  entities.OrderConsumerType `json:"consumer_type"`
	UnitID        uint                       `json:"unit_id"`
	UnitName      string                     `json:"unit_name"`
	UnitRate      int                        `json:"unit_rate"`
	UnitPrice     float64                    `json:"unit_price"`
	Quantity      int                        `json:"quantity"`
	TotalPrice    float64                    `json:"total_price"`
	Status        entities.OrderStatus       `json:"status"`
	PaymentMethod entities.PaymentMethod     `json:"payment_method"`
	PaymentStatus entities.PaymentStatus     `json:"payment_status"`
	Notes         string                     `json:"notes,omitempty"`
	Reason        string                     `json:"reason,omitempty"`
	CancelledBy   *uuid.UUID                 `json:"cancelled_by,omitempty"`
	TransactionID *uuid.UUID                 `json:"transaction_id,omitempty"`
	OrderDetails  map[string]any             `json:"order_details,omitempty"`
	CreatedAt     time.Time                  `json:"created_at"`
	UpdatedAt     time.Time                  `json:"updated_at"`
}

func ToOrderResponse(o *entities.Order) OrderResponse {
	var details map[string]any
	if len(o.OrderDetails) > 0 {
		_ = json.Unmarshal(o.OrderDetails, &details)
	}

	var addr *OrderAddressResponse
	if o.Address.ID != uuid.Nil {
		addr = &OrderAddressResponse{
			ID:          o.Address.ID,
			Title:       o.Address.Title,
			Description: o.Address.Description,
			Lat:         o.Address.Location.Lat,
			Lng:         o.Address.Location.Lng,
		}
	}

	return OrderResponse{
		ID:            o.ID,
		OrderNumber:   o.OrderNumber,
		ConsumerID:    o.ConsumerID,
		ProviderID:    o.ProviderID,
		StockID:       o.StockID,
		AddressID:     o.AddressID,
		Address:       addr,
		ConsumerType:  o.ConsumerType,
		UnitID:        o.UnitID,
		UnitName:      o.UnitName,
		UnitRate:      o.UnitRate,
		UnitPrice:     o.UnitPrice,
		Quantity:      o.Quantity,
		TotalPrice:    o.TotalPrice,
		Status:        o.Status,
		PaymentMethod: o.PaymentMethod,
		PaymentStatus: o.PaymentStatus,
		Notes:         o.Notes,
		Reason:        o.Reason,
		CancelledBy:   o.CancelledBy,
		TransactionID: o.TransactionID,
		OrderDetails:  details,
		CreatedAt:     o.CreatedAt,
		UpdatedAt:     o.UpdatedAt,
	}
}
