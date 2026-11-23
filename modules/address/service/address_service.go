package addressService

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	addressDto "github.com/najm-x/nova/modules/address/dto"
	addressRepo "github.com/najm-x/nova/modules/address/repo"
	"github.com/najm-x/nova/pkg/geo"
	"github.com/najm-x/nova/pkg/respons"
	"gorm.io/gorm"
)

type (
	AddressService interface {
		CreateAddress(c context.Context, userId string, req addressDto.CreateAddressRequest) (*addressDto.AddressResponse, error)
		GetAddress(c context.Context, userId string, addressId string) (*addressDto.AddressResponse, error)
		GetDefaultAddress(c context.Context, userId string) (*addressDto.AddressResponse, error)
		ListAddresses(c context.Context, userId string) ([]addressDto.AddressResponse, error)
		// Admin: no ownership restriction.
		AdminGetAddress(c context.Context, addressId string) (*addressDto.AddressResponse, error)
	}

	addressService struct {
		repo addressRepo.AddressRepo
	}
)

func NewAddressService(repo addressRepo.AddressRepo) AddressService {
	return &addressService{repo: repo}
}

func (s *addressService) CreateAddress(c context.Context, userId string, req addressDto.CreateAddressRequest) (*addressDto.AddressResponse, error) {
	uid, err := uuid.Parse(userId)
	if err != nil {
		return nil, respons.ErrBadRequest
	}
	if req.Location.Lat == nil || req.Location.Lng == nil {
		return nil, respons.ErrBadRequest
	}

	a := &entities.Address{
		UserID:      uid,
		Title:       req.Title,
		Description: req.Description,
		Location:    geo.Point{Lat: *req.Location.Lat, Lng: *req.Location.Lng},
		IsDefault:   req.IsDefault,
	}
	if err := s.repo.Create(c, a); err != nil {
		return nil, err
	}
	res := addressDto.ToAddressResponse(a)
	return &res, nil
}

func (s *addressService) GetAddress(c context.Context, userId string, addressId string) (*addressDto.AddressResponse, error) {
	if _, err := uuid.Parse(addressId); err != nil {
		return nil, addressDto.ErrAddressNotFound
	}
	a, err := s.repo.GetByIdAndUser(c, addressId, userId)
	return mapOne(a, err)
}

func (s *addressService) GetDefaultAddress(c context.Context, userId string) (*addressDto.AddressResponse, error) {
	a, err := s.repo.GetDefaultByUser(c, userId)
	return mapOne(a, err)
}

func (s *addressService) AdminGetAddress(c context.Context, addressId string) (*addressDto.AddressResponse, error) {
	if _, err := uuid.Parse(addressId); err != nil {
		return nil, addressDto.ErrAddressNotFound
	}
	a, err := s.repo.GetById(c, addressId)
	return mapOne(a, err)
}

func (s *addressService) ListAddresses(c context.Context, userId string) ([]addressDto.AddressResponse, error) {
	if _, err := uuid.Parse(userId); err != nil {
		return nil, respons.ErrBadRequest
	}
	items, err := s.repo.ListByUser(c, userId)
	if err != nil {
		return nil, err
	}
	res := make([]addressDto.AddressResponse, 0, len(items))
	for i := range items {
		res = append(res, addressDto.ToAddressResponse(&items[i]))
	}
	return res, nil
}

func mapOne(a *entities.Address, err error) (*addressDto.AddressResponse, error) {
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, addressDto.ErrAddressNotFound
		}
		return nil, err
	}
	res := addressDto.ToAddressResponse(a)
	return &res, nil
}
