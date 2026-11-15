// Package validation implements input validation and sanitization helpers
// (SRS NFR-SEC-05, NFR-USA-02). All checks return Arabic+English field
// violations suitable for the standard error envelope.
package validation

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"mawarid/backend/internal/apierr"
)

var (
	// digitsOnly matches pure digit strings.
	digitsOnly = regexp.MustCompile(`^[0-9]+$`)
	// validTime matches "HH:MM" 24-hour clock (SRS NFR-DAT-01).
	validTime = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
	// safeText rejects control characters used in injection/XSS payloads.
	safeText = regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F<>]`)
)

// NormalizePhone converts Yemeni phone formats to the canonical local form
// "7XXXXXXXX" (9 digits, leading 7). Accepts:
//
//	7XXXXXXXX | 07XXXXXXXX | +9677XXXXXXXX | 009677XXXXXXXX
//
// Returns "" when the input cannot be a valid Yemeni phone number.
func NormalizePhone(raw string) string {
	p := strings.TrimSpace(raw)
	p = strings.ReplaceAll(p, " ", "")
	p = strings.ReplaceAll(p, "-", "")
	if strings.HasPrefix(p, "+") {
		p = p[1:]
	}
	switch {
	case strings.HasPrefix(p, "00967"):
		p = p[len("00967"):]
	case strings.HasPrefix(p, "967"):
		p = p[len("967"):]
	}
	if strings.HasPrefix(p, "0") {
		p = p[1:]
	}
	if len(p) != 9 || !digitsOnly.MatchString(p) || p[0] != '7' {
		return ""
	}
	return p
}

// SanitizeText trims space and removes control characters and angle brackets
// from free-text fields (SRS NFR-SEC-05). NOT a substitute for output
// encoding; it reduces stored-payload risk for names and descriptions.
func SanitizeText(s string) string {
	s = strings.TrimSpace(s)
	s = safeText.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// Violations is a list accumulator.
type Violations []apierr.FieldViolation

// Add appends a violation.
func (v *Violations) Add(field, message, messageAr string) {
	*v = append(*v, apierr.FieldViolation{Field: field, Message: message + " / " + messageAr})
}

// Err returns the aggregated error, or nil when there are no violations.
func (v Violations) Err() *apierr.Error {
	if len(v) == 0 {
		return nil
	}
	return apierr.ErrValidation(v)
}

// Required validates a non-empty string field.
func (v *Violations) Required(field, value string) bool {
	if strings.TrimSpace(value) == "" {
		v.Add(field, "This field is required.", "هذا الحقل مطلوب.")
		return false
	}
	return true
}

// TextLength validates a UTF-8 length range (inclusive) for a sanitized field.
func (v *Violations) TextLength(field, value string, min, max int) bool {
	n := utf8.RuneCountInString(value)
	if n < min || n > max {
		v.Add(field,
			"Length must be between "+itoa(min)+" and "+itoa(max)+" characters.",
			"يجب أن يكون الطول بين "+itoaAr(min)+" و"+itoaAr(max)+" حرفاً.")
		return false
	}
	return true
}

// Phone validates and reports the normalized phone (empty when invalid).
func (v *Violations) Phone(field, raw string) string {
	if strings.TrimSpace(raw) == "" {
		v.Add(field, "This field is required.", "هذا الحقل مطلوب.")
		return ""
	}
	n := NormalizePhone(raw)
	if n == "" {
		v.Add(field, "Enter a valid Yemeni phone number.",
			"يرجى إدخال رقم هاتف يمني صحيح.")
	}
	return n
}

// Password enforces the password policy: at least 8 characters containing at
// least one letter and one digit. NOTE: the SRS does not spell out a policy;
// this minimal rule is a documented implementation decision — confirm with
// the team before tightening it.
func (v *Violations) Password(field, pw string) bool {
	if len(pw) < 8 {
		v.Add(field, "Password must be at least 8 characters.",
			"يجب أن تكون كلمة المرور 8 أحرف على الأقل.")
		return false
	}
	if len(pw) > 128 {
		v.Add(field, "Password must be at most 128 characters.",
			"يجب ألا تتجاوز كلمة المرور 128 حرفاً.")
		return false
	}
	var hasLetter, hasDigit bool
	for _, r := range pw {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= 0x0600 && r <= 0x06FF:
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		v.Add(field, "Password must contain letters and digits.",
			"يجب أن تحتوي كلمة المرور على حروف وأرقام.")
		return false
	}
	return true
}

// TimeHHMM validates a 24h "HH:MM" value (SRS NFR-DAT-01).
func (v *Violations) TimeHHMM(field, value string) bool {
	if !validTime.MatchString(value) {
		v.Add(field, "Time must be in HH:MM 24-hour format.",
			"يجب أن يكون الوقت بصيغة HH:MM بنظام 24 ساعة.")
		return false
	}
	return true
}

// DayOfWeek validates 0..6 (0=Sunday).
func (v *Violations) DayOfWeek(field string, day int) bool {
	if day < 0 || day > 6 {
		v.Add(field, "Day of week must be between 0 (Sunday) and 6 (Saturday).",
			"يجب أن يكون اليوم بين 0 (الأحد) و6 (السبت).")
		return false
	}
	return true
}

// OneOf validates membership in an allowed set.
func (v *Violations) OneOf(field, value string, allowed []string) bool {
	for _, a := range allowed {
		if value == a {
			return true
		}
	}
	v.Add(field, "Unsupported value.", "قيمة غير مدعومة.")
	return false
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// itoaAr renders small ASCII digits (kept ASCII for JSON safety).
func itoaAr(n int) string { return itoa(n) }
