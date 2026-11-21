package middleware

import (
	"github.com/gin-gonic/gin"
	userDto "github.com/najm-x/nova/modules/user/dto"
	"github.com/najm-x/nova/pkg/respons"
)

// RequireRole allows the request only when the session role matches one of roles.
// It must run after SessionHandler.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		role, _ := ctx.Get("role")
		name, _ := role.(string)
		for _, r := range roles {
			if name == r {
				ctx.Next()
				return
			}
		}
		respons.Abort(ctx, userDto.ErrForbidden)
	}
}
