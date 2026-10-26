package service

import (
	"testing"

	userDto "github.com/najm-x/nova/modules/user/dto"
)

func sp(s string) *string { return &s }
func ip(i int8) *int8     { return &i }

func TestValidateConsumerProfile(t *testing.T) {
	tests := []struct {
		name    string
		in      userDto.CreateConsumerRequest
		wantErr bool
	}{
		{"no flags", userDto.CreateConsumerRequest{}, false},
		{"driver without vehicle", userDto.CreateConsumerRequest{IsDriver: true}, true},
		{"driver missing type", userDto.CreateConsumerRequest{IsDriver: true, VehicleNumber: sp("A1")}, true},
		{"driver blank number", userDto.CreateConsumerRequest{IsDriver: true, VehicleNumber: sp("  "), VehicleType: sp("car")}, true},
		{"driver ok", userDto.CreateConsumerRequest{IsDriver: true, VehicleNumber: sp("A1"), VehicleType: sp("car")}, false},
		{"home without count", userDto.CreateConsumerRequest{IsHome: true}, true},
		{"home zero count", userDto.CreateConsumerRequest{IsHome: true, FamilyCount: ip(0)}, true},
		{"home ok", userDto.CreateConsumerRequest{IsHome: true, FamilyCount: ip(4)}, false},
		{"business without anything", userDto.CreateConsumerRequest{IsBusiness: true}, true},
		{"business without file", userDto.CreateConsumerRequest{IsBusiness: true, BusinessName: sp("x")}, true},
		{"business without name", userDto.CreateConsumerRequest{IsBusiness: true, BusinessFile: sp("k")}, true},
		{"business ok", userDto.CreateConsumerRequest{IsBusiness: true, BusinessName: sp("x"), BusinessFile: sp("k")}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConsumerProfile(&tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v got %v", tt.wantErr, err)
			}
		})
	}
}
