package router

import (
	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/config"
	"github.com/najm-x/nova/middleware"
	"github.com/najm-x/nova/modules/auth/handler"
)

// RegisterAuthRoutes mounts the auth endpoints under /auth on the given group.
func RegisterAuthRoutes(rg *gin.RouterGroup, h *handler.AuthHandler, cfg *config.Config) {
	auth := rg.Group("/auth").Use(middleware.CfTurnstileHanduler(cfg))
	authWithOutTurnstile := rg.Group("/auth")

	{
		auth.POST("/register/consumer", h.RegisterConsumer)
		auth.POST("/register/provider", h.RegisterProvider)
		auth.POST("/login", h.Login)
		auth.POST("/verify-phone", h.VerifyPhone)
		auth.POST("/resend-otp", h.ResendOtp)
		auth.POST("/forget-password", h.ForgetPassword)
		auth.POST("/reset-password", h.ResetPassword)
		authWithOutTurnstile.POST("/logout", h.LogOut)

	}
}
