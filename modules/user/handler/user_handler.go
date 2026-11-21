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

// ListUsers godoc
// @Summary      List users by status (admin)
// @Description  Paginated list of users. Defaults to status=pending (new registrations).
// @Tags         Admin Users
// @Produce      json
// @Param        page      query  int     false  "Page number (default 1)"
// @Param        per_page  query  int     false  "Items per page (default 10, max 100)"
// @Param        status    query  string  false  "pending|docs|active|rejected|suspended (default pending)"
// @Success      200  {object}  respons.Response{data=[]dto.UserResponseBase}
// @Router       /admin/users [get]
func (h *UserHandler) ListUsers(ctx *gin.Context) {
	var q dto.ListUsersQuery
	if err := ctx.ShouldBindQuery(&q); err != nil {
		_ = ctx.Error(err)
		return
	}
	users, meta, err := h.userService.ListUsersByStatus(ctx.Request.Context(), q)
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	respons.OKWithMeta(ctx, users, meta)
}

// GetUser godoc
// @Summary      Get registered user info (admin)
// @Tags         Admin Users
// @Produce      json
// @Param        id   path  string  true  "User ID (UUID)"
// @Success      200  {object}  respons.Response{data=dto.UserDetailResponse}
// @Failure      404  {object}  respons.Response
// @Router       /admin/users/{id} [get]
func (h *UserHandler) GetUser(ctx *gin.Context) {
	user, err := h.userService.GetUserDetail(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	respons.OK(ctx, user)
}

// ChangeUserStatus godoc
// @Summary      Accept or block a user (admin)
// @Description  action=accept sets status to active; action=block sets status to rejected.
// @Tags         Admin Users
// @Accept       json
// @Produce      json
// @Param        id    path  string                       true  "User ID (UUID)"
// @Param        body  body  dto.ChangeUserStatusRequest  true  "Action"
// @Success      200  {object}  respons.Response{data=dto.UserResponseBase}
// @Failure      404  {object}  respons.Response
// @Failure      409  {object}  respons.Response
// @Router       /admin/users/{id}/status [patch]
func (h *UserHandler) ChangeUserStatus(ctx *gin.Context) {
	var req dto.ChangeUserStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		_ = ctx.Error(err)
		return
	}
	user, err := h.userService.ChangeUserStatus(ctx.Request.Context(), ctx.Param("id"), req.Action)
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	respons.OK(ctx, user)
}

// CreateProviderType godoc
// @Summary      Create a provider type (admin)
// @Description  Creates a new provider category/type with a unique name and slug.
// @Tags         Admin Provider Types
// @Accept       json
// @Produce      json
// @Param        request  body      dto.CreateProviderTypeRequest  true  "Provider type payload"
// @Success      200      {object}  respons.Response{data=dto.ProviderTypeResponse}
// @Failure      400      {object}  respons.Response  "Invalid request body"
// @Failure      409      {object}  respons.Response  "Provider type already exists"
// @Failure      500      {object}  respons.Response  "Internal server error"
// @Router       /admin/provider-types [post]
func (h *UserHandler) CreateProviderType(ctx *gin.Context) {
	var req dto.CreateProviderTypeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		_ = ctx.Error(respons.ErrBadRequest)
		return
	}

	pt, err := h.userService.CreateProviderType(ctx.Request.Context(), req)
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	respons.OK(ctx, pt)
}

// ListProviderTypes godoc
// @Summary      List provider types
// @Description  Returns all available provider types.
// @Tags         Provider Types
// @Produce      json
// @Success      200  {object}  respons.Response{data=[]dto.ProviderTypeResponse}
// @Failure      500  {object}  respons.Response  "Internal server error"
// @Router       /provider-types [get]
func (h *UserHandler) ListProviderTypes(ctx *gin.Context) {
	types, err := h.userService.GetProviderTypes(ctx.Request.Context())
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	respons.OK(ctx, types)
}
