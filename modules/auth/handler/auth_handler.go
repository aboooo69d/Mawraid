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

// RegisterConsumer godoc
// @Summary      Register a new consumer
// @Description  Register a new consumer profile with encrypted id_image and optional business_file uploads.
// @Tags         Auth
// @Accept       multipart/form-data
// @Produce      json
// @Param        phone          formData  string  true   "Phone number (min 9, max 20)"
// @Param        password       formData  string  true   "Password (min 8, max 255)"
// @Param        name           formData  string  true   "Consumer full name"
// @Param        id_number      formData  string  true   "National ID or civil number"
// @Param        id_image       formData  file    true   "ID card photo (jpeg/png/webp, max 5MB)"
// @Param        is_driver      formData  bool    false  "Is consumer a driver"
// @Param        vehicle_number formData  string  false  "Vehicle plate number (required if is_driver is true)"
// @Param        vehicle_type   formData  string  false  "Vehicle type (required if is_driver is true)"
// @Param        is_home        formData  bool    false  "Is consumer a home/family head"
// @Param        family_count   formData  int     false  "Family count (>= 1, required if is_home is true)"
// @Param        is_business    formData  bool    false  "Is consumer a business"
// @Param        business_name  formData  string  false  "Business name (required if is_business is true)"
// @Param        business_file  formData  file    false  "Commercial registry or license file (required if is_business is true)"
// @Success      201            {object}  respons.SuccessResponse{data=dto.UserResponse}  "Registered successfully"
// @Failure      400            {object}  respons.ErrorResponse  "Invalid request or validation error"
// @Failure      409            {object}  respons.ErrorResponse  "Phone number already registered"
// @Failure      500            {object}  respons.ErrorResponse  "Internal server error"
// @Router       /auth/register/consumer [post]
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

// RegisterProvider godoc
// @Summary      Register a new provider
// @Description  Register a new service provider profile.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body      dto.CreateUserRequest  true  "Provider registration request payload"
// @Success      201     {object}  respons.SuccessResponse{data=dto.UserResponse}  "Registered successfully"
// @Failure      400     {object}  respons.ErrorResponse  "Invalid request or missing profile data"
// @Failure      409     {object}  respons.ErrorResponse  "Phone number already registered"
// @Failure      500     {object}  respons.ErrorResponse  "Internal server error"
// @Router       /auth/register/provider [post]
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

// Login godoc
// @Summary      Login user
// @Description  Authenticate user with phone and password.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body      dto.UserLoginRequest  true  "Login credentials"
// @Success      200     {object}  respons.SuccessResponse{data=dto.UserResponse}  "Logged in successfully"
// @Failure      400     {object}  respons.ErrorResponse  "Invalid request body"
// @Failure      401     {object}  respons.ErrorResponse  "Invalid credentials"
// @Failure      403     {object}  respons.ErrorResponse  "Account not verified or not active"
// @Failure      404     {object}  respons.ErrorResponse  "Account not found"
// @Router       /auth/login [post]
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

// VerifyPhone godoc
// @Summary      Verify phone with OTP
// @Description  Verifies the phone number using the 6-digit OTP code sent after registration.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body      dto.VerifyPhoneRequest  true  "Phone and 6-digit OTP code"
// @Success      200     {object}  respons.SuccessResponse  "Phone verified successfully"
// @Failure      400     {object}  respons.ErrorResponse  "Invalid request or invalid OTP code"
// @Failure      404     {object}  respons.ErrorResponse  "Account not found"
// @Failure      409     {object}  respons.ErrorResponse  "Account is already verified"
// @Failure      429     {object}  respons.ErrorResponse  "Rate limit exceeded (cooldown / max attempts)"
// @Router       /auth/verify-phone [post]
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
