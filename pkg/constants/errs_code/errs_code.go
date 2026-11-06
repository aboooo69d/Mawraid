package dto

const (
	// User Errors: 1000-1099
	ErrUserNotFound       = 1000
	ErrPhoneAlreadyExists = 1001
	ErrPhoneNotFound      = 1002
)

const (
	// Authentication Errors: 1100-1199
	ErrInvalidCredentials     = 1100
	ErrAcountNotActive        = 1101
	ErrAcountNotVerify        = 1102
	ErrAccountAlreadyVerified = 1103
	ErrInvalidResetKey        = 1104
)

const (
	// OTP Errors: 1200-1299
	ErrInvalidOtpCode     = 1200
	ErrCooldownActive     = 1201
	ErrDailyLimitReached  = 1202
	ErrVerifyLimitReached = 1203
	ErrInvalidOtp         = 1204
)

const (
	// Profile Errors: 1300-1399
	ErrInvalidProfileType  = 1300
	ErrMissingProfileData  = 1301
	ErrInvalidProfileField = 1302
)

const (
	// File Errors: 1400-1499
	ErrFileTooLarge    = 1400
	ErrFileTypeInvalid = 1401
	ErrFileEmpty       = 1402
	ErrAccountDocs     = 1403
)

const (
	// Account Status Errors: 1500-1599
	ErrAccountPending  = 1500
	ErrAccountRejected = 1501
	ErrStatusSuspended = 1502
)

const (
	// Server Errors: 5000-5099
	ErrInternalServer = 5000
)
