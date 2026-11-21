package router

import (
	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/middleware"
	roleRepo "github.com/najm-x/nova/modules/role/repo"
	"github.com/najm-x/nova/modules/user/handler"
)

func RegisterUserRoutes(rg *gin.RouterGroup, h *handler.UserHandler) {
	users := rg.Group("/user").Use(middleware.SessionHandler())
	{
		users.GET("/me/stock", h.GetUserStock)
	}

	admin := rg.Group("/admin/users", middleware.SessionHandler(), middleware.RequireRole(roleRepo.RoleAdmin))
	{
		admin.GET("", h.ListUsers)
		admin.GET("/:id", h.GetUser)
		admin.PATCH("/:id/status", h.ChangeUserStatus)
	}
}
