package stockService

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	roleRepo "github.com/najm-x/nova/modules/role/repo"
	stockDto "github.com/najm-x/nova/modules/stock/dto"
	stockFilter "github.com/najm-x/nova/modules/stock/filter"
	stockRepo "github.com/najm-x/nova/modules/stock/repo"
	"github.com/najm-x/nova/pkg/geo"
	"github.com/najm-x/nova/pkg/respons"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

type (
	StockService interface {
		// Stock
		CreateStock(c context.Context, userId string, req stockDto.CreateStockRequest) (*stockDto.StockResponse, error)
		GetStocks(c context.Context, q stockDto.StockQueryFilter) ([]stockDto.StockResponse, *respons.Meta, error)
		GetUserStocks(c context.Context, userId string) ([]stockDto.StockResponse, error)
		GetStock(c context.Context, userId string, stockId string) (*stockDto.StockResponse, error)

		// Units
		AddUnit(c context.Context, userId string, stockId string, req stockDto.CreateUnitRequest) (*stockDto.UnitResponse, error)
		UpdateUnit(c context.Context, userId string, stockId string, unitId uint, req stockDto.UpdateUnitRequest) (*stockDto.UnitResponse, error)

		// Transactions (add to / withdraw from the stock)
		CreateTransaction(c context.Context, userId string, stockId string, req stockDto.CreateTransactionRequest) (*stockDto.TransactionResponse, error)
		GetTransactions(c context.Context, userId string, stockId string, q stockDto.TransactionQuery) ([]stockDto.TransactionResponse, *respons.Meta, error)
		GetStockTypes(c context.Context, userId string, role string) ([]stockDto.StockTypeResponse, error)
	}

	stockService struct {
		stockRepo       stockRepo.StockRepo
		unitRepo        stockRepo.UnitRepo
		transactionRepo stockRepo.TransactionRepo
	}
)

func NewStockService(
	stockRepo stockRepo.StockRepo,
	unitRepo stockRepo.UnitRepo,
	transactionRepo stockRepo.TransactionRepo,
) StockService {
	return &stockService{
		stockRepo:       stockRepo,
		unitRepo:        unitRepo,
		transactionRepo: transactionRepo,
	}
}

// ==================== Stock ====================

// CreateStock creates a stock for the provider together with its units.
// Exactly one unit must be the main unit (is_main = true, rate = 1).
func (s *stockService) CreateStock(c context.Context, userId string, req stockDto.CreateStockRequest) (*stockDto.StockResponse, error) {
	uid, err := uuid.Parse(userId)
	if err != nil {
		return nil, respons.ErrBadRequest
	}

	providerTypeID, err := s.stockRepo.GetProviderTypeIdByUserId(c, userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, stockDto.ErrProviderRequired
		}
		return nil, err
	}

	stockType, err := s.stockRepo.GetStockTypeById(c, req.StockTypeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, stockDto.ErrStockTypeNotFound
		}
		return nil, err
	}

	if stockType.ProviderTypeID != providerTypeID {
		return nil, stockDto.ErrStockTypeNotAllowed
	}

	if !strings.EqualFold(strings.TrimSpace(req.StockType), strings.TrimSpace(stockType.Slug)) {
		return nil, stockDto.ErrStockTypeMismatch
	}

	address := strings.TrimSpace(req.Address)
	addrLen := utf8.RuneCountInString(address)
	if addrLen < 10 || addrLen > 200 {
		return nil, respons.ErrBadRequest
	}

	if req.Location.Lat == nil || req.Location.Lng == nil {
		return nil, respons.ErrBadRequest
	}
	lat, lng := *req.Location.Lat, *req.Location.Lng
	if math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 ||
		math.IsNaN(lng) || math.IsInf(lng, 0) || lng < -180 || lng > 180 {
		return nil, respons.ErrBadRequest
	}

	stockDetails, err := buildStockDetails(stockType.Slug, req.StockDetails)
	if err != nil {
		return nil, err
	}

	if err := validateNewUnits(req.Units); err != nil {
		return nil, err
	}

	stock := &entities.Stock{
		ID:           uuid.New(),
		Address:      address,
		Location:     geo.Point{Lat: lat, Lng: lng},
		StockTypeID:  stockType.ID,
		UserID:       uid,
		StockDetails: stockDetails,
		Units:        make([]entities.Unit, 0, len(req.Units)),
	}
	for _, u := range req.Units {
		stock.Units = append(stock.Units, entities.Unit{
			Name:   strings.TrimSpace(u.Name),
			Rate:   u.Rate,
			Price:  u.Price,
			IsMain: u.IsMain,
		})
	}

	if err := s.stockRepo.CreateStock(c, stock); err != nil {
		return nil, mapRepoErr(err)
	}

	// Reload to get StockType and DB defaults.
	return s.GetStock(c, userId, stock.ID.String())
}

func (s *stockService) GetStocks(c context.Context, q stockDto.StockQueryFilter) ([]stockDto.StockResponse, *respons.Meta, error) {
	if (q.Lat == nil) != (q.Lng == nil) {
		return nil, nil, respons.ErrBadRequest
	}
	if q.Lat != nil && q.Lng != nil {
		lat, lng := *q.Lat, *q.Lng
		if math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 ||
			math.IsNaN(lng) || math.IsInf(lng, 0) || lng < -180 || lng > 180 {
			return nil, nil, respons.ErrBadRequest
		}
	}
	if q.MaxDistance != nil {
		if q.Lat == nil || q.Lng == nil || math.IsNaN(*q.MaxDistance) || math.IsInf(*q.MaxDistance, 0) || *q.MaxDistance <= 0 {
			return nil, nil, respons.ErrBadRequest
		}
	}

	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	stocks, total, err := s.stockRepo.GetStocks(c, stockFilter.StockFilter{
		Name:        q.Name,
		StockType:   q.StockType,
		StockTypeID: q.StockTypeID,
		InStock:     q.InStock,
		Lat:         q.Lat,
		Lng:         q.Lng,
		MaxDistance: q.MaxDistance,
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		return nil, nil, err
	}

	res := make([]stockDto.StockResponse, 0, len(stocks))
	for i := range stocks {
		res = append(res, stockDto.ToStockResponse(&stocks[i]))
	}
	return res, buildMeta(page, pageSize, total), nil
}

func (s *stockService) GetUserStocks(c context.Context, userId string) ([]stockDto.StockResponse, error) {
	if _, err := uuid.Parse(userId); err != nil {
		return nil, respons.ErrBadRequest
	}

	stocks, err := s.stockRepo.GetStocksByUserId(c, userId)
	if err != nil {
		return nil, err
	}

	res := make([]stockDto.StockResponse, 0, len(stocks))
	for i := range stocks {
		res = append(res, stockDto.ToStockResponse(&stocks[i]))
	}
	return res, nil
}

// GetStock returns the stock info, its units and the available quantity in every unit.
func (s *stockService) GetStock(c context.Context, userId string, stockId string) (*stockDto.StockResponse, error) {
	stock, err := s.getOwnedStock(c, stockId, userId)
	if err != nil {
		return nil, err
	}
	res := stockDto.ToStockResponse(stock)
	return &res, nil
}

func (s *stockService) GetStockTypes(c context.Context, userId string, role string) ([]stockDto.StockTypeResponse, error) {
	var stockTypes []entities.StockType
	var err error

	if role == roleRepo.RoleAdmin {
		stockTypes, err = s.stockRepo.GetStockTypes(c)
		if err != nil {
			return nil, err
		}
	} else {
		if _, err := uuid.Parse(userId); err != nil {
			return nil, respons.ErrBadRequest
		}
		providerTypeID, err := s.stockRepo.GetProviderTypeIdByUserId(c, userId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, stockDto.ErrProviderRequired
			}
			return nil, err
		}
		stockTypes, err = s.stockRepo.GetStockTypesByProviderTypeId(c, providerTypeID)
		if err != nil {
			return nil, err
		}
	}

	types := make([]stockDto.StockTypeResponse, 0, len(stockTypes))
	for i := range stockTypes {
		types = append(types, stockDto.ToStockTypeResponse(&stockTypes[i]))
	}
	return types, nil
}

// ==================== Units ====================

// AddUnit adds an extra unit (e.g. "Box" = 12 pieces) to an existing stock.
func (s *stockService) AddUnit(c context.Context, userId string, stockId string, req stockDto.CreateUnitRequest) (*stockDto.UnitResponse, error) {
	stock, err := s.getOwnedStock(c, stockId, userId)
	if err != nil {
		return nil, err
	}

	if req.IsMain {
		return nil, stockDto.ErrMainUnitExists
	}
	name := strings.TrimSpace(req.Name)
	if unitNameTaken(stock.Units, name, 0) {
		return nil, stockDto.ErrDuplicateUnitName
	}

	if math.IsNaN(req.Price) || math.IsInf(req.Price, 0) || req.Price <= 0 {
		return nil, respons.ErrBadRequest
	}

	unit := &entities.Unit{
		Name:    name,
		Rate:    req.Rate,
		Price:   req.Price,
		StockID: stock.ID,
	}
	if err := s.unitRepo.CreateUnit(c, unit); err != nil {
		return nil, mapRepoErr(err)
	}

	res := stockDto.ToUnitResponse(*unit)
	return &res, nil
}

// UpdateUnit changes the name, rate, and/or price of a unit. Only the sent fields are changed.
func (s *stockService) UpdateUnit(c context.Context, userId string, stockId string, unitId uint, req stockDto.UpdateUnitRequest) (*stockDto.UnitResponse, error) {
	stock, err := s.getOwnedStock(c, stockId, userId)
	if err != nil {
		return nil, err
	}

	current := findUnit(stock.Units, unitId)
	if current == nil {
		return nil, stockDto.ErrUnitNotFound
	}

	updated := *current
	if req.Name != nil {
		updated.Name = strings.TrimSpace(*req.Name)
		if unitNameTaken(stock.Units, updated.Name, unitId) {
			return nil, stockDto.ErrDuplicateUnitName
		}
	}
	if req.Rate != nil {
		updated.Rate = *req.Rate
	}
	if req.Price != nil {
		if math.IsNaN(*req.Price) || math.IsInf(*req.Price, 0) || *req.Price <= 0 {
			return nil, respons.ErrBadRequest
		}
		updated.Price = *req.Price
	}

	if err := s.unitRepo.UpdateUnit(c, stock.ID.String(), unitId, &updated); err != nil {
		return nil, mapRepoErr(err)
	}

	res := stockDto.ToUnitResponse(updated)
	return &res, nil
}

// ==================== Transactions ====================

// CreateTransaction adds to (is_in = true) or withdraws from (is_in = false) the stock
// an amount of one of its units. The amount is converted to the main unit using the unit rate.
func (s *stockService) CreateTransaction(c context.Context, userId string, stockId string, req stockDto.CreateTransactionRequest) (*stockDto.TransactionResponse, error) {
	if req.Amount <= 0 {
		return nil, stockDto.ErrInvalidAmount
	}
	if req.IsIn == nil {
		return nil, respons.ErrBadRequest
	}

	stock, err := s.getOwnedStock(c, stockId, userId)
	if err != nil {
		return nil, err
	}

	unit := findUnit(stock.Units, req.UnitID)
	if unit == nil {
		return nil, stockDto.ErrUnitNotInStock
	}

	// Stock.Available is stored in the main unit.
	qty := float64(req.Amount) * float64(unit.Rate)

	t := &entities.Transaction{
		ID:          uuid.New(),
		Amount:      req.Amount,
		UnitID:      unit.ID,
		StockID:     stock.ID,
		IsIn:        *req.IsIn,
		Description: req.Description,
		UserID:      stock.UserID,
	}
	if err := s.transactionRepo.ApplyTransaction(c, t, qty); err != nil {
		return nil, mapRepoErr(err)
	}

	t.Unit = *unit
	res := stockDto.ToTransactionResponse(t)
	return &res, nil
}

// GetTransactions returns the stock history (newest first) with pagination.
func (s *stockService) GetTransactions(c context.Context, userId string, stockId string, q stockDto.TransactionQuery) ([]stockDto.TransactionResponse, *respons.Meta, error) {
	stock, err := s.getOwnedStock(c, stockId, userId)
	if err != nil {
		return nil, nil, err
	}

	txs, total, err := s.transactionRepo.GetTransactionsByStockId(c, stock.ID.String(), stockRepo.TransactionFilter{
		IsIn:     q.IsIn,
		Page:     q.Page,
		PageSize: q.PageSize,
	})
	if err != nil {
		return nil, nil, err
	}

	list := make([]stockDto.TransactionResponse, 0, len(txs))
	for i := range txs {
		list = append(list, stockDto.ToTransactionResponse(&txs[i]))
	}
	return list, buildMeta(q.Page, q.PageSize, total), nil
}

// ==================== Helpers ====================

func buildStockDetails(stockTypeSlug string, details map[string]any) (datatypes.JSON, error) {
	if strings.EqualFold(strings.TrimSpace(stockTypeSlug), stockDto.StockTypeSlugGaz) {
		if len(details) == 0 {
			return nil, stockDto.ErrInvalidStockDetails
		}
		raw, ok := details["gaz_stock_type"].(string)
		if !ok {
			return nil, stockDto.ErrInvalidStockDetails
		}
		gazType := strings.ToLower(strings.TrimSpace(raw))
		if gazType != stockDto.GazTypeExchange && gazType != stockDto.GazTypeRefill {
			return nil, stockDto.ErrInvalidStockDetails
		}
		b, err := json.Marshal(stockDto.GazStockDetails{GazStockType: gazType})
		if err != nil {
			return nil, stockDto.ErrInvalidStockDetails
		}
		return datatypes.JSON(b), nil
	}

	if len(details) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(details)
	if err != nil {
		return nil, stockDto.ErrInvalidStockDetails
	}
	return datatypes.JSON(b), nil
}

// getOwnedStock returns the stock (with units and type) only if it belongs to userId.
// A stock of another user is reported as "not found" so its existence is not leaked.
func (s *stockService) getOwnedStock(c context.Context, stockId string, userId string) (*entities.Stock, error) {
	if _, err := uuid.Parse(stockId); err != nil {
		return nil, stockDto.ErrStockNotFound
	}
	uid, err := uuid.Parse(userId)
	if err != nil {
		return nil, stockDto.ErrStockNotFound
	}

	stock, err := s.stockRepo.GetStockById(c, stockId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, stockDto.ErrStockNotFound
		}
		return nil, err
	}
	if stock.UserID != uid {
		return nil, stockDto.ErrStockNotFound
	}
	return stock, nil
}

// validateNewUnits checks the units sent when creating a stock.
func validateNewUnits(units []stockDto.CreateUnitRequest) error {
	mainCount := 0
	seen := make(map[string]bool, len(units))
	for _, u := range units {
		key := strings.ToLower(strings.TrimSpace(u.Name))
		if key == "" {
			return respons.ErrBadRequest
		}
		if seen[key] {
			return stockDto.ErrDuplicateUnitName
		}
		seen[key] = true

		if u.Rate < 1 {
			return respons.ErrBadRequest
		}
		if math.IsNaN(u.Price) || math.IsInf(u.Price, 0) || u.Price <= 0 {
			return respons.ErrBadRequest
		}
		if u.IsMain {
			mainCount++
			if u.Rate != 1 {
				return stockDto.ErrMainUnitRate
			}
		}
	}
	if mainCount != 1 {
		return stockDto.ErrMainUnitRequired
	}
	return nil
}

func findUnit(units []entities.Unit, id uint) *entities.Unit {
	for i := range units {
		if units[i].ID == id {
			return &units[i]
		}
	}
	return nil
}

// unitNameTaken reports whether another unit (other than exceptId) already has this name.
func unitNameTaken(units []entities.Unit, name string, exceptId uint) bool {
	for _, u := range units {
		if u.ID != exceptId && strings.EqualFold(u.Name, name) {
			return true
		}
	}
	return false
}

// mapRepoErr converts repo errors into API errors.
func mapRepoErr(err error) error {
	switch {
	case errors.Is(err, stockRepo.ErrInsufficientStock):
		return stockDto.ErrInsufficientStock
	case errors.Is(err, stockRepo.ErrMainUnitRequired):
		return stockDto.ErrMainUnitRequired
	case errors.Is(err, stockRepo.ErrMainUnitRate):
		return stockDto.ErrMainUnitRate
	case errors.Is(err, stockRepo.ErrInvalidUnitRate), errors.Is(err, stockRepo.ErrInvalidUnitPrice):
		return respons.ErrBadRequest
	case errors.Is(err, stockRepo.ErrUnitHasTransactions):
		return stockDto.ErrUnitHasTransactions
	case errors.Is(err, gorm.ErrRecordNotFound):
		return stockDto.ErrStockNotFound
	}
	return err
}

func buildMeta(page int, pageSize int, total int64) *respons.Meta {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
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
