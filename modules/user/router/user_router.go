package router

import (
	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/middleware"
	roleRepo "github.com/najm-x/nova/modules/role/repo"
	"github.com/najm-x/nova/modules/user/handler"
)

func RegisterUserRoutes(rg *gin.RouterGroup, h *handler.UserHandler) {
	rg.GET("/provider-types", h.ListProviderTypes)

	admin := rg.Group("/admin", middleware.SessionHandler(), middleware.RequireRole(roleRepo.RoleAdmin))
	{
		adminUsers := admin.Group("/users")
		{
			adminUsers.GET("", h.ListUsers)
			adminUsers.GET("/:id", h.GetUser)
			adminUsers.PATCH("/:id/status", h.ChangeUserStatus)
		}

		adminProviderTypes := admin.Group("/provider-types")
		{
			adminProviderTypes.POST("", h.CreateProviderType)
		}
	}
}
