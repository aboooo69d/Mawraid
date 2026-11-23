package addressHandler

import (
	"github.com/gin-gonic/gin"
	addressDto "github.com/najm-x/nova/modules/address/dto"
	addressService "github.com/najm-x/nova/modules/address/service"
	"github.com/najm-x/nova/pkg/respons"
)

type AddressHandler struct {
	svc addressService.AddressService
}

func NewAddressHandler(s addressService.AddressService) *AddressHandler {
	return &AddressHandler{svc: s}
}

// CreateAddress godoc
// @Summary      Add address
// @Description  Adds an address for the authenticated customer. The first address is automatically the default.
// @Tags         Address
// @Accept       json
// @Produce      json
// @Param        request  body      addressDto.CreateAddressRequest  true  "Address payload"
// @Success      200      {object}  respons.Response{data=addressDto.AddressResponse}
// @Failure      400      {object}  respons.Response
// @Failure      403      {object}  respons.Response
// @Router       /addresses/me [post]
func (h *AddressHandler) CreateAddress(ctx *gin.Context) {
	var req addressDto.CreateAddressRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		_ = ctx.Error(respons.ErrBadRequest)
		return
	}
	res, err := h.svc.CreateAddress(ctx.Request.Context(), ctx.GetString("user_id"), req)
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	respons.OK(ctx, res)
}

// ListMyAddresses godoc
// @Summary      List my addresses
// @Description  Returns all addresses, default first.
// @Tags         Address
// @Produce      json
// @Success      200  {object}  respons.Response{data=[]addressDto.AddressResponse}
// @Router       /addresses/me [get]
func (h *AddressHandler) ListMyAddresses(ctx *gin.Context) {
	h.list(ctx, ctx.GetString("user_id"))
}

// GetMyAddress godoc
// @Summary      Get my address
// @Tags         Address
// @Produce      json
// @Param        address_id  path  string  true  "Address ID (UUID)"
// @Success      200  {object}  respons.Response{data=addressDto.AddressResponse}
// @Failure      404  {object}  respons.Response
// @Router       /addresses/me/{address_id} [get]
func (h *AddressHandler) GetMyAddress(ctx *gin.Context) {
	res, err := h.svc.GetAddress(ctx.Request.Context(), ctx.GetString("user_id"), ctx.Param("address_id"))
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	respons.OK(ctx, res)
}

// GetMyDefaultAddress godoc
// @Summary      Get my default address
// @Tags         Address
// @Produce      json
// @Success      200  {object}  respons.Response{data=addressDto.AddressResponse}
// @Failure      404  {object}  respons.Response
// @Router       /addresses/me/default [get]
func (h *AddressHandler) GetMyDefaultAddress(ctx *gin.Context) {
	res, err := h.svc.GetDefaultAddress(ctx.Request.Context(), ctx.GetString("user_id"))
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	respons.OK(ctx, res)
}

// AdminListUserAddresses godoc
// @Summary      List a user's addresses (admin)
// @Tags         Admin Address
// @Produce      json
// @Param        user_id    path   string  true   "User ID (UUID)"
// @Success      200  {object}  respons.Response{data=[]addressDto.AddressResponse}
// @Router       /admin/users/{user_id}/addresses [get]
func (h *AddressHandler) AdminListUserAddresses(ctx *gin.Context) {
	h.list(ctx, ctx.Param("user_id"))
}

// AdminGetUserDefaultAddress godoc
// @Summary      Get a user's default address (admin)
// @Tags         Admin Address
// @Produce      json
// @Param        user_id  path  string  true  "User ID (UUID)"
// @Success      200  {object}  respons.Response{data=addressDto.AddressResponse}
// @Failure      404  {object}  respons.Response
// @Router       /admin/users/{user_id}/addresses/default [get]
func (h *AddressHandler) AdminGetUserDefaultAddress(ctx *gin.Context) {
	res, err := h.svc.GetDefaultAddress(ctx.Request.Context(), ctx.Param("user_id"))
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	respons.OK(ctx, res)
}

// AdminGetAddress godoc
// @Summary      Get address by ID (admin)
// @Tags         Admin Address
// @Produce      json
// @Param        address_id  path  string  true  "Address ID (UUID)"
// @Success      200  {object}  respons.Response{data=addressDto.AddressResponse}
// @Failure      404  {object}  respons.Response
// @Router       /admin/addresses/{address_id} [get]
func (h *AddressHandler) AdminGetAddress(ctx *gin.Context) {
	res, err := h.svc.AdminGetAddress(ctx.Request.Context(), ctx.Param("address_id"))
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	respons.OK(ctx, res)
}

func (h *AddressHandler) list(ctx *gin.Context, userId string) {
	items, err := h.svc.ListAddresses(ctx.Request.Context(), userId)
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	respons.OK(ctx, items)
}
