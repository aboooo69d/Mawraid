package service

import (
	"context"
	"errors"

	"github.com/najm-x/nova/modules/user/dto"
	"github.com/najm-x/nova/modules/user/repo"
	"gorm.io/gorm"
)

type (
	UserService interface {
		UpdateUser(c context.Context, id string, req dto.UpdateUserRequest) (*dto.UserResponse, error)
		GetUserById(c context.Context, id string) (*dto.UserResponse, error)
		GetAllUsers(c context.Context, page int, pageSize int) ([]dto.UserResponse, error)
		DeleteUser(c context.Context, id string) error
	}

	userService struct {
		userRepo repo.UserRepo
	}
)

func NewUserService(userRepo repo.UserRepo) UserService {
	return &userService{
		userRepo: userRepo,
	}
}

func (s *userService) UpdateUser(c context.Context, id string, req dto.UpdateUserRequest) (*dto.UserResponse, error) {

	existingUser, err := s.userRepo.GetUserById(c, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, dto.ErrUserNotFound
		}
		return nil, err
	}

	if req.Phone != nil && *req.Phone != existingUser.Phone {

		if _, err := s.userRepo.GetUserByPhone(c, *req.Phone); err == nil {
			return nil, dto.ErrPhoneAlreadyExists
		}
		existingUser.Phone = *req.Phone
	}

	if req.IsVerified != nil {
		existingUser.IsVerified = *req.IsVerified
	}
	if req.IsActive != nil {
		existingUser.IsActive = *req.IsActive
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

func (s *userService) GetUserById(c context.Context, id string) (*dto.UserResponse, error) {
	user, err := s.userRepo.GetUserById(c, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, dto.ErrUserNotFound
		}
		return nil, err
	}

	return dto.ToUserResponse(user), nil
}

func (s *userService) GetAllUsers(c context.Context, page int, pageSize int) ([]dto.UserResponse, error) {
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

	responses := make([]dto.UserResponse, len(users))
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
