package respons

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type successBody struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

type errorBody struct {
	Success bool   `json:"success"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Success writes a JSON success response.
func Success(c *gin.Context, status int, data any, message string) {
	c.JSON(status, successBody{Success: true, Data: data, Message: message})
}

// Error converts any error into a JSON response. *AppError keeps its own
// status/code; anything else becomes a generic 500 (details are not leaked).
func Error(c *gin.Context, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		c.AbortWithStatusJSON(appErr.Status, errorBody{Code: appErr.Code, Message: appErr.Message})
		return
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, errorBody{
		Code:    http.StatusInternalServerError,
		Message: "internal server error, please try again later",
	})
}

// BadRequest writes a 400 response (e.g. for request binding/validation errors).
func BadRequest(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusBadRequest, errorBody{Code: http.StatusBadRequest, Message: message})
}
