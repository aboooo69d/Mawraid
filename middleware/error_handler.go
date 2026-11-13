package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/pkg/constants/errs_code"
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
		respons.Fail(ctx, &respons.AppError{Status: http.StatusInternalServerError, Code: errs_code.ErrInternalServer, Message: "internal server error"})
	}
}
