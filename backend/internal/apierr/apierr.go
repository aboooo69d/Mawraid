// Package apierr defines the standard error envelope shared by every API
// failure response:
//
//	{
//	  "error": {
//	    "code":        "VALIDATION_ERROR",
//	    "message":     "English message",
//	    "message_ar":  "رسالة عربية",
//	    "request_id":  "…",
//	    "details":     { … }          // optional, field violations etc.
//	  }
//	}
//
// The machine-readable `code` is stable and documented in api/openapi.yaml.
// `message_ar` satisfies SRS NFR-USA-02 (Arabic server-side validation
// messages); clients may display it directly.
package apierr

import "fmt"

// Error is an API error with an HTTP status and stable code.
type Error struct {
	Status    int    `json:"-"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	MessageAr string `json:"message_ar"`
	Details   any    `json:"details,omitempty"`
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// body is the wire shape of the envelope.
type body struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	MessageAr string `json:"message_ar"`
	RequestID string `json:"request_id,omitempty"`
	Details   any    `json:"details,omitempty"`
}

// Envelope renders the wire envelope.
func (e *Error) Envelope(requestID string) body {
	return body{Code: e.Code, Message: e.Message, MessageAr: e.MessageAr, RequestID: requestID, Details: e.Details}
}

// FieldViolation describes one invalid input field.
type FieldViolation struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Constructors ---------------------------------------------------------------

// New builds an arbitrary error.
func New(status int, code, message, messageAr string) *Error {
	return &Error{Status: status, Code: code, Message: message, MessageAr: messageAr}
}

// WithDetails attaches a details payload.
func (e *Error) WithDetails(d any) *Error {
	c := *e
	c.Details = d
	return &c
}

// BadRequest — 400.
func BadRequest(code, message, messageAr string) *Error {
	return New(400, code, message, messageAr)
}

// Unauthorized — 401.
func Unauthorized(code, message, messageAr string) *Error {
	return New(401, code, message, messageAr)
}

// Forbidden — 403.
func Forbidden(code, message, messageAr string) *Error {
	return New(403, code, message, messageAr)
}

// NotFound — 404.
func NotFound(code, message, messageAr string) *Error {
	return New(404, code, message, messageAr)
}

// Conflict — 409.
func Conflict(code, message, messageAr string) *Error {
	return New(409, code, message, messageAr)
}

// Locked — 423 (account temporarily locked).
func Locked(code, message, messageAr string) *Error {
	return New(423, code, message, messageAr)
}

// TooManyRequests — 429.
func TooManyRequests(code, message, messageAr string) *Error {
	return New(429, code, message, messageAr)
}

// Internal — 500.
func Internal(message, messageAr string) *Error {
	return New(500, "INTERNAL_ERROR", message, messageAr)
}

// Common pre-built errors (codes are part of the API contract) ---------------

// ErrInvalidCredentials — wrong phone/password (identical for both to avoid
// account enumeration; SRS URS 1.3 scenario 3).
func ErrInvalidCredentials() *Error {
	return Unauthorized("INVALID_CREDENTIALS",
		"Phone number or password is incorrect.",
		"رقم الهاتف أو كلمة المرور غير صحيحة.")
}

// ErrAccountLocked — after max failed logins (SRS URS 1.3 scenario 5).
func ErrAccountLocked(minutes int) *Error {
	return Locked("ACCOUNT_LOCKED",
		fmt.Sprintf("Login attempts temporarily blocked. Try again in %d minutes.", minutes),
		fmt.Sprintf("تم إيقاف محاولات الدخول لمدة %d دقائق. يرجى المحاولة لاحقاً.", minutes))
}

// ErrAccountPending — login blocked while pending approval (URS 1.3 scenario 2).
func ErrAccountPending() *Error {
	return Forbidden("ACCOUNT_PENDING_APPROVAL",
		"Your account is still under review. You will be notified once it is activated.",
		"حسابك لا يزال قيد التقييم والمراجعة، سنقوم بإشعارك فور تفعيله.")
}

// ErrAccountRejected — login blocked for rejected accounts.
func ErrAccountRejected() *Error {
	return Forbidden("ACCOUNT_REJECTED",
		"Your account application was rejected. Check the SMS sent to you for the reason.",
		"تم رفض طلب حسابك. يرجى الاطلاع على الرسالة المرسلة لمعرفة السبب.")
}

// ErrAccountSuspended — login blocked for suspended accounts.
func ErrAccountSuspended() *Error {
	return Forbidden("ACCOUNT_SUSPENDED",
		"Your account is suspended. Contact support.",
		"حسابك موقوف. يرجى التواصل مع الدعم.")
}

// ErrDuplicateAccount — same phone or id number already registered
// (SRS FR-CON-02 / URS 1.1 scenario 5).
func ErrDuplicateAccount() *Error {
	return Conflict("ACCOUNT_ALREADY_EXISTS",
		"This phone number or ID number is already registered. Please sign in or reset your password.",
		"رقم الهاتف أو الهوية مسجل مسبقاً، يرجى تسجيل الدخول أو استعادة كلمة المرور.")
}

// ErrFieldRequired — generic required-field error (URS 1.3 scenario 4).
func ErrFieldRequired(field string) *Error {
	return BadRequest("VALIDATION_ERROR",
		"This field is required.",
		"هذا الحقل مطلوب.").WithDetails([]FieldViolation{{Field: field, Message: "هذا الحقل مطلوب."}})
}

// ErrValidation collects multiple field violations.
func ErrValidation(violations []FieldViolation) *Error {
	return BadRequest("VALIDATION_ERROR", "One or more fields are invalid.", "بعض الحقول غير صالحة.").WithDetails(violations)
}

// ErrNotFoundResource — generic 404.
func ErrNotFoundResource(resource string) *Error {
	return NotFound("RESOURCE_NOT_FOUND",
		resource+" not found.",
		"العنصر غير موجود.")
}

// ErrInvalidOTP — wrong or expired code (shared message to avoid probing).
func ErrInvalidOTP() *Error {
	return BadRequest("OTP_INVALID",
		"The verification code is invalid, expired, or already used.",
		"رمز التحقق غير صالح أو منتهي الصلاحية أو مستخدَم مسبقاً.")
}

// ErrOTPLimit — too many verification attempts for the same code
// (SRS NFR-SEC-03: max 3 attempts).
func ErrOTPLimit() *Error {
	return TooManyRequests("OTP_ATTEMPTS_EXCEEDED",
		"The maximum number of verification attempts for this code has been reached. Request a new code.",
		"تم بلوغ الحد الأقصى لمحاولات التحقق لهذا الرمز. يرجى طلب رمز جديد.")
}

// ErrOTPResend — resend too soon (SRS FR-AUTH-01: 60s) or too many resends.
func ErrOTPResend(retrySeconds int) *Error {
	return TooManyRequests("OTP_RESEND_TOO_SOON",
		fmt.Sprintf("Please wait %d seconds before requesting a new code.", retrySeconds),
		fmt.Sprintf("يرجى الانتظار %d ثوانٍ قبل طلب رمز جديد.", retrySeconds))
}

// ErrUnauthorizedNoToken — missing/invalid bearer token.
func ErrUnauthorizedNoToken() *Error {
	return Unauthorized("UNAUTHORIZED",
		"A valid access token is required.",
		"مطلوب رمز وصول صالح.")
}

// ErrForbiddenRole — role not permitted.
func ErrForbiddenRole() *Error {
	return Forbidden("FORBIDDEN",
		"You do not have permission to perform this action.",
		"ليست لديك صلاحية لتنفيذ هذا الإجراء.")
}
