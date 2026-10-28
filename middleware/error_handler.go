package middleware

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/pkg/respons"
)

func ErrorHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()

		if len(ctx.Errors) == 0 {
			return
		}

		var AppErr *respons.AppError
		err := ctx.Errors.Last().Err
		if errors.As(err, &AppErr) {
			respons.Fail(ctx, AppErr)
			return
		}
		respons.Fail(ctx, &respons.AppError{Status: 500, Code: 1000, Message: "Enternal server Erro"})
	}
}
