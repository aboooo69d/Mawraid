package dto

import (
	"encoding/json"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	errsCode "github.com/najm-x/nova/pkg/constants/errs_code"
	"github.com/najm-x/nova/pkg/geo"
	"github.com/najm-x/nova/pkg/respons"
)

var (
	ErrStockNotFound       = &respons.AppError{Status: 404, Code: errsCode.ErrStockNotFound, Message: "stock not found"}
	ErrUnitNotInStock      = &respons.AppError{Status: 400, Code: errsCode.ErrUnitNotInStock, Message: "unit does not belong to this stock"}
	ErrInsufficientStock   = &respons.AppError{Status: 409, Code: errsCode.ErrInsufficientStock, Message: "insufficient stock"}
	ErrInvalidAmount       = &respons.AppError{Status: 400, Code: errsCode.ErrInvalidAmount, Message: "amount must be greater than 0"}
	ErrStockTypeNotFound   = &respons.AppError{Status: 404, Code: errsCode.ErrStockTypeNotFound, Message: "stock type not found"}
	ErrUnitNotFound        = &respons.AppError{Status: 404, Code: errsCode.ErrUnitNotFound, Message: "unit not found"}
	ErrMainUnitRequired    = &respons.AppError{Status: 400, Code: errsCode.ErrMainUnitRequired, Message: "stock must have exactly one main unit"}
	ErrMainUnitRate        = &respons.AppError{Status: 400, Code: errsCode.ErrMainUnitRate, Message: "main unit rate must be 1"}
	ErrMainUnitExists      = &respons.AppError{Status: 400, Code: errsCode.ErrMainUnitExists, Message: "stock already has a main unit"}
	ErrDuplicateUnitName   = &respons.AppError{Status: 409, Code: errsCode.ErrDuplicateUnitName, Message: "a unit with this name already exists in the stock"}
	ErrUnitHasTransactions = &respons.AppError{Status: 409, Code: errsCode.ErrUnitHasTransactions, Message: "cannot change rate of a unit that has transactions"}
	ErrStockAlreadyExists  = &respons.AppError{Status: 409, Code: errsCode.ErrStockAlreadyExists, Message: "user already has a stock"}
	ErrStockTypeMismatch   = &respons.AppError{Status: 400, Code: errsCode.ErrStockTypeMismatch, Message: "stock type does not match the selected stock type"}
	ErrInvalidStockDetails = &respons.AppError{Status: 400, Code: errsCode.ErrInvalidStockDetails, Message: "invalid stock details"}
	ErrStockTypeNotAllowed = &respons.AppError{Status: 403, Code: errsCode.ErrStockTypeNotAllowed, Message: "stock type is not allowed for your provider type"}
	ErrProviderRequired    = &respons.AppError{Status: 403, Code: errsCode.ErrProviderRequired, Message: "provider account is required"}
)

const (
	StockTypeSlugGaz = "gaz"

	GazTypeExchange string = "exchange"
	GazTypeRefill   string = "refill"
)

// ==================== Stock Type ====================

type CreateStockTypeRequest struct {
	Name           string `json:"name" binding:"required,max=50"`
	Slug           string `json:"slug" binding:"required,max=50"`
	ProviderTypeID uint   `json:"provider_type_id" binding:"required"`
}

type UpdateStockTypeRequest struct {
	Name           *string `json:"name" binding:"omitempty,min=1,max=50"`
	Slug           *string `json:"slug" binding:"omitempty,min=1,max=50"`
	ProviderTypeID *uint   `json:"provider_type_id" binding:"omitempty,gt=0"`
}

type StockTypeResponse struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	ProviderTypeID uint   `json:"provider_type_id"`
}

// ==================== Unit ====================

// CreateUnitRequest is used both inside CreateStockRequest (one unit must have is_main = true)
// and alone to add a unit to an existing stock (is_main must be false).
type CreateUnitRequest struct {
	Name   string `json:"name" binding:"required,max=50"`
	Rate   int    `json:"rate" binding:"required,gte=1"`
	IsMain bool   `json:"is_main"`
}

// UpdateUnitRequest: only the sent fields are changed. The main unit cannot be switched.
type UpdateUnitRequest struct {
	Name *string `json:"name" binding:"omitempty,min=1,max=50"`
	Rate *int    `json:"rate" binding:"omitempty,gte=1"`
}

type UnitResponse struct {
	ID     uint   `json:"id"`
	Name   string `json:"name"`
	Rate   int    `json:"rate"`
	IsMain bool   `json:"is_main"`
}

// UnitBalance is the stock's available quantity expressed in one unit.
type UnitBalance struct {
	UnitID uint    `json:"unit_id"`
	Name   string  `json:"unit"`
	Amount float64 `json:"amount"`
}

type Location struct {
	Lat *float64 `json:"lat" binding:"required,gte=-90,lte=90"`
	Lng *float64 `json:"lng" binding:"required,gte=-180,lte=180"`
}

type LocationResponse struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// ==================== Stock ====================

type CreateStockRequest struct {
	StockTypeID  uint                `json:"stock_type_id" binding:"required"`
	StockType    string              `json:"stock_type" binding:"required"`
	Address      string              `json:"address" binding:"required,min=10,max=200"`
	Location     Location            `json:"location" binding:"required"`
	Units        []CreateUnitRequest `json:"units" binding:"required,min=1,dive"`
	StockDetails map[string]any      `json:"stock_details"`
}

type GazStockDetails struct {
	GazStockType string `json:"gaz_stock_type"`
}

type UpdateStockRequest struct {
	StockTypeID  *uint          `json:"stock_type_id" binding:"omitempty"`
	Address      *string        `json:"address" binding:"omitempty,min=10,max=200"`
	Location     *Location      `json:"location" binding:"omitempty"`
	StockDetails map[string]any `json:"stock_details" binding:"omitempty"`
}

type StockResponse struct {
	ID           uuid.UUID         `json:"id"`
	Available    float64           `json:"available"` // in the main unit
	Address      string            `json:"address"`
	Location     LocationResponse  `json:"location"`
	Units        []UnitResponse    `json:"units"`
	Balances     []UnitBalance     `json:"balances"` // available in every unit
	StockType    StockTypeResponse `json:"type"`
	StockDetails map[string]any    `json:"stock_details"`
	UserID       uuid.UUID         `json:"user_id"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// ==================== Transaction ====================

// CreateTransactionRequest adds (is_in = true) or withdraws (is_in = false) an amount of a unit.
// The stock id comes from the URL and the user id from the auth context.
type CreateTransactionRequest struct {
	Amount      int    `json:"amount" binding:"required,gt=0"`
	UnitID      uint   `json:"unit_id" binding:"required"`
	IsIn        *bool  `json:"is_in" binding:"required"` // pointer so `false` passes "required"
	Description string `json:"description" binding:"omitempty,max=1000"`
}

// TransactionQuery is bound from the query string: ?page=1&page_size=20&is_in=true
type TransactionQuery struct {
	Page     int   `form:"page" binding:"omitempty,min=1"`
	PageSize int   `form:"page_size" binding:"omitempty,min=1,max=100"`
	IsIn     *bool `form:"is_in"`
}

type TransactionResponse struct {
	ID          uuid.UUID    `json:"id"`
	Amount      int          `json:"amount"`
	Unit        UnitResponse `json:"unit"`
	StockID     uuid.UUID    `json:"stock_id"`
	IsIn        bool         `json:"is_in"`
	Description string       `json:"description,omitempty"`
	UserID      uuid.UUID    `json:"user_id"`
	CreatedAt   time.Time    `json:"created_at"`
}

// ==================== Mappers ====================

func ToUnitResponse(u entities.Unit) UnitResponse {
	return UnitResponse{
		ID:     u.ID,
		Name:   u.Name,
		Rate:   u.Rate,
		IsMain: u.IsMain,
	}
}

func ToLocationResponse(p geo.Point) LocationResponse {
	return LocationResponse{
		Lat: p.Lat,
		Lng: p.Lng,
	}
}

func ToStockResponse(st *entities.Stock) StockResponse {
	units := make([]UnitResponse, 0, len(st.Units))
	balances := make([]UnitBalance, 0, len(st.Units))
	for _, u := range st.Units {
		units = append(units, ToUnitResponse(u))
		amount := 0.0
		if u.Rate > 0 {
			amount = round2(st.Available / float64(u.Rate))
		}
		balances = append(balances, UnitBalance{
			UnitID: u.ID,
			Name:   u.Name,
			Amount: amount,
		})
	}

	var details map[string]any
	if len(st.StockDetails) > 0 {
		_ = json.Unmarshal(st.StockDetails, &details)
	}

	return StockResponse{
		ID:           st.ID,
		Available:    st.Available,
		Address:      st.Address,
		Location:     ToLocationResponse(st.Location),
		Units:        units,
		Balances:     balances,
		StockType:    ToStockTypeResponse(&st.StockType),
		StockDetails: details,
		UserID:       st.UserID,
		CreatedAt:    st.CreatedAt,
		UpdatedAt:    st.UpdatedAt,
	}
}

func ToTransactionResponse(t *entities.Transaction) TransactionResponse {
	return TransactionResponse{
		ID:          t.ID,
		Amount:      t.Amount,
		Unit:        ToUnitResponse(t.Unit),
		StockID:     t.StockID,
		IsIn:        t.IsIn,
		Description: t.Description,
		UserID:      t.UserID,
		CreatedAt:   t.CreatedAt,
	}
}

func ToStockTypeResponse(st *entities.StockType) StockTypeResponse {
	return StockTypeResponse{
		ID:             st.ID,
		Name:           st.Name,
		Slug:           st.Slug,
		ProviderTypeID: st.ProviderTypeID,
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
