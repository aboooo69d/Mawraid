package router

import (
	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/middleware"
	"github.com/najm-x/nova/modules/stock/handler"
)

func RegisterStockRoutes(rg *gin.RouterGroup, h *handler.ServiceHandler) {
	stocks := rg.Group("/stocks").Use(middleware.SessionHandler())
	{
		stocks.GET("/types", h.GetStockTypes)
		stocks.POST("", h.CreateStock)
		stocks.GET("", h.GetUserStock)
		stocks.GET("/:stock_id", h.GetStock)

		stocks.POST("/:stock_id/units", h.AddUnit)
		stocks.PATCH("/:stock_id/units/:unit_id", h.UpdateUnit)

		stocks.POST("/:stock_id/transactions", h.CreateTransaction)
		stocks.GET("/:stock_id/transactions", h.GetTransactions)
	}
}
