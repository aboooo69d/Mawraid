package smsSender

import (
	"fmt"
	"math/rand"
)

type SmsService interface {
	SendSMS(to string, msg string) error
	SendOTP(to string) error
}

type smsService struct {
	BaseUrl string
	ApiKey  string
}

func NewSmsService() SmsService {
	return &smsService{
		BaseUrl: "",
		ApiKey:  "",
	}
}

func (s *smsService) SendSMS(to string, msg string) error {
	return nil
}

func (s *smsService) SendOTP(to string) error {
	otp := s.GenerateOtp(6)
	s.SendSMS(to, fmt.Sprintf("your otp is %d", otp))
	return nil
}

func (s *smsService) GenerateOtp(digits int) int {
	max := 10 ^ (digits + 1)
	min := 10 ^ digits
	rand := rand.Intn(max-min) + min
	return rand

}
