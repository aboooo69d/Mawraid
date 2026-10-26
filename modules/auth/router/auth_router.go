package router

import (
	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/modules/auth/handler"
)

// RegisterAuthRoutes mounts the auth endpoints under /auth on the given group.
func RegisterAuthRoutes(rg *gin.RouterGroup, h *handler.AuthHandler) {
	auth := rg.Group("/auth")
	{
		auth.POST("/register/consumer", h.RegisterConsumer)
		auth.POST("/register/provider", h.RegisterProvider)
		auth.POST("/login", h.Login)
		auth.POST("/verify-phone", h.VerifyPhone)
	}
}
