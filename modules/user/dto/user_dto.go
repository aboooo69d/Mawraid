package dto

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	errsCode "github.com/najm-x/nova/pkg/constants/errs_code"
	"github.com/najm-x/nova/pkg/respons"
)

var (
	ErrPhoneAlreadyExists = &respons.AppError{Status: 404, Code: errsCode.ErrPhoneAlreadyExists, Message: "phone number is already registered"}
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidProfileType = errors.New("user cannot be both provider and consumer at the same time")
	ErrSendSMS            = errors.New("we can't send OTP to this number")
	ErrPhoneNotFound      = errors.New("this account not found")
	ErrAcountNotActive    = errors.New("")
	ErrAcountNotVerify    = errors.New("")
)

type CreateUserRequest struct {
	Phone    string                 `json:"phone" binding:"required,min=9,max=20"`
	Password string                 `json:"password" binding:"required,min=8,max=255"`
	RoleID   uint                   `json:"role_id" binding:"required"`
	Provider *CreateProviderRequest `json:"provider,omitempty"`
	Consumer *CreateConsumerRequest `json:"consumer,omitempty"`
}

type CreateProviderRequest struct {
	Name        string    `json:"name" binding:"required,max=250"`
	Description string    `json:"description" binding:"max=1000"`
	StartAt     time.Time `json:"start_at" binding:"required"`
	EndAt       time.Time `json:"end_at" binding:"required,gtfield=StartAt"`
}

type CreateConsumerRequest struct {
	Name     string `json:"name" binding:"required,max=250"`
	IdNumber string `json:"id_number" binding:"required,max=50"`
	IdImage  string `json:"id_image" binding:"omitempty,max=500"`

	IsDriver      bool    `json:"is_driver"`
	VehicleNumber *string `json:"vehicle_number,omitempty" binding:"omitempty,max=50"`
	VehicleType   *string `json:"vehicle_type,omitempty" binding:"omitempty,max=50"`

	IsHome      bool  `json:"is_home"`
	FamilyCount *int8 `json:"family_count,omitempty" binding:"omitempty,min=1"`

	IsBusiness   bool    `json:"is_business"`
	BusinessName *string `json:"business_name,omitempty" binding:"omitempty,max=200"`
	BusinessFile *string `json:"business_file,omitempty" binding:"omitempty,max=500"`
}
type UserLoginRequest struct {
	Phone    string `json:"phone" binding:"required,min=9,max=20"`
	Password string `json:"password" binding:"required,min=8,max=255"`
}
type UpdateUserRequest struct {
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
	FamilyCount   *int8   `json:"family_count,omitempty"`
	IsBusiness    *bool   `json:"is_business,omitempty"`
	BusinessName  *string `json:"business_name,omitempty" binding:"omitempty,max=200"`
	BusinessFile  *string `json:"business_file,omitempty" binding:"omitempty,max=500"`
}

type UserResponse struct {
	ID         uuid.UUID         `json:"id"`
	Phone      string            `json:"phone"`
	IsVerified bool              `json:"is_verified"`
	IsActive   bool              `json:"is_active"`
	RoleID     uint              `json:"role_id"`
	RoleName   string            `json:"role_name,omitempty"`
	Provider   *ProviderResponse `json:"provider,omitempty"`
	Consumer   *ConsumerResponse `json:"consumer,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

type ProviderResponse struct {
	UserID      uuid.UUID `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	StartAt     time.Time `json:"start_at"`
	EndAt       time.Time `json:"end_at"`
}

type ConsumerResponse struct {
	UserID        uuid.UUID `json:"user_id"`
	Name          string    `json:"name"`
	IdNumber      string    `json:"id_number"`
	IdImage       string    `json:"id_image,omitempty"`
	IsDriver      bool      `json:"is_driver"`
	VehicleNumber *string   `json:"vehicle_number,omitempty"`
	VehicleType   *string   `json:"vehicle_type,omitempty"`
	IsHome        bool      `json:"is_home"`
	FamilyCount   *int8     `json:"family_count,omitempty"`
	IsBusiness    bool      `json:"is_business"`
	BusinessName  *string   `json:"business_name,omitempty"`
	BusinessFile  *string   `json:"business_file,omitempty"`
}

func ToUserResponse(user *entities.User) *UserResponse {
	return &UserResponse{
		ID:         user.ID,
		Phone:      user.Phone,
		IsVerified: user.IsVerified,
		IsActive:   user.IsActive,
		RoleID:     user.RoleID,
		RoleName:   user.Role.Name,
		Provider:   ToProviderResponse(user.Provider),
		Consumer:   ToConsumerResponse(user.Consumer),
		CreatedAt:  user.CreatedAt,
		UpdatedAt:  user.UpdatedAt,
	}
}

func ToProviderResponse(user *entities.Provider) *ProviderResponse {
	if user == nil {
		return nil
	}
	return &ProviderResponse{
		UserID:      user.UserID,
		Name:        user.Name,
		Description: user.Description,
		StartAt:     user.StartAt,
		EndAt:       user.EndAt,
	}
}
func ToConsumerResponse(user *entities.Consumer) *ConsumerResponse {
	if user == nil {
		return nil
	}
	return &ConsumerResponse{
		UserID:        user.UserID,
		Name:          user.Name,
		IdNumber:      user.IdNumber,
		IdImage:       user.IdImage,
		IsDriver:      user.IsDriver,
		VehicleNumber: user.VehicleNumber,
		VehicleType:   user.VehicleType,
		IsHome:        user.IsHome,
		FamilyCount:   user.FamilyCount,
		IsBusiness:    user.IsBusiness,
		BusinessName:  user.BusinessName,
		BusinessFile:  user.BusinessFile,
	}
}
