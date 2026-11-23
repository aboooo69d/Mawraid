package router

import (
	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/middleware"
	roleRepo "github.com/najm-x/nova/modules/role/repo"
	"github.com/najm-x/nova/modules/stock/handler"
)

func RegisterStockRoutes(rg *gin.RouterGroup, h *handler.ServiceHandler) {
	// Shared: any authenticated provider/admin can browse stock types.
	stocks := rg.Group("/stocks", middleware.SessionHandler())
	{
		stocks.GET("/types", middleware.RequireRole(roleRepo.RoleProvider, roleRepo.RoleAdmin), h.GetStockTypes)
		stocks.GET("", middleware.RequireRole(roleRepo.RoleConsumer, roleRepo.RoleAdmin), h.GetStocks)
	}

	// Provider: manage the provider's own stocks.
	myStocks := rg.Group("/stocks/me", middleware.SessionHandler(), middleware.RequireRole(roleRepo.RoleProvider))
	{
		myStocks.POST("", h.CreateStock)
		myStocks.GET("", h.GetUserStocks)
		myStocks.GET("/:stock_id", h.GetStock)

		myStocks.POST("/:stock_id/units", h.AddUnit)
		myStocks.PATCH("/:stock_id/units/:unit_id", h.UpdateUnit)

		myStocks.POST("/:stock_id/transactions", h.CreateTransaction)
		myStocks.GET("/:stock_id/transactions", h.GetTransactions)
	}
}
