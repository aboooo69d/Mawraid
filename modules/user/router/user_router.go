package router

import (
	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/middleware"
	"github.com/najm-x/nova/modules/user/handler"
)

func RegisterUserRoutes(rg *gin.RouterGroup, h *handler.UserHandler) {
	users := rg.Group("/user").Use(middleware.SessionHandler())
	{
		users.GET("/me/stock", h.GetUserStock)
	}
}
