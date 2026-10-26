package errsCode

const (
	_ = 1000 + iota
	ErrUserNotFound
	ErrPhoneAlreadyExists
	ErrPhoneNotFound
	ErrInvalidProfileType
	ErrAcountNotActive
	ErrMissingProfileData
	ErrInvalidCredentials
	ErrInvalidOtpCode
	ErrAcountNotVerify
	ErrAccountAlreadyVerified
	ErrInternalServer
	// أخطاء الـ OTP المضافة حديثاً
	ErrCooldownActive     // 1012
	ErrDailyLimitReached  // 1013
	ErrVerifyLimitReached // 1014
	ErrInvalidOtp
	ErrInvalidProfileField
)
