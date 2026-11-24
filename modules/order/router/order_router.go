package orderRouter

import (
	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/middleware"
	orderHandler "github.com/najm-x/nova/modules/order/handler"
	roleRepo "github.com/najm-x/nova/modules/role/repo"
)

func RegisterOrderRoutes(rg *gin.RouterGroup, h *orderHandler.OrderHandler) {
	consumer := rg.Group("/orders", middleware.SessionHandler(), middleware.RequireRole(roleRepo.RoleConsumer))
	{
		consumer.POST("", h.CreateOrder)
		consumer.GET("/me", h.ListConsumerOrders)
		consumer.GET("/me/:order_id", h.GetConsumerOrder)
		consumer.PATCH("/me/:order_id/cancel", h.CancelOrder)
	}

	provider := rg.Group("/orders/provider", middleware.SessionHandler(), middleware.RequireRole(roleRepo.RoleProvider))
	{
		provider.GET("", h.ListProviderOrders)
		provider.GET("/:order_id", h.GetProviderOrder)
		provider.PATCH("/:order_id/accept", h.AcceptOrder)
		provider.PATCH("/:order_id/reject", h.RejectOrder)
		provider.PATCH("/:order_id/complete", h.CompleteOrder)
	}
}
