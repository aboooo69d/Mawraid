package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/modules/auth/dto"
	"github.com/najm-x/nova/modules/auth/service"
	userDto "github.com/najm-x/nova/modules/user/dto"
	"github.com/najm-x/nova/pkg/respons"
	"github.com/najm-x/nova/pkg/storage"
)

type AuthHandler struct {
	authService service.AuthService
	uploader    *storage.SecureUploader
}

func NewAuthHandler(authService service.AuthService, uploader *storage.SecureUploader) *AuthHandler {
	return &AuthHandler{authService: authService, uploader: uploader}
}

// RegisterConsumerForm is the multipart/form-data body of consumer registration.
// Files are read separately: id_image (required) and business_file (optional).
type RegisterConsumerForm struct {
	Phone    string `form:"phone" binding:"required,min=9,max=20"`
	Password string `form:"password" binding:"required,min=8,max=255"`
	Name     string `form:"name" binding:"required,max=250"`
	IdNumber string `form:"id_number" binding:"required,max=50"`

	IsDriver      bool    `form:"is_driver"`
	VehicleNumber *string `form:"vehicle_number" binding:"omitempty,max=50"`
	VehicleType   *string `form:"vehicle_type" binding:"omitempty,max=50"`

	IsHome      bool  `form:"is_home"`
	FamilyCount *int8 `form:"family_count" binding:"omitempty,min=1"`

	IsBusiness   bool    `form:"is_business"`
	BusinessName *string `form:"business_name" binding:"omitempty,max=200"`
}

func (h *AuthHandler) RegisterConsumer(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2*storage.MaxFileSize+(1<<20))

	var form RegisterConsumerForm
	if err := c.ShouldBind(&form); err != nil {
		respons.BadRequest(c, err.Error())
		return
	}

	idImage, err := c.FormFile("id_image")
	if err != nil {
		respons.BadRequest(c, "id_image file is required")
		return
	}
	businessFile, _ := c.FormFile("business_file")
	if !form.IsBusiness {
		businessFile = nil // not a business account: ignore any sent file
	}

	ctx := c.Request.Context()
	var uploaded []string
	cleanup := func() {
		for _, key := range uploaded {
			_ = h.uploader.Delete(context.Background(), key)
		}
	}

	idImageKey, err := h.uploader.Upload(ctx, "consumers/id", idImage, storage.ImageTypes)
	if err != nil {
		handleUploadError(c, err)
		return
	}
	uploaded = append(uploaded, idImageKey)

	consumer := &userDto.CreateConsumerRequest{
		Name:          form.Name,
		IdNumber:      form.IdNumber,
		IdImage:       idImageKey,
		IsDriver:      form.IsDriver,
		VehicleNumber: form.VehicleNumber,
		VehicleType:   form.VehicleType,
		IsHome:        form.IsHome,
		FamilyCount:   form.FamilyCount,
		IsBusiness:    form.IsBusiness,
		BusinessName:  form.BusinessName,
	}

	if businessFile != nil {
		key, err := h.uploader.Upload(ctx, "consumers/business", businessFile, storage.ImageAndPDF)
		if err != nil {
			cleanup()
			handleUploadError(c, err)
			return
		}
		uploaded = append(uploaded, key)
		consumer.BusinessFile = &key
	}

	user, err := h.authService.RegisterConsumer(ctx, userDto.CreateUserRequest{
		Phone:    form.Phone,
		Password: form.Password,
		Consumer: consumer,
	})
	if err != nil {
		cleanup() // do not leave orphan files if registration failed
		respons.Error(c, err)
		return
	}
	respons.Success(c, http.StatusCreated, user, "registered successfully, verification code sent")
}

func handleUploadError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, storage.ErrFileTooLarge),
		errors.Is(err, storage.ErrFileTypeInvalid),
		errors.Is(err, storage.ErrFileEmpty):
		respons.BadRequest(c, err.Error())
	default:
		respons.Error(c, err) // generic 500, details not leaked
	}
}

func (h *AuthHandler) RegisterProvider(c *gin.Context) {
	var req userDto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respons.BadRequest(c, err.Error())
		return
	}

	user, err := h.authService.RegisterProvider(c.Request.Context(), req)
	if err != nil {
		respons.Error(c, err)
		return
	}
	respons.Success(c, http.StatusCreated, user, "registered successfully, verification code sent")
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req userDto.UserLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respons.BadRequest(c, err.Error())
		return
	}

	user, err := h.authService.Login(c.Request.Context(), req)
	if err != nil {
		respons.Error(c, err)
		return
	}
	respons.Success(c, http.StatusOK, user, "logged in successfully")
}

func (h *AuthHandler) VerifyPhone(c *gin.Context) {
	var req dto.VerifyPhoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respons.BadRequest(c, err.Error())
		return
	}

	if err := h.authService.VerifyPhone(c.Request.Context(), req.Phone, req.Code); err != nil {
		respons.Error(c, err)
		return
	}
	respons.Success(c, http.StatusOK, nil, "phone verified successfully")
}
