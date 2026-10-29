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
			respons.Fail(ctx, authDto.ErrUserNotFound)
			ctx.Abort()
			return
		}

		ctx.Set("user_id", userId)
		ctx.Set("role", role)
		ctx.Next()
	}
}
