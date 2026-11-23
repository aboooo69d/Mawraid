package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	errsCode "github.com/najm-x/nova/pkg/constants/errs_code"
	"github.com/najm-x/nova/pkg/respons"
)

var ErrAddressNotFound = &respons.AppError{Status: 404, Code: errsCode.ErrAddressNotFound, Message: "address not found"}

type Location struct {
	Lat *float64 `json:"lat" binding:"required,gte=-90,lte=90"`
	Lng *float64 `json:"lng" binding:"required,gte=-180,lte=180"`
}

type LocationResponse struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// CreateAddressRequest: the first address of a user is always saved as default.
type CreateAddressRequest struct {
	Title       string   `json:"title" binding:"omitempty,max=100"`
	Description string   `json:"description" binding:"required,max=250"`
	Location    Location `json:"location" binding:"required"`
	IsDefault   bool     `json:"is_default"`
}

type AddressResponse struct {
	ID          uuid.UUID        `json:"id"`
	UserID      uuid.UUID        `json:"user_id"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Location    LocationResponse `json:"location"`
	IsDefault   bool             `json:"is_default"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

func ToAddressResponse(a *entities.Address) AddressResponse {
	return AddressResponse{
		ID:          a.ID,
		UserID:      a.UserID,
		Title:       a.Title,
		Description: a.Description,
		Location:    LocationResponse{Lat: a.Location.Lat, Lng: a.Location.Lng},
		IsDefault:   a.IsDefault,
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
	}
}
