package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	"github.com/najm-x/nova/modules/auth/dto"
	userDto "github.com/najm-x/nova/modules/user/dto"
	userRepo "github.com/najm-x/nova/modules/user/repo"
	valueHashing "github.com/najm-x/nova/pkg/value_hashing"
)

type (
	AuthService interface {
		RegisterConsumer(c context.Context, req userDto.CreateUserRequest) (*userDto.UserResponse, error)
		RegisterProvider(c context.Context, req userDto.CreateUserRequest) (*userDto.UserResponse, error)
		Login(c context.Context, req userDto.UserLoginRequest) (*userDto.UserResponse, error)
		VerifyPhone(c context.Context, phone string, code string) error
	}
	authService struct {
		userRepo   userRepo.UserRepo
		otpService OtpService
	}
)

func (s *authService) RegisterConsumer(c context.Context, req userDto.CreateUserRequest) (*userDto.UserResponse, error) {
	if req.Consumer != nil && req.Provider != nil {
		return nil, dto.ErrInvalidProfileType
	}

	if req.Consumer == nil {
		return nil, dto.ErrMissingProfileData
	}

	if _, err := s.userRepo.GetUserByPhone(c, req.Phone); err == nil {
		return nil, dto.ErrPhoneAlreadyExists
	}

	hashedPassword, err := valueHashing.HashValue(req.Password)
	if err != nil {
		return nil, dto.ErrInternalServer
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

	if err := s.userRepo.CreateUser(c, &newUser); err != nil {
		return nil, dto.ErrInternalServer
	}

	go s.otpService.SendOtp(context.Background(), newUser.Phone)

	return userDto.ToUserResponse(&newUser), nil
}

func (s *authService) RegisterProvider(c context.Context, req userDto.CreateUserRequest) (*userDto.UserResponse, error) {

	if req.Provider != nil && req.Consumer != nil {
		return nil, dto.ErrInvalidProfileType
	}

	if req.Provider == nil {
		return nil, dto.ErrMissingProfileData
	}

	if _, err := s.userRepo.GetUserByPhone(c, req.Phone); err == nil {
		return nil, dto.ErrPhoneAlreadyExists
	}

	hashedPassword, err := valueHashing.HashValue(req.Password)
	if err != nil {
		return nil, dto.ErrInternalServer
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

	if err := s.userRepo.CreateUser(c, &newUser); err != nil {
		return nil, dto.ErrInternalServer
	}

	go s.otpService.SendOtp(context.Background(), newUser.Phone)

	return userDto.ToUserResponse(&newUser), nil
}

func (s *authService) Login(c context.Context, req userDto.UserLoginRequest) (*userDto.UserResponse, error) {
	existingUser, err := s.userRepo.GetUserByPhone(c, req.Phone)
	if err != nil {
		return nil, dto.ErrPhoneNotFound
	}

	if !valueHashing.VerifyValue(req.Password, existingUser.Password) {
		return nil, dto.ErrInvalidCredentials
	}

	if !existingUser.IsActive {
		return nil, dto.ErrAcountNotActive
	}

	if !existingUser.IsVerified {
		return nil, dto.ErrAcountNotVerify
	}

	return userDto.ToUserResponse(existingUser), nil
}

func (s *authService) VerifyPhone(c context.Context, phone string, code string) error {
	user, err := s.userRepo.GetUserByPhone(c, phone)
	if err != nil {
		return dto.ErrPhoneNotFound
	}

	if user.IsVerified {
		return dto.ErrAccountAlreadyVerified
	}

	if err := s.otpService.VerifyOtp(c, phone, code); err != nil {
		return dto.ErrInvalidOtpCode
	}

	user.IsVerified = true

	if err := s.userRepo.UpdateUser(c, user.ID.String(), user); err != nil {
		return dto.ErrInternalServer
	}

	return nil
}

func NewAuthService(userRepo userRepo.UserRepo, otpService OtpService) AuthService {
	return &authService{
		userRepo:   userRepo,
		otpService: otpService,
	}
}
