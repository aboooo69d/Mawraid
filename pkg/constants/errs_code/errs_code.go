package errs_code

const (
	// User Errors: 1000-1099
	ErrUserNotFound              = 1000
	ErrPhoneAlreadyExists        = 1001
	ErrPhoneNotFound             = 1002
	ErrProviderTypeNotFound      = 1003
	ErrProviderTypeAlreadyExists = 1004
)

const (
	// Authentication Errors: 1100-1199
	ErrInvalidCredentials     = 1100
	ErrAcountNotActive        = 1101
	ErrAcountNotVerify        = 1102
	ErrAccountAlreadyVerified = 1103
	ErrInvalidResetKey        = 1104
	ErrTurnstileToken         = 1105
	ErrUnauthorized           = 1106
	ErrForbidden              = 1107
)

const (
	// OTP Errors: 1200-1299
	ErrInvalidOtpCode     = 1200
	ErrCooldownActive     = 1201
	ErrDailyLimitReached  = 1202
	ErrVerifyLimitReached = 1203
	ErrInvalidOtp         = 1204
	ErrToManyRequest      = 1205
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
	// Stock Errors: 1600-1699
	ErrStockNotFound       = 1600
	ErrUnitNotInStock      = 1601
	ErrInsufficientStock   = 1602
	ErrInvalidAmount       = 1603
	ErrStockTypeNotFound   = 1604
	ErrUnitNotFound        = 1605
	ErrMainUnitRequired    = 1606
	ErrMainUnitRate        = 1607
	ErrMainUnitExists      = 1608
	ErrDuplicateUnitName   = 1609
	ErrUnitHasTransactions = 1610
	ErrStockAlreadyExists  = 1611
	ErrStockTypeMismatch   = 1612
	ErrInvalidStockDetails = 1613
	ErrStockTypeNotAllowed = 1614
	ErrProviderRequired    = 1615
)

const (
	// Address Errors: 1700-1799
	ErrAddressNotFound = 1700
)

const (
	// Server Errors: 5000-5099
	ErrInternalServer = 5000
)
