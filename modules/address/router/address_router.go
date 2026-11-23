package addressRouter

import (
	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/middleware"
	addressHandler "github.com/najm-x/nova/modules/address/handler"
	roleRepo "github.com/najm-x/nova/modules/role/repo"
)

func RegisterAddressRoutes(rg *gin.RouterGroup, h *addressHandler.AddressHandler) {
	// Customer: own addresses.
	my := rg.Group("/addresses/me", middleware.SessionHandler(), middleware.RequireRole(roleRepo.RoleConsumer))
	{
		my.POST("", h.CreateAddress)
		my.GET("", h.ListMyAddresses)
		my.GET("/default", h.GetMyDefaultAddress)
		my.GET("/:address_id", h.GetMyAddress)
	}

	// Admin: read any user's addresses.
	admin := rg.Group("/admin", middleware.SessionHandler(), middleware.RequireRole(roleRepo.RoleAdmin))
	{
		admin.GET("/users/:user_id/addresses", h.AdminListUserAddresses)
		admin.GET("/users/:user_id/addresses/default", h.AdminGetUserDefaultAddress)
		admin.GET("/addresses/:address_id", h.AdminGetAddress)
	}
}
