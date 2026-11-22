package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/pkg/constants/errs_code"
	"github.com/najm-x/nova/pkg/logger"
	"github.com/najm-x/nova/pkg/respons"
)

func ErrorHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()

		if len(ctx.Errors) == 0 {
			return
		}

		log := logger.Log.With(
			"middleware", "error_handler",
			"method", ctx.Request.Method,
			"path", ctx.Request.URL.Path,
			"client_ip", ctx.ClientIP(),
		)

		var AppErr *respons.AppError
		err := ctx.Errors.Last().Err
		if errors.As(err, &AppErr) {
			if AppErr.Status >= http.StatusInternalServerError {
				log.Error(
					"request failed with app error",
					"status", AppErr.Status,
					"code", AppErr.Code,
					"error", err.Error(),
				)
			} else {
				log.Warn(
					"request failed with app error",
					"status", AppErr.Status,
					"code", AppErr.Code,
					"error", err.Error(),
				)
			}
			respons.Fail(ctx, AppErr)
			return
		}

		log.Error(
			"unhandled internal server error",
			"status", http.StatusInternalServerError,
			"code", errs_code.ErrInternalServer,
			"error", err.Error(),
		)
		respons.Fail(ctx, &respons.AppError{Status: http.StatusInternalServerError, Code: errs_code.ErrInternalServer, Message: "internal server error"})
	}
}
