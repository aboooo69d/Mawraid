package validator

import (
	"regexp"

	"github.com/go-playground/validator/v10"
)

var (
	yemenPhoneRegex = regexp.MustCompile(`^7[01378]\d{7}$`)
)

// validateYemenPhone
func validateYemenPhone(fl validator.FieldLevel) bool {
	phone, ok := fl.Field().Interface().(string)
	if !ok {
		return false
	}
	return yemenPhoneRegex.MatchString(phone)
}
