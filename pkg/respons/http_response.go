package respons

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SuccessResponse defines standard success envelope for API docs and responses.
type SuccessResponse struct {
	Success bool   `json:"success" example:"true"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty" example:"operation completed successfully"`
}

// ErrorResponse defines standard error envelope for API docs and responses.
type ErrorResponse struct {
	Success bool   `json:"success" example:"false"`
	Code    int    `json:"code" example:"1001"`
	Message string `json:"message" example:"error description"`
}

// Success writes a JSON success response.
func Success(c *gin.Context, status int, data any, message string) {
	c.JSON(status, SuccessResponse{Success: true, Data: data, Message: message})
}

// Error converts any error into a JSON response. *AppError keeps its own
// status/code; anything else becomes a generic 500 (details are not leaked).
func Error(c *gin.Context, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		c.AbortWithStatusJSON(appErr.Status, ErrorResponse{Success: false, Code: appErr.Code, Message: appErr.Message})
		return
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
		Success: false,
		Code:    http.StatusInternalServerError,
		Message: "internal server error, please try again later",
	})
}

// BadRequest writes a 400 response (e.g. for request binding/validation errors).
func BadRequest(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusBadRequest, ErrorResponse{Success: false, Code: http.StatusBadRequest, Message: message})
}
