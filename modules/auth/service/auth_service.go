package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	"github.com/najm-x/nova/modules/user/dto"
	userDto "github.com/najm-x/nova/modules/user/dto"
	userRepo "github.com/najm-x/nova/modules/user/repo"
	passwordHashing "github.com/najm-x/nova/pkg/password_hashing"
	smsSender "github.com/najm-x/nova/pkg/sms_sender"
)

type (
	AuthService interface {
		RegisterConsumer(c context.Context, req userDto.CreateUserRequest) (*userDto.UserResponse, error)
		RegisterProvider(c context.Context, req userDto.CreateUserRequest) (*userDto.UserResponse, error)
		Login(c context.Context, req dto.UserLoginRequest) (*userDto.UserResponse, error)
		VerifyPhone(c context.Context, phone string)
	}
	authService struct {
		userRepo userRepo.UserRepo
	}
)

func (s *authService) RegisterConsumer(c context.Context, req userDto.CreateUserRequest) (*userDto.UserResponse, error) {

	if req.Consumer == nil {
		return nil, dto.ErrInvalidProfileType
	}

	if _, err := s.userRepo.GetUserByPhone(c, req.Phone); err == nil {
		return nil, dto.ErrPhoneAlreadyExists
	}
	hashedPassword, err := passwordHashing.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	newUser := entities.User{
		ID:         uuid.New(),
		Phone:      req.Phone,
		Password:   hashedPassword,
		IsVerified: false,
	}

	newUser.Consumer = &entities.Consumer{
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

	err = s.userRepo.CreateUser(c, &newUser)
	if err != nil {
		return nil, err
	}
	otpSender := smsSender.NewSmsService()
	otpSender.SendOTP(req.Phone)

	return userDto.ToUserResponse(&newUser), nil
}
func (s *authService) RegisterProvider(c context.Context, req userDto.CreateUserRequest) (*userDto.UserResponse, error) {

	if req.Provider == nil {
		return nil, dto.ErrInvalidProfileType
	}

	if _, err := s.userRepo.GetUserByPhone(c, req.Phone); err == nil {
		return nil, dto.ErrPhoneAlreadyExists
	}
	hashedPassword, err := passwordHashing.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	newUser := entities.User{
		ID:         uuid.New(),
		Phone:      req.Phone,
		Password:   hashedPassword,
		IsVerified: false,
	}

	newUser.Provider = &entities.Provider{
		Name:        req.Provider.Name,
		Description: req.Provider.Description,
		StartAt:     req.Provider.StartAt,
		EndAt:       req.Provider.EndAt,
	}
	err = s.userRepo.CreateUser(c, &newUser)
	if err != nil {
		return nil, err
	}
	otpSender := smsSender.NewSmsService()
	otpSender.SendOTP(req.Phone)

	return userDto.ToUserResponse(&newUser), nil
}

// Login implements [AuthService].
func (s *authService) Login(c context.Context, req userDto.UserLoginRequest) (*userDto.UserResponse, error) {
	existingUser, err := s.userRepo.GetUserByPhone(c, req.Phone)
	if err != nil {
		return nil, dto.ErrPhoneNotFound
	}
	if !(passwordHashing.VerifyPassword(req.Password, existingUser.Password)) {
		return nil, dto.ErrPhoneNotFound
	}
	if !existingUser.IsActive {
		return nil, dto.ErrAcountNotActive
	}
	if !existingUser.IsVerified {
		return nil, dto.ErrAcountNotVerify
	}
	return userDto.ToUserResponse(existingUser), nil
}

// RegisterConsumer implements [AuthService].

// VerifyPhone implements [AuthService].
func (a *authService) VerifyPhone(c context.Context, phone string) {
	panic("unimplemented")
}

func NewAuthRepo(userRepo userRepo.UserRepo) AuthService {
	return &authService{
		userRepo: userRepo,
	}
}
