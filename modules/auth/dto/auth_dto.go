package dto

import (
	errsCode "github.com/najm-x/nova/pkg/constants/errs_code"
	"github.com/najm-x/nova/pkg/respons"
)

type VerifyPhoneRequest struct {
	Phone string `json:"phone" binding:"required,min=9,max=20"`
	Code  string `json:"code" binding:"required,len=6,numeric"`
}
type ResendOtpRequest struct {
	Phone string `json:"phone" binding:"required,min=9,max=20"`
}

var (
	// 409 Conflict - الرقم مسجل مسبقاً (تم تصحيحها من 404 إلى 409)
	ErrPhoneAlreadyExists = &respons.AppError{
		Status:  409,
		Code:    errsCode.ErrPhoneAlreadyExists,
		Message: "phone number is already registered",
	}

	// 404 Not Found - عدم وجود المستخدم أو الهاتف
	ErrUserNotFound = &respons.AppError{
		Status:  404,
		Code:    errsCode.ErrUserNotFound,
		Message: "user not found",
	}
	ErrPhoneNotFound = &respons.AppError{
		Status:  404,
		Code:    errsCode.ErrPhoneNotFound,
		Message: "this account not found",
	}

	// 400 Bad Request - أخطاء في البيانات المدخلة لنوع الملف الشخصي
	ErrInvalidProfileType = &respons.AppError{
		Status:  400,
		Code:    errsCode.ErrInvalidProfileType,
		Message: "user cannot be both provider and consumer at the same time",
	}
	ErrMissingProfileData = &respons.AppError{
		Status:  400,
		Code:    errsCode.ErrMissingProfileData,
		Message: "profile details are required for this registration type",
	}

	// 401 Unauthorized - بيانات الدخول أو الـ OTP غير صحيحة
	ErrInvalidCredentials = &respons.AppError{
		Status:  401,
		Code:    errsCode.ErrInvalidCredentials,
		Message: "invalid phone number or password",
	}
	ErrInvalidOtpCode = &respons.AppError{
		Status:  400,
		Code:    errsCode.ErrInvalidOtpCode,
		Message: "invalid or expired verification code",
	}

	// 403 Forbidden - حالة الحساب (تم الإبقاء على نفس التسمية الخاصة بك في errsCode)
	ErrAcountNotActive = &respons.AppError{
		Status:  403,
		Code:    errsCode.ErrAcountNotActive,
		Message: "account is not active",
	}
	ErrAcountNotVerify = &respons.AppError{
		Status:  403,
		Code:    errsCode.ErrAcountNotVerify,
		Message: "account is not verified",
	}

	// 409 Conflict - الحساب موثق بالفعل
	ErrAccountAlreadyVerified = &respons.AppError{
		Status:  409,
		Code:    errsCode.ErrAccountAlreadyVerified,
		Message: "account is already verified",
	}

	// 500 Internal Server Error - أخطاء النظام وقاعدة البيانات والتشفير
	ErrInternalServer = &respons.AppError{
		Status:  500,
		Code:    errsCode.ErrInternalServer,
		Message: "internal server error, please try again later",
	}
	ErrAccountPending = &respons.AppError{
		Status:  403,
		Code:    errsCode.ErrAccountPending,
		Message: "Acount is Pending",
	}
	ErrAccountDocs = &respons.AppError{
		Status:  403,
		Code:    errsCode.ErrAccountDocs,
		Message: "Acount is Docs",
	}
	ErrAccountRejected = &respons.AppError{
		Status:  403,
		Code:    errsCode.ErrAccountDocs,
		Message: "Acount is rejected",
	}
	StatusSuspended = &respons.AppError{
		Status:  403,
		Code:    errsCode.ErrStatusSuspended,
		Message: "Acount is rejected",
	}
)

// NewInvalidProfileField builds a 400 error describing which field is missing/invalid.
func NewInvalidProfileField(message string) *respons.AppError {
	return &respons.AppError{
		Status:  400,
		Code:    errsCode.ErrInvalidProfileField,
		Message: message,
	}
}
