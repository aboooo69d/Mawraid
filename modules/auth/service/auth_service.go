package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	"github.com/najm-x/nova/modules/auth/dto"
	roleRepo "github.com/najm-x/nova/modules/role/repo"
	userDto "github.com/najm-x/nova/modules/user/dto"
	userRepo "github.com/najm-x/nova/modules/user/repo"
	smsSender "github.com/najm-x/nova/pkg/sms_sender"
	valueHashing "github.com/najm-x/nova/pkg/value_hashing"
	"github.com/najm-x/nova/utils"
	"github.com/redis/go-redis/v9"
)

type (
	AuthService interface {
		RegisterConsumer(c context.Context, req userDto.CreateConsumerRequest) (*userDto.ConsumerResponse, error)
		RegisterProvider(c context.Context, req userDto.CreateProviderRequest) (*userDto.ProviderResponse, error)
		Login(c context.Context, req userDto.UserLoginRequest) (*userDto.UserResponseBase, error)
		VerifyPhone(c context.Context, phone string, code string) error
		ForgetPassword(c context.Context, phone string) error
		ResetPassword(c context.Context, req dto.RestPasswordRequest) error
	}
	authService struct {
		userRepo   userRepo.UserRepo
		redis      *redis.Client
		roleRepo   roleRepo.RoleRepo
		otpService OtpService
		sms        smsSender.Provider
	}
)

func (s *authService) RegisterConsumer(c context.Context, req userDto.CreateConsumerRequest) (*userDto.ConsumerResponse, error) {

	if _, err := s.userRepo.GetUserByPhone(c, req.Phone); err == nil {
		return nil, dto.ErrPhoneAlreadyExists
	}

	hashedPassword, err := valueHashing.HashValue(req.Password)
	if err != nil {
		return nil, dto.ErrInternalServer
	}

	role, err := s.roleRepo.GetRoleByName(c, roleRepo.RoleConsumer)
	if err != nil {
		return nil, dto.ErrInternalServer
	}

	newUser := entities.User{
		ID:         uuid.New(),
		Phone:      req.Phone,
		Password:   hashedPassword,
		RoleID:     role.ID,
		Role:       *role,
		IsVerified: false,
	}

	newUser.Consumer = &entities.Consumer{
		Name:          req.Name,
		IdNumber:      req.IdNumber,
		IdImage:       req.IdImage,
		IsDriver:      req.IsDriver,
		VehicleNumber: req.VehicleNumber,
		VehicleType:   req.VehicleType,
		IsHome:        req.IsHome,
		FamilyCount:   req.FamilyCount,
		IsBusiness:    req.IsBusiness,
		BusinessName:  req.BusinessName,
		BusinessFile:  req.BusinessFile,
	}

	if err := s.userRepo.CreateUser(c, &newUser); err != nil {
		return nil, dto.ErrInternalServer
	}

	go s.otpService.SendOtp(context.Background(), newUser.Phone)

	return userDto.ToConsumerResponse(&newUser), nil
}

func (s *authService) RegisterProvider(c context.Context, req userDto.CreateProviderRequest) (*userDto.ProviderResponse, error) {

	if _, err := s.userRepo.GetUserByPhone(c, req.Phone); err == nil {
		return nil, dto.ErrPhoneAlreadyExists
	}

	hashedPassword, err := valueHashing.HashValue(req.Password)
	if err != nil {
		return nil, dto.ErrInternalServer
	}

	role, err := s.roleRepo.GetRoleByName(c, roleRepo.RoleProvider)
	if err != nil {
		return nil, dto.ErrInternalServer
	}

	newUser := entities.User{
		ID:         uuid.New(),
		Phone:      req.Phone,
		Password:   hashedPassword,
		RoleID:     role.ID,
		Role:       *role,
		IsVerified: false,
	}

	newUser.Provider = &entities.Provider{
		Name:        req.Name,
		Description: req.Description,
		StartAt:     req.StartAt,
		EndAt:       req.EndAt,
	}

	if err := s.userRepo.CreateUser(c, &newUser); err != nil {
		return nil, dto.ErrInternalServer
	}

	go s.otpService.SendOtp(context.Background(), newUser.Phone)

	return userDto.ToProviderResponse(&newUser), nil
}

func (s *authService) Login(c context.Context, req userDto.UserLoginRequest) (*userDto.UserResponseBase, error) {
	existingUser, err := s.userRepo.GetUserByPhone(c, req.Phone)
	if err != nil {
		return nil, dto.ErrPhoneNotFound
	}

	if !valueHashing.VerifyValue(req.Password, existingUser.Password) {
		return nil, dto.ErrInvalidCredentials
	}

	if !existingUser.IsVerified {
		return nil, dto.ErrAcountNotVerify
	}

	if existingUser.Status == entities.StatusPending {
		return nil, dto.ErrAccountPending
	}

	if existingUser.Status == entities.StatusDocs {
		return nil, dto.ErrAccountDocs
	}

	if existingUser.Status == entities.StatusRejected {
		return nil, dto.ErrAccountRejected
	}

	if existingUser.Status == entities.StatusSuspended {
		return nil, dto.StatusSuspended
	}

	if existingUser.Status != entities.StatusActive {
		return nil, dto.ErrAcountNotActive
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

func (s *authService) ForgetPassword(c context.Context, phone string) error {
	if code, err := s.redis.Get(c, "forget:"+phone).Result(); err == nil {
		go s.sms.SendSMS(phone, "your repassword code is:"+code+"don't share it")
		return nil
	}
	_, err := s.userRepo.GetUserByPhone(c, phone)
	if err != nil {
		return nil
	}
	code, codeErr := utils.GenerateSecureToken()

	if codeErr != nil {
		return dto.ErrInternalServer
	}

	pip := s.redis.Pipeline()
	pip.Set(c, code, phone, 15*time.Minute)
	pip.Set(c, "forget:"+phone, code, 15*time.Minute)

	if _, err := pip.Exec(c); err != nil {
		return dto.ErrInternalServer
	}
	go s.sms.SendSMS(phone, "your repassword code is:"+code+"don't share it")

	return nil

}

func (s *authService) ResetPassword(c context.Context, req dto.RestPasswordRequest) error {
	// 1. Validate reset key
	phone, err := s.redis.Get(c, req.ResetKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return dto.ErrInvalidResetKey
		}

		return err
	}

	user, err := s.userRepo.GetUserById(c, phone)
	if err != nil {
		return dto.ErrUserNotFound
	}

	hashedPassword, err := valueHashing.HashValue(req.Password)
	if err != nil {
		return dto.ErrInternalServer
	}

	user.Password = hashedPassword

	if err := s.userRepo.UpdateUser(
		c,
		user.ID.String(),
		user,
	); err != nil {
		return err
	}

	if err := s.redis.Del(c, req.ResetKey).Err(); err != nil {
		return dto.ErrInternalServer
	}

	return nil
}

func NewAuthService(userRepo userRepo.UserRepo, roleRepo roleRepo.RoleRepo, otpService OtpService, redis *redis.Client, sms smsSender.Provider) AuthService {
	return &authService{
		userRepo:   userRepo,
		roleRepo:   roleRepo,
		otpService: otpService,
		redis:      redis,
		sms:        sms,
	}
}
