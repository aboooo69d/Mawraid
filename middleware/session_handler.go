package middleware

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	authDto "github.com/najm-x/nova/modules/auth/dto"
	"github.com/najm-x/nova/pkg/respons"
)

func SessionHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		userId := session.Get("user_id")
		role := session.Get("role")

		if userId == nil {
			respons.Abort(ctx, authDto.ErrUnauthorized)
			return
		}

		ctx.Set("user_id", userId)
		ctx.Set("role", role)
		ctx.Next()
	}
}

// RequireRole verifies that the logged-in user possesses one of the allowed roles.
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		roleVal, exists := ctx.Get("role")
		if !exists || roleVal == nil {
			respons.Abort(ctx, authDto.ErrUnauthorized)
			return
		}

		userRole, ok := roleVal.(string)
		if !ok {
			respons.Abort(ctx, authDto.ErrUnauthorized)
			return
		}

		for _, r := range allowedRoles {
			if r == userRole {
				ctx.Next()
				return
			}
		}

		respons.Abort(ctx, authDto.ErrAdminAccessRequired)
	}
}

// RequireAdmin is a convenience helper requiring the "admin" role.
func RequireAdmin() gin.HandlerFunc {
	return RequireRole("admin")
}
