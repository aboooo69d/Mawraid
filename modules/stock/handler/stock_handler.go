package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	stockDto "github.com/najm-x/nova/modules/stock/dto"
	stockService "github.com/najm-x/nova/modules/stock/service"
	"github.com/najm-x/nova/pkg/respons"
)

type ServiceHandler struct {
	stockService stockService.StockService
}

func NewStockHandler(s stockService.StockService) *ServiceHandler {
	return &ServiceHandler{
		stockService: s,
	}
}

// CreateStock godoc
// @Summary      Create a new stock
// @Description  Create a stock for the authenticated user together with its units (exactly one main unit with rate = 1).
// @Tags         Stock
// @Accept       json
// @Produce      json
// @Param        request  body      stockDto.CreateStockRequest  true  "Stock creation payload"
// @Success      200      {object}  respons.Response{data=stockDto.StockResponse}  "Stock created successfully"
// @Failure      400      {object}  respons.Response  "Invalid request or unit validation error"
// @Failure      404      {object}  respons.Response  "Stock type not found"
// @Failure      409      {object}  respons.Response  "Duplicate unit name"
// @Failure      500      {object}  respons.Response  "Internal server error"
// @Router       /stocks [post]
func (h *ServiceHandler) CreateStock(ctx *gin.Context) {
	var req stockDto.CreateStockRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		_ = ctx.Error(respons.ErrBadRequest)
		return
	}

	userId := ctx.GetString("user_id")
	stockInfo, err := h.stockService.CreateStock(ctx.Request.Context(), userId, req)
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	respons.OK(ctx, stockInfo)
}

// GetStocks godoc
// @Summary      List user stocks
// @Description  Returns a paginated list of stocks owned by the authenticated user.
// @Tags         Stock
// @Produce      json
// @Success      200        {object}  respons.Response{data=[]stockDto.StockResponse}  "Stocks retrieved successfully"
// @Failure      400        {object}  respons.Response  "Invalid pagination parameters"
// @Failure      500        {object}  respons.Response  "Internal server error"
// @Router       /stocks [get]
func (h *ServiceHandler) GetUserStock(ctx *gin.Context) {
	userId := ctx.GetString("user_id")

	stock, err := h.stockService.GetUserStock(ctx.Request.Context(), userId)
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	respons.OK(ctx, stock)
}

// GetStock godoc
// @Summary      Get stock details
// @Description  Returns a stock's info, units, and available balance in every unit.
// @Tags         Stock
// @Produce      json
// @Param        stock_id  path      string  true  "Stock ID (UUID)"
// @Success      200       {object}  respons.Response{data=stockDto.StockResponse}  "Stock retrieved successfully"
// @Failure      404       {object}  respons.Response  "Stock not found"
// @Failure      500       {object}  respons.Response  "Internal server error"
// @Router       /stocks/{stock_id} [get]
func (h *ServiceHandler) GetStock(ctx *gin.Context) {
	userId := ctx.GetString("user_id")
	stockId := getStockID(ctx)
	if stockId == "" || userId == "" {
		_ = ctx.Error(stockDto.ErrStockNotFound)
		return
	}

	stockInfo, err := h.stockService.GetStock(ctx.Request.Context(), userId, stockId)
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	respons.OK(ctx, stockInfo)
}

// AddUnit godoc
// @Summary      Add unit to stock
// @Description  Adds an extra (non-main) unit to an existing stock owned by the authenticated user.
// @Tags         Stock
// @Accept       json
// @Produce      json
// @Param        stock_id  path      string                      true  "Stock ID (UUID)"
// @Param        request   body      stockDto.CreateUnitRequest  true  "Unit creation payload"
// @Success      200       {object}  respons.Response{data=stockDto.UnitResponse}  "Unit added successfully"
// @Failure      400       {object}  respons.Response  "Invalid request or main unit already exists"
// @Failure      404       {object}  respons.Response  "Stock not found"
// @Failure      409       {object}  respons.Response  "Duplicate unit name"
// @Failure      500       {object}  respons.Response  "Internal server error"
// @Router       /stocks/{stock_id}/units [post]
func (h *ServiceHandler) AddUnit(ctx *gin.Context) {
	userId := ctx.GetString("user_id")
	stockId := getStockID(ctx)
	if stockId == "" || userId == "" {
		_ = ctx.Error(stockDto.ErrStockNotFound)
		return
	}

	var req stockDto.CreateUnitRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		_ = ctx.Error(respons.ErrBadRequest)
		return
	}

	unit, err := h.stockService.AddUnit(ctx.Request.Context(), userId, stockId, req)
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	respons.OK(ctx, unit)
}

// UpdateUnit godoc
// @Summary      Update stock unit
// @Description  Updates the name and/or rate of an existing unit in a stock.
// @Tags         Stock
// @Accept       json
// @Produce      json
// @Param        stock_id  path      string                      true  "Stock ID (UUID)"
// @Param        unit_id   path      int                         true  "Unit ID"
// @Param        request   body      stockDto.UpdateUnitRequest  true  "Unit update payload"
// @Success      200       {object}  respons.Response{data=stockDto.UnitResponse}  "Unit updated successfully"
// @Failure      400       {object}  respons.Response  "Invalid request or invalid main unit rate"
// @Failure      404       {object}  respons.Response  "Stock or unit not found"
// @Failure      409       {object}  respons.Response  "Duplicate unit name or unit has transactions"
// @Failure      500       {object}  respons.Response  "Internal server error"
// @Router       /stocks/{stock_id}/units/{unit_id} [patch]
func (h *ServiceHandler) UpdateUnit(ctx *gin.Context) {
	userId := ctx.GetString("user_id")
	stockId := getStockID(ctx)
	if stockId == "" || userId == "" {
		_ = ctx.Error(stockDto.ErrStockNotFound)
		return
	}

	unitId, ok := getUnitID(ctx)
	if !ok {
		_ = ctx.Error(stockDto.ErrUnitNotFound)
		return
	}

	var req stockDto.UpdateUnitRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		_ = ctx.Error(respons.ErrBadRequest)
		return
	}

	unit, err := h.stockService.UpdateUnit(ctx.Request.Context(), userId, stockId, unitId, req)
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	respons.OK(ctx, unit)
}

// CreateTransaction godoc
// @Summary      Create stock transaction
// @Description  Adds to (is_in = true) or withdraws from (is_in = false) the stock using one of its units.
// @Tags         Stock
// @Accept       json
// @Produce      json
// @Param        stock_id  path      string                             true  "Stock ID (UUID)"
// @Param        request   body      stockDto.CreateTransactionRequest  true  "Transaction payload"
// @Success      200       {object}  respons.Response{data=stockDto.TransactionResponse}  "Transaction created successfully"
// @Failure      400       {object}  respons.Response  "Invalid request, invalid amount, or unit not in stock"
// @Failure      404       {object}  respons.Response  "Stock not found"
// @Failure      409       {object}  respons.Response  "Insufficient stock"
// @Failure      500       {object}  respons.Response  "Internal server error"
// @Router       /stocks/{stock_id}/transactions [post]
func (h *ServiceHandler) CreateTransaction(ctx *gin.Context) {
	userId := ctx.GetString("user_id")
	stockId := getStockID(ctx)
	if stockId == "" || userId == "" {
		_ = ctx.Error(stockDto.ErrStockNotFound)
		return
	}

	var req stockDto.CreateTransactionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		_ = ctx.Error(respons.ErrBadRequest)
		return
	}

	tx, err := h.stockService.CreateTransaction(ctx.Request.Context(), userId, stockId, req)
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	respons.OK(ctx, tx)
}

// GetTransactions godoc
// @Summary      List stock transactions
// @Description  Returns the paginated transaction history (newest first) for a stock.
// @Tags         Stock
// @Produce      json
// @Param        stock_id   path      string  true   "Stock ID (UUID)"
// @Param        page       query     int     false  "Page number (default 1)"
// @Param        page_size  query     int     false  "Page size (default 20, max 100)"
// @Param        is_in      query     bool    false  "Filter by direction (true = in, false = out)"
// @Success      200        {object}  respons.Response{data=[]stockDto.TransactionResponse}  "Transactions retrieved successfully"
// @Failure      400        {object}  respons.Response  "Invalid query parameters"
// @Failure      404        {object}  respons.Response  "Stock not found"
// @Failure      500        {object}  respons.Response  "Internal server error"
// @Router       /stocks/{stock_id}/transactions [get]
func (h *ServiceHandler) GetTransactions(ctx *gin.Context) {
	userId := ctx.GetString("user_id")
	stockId := getStockID(ctx)
	if stockId == "" || userId == "" {
		_ = ctx.Error(stockDto.ErrStockNotFound)
		return
	}

	var q stockDto.TransactionQuery
	if err := ctx.ShouldBindQuery(&q); err != nil {
		_ = ctx.Error(respons.ErrBadRequest)
		return
	}

	txs, meta, err := h.stockService.GetTransactions(ctx.Request.Context(), userId, stockId, q)
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	respons.OKWithMeta(ctx, txs, meta)
}

func (h *ServiceHandler) GetStockTypes(ctx *gin.Context) {
	stockTypes, err := h.stockService.GetStockTypes(ctx.Request.Context())
	if err != nil {
		_ = ctx.Error(stockDto.ErrStockTypeNotFound)
		return
	}

	respons.OK(ctx, stockTypes)
}

func getStockID(ctx *gin.Context) string {
	if id := ctx.Param("stock_id"); id != "" {
		return id
	}
	if id := ctx.Param("id"); id != "" {
		return id
	}
	return ctx.Query("stock_id")
}

func getUnitID(ctx *gin.Context) (uint, bool) {
	raw := ctx.Param("unit_id")
	if raw == "" {
		raw = ctx.Query("unit_id")
	}
	if raw == "" {
		return 0, false
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), true
}

func parsePositiveIntQuery(ctx *gin.Context, key string) (int, bool) {
	raw, exists := ctx.GetQuery(key)
	if !exists || raw == "" {
		return 0, true
	}
	val, err := strconv.Atoi(raw)
	if err != nil || val < 1 {
		return 0, false
	}
	return val, true
}
