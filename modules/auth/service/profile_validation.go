package service

import (
	"strings"

	"github.com/najm-x/nova/modules/auth/dto"
	userDto "github.com/najm-x/nova/modules/user/dto"
)

// validateConsumerProfile enforces conditional rules: when a flag is true,
// its related fields must be present and valid.
//
//	is_driver   -> vehicle_number, vehicle_type
//	is_home     -> family_count (>= 1)
//	is_business -> business_name, business_file
func validateConsumerProfile(c *userDto.CreateConsumerRequest) error {
	if c.IsDriver {
		if isBlank(c.VehicleNumber) {
			return dto.NewInvalidProfileField("vehicle_number is required when is_driver is true")
		}
		if isBlank(c.VehicleType) {
			return dto.NewInvalidProfileField("vehicle_type is required when is_driver is true")
		}
	}

	if c.IsHome {
		if c.FamilyCount == nil || *c.FamilyCount < 1 {
			return dto.NewInvalidProfileField("family_count must be at least 1 when is_home is true")
		}
	}

	if c.IsBusiness {
		if isBlank(c.BusinessName) {
			return dto.NewInvalidProfileField("business_name is required when is_business is true")
		}
		if isBlank(c.BusinessFile) {
			return dto.NewInvalidProfileField("business_file is required when is_business is true")
		}
	}

	return nil
}

func isBlank(s *string) bool {
	return s == nil || strings.TrimSpace(*s) == ""
}
