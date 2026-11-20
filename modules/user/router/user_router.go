package router

import (
	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/middleware"
	"github.com/najm-x/nova/modules/user/handler"
)

// RegisterUserRoutes mounts administrative user management routes.
func RegisterUserRoutes(rg *gin.RouterGroup, h *handler.UserHandler) {
	adminUsers := rg.Group("/admin/users").Use(
		middleware.SessionHandler(),
		middleware.RequireAdmin(),
	)
	{
		adminUsers.GET("", h.GetAllUsers)
		adminUsers.GET("/:id", h.GetUserById)
		adminUsers.PUT("/:id", h.UpdateUser)
		adminUsers.DELETE("/:id", h.DeleteUser)
	}
}
