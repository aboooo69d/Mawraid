package validator_test

import (
	"regexp"
	"testing"
)

var yemenPhoneRegex = regexp.MustCompile(`^7[01378]\d{7}$`)

// TestYemenPhoneValidation checks valid and invalid Yemeni phone numbers.
func TestYemenPhoneValidation(t *testing.T) {
	testCases := []struct {
		name     string
		phone    string
		expected bool
	}{
		// Valid numbers (Sabafon: 71, You: 73, Yemen Mobile: 77, Y: 70, Way: 78)
		{"Valid Yemen Mobile (77)", "771234567", true},
		{"Valid You/MTN (73)", "731234567", true},
		{"Valid Sabafon (71)", "711234567", true},
		{"Valid Y Telecom (70)", "701234567", true},
		{"Valid Way (78)", "781234567", true},

		// Invalid numbers
		{"Invalid Prefix (79)", "791234567", false},
		{"Invalid Prefix (72)", "721234567", false},
		{"Invalid Prefix (74)", "741234567", false},
		{"Too Short (8 digits)", "77123456", false},
		{"Too Long (10 digits)", "7712345678", false},
		{"Contains Letters", "77123456a", false},
		{"Contains Special Characters", "77123-567", false},
		{"Empty String", "", false},
		{"Leading Zero", "0771234567", false},
		{"Country Code Prefix", "+967771234567", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := yemenPhoneRegex.MatchString(tc.phone)
			if result != tc.expected {
				t.Errorf("phone %q: expected match=%v, got=%v", tc.phone, tc.expected, result)
			}
		})
	}
}

