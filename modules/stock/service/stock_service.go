package stockService

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	stockDto "github.com/najm-x/nova/modules/stock/dto"
	stockRepo "github.com/najm-x/nova/modules/stock/repo"
	"github.com/najm-x/nova/pkg/respons"
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
		GetUserStock(c context.Context, userId string) (*stockDto.StockResponse, error)
		GetStock(c context.Context, userId string, stockId string) (*stockDto.StockResponse, error)

		// Units
		AddUnit(c context.Context, userId string, stockId string, req stockDto.CreateUnitRequest) (*stockDto.UnitResponse, error)
		UpdateUnit(c context.Context, userId string, stockId string, unitId uint, req stockDto.UpdateUnitRequest) (*stockDto.UnitResponse, error)

		// Transactions (add to / withdraw from the stock)
		CreateTransaction(c context.Context, userId string, stockId string, req stockDto.CreateTransactionRequest) (*stockDto.TransactionResponse, error)
		GetTransactions(c context.Context, userId string, stockId string, q stockDto.TransactionQuery) ([]stockDto.TransactionResponse, *respons.Meta, error)
		GetStockTypes(c context.Context) ([]stockDto.StockTypeResponse, error)
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

// CreateStock creates a stock for the user together with its units.
// A user can only have one stock. Exactly one unit must be the main unit (is_main = true, rate = 1).
func (s *stockService) CreateStock(c context.Context, userId string, req stockDto.CreateStockRequest) (*stockDto.StockResponse, error) {
	uid, err := uuid.Parse(userId)
	if err != nil {
		return nil, respons.ErrBadRequest
	}

	hasStock, err := s.stockRepo.UserHasStock(c, userId)
	if err != nil {
		return nil, err
	}
	if hasStock {
		return nil, stockDto.ErrStockAlreadyExists
	}

	exists, err := s.stockRepo.StockTypeExists(c, req.StockTypeID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, stockDto.ErrStockTypeNotFound
	}

	if err := validateNewUnits(req.Units); err != nil {
		return nil, err
	}

	stock := &entities.Stock{
		ID:          uuid.New(),
		StockTypeID: req.StockTypeID,
		UserID:      uid,
		Units:       make([]entities.Unit, 0, len(req.Units)),
	}
	for _, u := range req.Units {
		stock.Units = append(stock.Units, entities.Unit{
			Name:   strings.TrimSpace(u.Name),
			Rate:   u.Rate,
			IsMain: u.IsMain,
		})
	}

	if err := s.stockRepo.CreateStock(c, stock); err != nil {
		return nil, mapRepoErr(err)
	}

	// Reload to get StockType and DB defaults.
	return s.GetStock(c, userId, stock.ID.String())
}

func (s *stockService) GetUserStock(c context.Context, userId string) (*stockDto.StockResponse, error) {

	stock, err := s.stockRepo.GetStockByUserId(c, userId)
	if err != nil {
		return nil, err
	}
	res := stockDto.ToStockResponse(&stock)
	return &res, nil
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

func (s *stockService) GetStockTypes(c context.Context) ([]stockDto.StockTypeResponse, error) {

	stockTypes, err := s.stockRepo.GetStockTypes(c)
	if err != nil {
		return nil, err
	}
	types := make([]stockDto.StockTypeResponse, 0, len(stockTypes))

	for _, val := range stockTypes {
		types = append(types, stockDto.ToStockTypeResponse(&val))
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

	unit := &entities.Unit{
		Name:    name,
		Rate:    req.Rate,
		StockID: stock.ID,
	}
	if err := s.unitRepo.CreateUnit(c, unit); err != nil {
		return nil, mapRepoErr(err)
	}

	res := stockDto.ToUnitResponse(*unit)
	return &res, nil
}

// UpdateUnit changes the name and/or rate of a unit. Only the sent fields are changed.
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

	page, pageSize := normalizePage(q.Page, q.PageSize)
	txs, total, err := s.transactionRepo.GetTransactionsByStockId(c, stock.ID.String(), stockRepo.TransactionFilter{
		IsIn:   q.IsIn,
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	})
	if err != nil {
		return nil, nil, err
	}

	list := make([]stockDto.TransactionResponse, 0, len(txs))
	for i := range txs {
		list = append(list, stockDto.ToTransactionResponse(&txs[i]))
	}
	return list, buildMeta(page, pageSize, total), nil
}

// ==================== Helpers ====================

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
		if seen[key] {
			return stockDto.ErrDuplicateUnitName
		}
		seen[key] = true

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
	case errors.Is(err, stockRepo.ErrInvalidUnitRate):
		return respons.ErrBadRequest
	case errors.Is(err, stockRepo.ErrUnitHasTransactions):
		return stockDto.ErrUnitHasTransactions
	case errors.Is(err, stockRepo.ErrStockAlreadyExists):
		return stockDto.ErrStockAlreadyExists
	case errors.Is(err, gorm.ErrRecordNotFound):
		return stockDto.ErrStockNotFound
	}
	return err
}

func normalizePage(page int, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

func buildMeta(page int, pageSize int, total int64) *respons.Meta {
	return &respons.Meta{
		Page:       page,
		PerPage:    pageSize,
		Total:      int(total),
		TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize)),
	}
}
