package service

import (
	"context"
	"errors"

	"github.com/najm-x/nova/database/entities"
	"github.com/najm-x/nova/modules/user/dto"
	"github.com/najm-x/nova/modules/user/repo"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type (
	UserService interface {
		CreateUser(c context.Context, req dto.CreateUserRequest) (dto.UserResponse, error)
		UpdateUser(c context.Context, id string, req dto.UpdateUserRequest) (dto.UserResponse, error)
		GetUserById(c context.Context, id string) (dto.UserResponse, error)
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

// 1. إنشاء مستخدم جديد
func (s *userService) CreateUser(c context.Context, req dto.CreateUserRequest) (dto.UserResponse, error) {
	// التحقق من عدم إرسال Provider و Consumer في نفس الوقت
	if req.Provider != nil && req.Consumer != nil {
		return dto.UserResponse{}, dto.ErrInvalidProfileType
	}

	// التحقق من عدم وجود رقم الهاتف مسبقاً
	if _, err := s.userRepo.GetUserByPhone(c, req.Phone); err == nil {
		return dto.UserResponse{}, dto.ErrPhoneAlreadyExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.UserResponse{}, err
	}

	// تشفير كلمة المرور
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return dto.UserResponse{}, err
	}

	user := &entities.User{
		Phone:    req.Phone,
		Password: string(hashedPassword),
		RoleID:   req.RoleID,
		IsActive: true, // يمكنك جعلها false إذا كان التفعيل يتطلب OTP أولاً
	}

	// ربط بيانات المزود (Provider) إن وجدت
	if req.Provider != nil {
		user.Provider = &entities.Provider{
			Name:        req.Provider.Name,
			Description: req.Provider.Description,
			StartAt:     req.Provider.StartAt,
			EndAt:       req.Provider.EndAt,
		}
	}

	// ربط بيانات المستهلك (Consumer) إن وجدت
	if req.Consumer != nil {
		user.Consumer = &entities.Consumer{
			Name:          req.Consumer.Name,
			IdNumber:      req.Consumer.IdNumber,
			IdImage:       req.Consumer.IdImage,
			IsDriver:      req.Consumer.IsDriver,
			VehicleNumber: req.Consumer.VehicleNumber,
			VehicleType:   req.Consumer.VehicleType,
			IsHome:        req.Consumer.IsHome,
			FamilyCount:   req.Consumer.FamilyCount,
			IsBusiness:    req.Consumer.IsBusiness,
			BusinessName:  req.Consumer.BusinessName,
			BusinessFile:  req.Consumer.BusinessFile,
		}
	}

	// حفظ المستخدم في قاعدة البيانات (GORM ينشئ العلاقات تلقائياً داخل Transaction)
	if err := s.userRepo.CreateUser(c, user); err != nil {
		return dto.UserResponse{}, err
	}

	return dto.ToUserResponse(user), nil
}

// 2. تحديث بيانات المستخدم
func (s *userService) UpdateUser(c context.Context, id string, req dto.UpdateUserRequest) (dto.UserResponse, error) {
	// جلب المستخدم أولاً للتأكد من وجوده
	existingUser, err := s.userRepo.GetUserById(c, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.UserResponse{}, dto.ErrUserNotFound
		}
		return dto.UserResponse{}, err
	}

	// تحديث الحقول التي تم إرسالها فقط
	if req.Phone != nil && *req.Phone != existingUser.Phone {
		// التأكد أن الرقم الجديد غير مستخدم من شخص آخر
		if _, err := s.userRepo.GetUserByPhone(c, *req.Phone); err == nil {
			return dto.UserResponse{}, dto.ErrPhoneAlreadyExists
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

	// تحديث بيانات الـ Provider إن وجدت
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

	if err := s.userRepo.UpdateUser(c, id, &existingUser); err != nil {
		return dto.UserResponse{}, err
	}

	return dto.ToUserResponse(&existingUser), nil
}

func (s *userService) GetUserById(c context.Context, id string) (dto.UserResponse, error) {
	user, err := s.userRepo.GetUserById(c, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.UserResponse{}, dto.ErrUserNotFound
		}
		return dto.UserResponse{}, err
	}

	return dto.ToUserResponse(&user), nil
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
		responses[i] = dto.ToUserResponse(&users[i])
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
