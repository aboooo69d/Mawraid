package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	stockDto "github.com/najm-x/nova/modules/stock/dto"
	"github.com/najm-x/nova/modules/user/dto"
	"github.com/najm-x/nova/modules/user/repo"
	"github.com/najm-x/nova/pkg/respons"
	"gorm.io/gorm"
)

type (
	UserService interface {
		UpdateUser(c context.Context, id string, req dto.UpdateUserRequest) (*dto.UserResponseBase, error)
		GetUserById(c context.Context, id string) (*dto.UserResponseBase, error)
		GetAllUsers(c context.Context, page int, pageSize int) ([]dto.UserResponseBase, error)
		DeleteUser(c context.Context, id string) error
		ListUsersByStatus(c context.Context, q dto.ListUsersQuery) ([]dto.UserResponseBase, *respons.Meta, error)
		GetUserDetail(c context.Context, id string) (*dto.UserDetailResponse, error)
		ChangeUserStatus(c context.Context, id string, action string) (*dto.UserResponseBase, error)
		GetUserStock(c context.Context, userId string) (*stockDto.StockResponse, error)
		CreateProviderType(c context.Context, req dto.CreateProviderTypeRequest) (*dto.ProviderTypeResponse, error)
		GetProviderTypes(c context.Context) ([]dto.ProviderTypeResponse, error)
	}

	userService struct {
		userRepo  repo.UserRepo
		stockRepo repo.UserStockRepo
	}
)

func NewUserService(userRepo repo.UserRepo, stockRepo repo.UserStockRepo) UserService {
	return &userService{
		userRepo:  userRepo,
		stockRepo: stockRepo,
	}
}

func (s *userService) UpdateUser(c context.Context, id string, req dto.UpdateUserRequest) (*dto.UserResponseBase, error) {

	existingUser, err := s.userRepo.GetUserById(c, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, dto.ErrUserNotFound
		}
		return nil, err
	}

	if req.Phone != "" && req.Phone != existingUser.Phone {

		if _, err := s.userRepo.GetUserByPhone(c, req.Phone); err == nil {
			return nil, dto.ErrPhoneAlreadyExists
		}
		existingUser.Phone = req.Phone
	}

	if req.IsVerified != nil {
		existingUser.IsVerified = *req.IsVerified
	}

	if req.RoleID != nil {
		existingUser.RoleID = *req.RoleID
	}

	if req.Provider != nil && existingUser.Provider != nil {
		if req.Provider.Name != nil {
			existingUser.Provider.Name = *req.Provider.Name
		}
		if req.Provider.Description != nil {
			existingUser.Provider.Description = *req.Provider.Description
		}
		if req.Provider.StartAt != nil {
			existingUser.Provider.StartAt = *req.Provider.StartAt
		}
		if req.Provider.EndAt != nil {
			existingUser.Provider.EndAt = *req.Provider.EndAt
		}
		if req.Provider.ProviderTypeID != nil {
			pt, err := s.userRepo.GetProviderTypeById(c, *req.Provider.ProviderTypeID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, dto.ErrProviderTypeNotFound
				}
				return nil, err
			}
			existingUser.Provider.ProviderTypeID = pt.ID
			existingUser.Provider.ProviderType = *pt
		}
	}

	if req.Consumer != nil && existingUser.Consumer != nil {
		if req.Consumer.Name != nil {
			existingUser.Consumer.Name = *req.Consumer.Name
		}
		if req.Consumer.IdNumber != nil {
			existingUser.Consumer.IdNumber = *req.Consumer.IdNumber
		}
		if req.Consumer.IdImage != nil {
			existingUser.Consumer.IdImage = *req.Consumer.IdImage
		}
		if req.Consumer.IsDriver != nil {
			existingUser.Consumer.IsDriver = *req.Consumer.IsDriver
		}
		if req.Consumer.VehicleNumber != nil {
			existingUser.Consumer.VehicleNumber = req.Consumer.VehicleNumber
		}
		if req.Consumer.VehicleType != nil {
			existingUser.Consumer.VehicleType = req.Consumer.VehicleType
		}
		if req.Consumer.IsHome != nil {
			existingUser.Consumer.IsHome = *req.Consumer.IsHome
		}
		if req.Consumer.FamilyCount != nil {
			existingUser.Consumer.FamilyCount = req.Consumer.FamilyCount
		}
		if req.Consumer.IsBusiness != nil {
			existingUser.Consumer.IsBusiness = *req.Consumer.IsBusiness
		}
		if req.Consumer.BusinessName != nil {
			existingUser.Consumer.BusinessName = req.Consumer.BusinessName
		}
		if req.Consumer.BusinessFile != nil {
			existingUser.Consumer.BusinessFile = req.Consumer.BusinessFile
		}
	}

	if err := s.userRepo.UpdateUser(c, id, existingUser); err != nil {
		return nil, err
	}

	return dto.ToUserResponse(existingUser), nil
}

func (s *userService) GetUserById(c context.Context, id string) (*dto.UserResponseBase, error) {
	user, err := s.userRepo.GetUserById(c, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, dto.ErrUserNotFound
		}
		return nil, err
	}

	return dto.ToUserResponse(user), nil
}

func (s *userService) GetAllUsers(c context.Context, page int, pageSize int) ([]dto.UserResponseBase, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	users, err := s.userRepo.GetAllUsers(c, pageSize, offset)
	if err != nil {
		return nil, err
	}

	responses := make([]dto.UserResponseBase, len(users))
	for i := range users {
		responses[i] = *dto.ToUserResponse(&users[i])
	}

	return responses, nil
}

func (s *userService) DeleteUser(c context.Context, id string) error {
	err := s.userRepo.DeleteUser(c, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.ErrUserNotFound
		}
		return err
	}
	return nil
}

func (s *userService) GetUserStock(c context.Context, userId string) (*stockDto.StockResponse, error) {
	if _, err := uuid.Parse(userId); err != nil {
		return nil, respons.ErrBadRequest
	}

	stock, err := s.stockRepo.GetStockByUserId(c, userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, stockDto.ErrStockNotFound
		}
		return nil, err
	}

	res := stockDto.ToStockResponse(stock)
	return &res, nil
}

// ListUsersByStatus returns a page of users filtered by status (default: pending = new registrations).
func (s *userService) ListUsersByStatus(c context.Context, q dto.ListUsersQuery) ([]dto.UserResponseBase, *respons.Meta, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PerPage < 1 || q.PerPage > 100 {
		q.PerPage = 10
	}
	status := entities.UserStatus(q.Status)
	if status == "" {
		status = entities.StatusPending
	}

	users, total, err := s.userRepo.ListUsersByStatus(c, status, q.PerPage, (q.Page-1)*q.PerPage)
	if err != nil {
		return nil, nil, err
	}

	items := make([]dto.UserResponseBase, len(users))
	for i := range users {
		items[i] = *dto.ToUserResponse(&users[i])
	}
	meta := &respons.Meta{
		Page:       q.Page,
		PerPage:    q.PerPage,
		Total:      int(total),
		TotalPages: int((total + int64(q.PerPage) - 1) / int64(q.PerPage)),
	}
	return items, meta, nil
}

func (s *userService) GetUserDetail(c context.Context, id string) (*dto.UserDetailResponse, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, respons.ErrBadRequest
	}
	user, err := s.userRepo.GetUserById(c, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, dto.ErrUserNotFound
		}
		return nil, err
	}
	return dto.ToUserDetailResponse(user), nil
}

// ChangeUserStatus applies an admin action: accept -> active, block -> rejected.
func (s *userService) ChangeUserStatus(c context.Context, id string, action string) (*dto.UserResponseBase, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, respons.ErrBadRequest
	}

	var newStatus entities.UserStatus
	switch action {
	case dto.ActionAccept:
		newStatus = entities.StatusActive
	case dto.ActionBlock:
		newStatus = entities.StatusRejected
	default:
		return nil, respons.ErrBadRequest
	}

	user, err := s.userRepo.GetUserById(c, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, dto.ErrUserNotFound
		}
		return nil, err
	}
	if user.Status == newStatus {
		return nil, dto.ErrInvalidStatusChange
	}

	if err := s.userRepo.UpdateUserStatus(c, id, newStatus); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, dto.ErrUserNotFound
		}
		return nil, err
	}
	user.Status = newStatus
	return dto.ToUserResponse(user), nil
}

func (s *userService) CreateProviderType(c context.Context, req dto.CreateProviderTypeRequest) (*dto.ProviderTypeResponse, error) {
	name := strings.TrimSpace(req.Name)
	slug := strings.ToLower(strings.TrimSpace(req.Slug))
	if name == "" || slug == "" {
		return nil, respons.ErrBadRequest
	}

	exists, err := s.userRepo.ProviderTypeExistsByNameOrSlug(c, name, slug)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, dto.ErrProviderTypeAlreadyExists
	}

	pt := &entities.ProviderType{
		Name: name,
		Slug: slug,
	}
	if err := s.userRepo.CreateProviderType(c, pt); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, dto.ErrProviderTypeAlreadyExists
		}
		return nil, err
	}

	res := dto.ToProviderTypeResponse(pt)
	return &res, nil
}

func (s *userService) GetProviderTypes(c context.Context) ([]dto.ProviderTypeResponse, error) {
	providerTypes, err := s.userRepo.GetProviderTypes(c)
	if err != nil {
		return nil, err
	}

	res := make([]dto.ProviderTypeResponse, 0, len(providerTypes))
	for i := range providerTypes {
		res = append(res, dto.ToProviderTypeResponse(&providerTypes[i]))
	}
	return res, nil
}
