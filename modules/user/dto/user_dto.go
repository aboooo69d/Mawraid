package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	errsCode "github.com/najm-x/nova/pkg/constants/errs_code"
	"github.com/najm-x/nova/pkg/respons"
)

var (
	ErrPhoneAlreadyExists = &respons.AppError{
		Status:  404,
		Code:    errsCode.ErrPhoneAlreadyExists,
		Message: "phone number is already registered",
	}
	ErrUserNotFound       = &respons.AppError{Status: 404, Code: errsCode.ErrUserNotFound, Message: "user not found"}
	ErrInvalidProfileType = &respons.AppError{Status: 400, Code: errsCode.ErrInvalidProfileType, Message: "user cannot be both provider and consumer at the same time"}
	ErrPhoneNotFound      = &respons.AppError{Status: 404, Code: errsCode.ErrPhoneNotFound, Message: "this account not found"}
	ErrAcountNotActive    = &respons.AppError{Status: 403, Code: errsCode.ErrAcountNotActive, Message: "account is not active"}
	ErrAcountNotVerify    = &respons.AppError{Status: 403, Code: errsCode.ErrAcountNotVerify, Message: "account is not verified"}
)

type UserBase struct {
	Phone    string `json:"phone" binding:"required,yemen_phone"`
	Password string `json:"password" binding:"required,min=8,max=255"`
}
type CreateUserRequest struct {
	Phone    string                 `json:"phone" binding:"required,min=9,max=20"`
	Password string                 `json:"password" binding:"required,min=8,max=255"`
	Provider *CreateProviderRequest `json:"provider,omitempty"`
	Consumer *CreateConsumerRequest `json:"consumer,omitempty"`
}

type CreateProviderRequest struct {
	UserBase
	Name        string    `json:"name" binding:"required,min=10,max=50"`
	Description string    `json:"description" binding:"max=1000"`
	StartAt     time.Time `json:"start_at" binding:"required"`
	EndAt       time.Time `json:"end_at" binding:"required,gtfield=StartAt"`
}

type CreateConsumerRequest struct {
	UserBase
	Name     string `json:"name" binding:"required,min=10,max=50"`
	IdNumber string `json:"id_number" binding:"required,min=9,max=20"`
	IdImage  string `json:"id_image" binding:"omitempty,max=500"`

	IsDriver      bool    `json:"is_driver"`
	VehicleNumber *string `json:"vehicle_number,omitempty" binding:"required_if=IsDriver true,omitempty,min=2,max=50"`
	VehicleType   *string `json:"vehicle_type,omitempty" binding:"required_if=IsDriver true,omitempty,min=2,max=50"`

	IsHome      bool `json:"is_home"`
	FamilyCount *int `json:"family_count,omitempty" binding:"required_if=IsHome true,omitempty,min=1,max=50"`

	IsBusiness   bool    `json:"is_business"`
	BusinessName *string `json:"business_name,omitempty" binding:"required_if=IsBusiness true,omitempty,min=2,max=200"`
	BusinessFile *string `json:"business_file,omitempty" binding:"required_if=IsBusiness true,omitempty,max=500"`
}
type UserLoginRequest struct {
	Phone    string `json:"phone" binding:"required,min=9,max=20"`
	Password string `json:"password" binding:"required,min=8,max=255"`
}
type UpdateUserRequest struct {
	UserBase
	Phone      *string                `json:"phone,omitempty" binding:"omitempty,min=9,max=20"`
	IsVerified *bool                  `json:"is_verified,omitempty"`
	IsActive   *bool                  `json:"is_active,omitempty"`
	RoleID     *uint                  `json:"role_id,omitempty"`
	Provider   *UpdateProviderRequest `json:"provider,omitempty"`
	Consumer   *UpdateConsumerRequest `json:"consumer,omitempty"`
}

type UpdateProviderRequest struct {
	Name        *string    `json:"name,omitempty" binding:"omitempty,max=250"`
	Description *string    `json:"description,omitempty" binding:"omitempty,max=1000"`
	StartAt     *time.Time `json:"start_at,omitempty"`
	EndAt       *time.Time `json:"end_at,omitempty"`
}

type UpdateConsumerRequest struct {
	Name          *string `json:"name,omitempty" binding:"omitempty,max=250"`
	IdNumber      *string `json:"id_number,omitempty" binding:"omitempty,max=50"`
	IdImage       *string `json:"id_image,omitempty" binding:"omitempty,max=500"`
	IsDriver      *bool   `json:"is_driver,omitempty"`
	VehicleNumber *string `json:"vehicle_number,omitempty" binding:"omitempty,max=50"`
	VehicleType   *string `json:"vehicle_type,omitempty" binding:"omitempty,max=50"`
	IsHome        *bool   `json:"is_home,omitempty"`
	FamilyCount   *int    `json:"family_count,omitempty"`
	IsBusiness    *bool   `json:"is_business,omitempty"`
	BusinessName  *string `json:"business_name,omitempty" binding:"omitempty,max=200"`
	BusinessFile  *string `json:"business_file,omitempty" binding:"omitempty,max=500"`
}

type UserResponseBase struct {
	ID         uuid.UUID `json:"id"`
	Phone      string    `json:"phone"`
	IsVerified bool      `json:"is_verified"`
	IsActive   bool      `json:"is_active"`
	RoleID     uint      `json:"role_id"`
	RoleName   string    `json:"role_name,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ProviderResponse struct {
	UserResponseBase
	Name        string    `json:"name"`
	Description string    `json:"description"`
	StartAt     time.Time `json:"start_at"`
	EndAt       time.Time `json:"end_at"`
}

type ConsumerResponse struct {
	UserResponseBase
	Name          string  `json:"name"`
	IdNumber      string  `json:"id_number"`
	IdImage       string  `json:"id_image,omitempty"`
	IsDriver      bool    `json:"is_driver"`
	VehicleNumber *string `json:"vehicle_number,omitempty"`
	VehicleType   *string `json:"vehicle_type,omitempty"`
	IsHome        bool    `json:"is_home"`
	FamilyCount   *int    `json:"family_count,omitempty"`
	IsBusiness    bool    `json:"is_business"`
	BusinessName  *string `json:"business_name,omitempty"`
	BusinessFile  *string `json:"business_file,omitempty"`
}

func ToUserResponse(user *entities.User) *UserResponseBase {
	return &UserResponseBase{
		ID:         user.ID,
		Phone:      user.Phone,
		IsVerified: user.IsVerified,
		IsActive:   user.IsActive,
		RoleID:     user.RoleID,
		RoleName:   user.Role.Name,
		CreatedAt:  user.CreatedAt,
		UpdatedAt:  user.UpdatedAt,
	}
}

func ToProviderResponse(user *entities.User) *ProviderResponse {
	return &ProviderResponse{
		UserResponseBase: *ToUserResponse(user),
		Name:             user.Provider.Name,
		Description:      user.Provider.Description,
		StartAt:          user.Provider.StartAt,
		EndAt:            user.Provider.EndAt,
	}
}
func ToConsumerResponse(user *entities.User) *ConsumerResponse {
	return &ConsumerResponse{
		UserResponseBase: *ToUserResponse(user),
		Name:             user.Consumer.Name,
		IdNumber:         user.Consumer.IdNumber,
		IdImage:          user.Consumer.IdImage,
		IsDriver:         user.Consumer.IsDriver,
		VehicleNumber:    user.Consumer.VehicleNumber,
		VehicleType:      user.Consumer.VehicleType,
		IsHome:           user.Consumer.IsHome,
		FamilyCount:      user.Consumer.FamilyCount,
		IsBusiness:       user.Consumer.IsBusiness,
		BusinessName:     user.Consumer.BusinessName,
		BusinessFile:     user.Consumer.BusinessFile,
	}
}
