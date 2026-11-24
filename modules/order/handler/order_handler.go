package orderHandler

import (
	"errors"
	"io"

	"github.com/gin-gonic/gin"
	orderDto "github.com/najm-x/nova/modules/order/dto"
	orderService "github.com/najm-x/nova/modules/order/service"
	"github.com/najm-x/nova/pkg/respons"
)

type OrderHandler struct {
	svc orderService.OrderService
}

func NewOrderHandler(s orderService.OrderService) *OrderHandler {
	return &OrderHandler{svc: s}
}

// CreateOrder godoc
// @Summary      Create an order
// @Description  Books a single unit from a stock for the authenticated consumer and reserves stock immediately.
// @Tags         Order
// @Accept       json
// @Produce      json
// @Param        request  body      orderDto.CreateOrderRequest  true  "Order creation payload"
// @Success      200      {object}  respons.Response{data=orderDto.OrderResponse}  "Order created successfully"
// @Failure      400      {object}  respons.Response  "Invalid request, consumer type, or unit not in stock"
// @Failure      403      {object}  respons.Response  "Consumer account required or consumer type not enabled"
// @Failure      404      {object}  respons.Response  "Stock or address not found"
// @Failure      409      {object}  respons.Response  "Insufficient stock"
// @Router       /orders [post]
func (h *OrderHandler) CreateOrder(ctx *gin.Context) {
	var req orderDto.CreateOrderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		_ = ctx.Error(respons.ErrBadRequest)
		return
	}

	res, err := h.svc.CreateOrder(ctx.Request.Context(), ctx.GetString("user_id"), req)
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	respons.OK(ctx, res)
}

// ListConsumerOrders godoc
// @Summary      List my orders
// @Description  Returns paginated orders placed by the authenticated consumer.
// @Tags         Order
// @Produce      json
// @Param        status     query     string  false  "Filter by status"
// @Param        stock_id   query     string  false  "Filter by stock UUID"
// @Param        page       query     int     false  "Page number (default 1)"
// @Param        page_size  query     int     false  "Page size (default 20, max 100)"
// @Success      200        {object}  respons.Response{data=[]orderDto.OrderResponse}  "Orders retrieved successfully"
// @Failure      400        {object}  respons.Response  "Invalid query parameters"
// @Router       /orders/me [get]
func (h *OrderHandler) ListConsumerOrders(ctx *gin.Context) {
	var q orderDto.OrderQueryFilter
	if err := ctx.ShouldBindQuery(&q); err != nil {
		_ = ctx.Error(respons.ErrBadRequest)
		return
	}

	orders, meta, err := h.svc.ListConsumerOrders(ctx.Request.Context(), ctx.GetString("user_id"), q)
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	respons.OKWithMeta(ctx, orders, meta)
}

// GetConsumerOrder godoc
// @Summary      Get my order details
// @Description  Returns details for a single order owned by the authenticated consumer.
// @Tags         Order
// @Produce      json
// @Param        order_id  path      string  true  "Order ID (UUID)"
// @Success      200       {object}  respons.Response{data=orderDto.OrderResponse}  "Order retrieved successfully"
// @Failure      404       {object}  respons.Response  "Order not found"
// @Router       /orders/me/{order_id} [get]
func (h *OrderHandler) GetConsumerOrder(ctx *gin.Context) {
	res, err := h.svc.GetConsumerOrder(ctx.Request.Context(), ctx.GetString("user_id"), ctx.Param("order_id"))
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	respons.OK(ctx, res)
}

// CancelOrder godoc
// @Summary      Cancel pending order
// @Description  Cancels a pending order for the authenticated consumer and restores stock availability.
// @Tags         Order
// @Accept       json
// @Produce      json
// @Param        order_id  path      string                       true   "Order ID (UUID)"
// @Param        request   body      orderDto.OrderActionRequest  false  "Optional cancellation reason"
// @Success      200       {object}  respons.Response{data=orderDto.OrderResponse}  "Order cancelled"
// @Failure      400       {object}  respons.Response  "Invalid request"
// @Failure      404       {object}  respons.Response  "Order not found"
// @Failure      409       {object}  respons.Response  "Order status transition is not allowed"
// @Router       /orders/me/{order_id}/cancel [patch]
func (h *OrderHandler) CancelOrder(ctx *gin.Context) {
	req, ok := bindOptionalReason(ctx)
	if !ok {
		return
	}

	res, err := h.svc.CancelOrder(ctx.Request.Context(), ctx.GetString("user_id"), ctx.Param("order_id"), req)
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	respons.OK(ctx, res)
}

// ListProviderOrders godoc
// @Summary      List provider orders
// @Description  Returns paginated incoming orders for the authenticated provider.
// @Tags         Order
// @Produce      json
// @Param        status     query     string  false  "Filter by status"
// @Param        stock_id   query     string  false  "Filter by stock UUID"
// @Param        page       query     int     false  "Page number (default 1)"
// @Param        page_size  query     int     false  "Page size (default 20, max 100)"
// @Success      200        {object}  respons.Response{data=[]orderDto.OrderResponse}  "Orders retrieved successfully"
// @Failure      400        {object}  respons.Response  "Invalid query parameters"
// @Router       /orders/provider [get]
func (h *OrderHandler) ListProviderOrders(ctx *gin.Context) {
	var q orderDto.OrderQueryFilter
	if err := ctx.ShouldBindQuery(&q); err != nil {
		_ = ctx.Error(respons.ErrBadRequest)
		return
	}

	orders, meta, err := h.svc.ListProviderOrders(ctx.Request.Context(), ctx.GetString("user_id"), q)
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	respons.OKWithMeta(ctx, orders, meta)
}

// GetProviderOrder godoc
// @Summary      Get provider order details
// @Description  Returns details for a single incoming order belonging to the authenticated provider.
// @Tags         Order
// @Produce      json
// @Param        order_id  path      string  true  "Order ID (UUID)"
// @Success      200       {object}  respons.Response{data=orderDto.OrderResponse}  "Order retrieved successfully"
// @Failure      404       {object}  respons.Response  "Order not found"
// @Router       /orders/provider/{order_id} [get]
func (h *OrderHandler) GetProviderOrder(ctx *gin.Context) {
	res, err := h.svc.GetProviderOrder(ctx.Request.Context(), ctx.GetString("user_id"), ctx.Param("order_id"))
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	respons.OK(ctx, res)
}

// AcceptOrder godoc
// @Summary      Accept pending order
// @Description  Accepts a pending order for the authenticated provider and decrements stock pending_orders.
// @Tags         Order
// @Produce      json
// @Param        order_id  path      string  true  "Order ID (UUID)"
// @Success      200       {object}  respons.Response{data=orderDto.OrderResponse}  "Order accepted"
// @Failure      404       {object}  respons.Response  "Order not found"
// @Failure      409       {object}  respons.Response  "Order status transition is not allowed"
// @Router       /orders/provider/{order_id}/accept [patch]
func (h *OrderHandler) AcceptOrder(ctx *gin.Context) {
	res, err := h.svc.AcceptOrder(ctx.Request.Context(), ctx.GetString("user_id"), ctx.Param("order_id"))
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	respons.OK(ctx, res)
}

// RejectOrder godoc
// @Summary      Reject pending order
// @Description  Rejects a pending order for the authenticated provider, restores stock availability, and decrements pending_orders.
// @Tags         Order
// @Accept       json
// @Produce      json
// @Param        order_id  path      string                       true   "Order ID (UUID)"
// @Param        request   body      orderDto.OrderActionRequest  false  "Optional rejection reason"
// @Success      200       {object}  respons.Response{data=orderDto.OrderResponse}  "Order rejected"
// @Failure      400       {object}  respons.Response  "Invalid request"
// @Failure      404       {object}  respons.Response  "Order not found"
// @Failure      409       {object}  respons.Response  "Order status transition is not allowed"
// @Router       /orders/provider/{order_id}/reject [patch]
func (h *OrderHandler) RejectOrder(ctx *gin.Context) {
	req, ok := bindOptionalReason(ctx)
	if !ok {
		return
	}

	res, err := h.svc.RejectOrder(ctx.Request.Context(), ctx.GetString("user_id"), ctx.Param("order_id"), req)
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	respons.OK(ctx, res)
}

// CompleteOrder godoc
// @Summary      Complete accepted order
// @Description  Marks an accepted order as completed and increments stock completed_orders.
// @Tags         Order
// @Produce      json
// @Param        order_id  path      string  true  "Order ID (UUID)"
// @Success      200       {object}  respons.Response{data=orderDto.OrderResponse}  "Order completed"
// @Failure      404       {object}  respons.Response  "Order not found"
// @Failure      409       {object}  respons.Response  "Order status transition is not allowed"
// @Router       /orders/provider/{order_id}/complete [patch]
func (h *OrderHandler) CompleteOrder(ctx *gin.Context) {
	res, err := h.svc.CompleteOrder(ctx.Request.Context(), ctx.GetString("user_id"), ctx.Param("order_id"))
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	respons.OK(ctx, res)
}

func bindOptionalReason(ctx *gin.Context) (orderDto.OrderActionRequest, bool) {
	var req orderDto.OrderActionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		_ = ctx.Error(respons.ErrBadRequest)
		return req, false
	}
	return req, true
}
