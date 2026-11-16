package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/najm-x/nova/modules/user/dto"
	"github.com/najm-x/nova/modules/user/service"
	"github.com/najm-x/nova/pkg/respons"
)

type UserHandler struct {
	userService service.UserService
}

func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

// GetUserStock godoc
// @Summary      Get current user stock
// @Description  Returns the stock details, units, and available balances for the authenticated user.
// @Tags         User
// @Produce      json
// @Success      200  {object}  respons.Response{data=stockDto.StockResponse}  "User stock retrieved successfully"
// @Failure      400  {object}  respons.Response  "Invalid user ID"
// @Failure      404  {object}  respons.Response  "Stock not found"
// @Failure      500  {object}  respons.Response  "Internal server error"
// @Router       /user/me/stock [get]
func (h *UserHandler) GetUserStock(ctx *gin.Context) {
	rawUserId, exists := ctx.Get("user_id")
	if !exists || rawUserId == nil {
		_ = ctx.Error(dto.ErrUserNotFound)
		return
	}

	var userId string
	switch v := rawUserId.(type) {
	case string:
		userId = v
	case uuid.UUID:
		userId = v.String()
	default:
		userId = fmt.Sprint(v)
	}

	if userId == "" {
		_ = ctx.Error(dto.ErrUserNotFound)
		return
	}

	stock, err := h.userService.GetUserStock(ctx.Request.Context(), userId)
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	respons.OK(ctx, stock)
}
